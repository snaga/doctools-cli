package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"doctools-cli/internal/version"
	"doctools-cli/pkg/docsearch"
)

// Options holds command-line configuration for docsearch-gui.
type Options struct {
	ConfigPath     string
	Port           int
	HotkeyInterval int
	ShowVersion    bool
}

// parseFlags parses command-line arguments into Options.
func parseFlags(args []string, output io.Writer) (Options, error) {
	fs := flag.NewFlagSet("docsearch-gui", flag.ContinueOnError)
	fs.SetOutput(output)

	var opts Options
	fs.StringVar(&opts.ConfigPath, "config", "", "Path to docsearch.json configuration file")
	fs.IntVar(&opts.Port, "port", 0, "Server port override")
	fs.IntVar(&opts.HotkeyInterval, "hotkey-interval", 0, "Hotkey double-tap interval in milliseconds")
	fs.BoolVar(&opts.ShowVersion, "version", false, "Print version information")
	fs.BoolVar(&opts.ShowVersion, "v", false, "Print version information (shorthand)")

	err := fs.Parse(args)
	return opts, err
}

// resolveConfigPath determines the config file path according to specifications:
// 1. Explicit flag value if non-empty
// 2. docsearch.json in current working directory
// 3. docsearch.json in executable directory
// 4. Default fallback: "docsearch.json"
func resolveConfigPath(flagConfig string) string {
	if flagConfig != "" {
		return flagConfig
	}

	// 1. Current working directory
	cwdConfig := "docsearch.json"
	if _, err := os.Stat(cwdConfig); err == nil {
		return cwdConfig
	}

	// 2. Executable directory
	if exe, err := os.Executable(); err == nil {
		exeConfig := filepath.Join(filepath.Dir(exe), "docsearch.json")
		if _, err := os.Stat(exeConfig); err == nil {
			return exeConfig
		}
	}

	return cwdConfig
}

// buildSearchURL constructs the WebUI search URL with properly encoded query parameters.
func buildSearchURL(serverAddr string, fallbackHost string, fallbackPort int, query string, selectedIndexes []string) string {
	host := fallbackHost
	port := fallbackPort

	if serverAddr != "" {
		h, p, err := net.SplitHostPort(serverAddr)
		if err == nil {
			if h != "" && h != "0.0.0.0" && h != "::" && h != "[::]" {
				host = h
			}
			if parsedPort, err := strconv.Atoi(p); err == nil && parsedPort > 0 {
				port = parsedPort
			}
		}
	}

	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}

	baseURL := fmt.Sprintf("http://%s:%d/", host, port)
	params := url.Values{}

	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery != "" {
		params.Set("q", trimmedQuery)
	}

	if len(selectedIndexes) > 0 {
		params.Set("indexes", strings.Join(selectedIndexes, ","))
	}

	qs := params.Encode()
	if qs != "" {
		return baseURL + "?" + qs
	}
	return baseURL
}

var execCommand = exec.Command

// defaultOpenURL launches the default web browser to open the specified URL.
func defaultOpenURL(urlStr string) error {
	if runtime.GOOS == "windows" {
		cmd := execCommand("rundll32.exe", "url.dll,FileProtocolHandler", urlStr)
		return cmd.Start()
	} else if runtime.GOOS == "darwin" {
		cmd := execCommand("open", urlStr)
		return cmd.Start()
	}
	cmd := execCommand("xdg-open", urlStr)
	return cmd.Start()
}

// App encapsulates the lifecycle and services of docsearch-gui.
type App struct {
	Options      Options
	Stdout       io.Writer
	Stderr       io.Writer
	OpenURLFunc  func(urlStr string) error

	mu           sync.Mutex
	cfg          *docsearch.Config
	server       *docsearch.Server
	hook         *docsearch.KeyboardHook
	activeWindow *docsearch.SearchWindow
	readyCh      chan struct{}
}

// NewApp creates a new App instance.
func NewApp(opts Options, stdout, stderr io.Writer) *App {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &App{
		Options: opts,
		Stdout:  stdout,
		Stderr:  stderr,
		readyCh: make(chan struct{}),
	}
}

// Ready returns a channel that is closed when the server and hook have started.
func (app *App) Ready() <-chan struct{} {
	return app.readyCh
}

