//go:build windows

package docsearch

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")

	callTrackPopupMenu = func(hMenu, uFlags, x, y, nReserved, hWnd, prcRect uintptr) uintptr {
		ret, _, _ := procTrackPopupMenu.Call(hMenu, uFlags, x, y, nReserved, hWnd, prcRect)
		return ret
	}
)

const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	wmUser         = 0x0400
	wmTrayCallback = wmUser + 100

	wmRButtonUp     = 0x0205
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203

	mfString    = 0x00000000
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	idiApplication = 32512
)

type notifyIconDataW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeoutOrVer    uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

var (
	trayClassAtom uint16
	trayClassMu   sync.Mutex
	trayClassName = windows.StringToUTF16Ptr("DocSearchTrayWindowClass")

	trayMapMu sync.RWMutex
	trayMap   = make(map[uintptr]*TrayIcon)
)

func ensureTrayClassRegistered() error {
	trayClassMu.Lock()
	defer trayClassMu.Unlock()

	if trayClassAtom != 0 {
		return nil
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	var wc wndClassExW
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.lpfnWndProc = syscall.NewCallback(trayWndProc)
	wc.hInstance = hInst
	wc.lpszClassName = trayClassName

	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return fmt.Errorf("RegisterClassExW failed for tray window: %w", err)
	}
	trayClassAtom = uint16(ret)
	return nil
}

func trayWndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		trayMapMu.RLock()
		t := trayMap[hWnd]
		trayMapMu.RUnlock()

		if t != nil {
			evt := uint32(lParam & 0xffff)
			t.handleTrayEvent(evt)
		}
		return 0

	case wmClose:
		procDestroyWindow.Call(hWnd)
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0

	default:
		ret, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

func (t *TrayIcon) handleTrayEvent(event uint32) {
	switch event {
	case wmRButtonUp:
		t.handleContextMenu()
	case wmLButtonUp, wmLButtonDblClk:
		if t.cbs.OnOpen != nil {
			go t.cbs.OnOpen()
		}
	}
}

func (t *TrayIcon) handleContextMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	menuItem1 := windows.StringToUTF16Ptr("🔍 検索画面を開く")
	menuItem2 := windows.StringToUTF16Ptr("⚙️ 設定")
	menuItem3 := windows.StringToUTF16Ptr("🚪 終了")

	procAppendMenuW.Call(hMenu, mfString, 1, uintptr(unsafe.Pointer(menuItem1)))
	procAppendMenuW.Call(hMenu, mfString, 2, uintptr(unsafe.Pointer(menuItem2)))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, 3, uintptr(unsafe.Pointer(menuItem3)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	procSetForegroundWindow.Call(t.hWnd)
	cmd := callTrackPopupMenu(
		hMenu,
		uintptr(tpmRightButton|tpmReturnCmd),
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		t.hWnd,
		0,
	)
	procPostMessageW.Call(t.hWnd, 0, 0, 0)

	t.handleMenuAction(int(cmd))
}

// Start registers the tray icon and runs the message loop in a dedicated OS thread.
func (t *TrayIcon) Start() error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("tray icon is already running")
	}
	t.running = true
	t.done = make(chan struct{})
	t.mu.Unlock()

	ready := make(chan error, 1)
	go t.run(ready)

	if err := <-ready; err != nil {
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
		return err
	}
	return nil
}

func (t *TrayIcon) run(ready chan error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	if err := ensureTrayClassRegistered(); err != nil {
		ready <- err
		return
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hWnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(trayClassName)),
		uintptr(unsafe.Pointer(trayClassName)),
		0,
		0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if hWnd == 0 {
		ready <- fmt.Errorf("CreateWindowExW failed for tray window: %w", err)
		return
	}
	defer procDestroyWindow.Call(hWnd)

	t.mu.Lock()
	t.hWnd = hWnd
	t.threadID = windows.GetCurrentThreadId()
	t.mu.Unlock()

	trayMapMu.Lock()
	trayMap[hWnd] = t
	trayMapMu.Unlock()

	defer func() {
		trayMapMu.Lock()
		delete(trayMap, hWnd)
		trayMapMu.Unlock()
	}()

	// Load default application icon
	hIcon, _, _ := procLoadIconW.Call(0, uintptr(idiApplication))

	var nid notifyIconDataW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = hWnd
	nid.uID = 1
	nid.uFlags = nifMessage | nifIcon | nifTip
	nid.uCallbackMessage = wmTrayCallback
	nid.hIcon = hIcon

	tipRunes := windows.StringToUTF16("DocSearch - 全文検索")
	copy(nid.szTip[:], tipRunes)

	// Shell_NotifyIconW will succeed when a shell tray (taskbar) is present.
	// In headless, CI, or sessions without Shell_TrayWnd, it returns 0.
	ret, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret != 0 {
		t.mu.Lock()
		t.iconAdded = true
		t.mu.Unlock()
	}

	defer func() {
		t.mu.Lock()
		added := t.iconAdded
		t.iconAdded = false
		t.mu.Unlock()

		if added {
			var delNid notifyIconDataW
			delNid.cbSize = uint32(unsafe.Sizeof(delNid))
			delNid.hWnd = hWnd
			delNid.uID = 1
			procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&delNid)))
		}
	}()

	ready <- nil

	// Message loop
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// Stop removes the tray icon, posts WM_QUIT, destroys the window, and waits for the message loop to exit.
func (t *TrayIcon) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = false
	hWnd := t.hWnd
	threadID := t.threadID
	done := t.done
	added := t.iconAdded
	t.iconAdded = false
	t.mu.Unlock()

	if hWnd != 0 {
		if added {
			var nid notifyIconDataW
			nid.cbSize = uint32(unsafe.Sizeof(nid))
			nid.hWnd = hWnd
			nid.uID = 1
			procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		}

		procPostMessageW.Call(hWnd, wmClose, 0, 0)
		if threadID != 0 {
			procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
		}
	}

	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}
