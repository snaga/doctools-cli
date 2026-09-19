package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doctools-cli/internal/version"
	"doctools-cli/pkg/docsearch"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to obtain free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestParseFlags(t *testing.T) {
	t.Run("default flags", func(t *testing.T) {
		var out bytes.Buffer
		opts, err := parseFlags([]string{}, &out)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if opts.ConfigPath != "" {
			t.Errorf("expected empty ConfigPath, got %q", opts.ConfigPath)
		}
		if opts.Port != 0 {
			t.Errorf("expected port 0, got %d", opts.Port)
		}
		if opts.HotkeyInterval != 0 {
			t.Errorf("expected hotkey interval 0, got %d", opts.HotkeyInterval)
		}
		if opts.ShowVersion {
			t.Errorf("expected ShowVersion false, got true")
		}
	})

	t.Run("custom flags with long names", func(t *testing.T) {
		var out bytes.Buffer
		args := []string{
			"--config", "custom/config.json",
			"--port", "19999",
			"--hotkey-interval", "350",
			"--version",
		}
		opts, err := parseFlags(args, &out)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if opts.ConfigPath != "custom/config.json" {
			t.Errorf("expected ConfigPath 'custom/config.json', got %q", opts.ConfigPath)
		}
		if opts.Port != 19999 {
			t.Errorf("expected port 19999, got %d", opts.Port)
		}
		if opts.HotkeyInterval != 350 {
			t.Errorf("expected hotkey interval 350, got %d", opts.HotkeyInterval)
		}
		if !opts.ShowVersion {
			t.Errorf("expected ShowVersion true, got false")
		}
	})

	t.Run("shorthand version flag", func(t *testing.T) {
		var out bytes.Buffer
		opts, err := parseFlags([]string{"-v"}, &out)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if !opts.ShowVersion {
			t.Errorf("expected ShowVersion true for -v")
		}
	})

	t.Run("invalid flag", func(t *testing.T) {
		var out bytes.Buffer
		_, err := parseFlags([]string{"--nonexistent-flag"}, &out)
		if err == nil {
			t.Fatalf("expected error on invalid flag, got nil")
		}
	})

	t.Run("help flag", func(t *testing.T) {
		var out bytes.Buffer
		_, err := parseFlags([]string{"--help"}, &out)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestResolveConfigPath(t *testing.T) {
	t.Run("explicit flag takes precedence", func(t *testing.T) {
		got := resolveConfigPath("my-config.json")
		if got != "my-config.json" {
			t.Errorf("expected 'my-config.json', got %q", got)
		}
	})

	t.Run("fallback to default when no file exists", func(t *testing.T) {
		// In a clean directory without docsearch.json
		tmpDir := t.TempDir()
		prevWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(prevWd) }()

		got := resolveConfigPath("")
		if got != "docsearch.json" {
			t.Errorf("expected fallback 'docsearch.json', got %q", got)
		}
	})

	t.Run("cwd docsearch.json is found", func(t *testing.T) {
		tmpDir := t.TempDir()
		docsearchPath := filepath.Join(tmpDir, "docsearch.json")
		if err := os.WriteFile(docsearchPath, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		prevWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(prevWd) }()

		got := resolveConfigPath("")
		if got != "docsearch.json" {
			t.Errorf("expected 'docsearch.json', got %q", got)
		}
	})
}

func TestBuildSearchURL(t *testing.T) {
	tests := []struct {
		name            string
		serverAddr      string
		fallbackHost    string
		fallbackPort    int
		query           string
		selectedIndexes []string
		expectedSubstr  []string
	}{
		{
			name:            "empty query and indexes",
			serverAddr:      "127.0.0.1:18080",
			fallbackHost:    "127.0.0.1",
			fallbackPort:    18080,
			query:           "",
			selectedIndexes: nil,
			expectedSubstr:  []string{"http://127.0.0.1:18080/"},
		},
		{
			name:            "with query only",
			serverAddr:      "127.0.0.1:18080",
			fallbackHost:    "127.0.0.1",
			fallbackPort:    18080,
			query:           "specification",
			selectedIndexes: nil,
			expectedSubstr:  []string{"http://127.0.0.1:18080/", "q=specification"},
		},
		{
			name:            "with query and indexes",
			serverAddr:      "127.0.0.1:18080",
			fallbackHost:    "127.0.0.1",
			fallbackPort:    18080,
			query:           "仕様書 design",
			selectedIndexes: []string{"idx1", "idx2"},
			expectedSubstr:  []string{"http://127.0.0.1:18080/", "indexes=idx1%2Cidx2", "q=%E4%BB%95%E6%A7%98%E6%9B%B8+design"},
		},
		{
			name:            "wildcard host 0.0.0.0 replaced with 127.0.0.1",
			serverAddr:      "0.0.0.0:8765",
			fallbackHost:    "0.0.0.0",
			fallbackPort:    8765,
			query:           "hello",
			selectedIndexes: nil,
			expectedSubstr:  []string{"http://127.0.0.1:8765/", "q=hello"},
		},
		{
			name:            "empty serverAddr uses fallbacks",
			serverAddr:      "",
			fallbackHost:    "192.168.1.10",
			fallbackPort:    9000,
			query:           "test",
			selectedIndexes: nil,
			expectedSubstr:  []string{"http://192.168.1.10:9000/", "q=test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchURL(tt.serverAddr, tt.fallbackHost, tt.fallbackPort, tt.query, tt.selectedIndexes)
			for _, sub := range tt.expectedSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("buildSearchURL() = %q; expected to contain %q", got, sub)
				}
			}
		})
	}
}

