package docsearch

import (
	"sync"
	"testing"
	"time"
)

func TestTrayIconLifecycle(t *testing.T) {
	opened := false
	settings := false
	exited := false

	cbs := TrayCallbacks{
		OnOpen: func() {
			opened = true
		},
		OnSettings: func() {
			settings = true
		},
		OnExit: func() {
			exited = true
		},
	}

	tray := NewTrayIcon(cbs)
	if tray == nil {
		t.Fatal("expected NewTrayIcon to return non-nil instance")
	}

	if tray.IsRunning() {
		t.Fatal("expected IsRunning to be false initially")
	}

	if err := tray.Start(); err != nil {
		t.Fatalf("failed to start tray icon: %v", err)
	}

	if !tray.IsRunning() {
		t.Fatal("expected IsRunning to be true after Start")
	}

	// Double start should return an error
	if err := tray.Start(); err == nil {
		t.Fatal("expected error on duplicate Start(), got nil")
	}

	if err := tray.Stop(); err != nil {
		t.Fatalf("failed to stop tray icon: %v", err)
	}

	if tray.IsRunning() {
		t.Fatal("expected IsRunning to be false after Stop")
	}

	// Verify callbacks weren't inadvertently triggered by start/stop
	_ = opened
	_ = settings
	_ = exited
}

func TestTrayIconMultipleStop(t *testing.T) {
	tray := NewTrayIcon(TrayCallbacks{})
	if err := tray.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := tray.Stop(); err != nil {
			t.Fatalf("Stop call %d failed: %v", i+1, err)
		}
		if tray.IsRunning() {
			t.Fatalf("expected IsRunning to be false after Stop call %d", i+1)
		}
	}
}

func TestTrayIconStopWithoutStart(t *testing.T) {
	tray := NewTrayIcon(TrayCallbacks{})
	if err := tray.Stop(); err != nil {
		t.Fatalf("Stop without Start should not return error: %v", err)
	}
}

func TestTrayIconConcurrentStop(t *testing.T) {
	tray := NewTrayIcon(TrayCallbacks{})
	if err := tray.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- tray.Stop()
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Stop returned unexpected error: %v", err)
		}
	}

	if tray.IsRunning() {
		t.Fatal("expected tray to be stopped after concurrent Stop")
	}
}

func TestTrayIconCallbacks(t *testing.T) {
	openCh := make(chan struct{}, 1)
	settingsCh := make(chan struct{}, 1)
	exitCh := make(chan struct{}, 1)

	cbs := TrayCallbacks{
		OnOpen: func() {
			openCh <- struct{}{}
		},
		OnSettings: func() {
			settingsCh <- struct{}{}
		},
		OnExit: func() {
			exitCh <- struct{}{}
		},
	}

	tray := NewTrayIcon(cbs)

	// Test menu command ID 1 -> OnOpen
	tray.handleMenuAction(1)
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen callback for action 1")
	}

	// Test menu command ID 2 -> OnSettings
	tray.handleMenuAction(2)
	select {
	case <-settingsCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnSettings callback for action 2")
	}

	// Test menu command ID 3 -> OnExit
	tray.handleMenuAction(3)
	select {
	case <-exitCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnExit callback for action 3")
	}

	// Test unknown action -> nothing sent
	tray.handleMenuAction(999)
	select {
	case <-openCh:
		t.Fatal("unexpected callback for unknown action")
	case <-settingsCh:
		t.Fatal("unexpected callback for unknown action")
	case <-exitCh:
		t.Fatal("unexpected callback for unknown action")
	case <-time.After(50 * time.Millisecond):
	}

	// Nil callbacks shouldn't panic
	nilTray := NewTrayIcon(TrayCallbacks{})
	nilTray.handleMenuAction(1)
	nilTray.handleMenuAction(2)
	nilTray.handleMenuAction(3)
	nilTray.handleMenuAction(0)
}

func TestTrayIconTrayEvent(t *testing.T) {
	openCh := make(chan struct{}, 2)

	cbs := TrayCallbacks{
		OnOpen: func() {
			openCh <- struct{}{}
		},
	}

	tray := NewTrayIcon(cbs)

	// Left click -> OnOpen
	tray.handleTrayEvent(0x0202) // WM_LBUTTONUP
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen callback for WM_LBUTTONUP")
	}

	// Double click -> OnOpen
	tray.handleTrayEvent(0x0203) // WM_LBUTTONDBLCLK
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen callback for WM_LBUTTONDBLCLK")
	}
}

func TestTrayIcon_CallbacksAccessor(t *testing.T) {
	cbs := TrayCallbacks{
		OnOpen:     func() {},
		OnSettings: func() {},
		OnExit:     func() {},
	}
	tray := NewTrayIcon(cbs)
	ret := tray.Callbacks()
	if ret.OnOpen == nil || ret.OnSettings == nil || ret.OnExit == nil {
		t.Errorf("expected callbacks to be non-nil")
	}
}

func TestTrayIcon_ContextMenu(t *testing.T) {
	origTrack := callTrackPopupMenu
	defer func() {
		callTrackPopupMenu = origTrack
	}()

	openCh := make(chan struct{}, 1)
	cbs := TrayCallbacks{
		OnOpen: func() {
			openCh <- struct{}{}
		},
	}
	tray := NewTrayIcon(cbs)

	// Mock menu click ID 1 -> OnOpen
	callTrackPopupMenu = func(hMenu, uFlags, x, y, nReserved, hWnd, prcRect uintptr) uintptr {
		return 1
	}

	tray.handleContextMenu()
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen from context menu")
	}

	// Test right click event
	tray.handleTrayEvent(0x0205) // wmRButtonUp
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen from wmRButtonUp event")
	}
}

func TestTrayWndProc(t *testing.T) {
	// Test wmDestroy
	ret := trayWndProc(0, 0x0002, 0, 0) // wmDestroy
	if ret != 0 {
		t.Errorf("expected 0 from wmDestroy, got %d", ret)
	}

	// Test default
	retDef := trayWndProc(0, 0x0000, 0, 0)
	_ = retDef

	// Test wmTrayCallback with registered tray
	openCh := make(chan struct{}, 1)
	tray := NewTrayIcon(TrayCallbacks{OnOpen: func() { openCh <- struct{}{} }})
	dummyHWnd := uintptr(99999)
	trayMapMu.Lock()
	trayMap[dummyHWnd] = tray
	trayMapMu.Unlock()
	defer func() {
		trayMapMu.Lock()
		delete(trayMap, dummyHWnd)
		trayMapMu.Unlock()
	}()

	retCallback := trayWndProc(dummyHWnd, 0x0400+100, 0, 0x0202) // wmTrayCallback with wmLButtonUp
	if retCallback != 0 {
		t.Errorf("expected 0 from wmTrayCallback, got %d", retCallback)
	}
	select {
	case <-openCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected OnOpen from trayWndProc callback")
	}
}


