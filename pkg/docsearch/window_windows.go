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
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassExW      = user32.NewProc("RegisterClassExW")
	procUnregisterClassW      = user32.NewProc("UnregisterClassW")
	procCreateWindowExW       = user32.NewProc("CreateWindowExW")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
	procUpdateWindow          = user32.NewProc("UpdateWindow")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procSetFocus              = user32.NewProc("SetFocus")
	procGetFocus              = user32.NewProc("GetFocus")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procDefWindowProcW        = user32.NewProc("DefWindowProcW")
	procSendMessageW          = user32.NewProc("SendMessageW")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procSetWindowTextW        = user32.NewProc("SetWindowTextW")
	procGetWindowTextLengthW  = user32.NewProc("GetWindowTextLengthW")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")
	procGetStockObject        = gdi32.NewProc("GetStockObject")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
)

const (
	wsExTopMost    = 0x00000008
	wsExToolWindow = 0x00000080
	wsExClientEdge = 0x00000200
	wsPopup        = 0x80000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsTabStop      = 0x00010000
	wsBorder       = 0x00800000

	esAutoHScroll = 0x0080
	esLeft        = 0x0000

	bsAutoCheckbox = 0x00000003
	bmGetCheck     = 0x00F0
	bmSetCheck     = 0x00F1
	bstUnchecked   = 0x0000
	bstChecked     = 0x0001

	swHide   = 0
	swShow   = 5

	emSetSel = 0x00B1

	wmSetFont = 0x0030
	wmClose   = 0x0010
	wmDestroy = 0x0002

	colorBtnFace      = 15
	defaultGuiFont    = 17
	smCxScreen        = 0
	smCyScreen        = 1

	vkTab    = 0x09
	vkReturn = 0x0D
	vkEscape = 0x1B
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

var (
	wndClassAtom uint16
	classMu      sync.Mutex
	wndClassName = windows.StringToUTF16Ptr("DocSearchFloatingWindow")
)

func globalWndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmDestroy:
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

// SearchWindow represents the native Win32 floating search window.
type SearchWindow struct {
	mu         sync.RWMutex
	indexes    []IndexConfig
	onSearch   SearchCallback
	options    WindowOptions
	visible    bool
	closed     bool

	hwnd       uintptr
	editHWnd   uintptr
	checkHWnds []uintptr
	threadID   uint32
	done       chan struct{}
}

// NewSearchWindow creates and initializes a Win32 native floating search window.
func NewSearchWindow(indexes []IndexConfig, onSearch SearchCallback) (*SearchWindow, error) {
	opts := DefaultWindowOptions()

	w := &SearchWindow{
		indexes:    indexes,
		onSearch:   onSearch,
		options:    opts,
		checkHWnds: make([]uintptr, 0, len(indexes)),
		done:       make(chan struct{}),
	}

	readyCh := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		w.threadID = windows.GetCurrentThreadId()

		// Register window class once
		classMu.Lock()
		hInst, _, _ := procGetModuleHandleW.Call(0)
		if wndClassAtom == 0 {
			wndProcPtr := syscall.NewCallback(globalWndProc)
			bgBrush, _, _ := procCreateSolidBrush.Call(uintptr(0x00F0F0F0)) // Light gray background
			if bgBrush == 0 {
				bgBrush, _, _ = procGetStockObject.Call(uintptr(colorBtnFace))
			}

			wc := wndClassExW{
				cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
				lpfnWndProc:   wndProcPtr,
				hInstance:     hInst,
				hbrBackground: bgBrush,
				lpszClassName: wndClassName,
			}
			atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
			wndClassAtom = uint16(atom)
		}
		classMu.Unlock()

		// Calculate screen center position
		screenWidth, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
		screenHeight, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
		winX := (int(screenWidth) - opts.Width) / 2
		winY := (int(screenHeight) - opts.Height) / 3 // Slightly above center

		// Create main borderless popup window
		winTitle := windows.StringToUTF16Ptr("DocSearch")
		hwnd, _, err := procCreateWindowExW.Call(
			uintptr(wsExTopMost|wsExToolWindow),
			uintptr(unsafe.Pointer(wndClassName)),
			uintptr(unsafe.Pointer(winTitle)),
			uintptr(wsPopup|wsBorder),
			uintptr(winX),
			uintptr(winY),
			uintptr(opts.Width),
			uintptr(opts.Height),
			0,
			0,
			hInst,
			0,
		)
		if hwnd == 0 {
			readyCh <- fmt.Errorf("CreateWindowEx for main window failed: %w", err)
			return
		}

		w.hwnd = hwnd

		// Stock GUI font
		hFont, _, _ := procGetStockObject.Call(uintptr(defaultGuiFont))

		// Create Edit Control (Search Bar)
		editClass := windows.StringToUTF16Ptr("EDIT")
		editHWnd, _, err := procCreateWindowExW.Call(
			uintptr(wsExClientEdge),
			uintptr(unsafe.Pointer(editClass)),
			0,
			uintptr(wsChild|wsVisible|wsTabStop|esLeft|esAutoHScroll),
			uintptr(16),
			uintptr(16),
			uintptr(opts.Width-32),
			uintptr(30),
			hwnd,
			0,
			hInst,
			0,
		)
		if editHWnd == 0 {
			readyCh <- fmt.Errorf("CreateWindowEx for edit control failed: %w", err)
			return
		}
		w.editHWnd = editHWnd
		if hFont != 0 {
			procSendMessageW.Call(editHWnd, uintptr(wmSetFont), hFont, 1)
		}

		// Create Checkboxes for each index
		btnClass := windows.StringToUTF16Ptr("BUTTON")
		chkX := 16
		chkY := 56
		chkHeight := 24
		for _, idx := range w.indexes {
			namePtr := windows.StringToUTF16Ptr(idx.Name)
			chkWidth := len([]rune(idx.Name))*16 + 32
			if chkWidth < 80 {
				chkWidth = 80
			}

			chkHWnd, _, _ := procCreateWindowExW.Call(
				0,
				uintptr(unsafe.Pointer(btnClass)),
				uintptr(unsafe.Pointer(namePtr)),
				uintptr(wsChild|wsVisible|wsTabStop|bsAutoCheckbox),
				uintptr(chkX),
				uintptr(chkY),
				uintptr(chkWidth),
				uintptr(chkHeight),
				hwnd,
				0,
				hInst,
				0,
			)
			if chkHWnd != 0 {
				if hFont != 0 {
					procSendMessageW.Call(chkHWnd, uintptr(wmSetFont), hFont, 1)
				}
				if idx.DefaultSelected {
					procSendMessageW.Call(chkHWnd, uintptr(bmSetCheck), uintptr(bstChecked), 0)
				}
				w.checkHWnds = append(w.checkHWnds, chkHWnd)
				chkX += chkWidth + 8
			}
		}

		readyCh <- nil

		// Windows Message Loop with key interception
		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}

			if m.Message == wmKeyDown {
				if w.handleKeyDown(m.WParam) {
					continue
				}
			}

			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}

		w.mu.Lock()
		w.closed = true
		w.visible = false
		w.mu.Unlock()
		close(w.done)
	}()

	if err := <-readyCh; err != nil {
		return nil, err
	}

	return w, nil
}