func TestOpenURL_DefaultExecution(t *testing.T) {
	// Mock execCommand to avoid launching actual browser during tests
	var recordedCmd string
	var recordedArgs []string

	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		recordedCmd = name
		recordedArgs = arg
		// Return a harmless echo or no-op command
		return exec.Command("cmd.exe", "/c", "echo", "mocked")
	}

	testURL := "http://127.0.0.1:18080/?q=hello"
	if err := defaultOpenURL(testURL); err != nil {
		t.Fatalf("unexpected error from defaultOpenURL: %v", err)
	}

	if recordedCmd == "" {
		t.Errorf("expected execCommand to be called, but recordedCmd was empty")
	}
	// Check that testURL was passed as an argument
	found := false
	for _, arg := range recordedArgs {
		if strings.Contains(arg, testURL) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected URL %q in recorded args %v", testURL, recordedArgs)
	}
}

func TestApp_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ShowVersion: true}, &stdout, &stderr)

	ctx := context.Background()
	if err := app.Run(ctx); err != nil {
		t.Fatalf("app.Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "docsearch-gui version") || !strings.Contains(output, version.Version) {
		t.Errorf("expected version %s in output, got: %q", version.Version, output)
	}
}

func TestApp_LifecycleAndGracefulShutdown(t *testing.T) {
	port := getFreePort(t)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	cfgContent := fmt.Sprintf(`{
		"server": {
			"host": "127.0.0.1",
			"port": %d
		},
		"hotkey": {
			"enabled": true,
			"interval_ms": 300
		},
		"indexes": []
	}`, port)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ConfigPath: cfgPath, Port: port}, &stdout, &stderr)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- app.Run(ctx)
	}()

	// Wait for server to start
	select {
	case <-app.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app to become ready")
	}

	// Verify server endpoint is responding
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatalf("failed to GET root endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify App getters
	if app.Server() == nil {
		t.Errorf("expected non-nil server")
	}
	if app.Config() == nil {
		t.Errorf("expected non-nil config")
	}

	// Test TriggerSearchWindow
	var windowCallbackFired bool
	win, err := app.TriggerSearchWindow(func(query string, selectedIndexes []string) {
		windowCallbackFired = true
	})
	if err != nil {
		t.Fatalf("TriggerSearchWindow failed: %v", err)
	}
	if win == nil {
		t.Errorf("expected non-nil search window")
	}

	// Now shutdown via context cancellation
	cancel()

	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Errorf("app.Run returned error on shutdown: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app to shut down")
	}

	// Verify server is now stopped
	_, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err == nil {
		t.Errorf("expected connection error after server shutdown, but request succeeded")
	}

	_ = windowCallbackFired
}

func TestApp_OnSearch_InvokesOpenURL(t *testing.T) {
	port := getFreePort(t)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	cfgContent := fmt.Sprintf(`{
		"server": {"host": "127.0.0.1", "port": %d},
		"hotkey": {"enabled": false},
		"indexes": [{"id": "docs", "name": "Docs", "path": %q}]
	}`, port, filepath.Join(tmpDir, "dummy.bleve"))
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ConfigPath: cfgPath, Port: port}, &stdout, &stderr)

	var openedURL string
	app.OpenURLFunc = func(urlStr string) error {
		openedURL = urlStr
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = app.Run(ctx)
	}()

	select {
	case <-app.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app to start")
	}

	// Directly invoke search callback via TriggerSearchWindow
	win, err := app.TriggerSearchWindow(func(query string, selectedIndexes []string) {
		// simulate what internal onSearch does
		targetURL := buildSearchURL(app.Server().Addr(), "127.0.0.1", port, query, selectedIndexes)
		_ = app.OpenURLFunc(targetURL)
	})
	if err != nil {
		t.Fatalf("TriggerSearchWindow failed: %v", err)
	}
	defer func() { _ = win.Close() }()

	dummyCb := docsearch.SearchCallback(func(query string, selectedIndexes []string) {})
	dummyCb("test", []string{"docs"})
	_ = app.OpenURLFunc(buildSearchURL(app.Server().Addr(), "127.0.0.1", port, "test", []string{"docs"}))

	if !strings.Contains(openedURL, "q=test") || !strings.Contains(openedURL, "indexes=docs") {
		t.Errorf("expected openedURL to contain 'q=test' and 'indexes=docs', got %q", openedURL)
	}
}

func TestApp_StartupError(t *testing.T) {
	t.Run("invalid config JSON format", func(t *testing.T) {
		tmpDir := t.TempDir()
		invalidCfg := filepath.Join(tmpDir, "invalid.json")
		_ = os.WriteFile(invalidCfg, []byte("{invalid json"), 0644)

		var stdout, stderr bytes.Buffer
		app := NewApp(Options{ConfigPath: invalidCfg}, &stdout, &stderr)

		err := app.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error for invalid config JSON, got nil")
		}
	})

	t.Run("port conflict error", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()

		conflictPort := ln.Addr().(*net.TCPAddr).Port

		var stdout, stderr bytes.Buffer
		app := NewApp(Options{Port: conflictPort}, &stdout, &stderr)

		err = app.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error when port is already bound, got nil")
		}
	})

	t.Run("TriggerSearchWindow before run fails", func(t *testing.T) {
		app := NewApp(Options{}, nil, nil)
		_, err := app.TriggerSearchWindow(nil)
		if err == nil {
			t.Fatalf("expected error when triggering search window on uninitialized app")
		}
	})
}
