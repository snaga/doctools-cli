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
	Foreground     bool
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
	fs.BoolVar(&opts.Foreground, "foreground", false, "Run in foreground and log to console")
	fs.BoolVar(&opts.Foreground, "f", false, "Run in foreground and log to console (shorthand)")

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
	Options     Options
	Stdout      io.Writer
	Stderr      io.Writer
	OpenURLFunc func(urlStr string) error

	mu           sync.Mutex
	logMu        sync.Mutex
	cfg          *docsearch.Config
	server       *docsearch.Server
	hook         *docsearch.KeyboardHook
	tray         *docsearch.TrayIcon
	activeWindow *docsearch.SearchWindow // Deprecated: preserved for backward compatibility
	readyCh      chan struct{}
	cancel       context.CancelFunc
}

func (app *App) logf(format string, a ...any) {
	app.logMu.Lock()
	defer app.logMu.Unlock()
	fmt.Fprintf(app.Stderr, format, a...)
}

func (app *App) logln(a ...any) {
	app.logMu.Lock()
	defer app.logMu.Unlock()
	fmt.Fprintln(app.Stderr, a...)
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

// Tray returns the active TrayIcon instance (if any).
func (app *App) Tray() *docsearch.TrayIcon {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.tray
}

// Hook returns the active KeyboardHook instance (if any).
func (app *App) Hook() *docsearch.KeyboardHook {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.hook
}

// Config returns the loaded configuration.
func (app *App) Config() *docsearch.Config {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.cfg
}

// OpenSearch opens the DocSearch search page in the default web browser.
func (app *App) OpenSearch() error {
	app.mu.Lock()
	srv := app.server
	cfg := app.cfg
	openFn := app.OpenURLFunc
	app.mu.Unlock()

	hasClients := srv != nil && srv.HasActiveWebClients()
	activated := docsearch.ActivateDocSearchWindow()

	if hasClients || activated {
		if srv != nil {
			srv.NotifyWebClients("focus")
		}
		docsearch.ActivateDocSearchWindow()
		return nil
	}

	if openFn == nil {
		openFn = defaultOpenURL
	}

	var targetURL string
	if srv != nil {
		targetURL = srv.LaunchURL("", nil)
	} else if cfg != nil {
		targetURL = fmt.Sprintf("http://%s:%d/launch", cfg.Server.Host, cfg.Server.Port)
	} else {
		targetURL = "http://127.0.0.1:18080/launch"
	}

	app.logf("Opening DocSearch in browser: %s\n", targetURL)
	return openFn(targetURL)
}

// OpenSettings opens the DocSearch settings page in the default web browser.
func (app *App) OpenSettings() error {
	app.mu.Lock()
	srv := app.server
	cfg := app.cfg
	openFn := app.OpenURLFunc
	app.mu.Unlock()

	hasClients := srv != nil && srv.HasActiveWebClients()
	activated := docsearch.ActivateDocSearchWindow()

	if hasClients || activated {
		if srv != nil {
			srv.NotifyWebClients("settings")
		}
		docsearch.ActivateDocSearchWindow()
		return nil
	}

	if openFn == nil {
		openFn = defaultOpenURL
	}

	var baseURL string
	if srv != nil {
		baseURL = srv.LaunchURL("", nil)
	} else if cfg != nil {
		baseURL = fmt.Sprintf("http://%s:%d/launch", cfg.Server.Host, cfg.Server.Port)
	} else {
		baseURL = "http://127.0.0.1:18080/launch"
	}

	targetURL := baseURL
	if strings.Contains(targetURL, "?") {
		targetURL += "&settings=1"
	} else {
		targetURL += "?settings=1"
	}

	app.logf("Opening DocSearch settings in browser: %s\n", targetURL)
	return openFn(targetURL)
}

// Run executes the docsearch-gui lifecycle until context cancellation or error.
func (app *App) Run(ctx context.Context) error {
	if app.Options.ShowVersion {
		fmt.Fprintf(app.Stdout, "docsearch-gui version %s\n", version.Version)
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	app.mu.Lock()
	app.cancel = cancel
	app.mu.Unlock()

	defer func() {
		app.logln("Shutting down docsearch-gui...")
		app.Shutdown()
		app.logln("docsearch-gui stopped.")
	}()

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
	srv.SetConfigFile(cfgPath)

	bindAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := srv.Start(bindAddr); err != nil {
		return fmt.Errorf("failed to start server on %s: %w", bindAddr, err)
	}

	app.mu.Lock()
	app.server = srv
	app.mu.Unlock()

	app.logf("docsearch-gui server listening on %s (configured %s)\n", srv.Addr(), bindAddr)

	// Initialize keyboard hook
	if cfg.Hotkey.Enabled {
		hook := docsearch.NewKeyboardHook(cfg.Hotkey.IntervalMS, func() {
			if err := app.OpenSearch(); err != nil {
				app.logf("Failed to open search from hotkey: %v\n", err)
			}
		})

		app.mu.Lock()
		app.hook = hook
		app.mu.Unlock()

		if err := hook.Start(); err != nil {
			app.logf("Warning: failed to start keyboard hook: %v\n", err)
		} else {
			app.logf("Keyboard hook registered (Double-tap Ctrl within %d ms)\n", cfg.Hotkey.IntervalMS)
		}
	}

	// Initialize system tray icon
	tray := docsearch.NewTrayIcon(docsearch.TrayCallbacks{
		OnOpen: func() {
			if err := app.OpenSearch(); err != nil {
				app.logf("Failed to open search from tray: %v\n", err)
			}
		},
		OnSettings: func() {
			if err := app.OpenSettings(); err != nil {
				app.logf("Failed to open settings from tray: %v\n", err)
			}
		},
		OnExit: func() {
			app.Stop()
		},
	})

	app.mu.Lock()
	app.tray = tray
	app.mu.Unlock()

	if err := tray.Start(); err != nil {
		app.logf("Warning: failed to start tray icon: %v\n", err)
	}

	// Signal readiness
	close(app.readyCh)
	app.logln("docsearch-gui started successfully and is ready.")

	// Wait for context cancellation, OS signal, or tray OnExit
	<-ctx.Done()
	app.logf("docsearch-gui received stop signal/context done: %v\n", ctx.Err())

	return nil
}

// Stop initiates graceful shutdown of the application.
func (app *App) Stop() {
	app.mu.Lock()
	cancel := app.cancel
	app.mu.Unlock()

	if cancel != nil {
		cancel()
	}
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

// Shutdown stops the tray icon, keyboard hook, active window, and HTTP server.
func (app *App) Shutdown() {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.tray != nil {
		_ = app.tray.Stop()
		app.tray = nil
	}
	if app.hook != nil {
		app.hook.Stop()
		app.hook = nil
	}
	if app.activeWindow != nil {
		_ = app.activeWindow.Close()
		app.activeWindow = nil
	}
	if app.server != nil {
		_ = app.server.Stop()
		app.server = nil
	}
}

func buildChildArgs(args []string) []string {
	var filtered []string
	for _, arg := range args {
		if arg == "--foreground" || arg == "-f" || strings.HasPrefix(arg, "--foreground=") || strings.HasPrefix(arg, "-f=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return append(filtered, "--foreground")
}

func spawnBackgroundProcess(args []string, stdout, stderr io.Writer) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	logPath := "docsearch-gui.log"
	if cwd != "" {
		logPath = filepath.Join(cwd, "docsearch-gui.log")
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	childArgs := buildChildArgs(args)
	buildCmd := func() *exec.Cmd {
		c := execCommand(exePath, childArgs...)
		if cwd != "" {
			c.Dir = cwd
		}
		c.Stdout = logFile
		c.Stderr = logFile
		configureDetachedProcess(c)
		return c
	}

	cmd := buildCmd()
	if err := cmd.Start(); err != nil {
		// If Start fails (e.g. parent Job Object disallows breakaway with ERROR_ACCESS_DENIED),
		// retry as a fallback without the breakaway flag (0x01000000).
		fallbackCmd := buildCmd()
		if removeBreakawayFlag(fallbackCmd) {
			if retryErr := fallbackCmd.Start(); retryErr == nil {
				fmt.Fprintln(stdout, "DocSearch started in background (tray resident).")
				return nil
			}
		}
		return fmt.Errorf("failed to start background process: %w", err)
	}

	fmt.Fprintln(stdout, "DocSearch started in background (tray resident).")
	return nil
}

var spawnBackgroundProcessFn = spawnBackgroundProcess
var runAppFn = func(app *App, ctx context.Context) error {
	return app.Run(ctx)
}

func realMain(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if !opts.Foreground && !opts.ShowVersion {
		if err := spawnBackgroundProcessFn(args, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "Error spawning background process: %v\n", err)
			return 1
		}
		return 0
	}

	app := NewApp(opts, stdout, stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runAppFn(app, ctx); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}

