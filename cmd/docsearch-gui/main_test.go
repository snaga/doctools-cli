package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		if opts.Foreground {
			t.Errorf("expected Foreground false by default, got true")
		}
	})

	t.Run("custom flags with long names", func(t *testing.T) {
		var out bytes.Buffer
		args := []string{
			"--config", "custom/config.json",
			"--port", "19999",
			"--hotkey-interval", "350",
			"--version",
			"--foreground",
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
		if !opts.Foreground {
			t.Errorf("expected Foreground true, got false")
		}
	})

	t.Run("shorthand version and foreground flags", func(t *testing.T) {
		var out bytes.Buffer
		opts, err := parseFlags([]string{"-v", "-f"}, &out)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if !opts.ShowVersion {
			t.Errorf("expected ShowVersion true for -v")
		}
		if !opts.Foreground {
			t.Errorf("expected Foreground true for -f")
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

	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "docsearch-gui started successfully and is ready.") {
		t.Errorf("expected 'started successfully' message in stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "docsearch-gui received stop signal/context done:") {
		t.Errorf("expected 'received stop signal' message in stderr, got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "docsearch-gui stopped.") {
		t.Errorf("expected 'docsearch-gui stopped.' message in stderr, got: %q", stderrOutput)
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

func TestApp_OpenSearchAndOpenSettings(t *testing.T) {
	port := getFreePort(t)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	cfgContent := fmt.Sprintf(`{
		"server": {"host": "127.0.0.1", "port": %d},
		"hotkey": {"enabled": false},
		"indexes": []
	}`, port)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ConfigPath: cfgPath, Port: port}, &stdout, &stderr)

	var lastURL string
	app.OpenURLFunc = func(urlStr string) error {
		lastURL = urlStr
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

	// Test OpenSearch
	expectedSearchURL := fmt.Sprintf("http://127.0.0.1:%d/launch", port)
	if err := app.OpenSearch(); err != nil {
		t.Fatalf("OpenSearch returned error: %v", err)
	}
	if lastURL != expectedSearchURL {
		t.Errorf("OpenSearch URL = %q; expected %q", lastURL, expectedSearchURL)
	}

	// Test OpenSettings
	expectedSettingsURL := fmt.Sprintf("http://127.0.0.1:%d/launch?settings=1", port)
	if err := app.OpenSettings(); err != nil {
		t.Fatalf("OpenSettings returned error: %v", err)
	}
	if lastURL != expectedSettingsURL {
		t.Errorf("OpenSettings URL = %q; expected %q", lastURL, expectedSettingsURL)
	}
}

func TestApp_HotkeyTrigger_InvokesOpenSearch(t *testing.T) {
	port := getFreePort(t)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	cfgContent := fmt.Sprintf(`{
		"server": {"host": "127.0.0.1", "port": %d},
		"hotkey": {"enabled": true, "interval_ms": 400},
		"indexes": []
	}`, port)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ConfigPath: cfgPath, Port: port}, &stdout, &stderr)

	urlCh := make(chan string, 1)
	app.OpenURLFunc = func(urlStr string) error {
		select {
		case urlCh <- urlStr:
		default:
		}
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

	hook := app.Hook()
	if hook == nil {
		t.Fatal("expected keyboard hook to be non-nil")
	}
	detector := hook.Detector()
	if detector == nil {
		t.Fatal("expected detector to be non-nil")
	}

	// Simulate double-tap Ctrl within interval
	now := time.Now()
	detector.ProcessKeyEvent(docsearch.VK_CONTROL, true, now)
	detector.ProcessKeyEvent(docsearch.VK_CONTROL, false, now.Add(50*time.Millisecond))
	detector.ProcessKeyEvent(docsearch.VK_CONTROL, true, now.Add(100*time.Millisecond))

	expectedURL := fmt.Sprintf("http://127.0.0.1:%d/launch", port)
	select {
	case openedURL := <-urlCh:
		if openedURL != expectedURL {
			t.Errorf("Hotkey triggered URL = %q; expected %q", openedURL, expectedURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hotkey trigger to invoke OpenSearch")
	}
}

func TestApp_TrayCallbacks(t *testing.T) {
	port := getFreePort(t)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	cfgContent := fmt.Sprintf(`{
		"server": {"host": "127.0.0.1", "port": %d},
		"hotkey": {"enabled": false},
		"indexes": []
	}`, port)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	var stdout, stderr bytes.Buffer
	app := NewApp(Options{ConfigPath: cfgPath, Port: port}, &stdout, &stderr)

	urlCh := make(chan string, 2)
	app.OpenURLFunc = func(urlStr string) error {
		urlCh <- urlStr
		return nil
	}

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- app.Run(context.Background())
	}()

	select {
	case <-app.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app to start")
	}

	tray := app.Tray()
	if tray == nil {
		t.Fatal("expected tray icon to be non-nil")
	}

	cbs := tray.Callbacks()
	if cbs.OnOpen == nil {
		t.Fatal("expected OnOpen callback to be configured")
	}
	if cbs.OnSettings == nil {
		t.Fatal("expected OnSettings callback to be configured")
	}
	if cbs.OnExit == nil {
		t.Fatal("expected OnExit callback to be configured")
	}

	// Test OnOpen
	cbs.OnOpen()
	expectedOpenURL := fmt.Sprintf("http://127.0.0.1:%d/launch", port)
	select {
	case openedURL := <-urlCh:
		if openedURL != expectedOpenURL {
			t.Errorf("Tray OnOpen URL = %q; expected %q", openedURL, expectedOpenURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnOpen to invoke OpenURLFunc")
	}

	// Test OnSettings
	cbs.OnSettings()
	expectedSettingsURL := fmt.Sprintf("http://127.0.0.1:%d/launch?settings=1", port)
	select {
	case openedURL := <-urlCh:
		if openedURL != expectedSettingsURL {
			t.Errorf("Tray OnSettings URL = %q; expected %q", openedURL, expectedSettingsURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnSettings to invoke OpenURLFunc")
	}

	// Test OnExit triggers Graceful Shutdown of app.Run
	cbs.OnExit()

	select {
	case runErr := <-runErrCh:
		if runErr != nil {
			t.Errorf("app.Run returned error on tray exit: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app.Run to shut down via Tray OnExit")
	}
}

func TestApp_Accessors(t *testing.T) {
	app := NewApp(Options{}, nil, nil)
	if app.ActiveWindow() != nil {
		t.Errorf("expected ActiveWindow nil initially")
	}
	if app.Tray() != nil {
		t.Errorf("expected Tray nil initially")
	}
	if app.Hook() != nil {
		t.Errorf("expected Hook nil initially")
	}
}

func TestApp_OpenSearch_OpenSettings_WithActiveClients(t *testing.T) {
	cleanup := docsearch.SetActivateDocSearchWindowFnForTesting(func() bool {
		return false
	})
	defer cleanup()

	cfg := &docsearch.Config{
		Server: docsearch.ServerConfig{
			Host: "127.0.0.1",
			Port: 18080,
		},
	}
	srv, err := docsearch.NewServer(cfg, "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect SSE client
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/events: %v", err)
	}
	defer resp.Body.Close()

	// Wait for active client
	var active bool
	for i := 0; i < 50; i++ {
		if srv.HasActiveWebClients() {
			active = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !active {
		t.Fatalf("expected HasActiveWebClients() to be true")
	}

	app := NewApp(Options{}, nil, nil)
	app.server = srv
	app.cfg = cfg

	var openURLCalled bool
	app.OpenURLFunc = func(u string) error {
		openURLCalled = true
		return nil
	}

	reader := bufio.NewReader(resp.Body)

	// Test OpenSearch()
	if err := app.OpenSearch(); err != nil {
		t.Fatalf("OpenSearch returned error: %v", err)
	}
	if openURLCalled {
		t.Errorf("expected OpenURLFunc NOT to be called when active clients exist")
	}

	// Verify SSE received "focus"
	var foundFocus bool
	for i := 0; i < 10; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "focus") {
			foundFocus = true
			break
		}
	}
	if !foundFocus {
		t.Errorf("expected SSE client to receive focus event")
	}

	// Test OpenSettings()
	openURLCalled = false
	if err := app.OpenSettings(); err != nil {
		t.Fatalf("OpenSettings returned error: %v", err)
	}
	if openURLCalled {
		t.Errorf("expected OpenURLFunc NOT to be called when active clients exist")
	}

	// Verify SSE received "settings"
	var foundSettings bool
	for i := 0; i < 10; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "settings") {
			foundSettings = true
			break
		}
	}
	if !foundSettings {
		t.Errorf("expected SSE client to receive settings event")
	}
}

func TestApp_OpenSearch_OpenSettings_NoClients_NoWindow(t *testing.T) {
	cleanup := docsearch.SetActivateDocSearchWindowFnForTesting(func() bool {
		return false
	})
	defer cleanup()

	cfg := &docsearch.Config{
		Server: docsearch.ServerConfig{
			Host: "127.0.0.1",
			Port: 18080,
		},
	}
	srv, err := docsearch.NewServer(cfg, "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	app := NewApp(Options{}, nil, nil)
	app.server = srv
	app.cfg = cfg

	var openedURLs []string
	app.OpenURLFunc = func(u string) error {
		openedURLs = append(openedURLs, u)
		return nil
	}

	// OpenSearch
	if err := app.OpenSearch(); err != nil {
		t.Fatalf("OpenSearch error: %v", err)
	}
	if len(openedURLs) != 1 {
		t.Fatalf("expected 1 URL opened, got %d", len(openedURLs))
	}
	if !strings.HasPrefix(openedURLs[0], "http://127.0.0.1:18080") {
		t.Errorf("unexpected URL opened: %s", openedURLs[0])
	}

	// OpenSettings
	if err := app.OpenSettings(); err != nil {
		t.Fatalf("OpenSettings error: %v", err)
	}
	if len(openedURLs) != 2 {
		t.Fatalf("expected 2 URLs opened, got %d", len(openedURLs))
	}
	if !strings.Contains(openedURLs[1], "settings=1") {
		t.Errorf("expected settings URL to contain settings=1, got %s", openedURLs[1])
	}
}

func TestApp_OpenSearch_OpenSettings_WindowActivated(t *testing.T) {
	cleanup := docsearch.SetActivateDocSearchWindowFnForTesting(func() bool {
		return true
	})
	defer cleanup()

	app := NewApp(Options{}, nil, nil)
	openURLCalled := false
	app.OpenURLFunc = func(u string) error {
		openURLCalled = true
		return nil
	}

	if err := app.OpenSearch(); err != nil {
		t.Fatalf("OpenSearch error: %v", err)
	}
	if openURLCalled {
		t.Errorf("expected OpenURLFunc NOT to be called when window is activated")
	}

	if err := app.OpenSettings(); err != nil {
		t.Fatalf("OpenSettings error: %v", err)
	}
	if openURLCalled {
		t.Errorf("expected OpenURLFunc NOT to be called when window is activated")
	}
}

func TestBuildChildArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty args",
			input:    []string{},
			expected: []string{"--foreground"},
		},
		{
			name:     "args without foreground",
			input:    []string{"--port", "18080", "--config", "conf.json"},
			expected: []string{"--port", "18080", "--config", "conf.json", "--foreground"},
		},
		{
			name:     "args with --foreground",
			input:    []string{"--port", "18080", "--foreground", "--config", "conf.json"},
			expected: []string{"--port", "18080", "--config", "conf.json", "--foreground"},
		},
		{
			name:     "args with -f shorthand",
			input:    []string{"-f", "--port", "18080"},
			expected: []string{"--port", "18080", "--foreground"},
		},
		{
			name:     "args with --foreground=true or -f=true",
			input:    []string{"--foreground=true", "-f=true", "--port", "18080"},
			expected: []string{"--port", "18080", "--foreground"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildChildArgs(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("buildChildArgs() len = %d, expected %d (%v vs %v)", len(got), len(tt.expected), got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %q, expected %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestRealMain_Detach(t *testing.T) {
	origSpawn := spawnBackgroundProcessFn
	origRun := runAppFn
	defer func() {
		spawnBackgroundProcessFn = origSpawn
		runAppFn = origRun
	}()

	t.Run("default execution spawns background process and returns 0", func(t *testing.T) {
		var spawnCalled bool
		var capturedArgs []string
		var runCalled bool

		spawnBackgroundProcessFn = func(args []string, stdout, stderr io.Writer) error {
			spawnCalled = true
			capturedArgs = args
			return nil
		}
		runAppFn = func(app *App, ctx context.Context) error {
			runCalled = true
			return nil
		}

		var stdout, stderr bytes.Buffer
		testArgs := []string{"--port", "18080"}
		code := realMain(testArgs, &stdout, &stderr)

		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if !spawnCalled {
			t.Errorf("expected spawnBackgroundProcessFn to be called")
		}
		if runCalled {
			t.Errorf("expected runAppFn NOT to be called")
		}
		if len(capturedArgs) != 2 || capturedArgs[0] != "--port" || capturedArgs[1] != "18080" {
			t.Errorf("unexpected args passed to spawnBackgroundProcessFn: %v", capturedArgs)
		}
	})

	t.Run("spawn error returns 1 and prints to stderr", func(t *testing.T) {
		spawnBackgroundProcessFn = func(args []string, stdout, stderr io.Writer) error {
			return errors.New("failed to spawn child process")
		}

		var stdout, stderr bytes.Buffer
		code := realMain([]string{"--port", "18080"}, &stdout, &stderr)

		if code != 1 {
			t.Errorf("expected exit code 1 on spawn error, got %d", code)
		}
		if !strings.Contains(stderr.String(), "Error spawning background process") {
			t.Errorf("expected stderr to contain error message, got %q", stderr.String())
		}
	})
}

func TestRealMain_Foreground(t *testing.T) {
	origSpawn := spawnBackgroundProcessFn
	origRun := runAppFn
	defer func() {
		spawnBackgroundProcessFn = origSpawn
		runAppFn = origRun
	}()

	t.Run("runs foreground when --foreground is passed", func(t *testing.T) {
		var spawnCalled bool
		var runCalled bool
		var capturedOpts Options

		spawnBackgroundProcessFn = func(args []string, stdout, stderr io.Writer) error {
			spawnCalled = true
			return nil
		}
		runAppFn = func(app *App, ctx context.Context) error {
			runCalled = true
			capturedOpts = app.Options
			return nil
		}

		var stdout, stderr bytes.Buffer
		code := realMain([]string{"--foreground", "--port", "19090"}, &stdout, &stderr)

		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if spawnCalled {
			t.Errorf("expected spawnBackgroundProcessFn NOT to be called")
		}
		if !runCalled {
			t.Errorf("expected runAppFn to be called")
		}
		if !capturedOpts.Foreground {
			t.Errorf("expected Options.Foreground to be true")
		}
		if capturedOpts.Port != 19090 {
			t.Errorf("expected Port 19090, got %d", capturedOpts.Port)
		}
	})

	t.Run("runs foreground when -f is passed", func(t *testing.T) {
		var spawnCalled bool
		var runCalled bool
		var capturedOpts Options

		spawnBackgroundProcessFn = func(args []string, stdout, stderr io.Writer) error {
			spawnCalled = true
			return nil
		}
		runAppFn = func(app *App, ctx context.Context) error {
			runCalled = true
			capturedOpts = app.Options
			return nil
		}

		var stdout, stderr bytes.Buffer
		code := realMain([]string{"-f", "--hotkey-interval", "250"}, &stdout, &stderr)

		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if spawnCalled {
			t.Errorf("expected spawnBackgroundProcessFn NOT to be called")
		}
		if !runCalled {
			t.Errorf("expected runAppFn to be called")
		}
		if !capturedOpts.Foreground {
			t.Errorf("expected Options.Foreground to be true")
		}
		if capturedOpts.HotkeyInterval != 250 {
			t.Errorf("expected HotkeyInterval 250, got %d", capturedOpts.HotkeyInterval)
		}
	})

	t.Run("run error returns 1", func(t *testing.T) {
		spawnBackgroundProcessFn = func(args []string, stdout, stderr io.Writer) error {
			return nil
		}
		runAppFn = func(app *App, ctx context.Context) error {
			return errors.New("server startup failure")
		}

		var stdout, stderr bytes.Buffer
		code := realMain([]string{"--foreground"}, &stdout, &stderr)

		if code != 1 {
			t.Errorf("expected exit code 1 on runApp failure, got %d", code)
		}
		if !strings.Contains(stderr.String(), "server startup failure") {
			t.Errorf("expected stderr to contain failure message, got %q", stderr.String())
		}
	})

	t.Run("help flag returns 0 without running", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := realMain([]string{"--help"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for --help, got %d", code)
		}
	})

	t.Run("invalid flag returns 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := realMain([]string{"--invalid-flag"}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("expected exit code 1 for invalid flag, got %d", code)
		}
	})
}

func TestSpawnBackgroundProcess(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	var calledPath string
	var calledArgs []string
	var capturedCmds []*exec.Cmd

	execCommand = func(name string, arg ...string) *exec.Cmd {
		calledPath = name
		calledArgs = arg
		// Create a command that writes to stdout so we can verify log file output
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/c", "echo background output")
		} else {
			cmd = exec.Command("echo", "background output")
		}
		capturedCmds = append(capturedCmds, cmd)
		return cmd
	}

	var stdout, stderr bytes.Buffer
	inputArgs := []string{"--port", "18080", "-f", "--config", "mycfg.json"}
	err = spawnBackgroundProcess(inputArgs, &stdout, &stderr)
	if err != nil {
		t.Fatalf("spawnBackgroundProcess failed: %v", err)
	}

	if len(capturedCmds) == 0 {
		t.Fatalf("expected at least one command to be captured")
	}
	lastCmd := capturedCmds[len(capturedCmds)-1]

	// Verify working directory is explicitly set
	if lastCmd.Dir != tmpDir {
		t.Errorf("lastCmd.Dir = %q; expected %q", lastCmd.Dir, tmpDir)
	}

	// Verify cmd.Stdout and cmd.Stderr are redirected
	if lastCmd.Stdout == nil {
		t.Errorf("expected lastCmd.Stdout to be redirected")
	}
	if lastCmd.Stderr == nil {
		t.Errorf("expected lastCmd.Stderr to be redirected")
	}

	// Verify log file is created in working directory
	logPath := filepath.Join(tmpDir, "docsearch-gui.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("expected log file %q to exist: %v", logPath, err)
	}

	// Wait for test command to exit and verify log content
	if lastCmd.Process != nil {
		_ = lastCmd.Wait()
		content, readErr := os.ReadFile(logPath)
		if readErr != nil {
			t.Errorf("failed to read log file: %v", readErr)
		} else if !strings.Contains(string(content), "background output") {
			t.Errorf("expected log file to contain 'background output', got: %q", string(content))
		}
	}

	// Verify executable path
	expectedExe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable failed: %v", err)
	}
	if calledPath != expectedExe {
		t.Errorf("calledPath = %q; expected %q", calledPath, expectedExe)
	}

	// Verify childArgs: -f is removed, and --foreground is present
	for _, a := range calledArgs {
		if a == "-f" {
			t.Errorf("calledArgs should not contain -f: %v", calledArgs)
		}
	}
	hasForeground := false
	for _, a := range calledArgs {
		if a == "--foreground" {
			hasForeground = true
		}
	}
	if !hasForeground {
		t.Errorf("expected --foreground in calledArgs: %v", calledArgs)
	}

	// Verify creation flags on Windows
	if runtime.GOOS == "windows" {
		firstFlags := getDetachedCreationFlags(capturedCmds[0])
		if firstFlags != expectedDetachedFlags {
			t.Errorf("initial creation flags = %d, expected %d", firstFlags, expectedDetachedFlags)
		}
		if len(capturedCmds) > 1 {
			fallbackFlags := getDetachedCreationFlags(capturedCmds[1])
			expectedFallback := expectedDetachedFlags &^ flagBreakawayFromJob
			if fallbackFlags != expectedFallback {
				t.Errorf("fallback creation flags = %d, expected %d", fallbackFlags, expectedFallback)
			}
		}
	}

	// Verify stdout announcement
	expectedMsg := "DocSearch started in background (tray resident).\n"
	if stdout.String() != expectedMsg {
		t.Errorf("stdout = %q, expected %q", stdout.String(), expectedMsg)
	}

	t.Run("Start error is returned when all attempts fail", func(t *testing.T) {
		execCommand = func(name string, arg ...string) *exec.Cmd {
			// Non-existent command that fails Start()
			return exec.Command("nonexistent_binary_for_testing_12345")
		}
		var out, errOut bytes.Buffer
		err := spawnBackgroundProcess([]string{}, &out, &errOut)
		if err == nil {
			t.Errorf("expected error on failed Start(), got nil")
		}
	})

	t.Run("fallback when breakaway fails", func(t *testing.T) {
		var invocations int
		var firstCmd, secondCmd *exec.Cmd

		execCommand = func(name string, arg ...string) *exec.Cmd {
			invocations++
			if invocations == 1 {
				// Return a command that fails Start() (e.g., simulating ERROR_ACCESS_DENIED from parent job)
				firstCmd = exec.Command("nonexistent_binary_for_testing_12345")
				return firstCmd
			}
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd.exe", "/c", "echo fallback success")
			} else {
				cmd = exec.Command("echo", "fallback success")
			}
			secondCmd = cmd
			return cmd
		}

		var out, errOut bytes.Buffer
		err := spawnBackgroundProcess([]string{}, &out, &errOut)
		if err != nil {
			t.Fatalf("expected fallback to succeed, got error: %v", err)
		}

		if invocations != 2 {
			t.Errorf("expected 2 invocations, got %d", invocations)
		}

		if runtime.GOOS == "windows" {
			if getDetachedCreationFlags(firstCmd)&flagBreakawayFromJob == 0 {
				t.Errorf("expected firstCmd to have flagBreakawayFromJob")
			}
			if getDetachedCreationFlags(secondCmd)&flagBreakawayFromJob != 0 {
				t.Errorf("expected secondCmd to NOT have flagBreakawayFromJob")
			}
		}
	})
}

func TestConfigureDetachedProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	configureDetachedProcess(cmd)
	flags := getDetachedCreationFlags(cmd)
	if flags != expectedDetachedFlags {
		t.Errorf("creation flags = %d, expected %d", flags, expectedDetachedFlags)
	}

	if runtime.GOOS == "windows" {
		if flags&flagBreakawayFromJob == 0 {
			t.Errorf("expected flagBreakawayFromJob in creation flags")
		}
		if !removeBreakawayFlag(cmd) {
			t.Errorf("expected removeBreakawayFlag to return true")
		}
		if getDetachedCreationFlags(cmd)&flagBreakawayFromJob != 0 {
			t.Errorf("expected flagBreakawayFromJob to be removed after removeBreakawayFlag")
		}
		if removeBreakawayFlag(cmd) {
			t.Errorf("expected removeBreakawayFlag to return false when flag already removed")
		}
	}
}
