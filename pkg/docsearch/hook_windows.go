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
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	whKeyboardLL = 13

	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012
)

// point represents a 2D coordinate for Win32 MSG.
type point struct {
	X int32
	Y int32
}

// msg represents the Win32 MSG structure.
type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// kbdLLHookStruct represents the Win32 KBDLLHOOKSTRUCT structure passed to WH_KEYBOARD_LL hook.
type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

var (
	activeHookMu sync.Mutex
	primaryHook  *KeyboardHook

	hookCallbackPtr = syscall.NewCallback(hookCallback)
)

// processHookEvent processes a keyboard event for the registered primary hook.
func processHookEvent(wParam uintptr, kbd *kbdLLHookStruct) {
	if kbd == nil {
		return
	}
	isDown := wParam == wmKeyDown || wParam == wmSysKeyDown
	isUp := wParam == wmKeyUp || wParam == wmSysKeyUp

	if isDown || isUp {
		activeHookMu.Lock()
		hook := primaryHook
		activeHookMu.Unlock()

		if hook != nil && hook.detector != nil {
			hook.detector.Process(kbd.VkCode, isDown)
		}
	}
}

// hookCallback is the low-level keyboard hook procedure invoked by Windows.
func hookCallback(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && lParam != 0 {
		kbd := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		processHookEvent(wParam, kbd)
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// Start installs the global low-level keyboard hook and runs the Windows message loop
// in a dedicated OS thread.
func (h *KeyboardHook) Start() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return fmt.Errorf("keyboard hook is already running")
	}
	h.running = true
	h.done = make(chan struct{})
	h.mu.Unlock()

	errCh := make(chan error, 1)
	readyCh := make(chan struct{})

	go func() {
		// Low-level hooks and Windows message queues must be pinned to an OS thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		threadID := windows.GetCurrentThreadId()

		h.mu.Lock()
		h.threadID = threadID
		h.mu.Unlock()

		activeHookMu.Lock()
		primaryHook = h
		activeHookMu.Unlock()

		defer func() {
			activeHookMu.Lock()
			if primaryHook == h {
				primaryHook = nil
			}
			activeHookMu.Unlock()

			h.mu.Lock()
			if h.hookH != 0 {
				procUnhookWindowsHookEx.Call(h.hookH)
				h.hookH = 0
			}
			h.running = false
			h.mu.Unlock()

			close(h.done)
		}()

		hmod, _, _ := procGetModuleHandleW.Call(0)
		hhk, _, err := procSetWindowsHookExW.Call(
			uintptr(whKeyboardLL),
			hookCallbackPtr,
			hmod,
			0,
		)
		if hhk == 0 {
			errCh <- fmt.Errorf("SetWindowsHookEx failed: %w", err)
			return
		}

		h.mu.Lock()
		h.hookH = hhk
		h.mu.Unlock()

		close(readyCh)

		// Windows Message Loop
		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(
				uintptr(unsafe.Pointer(&m)),
				0,
				0,
				0,
			)
			// GetMessage returns:
			//   0 when WM_QUIT is retrieved
			//  -1 on error
			//  >0 for regular messages
			if int32(ret) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()

	select {
	case err := <-errCh:
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	case <-readyCh:
		return nil
	case <-time.After(3 * time.Second):
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return fmt.Errorf("timeout waiting for keyboard hook to start")
	}
}

// Stop unhooks the Windows hook procedure and safely terminates the message loop.
func (h *KeyboardHook) Stop() error {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = false

	hookH := h.hookH
	threadID := h.threadID
	h.hookH = 0
	h.mu.Unlock()

	// Unhook immediately if handle exists
	if hookH != 0 {
		procUnhookWindowsHookEx.Call(hookH)
	}

	// Post WM_QUIT to break the GetMessage loop
	if threadID != 0 {
		procPostThreadMessageW.Call(
			uintptr(threadID),
			uintptr(wmQuit),
			0,
			0,
		)
	}

	// Wait for background goroutine to clean up and exit
	if h.done != nil {
		select {
		case <-h.done:
		case <-time.After(3 * time.Second):
			return fmt.Errorf("timeout waiting for keyboard hook to stop")
		}
	}

	return nil
}