// handleKeyDown processes global shortcuts while window controls have focus.
func (w *SearchWindow) handleKeyDown(wParam uintptr) bool {
	switch wParam {
	case vkEscape:
		_ = w.Hide()
		return true
	case vkReturn:
		w.submitSearch()
		return true
	case vkTab:
		w.focusNextControl()
		return true
	default:
		return false
	}
}

// submitSearch retrieves search query and selected indexes, hides the window, and invokes onSearch.
func (w *SearchWindow) submitSearch() {
	query := w.GetQuery()
	selected := w.GetSelectedIndexes()
	_ = w.Hide()

	w.mu.RLock()
	cb := w.onSearch
	w.mu.RUnlock()

	if cb != nil {
		go cb(query, selected)
	}
}

// focusNextControl cycles focus between the edit box and checkbox controls.
func (w *SearchWindow) focusNextControl() {
	w.mu.RLock()
	defer w.mu.RUnlock()

	controls := make([]uintptr, 0, 1+len(w.checkHWnds))
	if w.editHWnd != 0 {
		controls = append(controls, w.editHWnd)
	}
	controls = append(controls, w.checkHWnds...)

	if len(controls) == 0 {
		return
	}

	curFocus, _, _ := procGetFocus.Call()
	nextIdx := 0
	for i, c := range controls {
		if c == curFocus {
			nextIdx = (i + 1) % len(controls)
			break
		}
	}

	nextHWnd := controls[nextIdx]
	procSetFocus.Call(nextHWnd)
	if nextHWnd == w.editHWnd {
		procSendMessageW.Call(w.editHWnd, uintptr(emSetSel), 0, ^uintptr(0))
	}
}