// Server returns the active HTTP server instance (if any).
func (app *App) Server() *docsearch.Server {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.server
}

// ActiveWindow returns the currently active SearchWindow (if any).
func (app *App) ActiveWindow() *docsearch.SearchWindow {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.activeWindow
}

// Config returns the loaded configuration.
func (app *App) Config() *docsearch.Config {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.cfg
}

// Run executes the docsearch-gui lifecycle until context cancellation or error.
func (app *App) Run(ctx context.Context) error {
	if app.Options.ShowVersion {
		fmt.Fprintf(app.Stdout, "docsearch-gui version %s\n", version.Version)
		return nil
	}

	cfgPath := resolveConfigPath(app.Options.ConfigPath)
	cfg, err := docsearch.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if app.Options.Port > 0 {
		cfg.Server.Port = app.Options.Port
	}
	if app.Options.HotkeyInterval > 0 {
		cfg.Hotkey.IntervalMS = app.Options.HotkeyInterval
	}

	app.mu.Lock()
	app.cfg = cfg
	app.mu.Unlock()

	// Initialize HTTP server
	srv, err := docsearch.NewServer(cfg, "")
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	bindAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := srv.Start(bindAddr); err != nil {
		return fmt.Errorf("failed to start server on %s: %w", bindAddr, err)
	}

	app.mu.Lock()
	app.server = srv
	app.mu.Unlock()

	fmt.Fprintf(app.Stderr, "docsearch-gui server listening on %s (configured %s)\n", srv.Addr(), bindAddr)

	// Callback when search is submitted from the floating window
	onSearch := func(query string, selectedIndexes []string) {
		targetURL := buildSearchURL(srv.Addr(), cfg.Server.Host, cfg.Server.Port, query, selectedIndexes)
		fmt.Fprintf(app.Stderr, "Opening search results in browser: %s\n", targetURL)

		openFn := app.OpenURLFunc
		if openFn == nil {
			openFn = defaultOpenURL
		}
		if err := openFn(targetURL); err != nil {
			fmt.Fprintf(app.Stderr, "Failed to launch browser: %v\n", err)
		}
	}

	// Initialize keyboard hook
	if cfg.Hotkey.Enabled {
		hook := docsearch.NewKeyboardHook(cfg.Hotkey.IntervalMS, func() {
			app.mu.Lock()
			defer app.mu.Unlock()

			win, err := docsearch.ShowSearchWindow(cfg.Indexes, onSearch)
			if err != nil {
				fmt.Fprintf(app.Stderr, "Failed to display search window: %v\n", err)
				return
			}
			app.activeWindow = win
		})

		app.mu.Lock()
		app.hook = hook
		app.mu.Unlock()

		if err := hook.Start(); err != nil {
			fmt.Fprintf(app.Stderr, "Warning: failed to start keyboard hook: %v\n", err)
		} else {
			fmt.Fprintf(app.Stderr, "Keyboard hook registered (Double-tap Ctrl within %d ms)\n", cfg.Hotkey.IntervalMS)
		}
	}

	// Signal readiness
	close(app.readyCh)

	// Wait for context cancellation or OS signal
	<-ctx.Done()

	fmt.Fprintln(app.Stderr, "Shutting down docsearch-gui...")
	app.Shutdown()
	return nil
}

// TriggerSearchWindow manually triggers the search floating window (useful for programmatic invocation / testing).
func (app *App) TriggerSearchWindow(onSearch docsearch.SearchCallback) (*docsearch.SearchWindow, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.cfg == nil {
		return nil, fmt.Errorf("app is not running or configuration is missing")
	}

	win, err := docsearch.ShowSearchWindow(app.cfg.Indexes, onSearch)
	if err != nil {
		return nil, err
	}
	app.activeWindow = win
	return win, nil
}

// Shutdown stops the keyboard hook, closes the active search window, and closes the server.
func (app *App) Shutdown() {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.hook != nil {
		app.hook.Stop()
		app.hook = nil
	}
	if app.activeWindow != nil {
		_ = app.activeWindow.Close()
		app.activeWindow = nil
	}
	if app.server != nil {
		_ = app.server.Close()
		app.server = nil
	}
}

func main() {
	opts, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	app := NewApp(opts, os.Stdout, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