// Show makes the search window visible, brings it to foreground, and focuses the edit box.
func (w *SearchWindow) Show() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("search window is closed")
	}

	procShowWindow.Call(w.hwnd, uintptr(swShow))
	procSetForegroundWindow.Call(w.hwnd)
	if w.editHWnd != 0 {
		procSetFocus.Call(w.editHWnd)
		procSendMessageW.Call(w.editHWnd, uintptr(emSetSel), 0, ^uintptr(0))
	}
	w.visible = true
	return nil
}

// Hide hides the search window without destroying its resources.
func (w *SearchWindow) Hide() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	procShowWindow.Call(w.hwnd, uintptr(swHide))
	w.visible = false
	return nil
}

// IsVisible returns true if the search window is currently shown.
func (w *SearchWindow) IsVisible() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.visible
}

// isClosed returns true if the search window is destroyed.
func (w *SearchWindow) isClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed
}

// GetQuery retrieves the text currently entered into the edit control.
func (w *SearchWindow) GetQuery() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.editHWnd == 0 {
		return ""
	}

	lenRet, _, _ := procGetWindowTextLengthW.Call(w.editHWnd)
	length := int(lenRet)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(w.editHWnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return windows.UTF16ToString(buf)
}

// SetQuery sets the text in the edit control and selects all text.
func (w *SearchWindow) SetQuery(q string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.editHWnd == 0 {
		return
	}

	ptr := windows.StringToUTF16Ptr(q)
	procSetWindowTextW.Call(w.editHWnd, uintptr(unsafe.Pointer(ptr)))
	procSendMessageW.Call(w.editHWnd, uintptr(emSetSel), 0, ^uintptr(0))
}

// GetSelectedIndexes returns the list of index IDs whose checkboxes are currently checked.
func (w *SearchWindow) GetSelectedIndexes() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	selected := make([]string, 0, len(w.indexes))
	for i, chkHWnd := range w.checkHWnds {
		if i < len(w.indexes) {
			ret, _, _ := procSendMessageW.Call(chkHWnd, uintptr(bmGetCheck), 0, 0)
			if ret == uintptr(bstChecked) {
				selected = append(selected, w.indexes[i].ID)
			}
		}
	}
	return selected
}

// SetSelectedIndexes updates the checked states of checkboxes matching the given index IDs.
func (w *SearchWindow) SetSelectedIndexes(ids []string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	idMap := make(map[string]bool, len(ids))
	for _, id := range ids {
		idMap[id] = true
	}

	for i, chkHWnd := range w.checkHWnds {
		if i < len(w.indexes) {
			checkVal := bstUnchecked
			if idMap[w.indexes[i].ID] {
				checkVal = bstChecked
			}
			procSendMessageW.Call(chkHWnd, uintptr(bmSetCheck), uintptr(checkVal), 0)
		}
	}
}

// Close destroys the Win32 window and terminates its message loop.
func (w *SearchWindow) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	hwnd := w.hwnd
	threadID := w.threadID
	w.closed = true
	w.visible = false
	w.mu.Unlock()

	if hwnd != 0 {
		procDestroyWindow.Call(hwnd)
	}
	if threadID != 0 {
		procPostMessageW.Call(hwnd, uintptr(wmClose), 0, 0)
		procPostThreadMessageW.Call(uintptr(threadID), uintptr(wmQuit), 0, 0)
	}

	if w.done != nil {
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
		}
	}

	return nil
}
