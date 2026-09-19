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
	procEnableWindow          = user32.NewProc("EnableWindow")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")
	procGetStockObject        = gdi32.NewProc("GetStockObject")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procSetTextColor          = gdi32.NewProc("SetTextColor")
	procSetBkMode             = gdi32.NewProc("SetBkMode")
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
	ssLeft        = 0x0000

	bsAutoCheckbox = 0x00000003
	bsPushButton   = 0x00000000
	bmGetCheck     = 0x00F0
	bmSetCheck     = 0x00F1
	bstUnchecked   = 0x0000
	bstChecked     = 0x0001

	swHide   = 0
	swShow   = 5

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	emSetSel = 0x00B1

	wmSetFont        = 0x0030
	wmCommand        = 0x0111
	wmCtlColorStatic = 0x0138
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

	windowMapMu sync.RWMutex
	windowMap   = make(map[uintptr]*SearchWindow)
)

func globalWndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmDestroy:
		return 0
	case wmCommand:
		windowMapMu.RLock()
		w := windowMap[hWnd]
		windowMapMu.RUnlock()
		if w != nil && lParam != 0 && lParam == w.addBtnHWnd {
			go w.handleAddIndexClicked()
			return 0
		}
		ret, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
		return ret
	case wmCtlColorStatic:
		// Amber/Red color for warning static label: RGB(204, 32, 0) -> 0x000020CC
		procSetTextColor.Call(wParam, uintptr(0x000020CC))
		procSetBkMode.Call(wParam, 1) // TRANSPARENT
		bgBrush, _, _ := procGetStockObject.Call(uintptr(colorBtnFace))
		return bgBrush
	default:
		ret, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

// SearchWindow represents the native Win32 floating search window.
type SearchWindow struct {
	mu           sync.RWMutex
	indexes      []IndexConfig
	onSearch     SearchCallback
	onIndexAdded IndexAddedCallback
	configPath   string
	options      WindowOptions
	visible      bool
	closed       bool
	hasWarning   bool

	hwnd        uintptr
	editHWnd    uintptr
	warningHWnd uintptr
	addBtnHWnd  uintptr
	checkHWnds  []uintptr
	threadID    uint32
	done        chan struct{}
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

		windowMapMu.Lock()
		windowMap[hwnd] = w
		windowMapMu.Unlock()

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

		// Create Warning Static Label (shown when no valid indexes exist)
		staticClass := windows.StringToUTF16Ptr("STATIC")
		warningText := windows.StringToUTF16Ptr(MissingIndexWarningText)
		warningHWnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(staticClass)),
			uintptr(unsafe.Pointer(warningText)),
			uintptr(wsChild|ssLeft),
			uintptr(16),
			uintptr(56),
			uintptr(opts.Width-32-78),
			uintptr(24),
			hwnd,
			0,
			hInst,
			0,
		)
		w.warningHWnd = warningHWnd
		if warningHWnd != 0 && hFont != 0 {
			procSendMessageW.Call(warningHWnd, uintptr(wmSetFont), hFont, 1)
		}

		// Create [＋ 追加] Button
		btnClass := windows.StringToUTF16Ptr("BUTTON")
		addBtnText := windows.StringToUTF16Ptr("[＋ 追加]")
		addBtnX := opts.Width - 16 - 74
		addBtnHWnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(btnClass)),
			uintptr(unsafe.Pointer(addBtnText)),
			uintptr(wsChild|wsVisible|wsTabStop|bsPushButton),
			uintptr(addBtnX),
			uintptr(56),
			uintptr(74),
			uintptr(24),
			hwnd,
			0,
			hInst,
			0,
		)
		w.addBtnHWnd = addBtnHWnd
		if addBtnHWnd != 0 && hFont != 0 {
			procSendMessageW.Call(addBtnHWnd, uintptr(wmSetFont), hFont, 1)
		}

		// Pre-create checkbox controls pool on GUI thread
		const maxCheckboxes = 16
		for i := 0; i < maxCheckboxes; i++ {
			chkHWnd, _, _ := procCreateWindowExW.Call(
				0,
				uintptr(unsafe.Pointer(btnClass)),
				0,
				uintptr(wsChild|wsTabStop|bsAutoCheckbox),
				uintptr(16),
				uintptr(56),
				uintptr(80),
				uintptr(24),
				hwnd,
				0,
				hInst,
				0,
			)
			if chkHWnd != 0 {
				if hFont != 0 {
					procSendMessageW.Call(chkHWnd, uintptr(wmSetFont), hFont, 1)
				}
				w.checkHWnds = append(w.checkHWnds, chkHWnd)
			}
		}

		for i, idx := range w.indexes {
			if idx.DefaultSelected && i < len(w.checkHWnds) {
				procSendMessageW.Call(w.checkHWnds[i], uintptr(bmSetCheck), uintptr(bstChecked), 0)
			}
		}

		// Initialize checkboxes and health check status
		w.refreshIndexHealthLocked()

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

	controls := make([]uintptr, 0, 2+len(w.indexes))
	if w.editHWnd != 0 {
		controls = append(controls, w.editHWnd)
	}
	for i := 0; i < len(w.indexes) && i < len(w.checkHWnds); i++ {
		controls = append(controls, w.checkHWnds[i])
	}
	if w.addBtnHWnd != 0 {
		controls = append(controls, w.addBtnHWnd)
	}

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

// refreshIndexHealthLocked checks index health, updates warning and checkbox states.
// Must be called with w.mu held.
func (w *SearchWindow) refreshIndexHealthLocked() {
	statuses := CheckIndexHealth(w.indexes)
	validCount := 0
	for _, s := range statuses {
		if s.Exists {
			validCount++
		}
	}

	w.hasWarning = (validCount == 0)

	if validCount == 0 {
		// Show warning label
		if w.warningHWnd != 0 {
			procShowWindow.Call(w.warningHWnd, uintptr(swShow))
		}
		// Hide all checkboxes
		for _, chkHWnd := range w.checkHWnds {
			procShowWindow.Call(chkHWnd, uintptr(swHide))
		}
		return
	}

	// Hide warning label when valid indexes exist
	if w.warningHWnd != 0 {
		procShowWindow.Call(w.warningHWnd, uintptr(swHide))
	}

	chkX := 16
	chkY := 56
	chkHeight := 24

	// Update each checkbox from pre-created pool
	for i := 0; i < len(w.checkHWnds); i++ {
		chkHWnd := w.checkHWnds[i]
		if i < len(w.indexes) {
			s := statuses[i]
			if s.Exists {
				namePtr := windows.StringToUTF16Ptr(s.Index.Name)
				procSetWindowTextW.Call(chkHWnd, uintptr(unsafe.Pointer(namePtr)))
				procEnableWindow.Call(chkHWnd, 1)
				chkWidth := len([]rune(s.Index.Name))*16 + 32
				if chkWidth < 80 {
					chkWidth = 80
				}
				procSetWindowPos.Call(chkHWnd, 0, uintptr(chkX), uintptr(chkY), uintptr(chkWidth), uintptr(chkHeight), uintptr(swpNoZOrder|swpNoActivate))
				procShowWindow.Call(chkHWnd, uintptr(swShow))
				chkX += chkWidth + 8
			} else {
				label := FormatMissingIndexLabel(s.Index.Name)
				labelPtr := windows.StringToUTF16Ptr(label)
				procSetWindowTextW.Call(chkHWnd, uintptr(unsafe.Pointer(labelPtr)))
				procSendMessageW.Call(chkHWnd, uintptr(bmSetCheck), uintptr(bstUnchecked), 0)
				procEnableWindow.Call(chkHWnd, 0)
				chkWidth := len([]rune(label))*14 + 32
				if chkWidth < 120 {
					chkWidth = 120
				}
				procSetWindowPos.Call(chkHWnd, 0, uintptr(chkX), uintptr(chkY), uintptr(chkWidth), uintptr(chkHeight), uintptr(swpNoZOrder|swpNoActivate))
				procShowWindow.Call(chkHWnd, uintptr(swShow))
				chkX += chkWidth + 8
			}
		} else {
			// Hide unused checkboxes in pool
			procShowWindow.Call(chkHWnd, uintptr(swHide))
		}
	}
}

// Show makes the search window visible, brings it to foreground, and focuses the edit box.
func (w *SearchWindow) Show() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("search window is closed")
	}

	w.refreshIndexHealthLocked()

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

// HasWarning returns true if there are no valid indexes available.
func (w *SearchWindow) HasWarning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.hasWarning
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

// GetSelectedIndexes returns the list of index IDs whose checkboxes are currently checked and healthy.
func (w *SearchWindow) GetSelectedIndexes() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	statuses := CheckIndexHealth(w.indexes)

	selected := make([]string, 0, len(w.indexes))
	for i, chkHWnd := range w.checkHWnds {
		if i < len(w.indexes) {
			if !statuses[i].Exists {
				continue
			}
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

	statuses := CheckIndexHealth(w.indexes)

	for i, chkHWnd := range w.checkHWnds {
		if i < len(w.indexes) {
			checkVal := bstUnchecked
			if statuses[i].Exists && idMap[w.indexes[i].ID] {
				checkVal = bstChecked
			}
			procSendMessageW.Call(chkHWnd, uintptr(bmSetCheck), uintptr(checkVal), 0)
		}
	}
}

// SetIndexes updates the configured search indexes and refreshes health status.
func (w *SearchWindow) SetIndexes(indexes []IndexConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.indexes = indexes
	w.refreshIndexHealthLocked()
	for i, idx := range w.indexes {
		if idx.DefaultSelected && i < len(w.checkHWnds) {
			procSendMessageW.Call(w.checkHWnds[i], uintptr(bmSetCheck), uintptr(bstChecked), 0)
		}
	}
}

// handleAddIndexClicked is triggered when the [＋ 追加] button is clicked.
func (w *SearchWindow) handleAddIndexClicked() {
	w.mu.RLock()
	hwnd := w.hwnd
	w.mu.RUnlock()

	path, ok, err := OpenIndexDialog(hwnd)
	if err != nil || !ok || path == "" {
		return
	}

	newIdx, err := GenerateIndexConfigFromPath(path)
	if err != nil {
		return
	}

	w.AddIndex(newIdx)
}

// AddIndex adds a new index dynamically, updating selection, health, and persistence.
func (w *SearchWindow) AddIndex(newIdx IndexConfig) {
	w.mu.Lock()
	for _, idx := range w.indexes {
		if idx.Path == newIdx.Path || idx.ID == newIdx.ID {
			w.mu.Unlock()
			return
		}
	}
	w.indexes = append(w.indexes, newIdx)
	configPath := w.configPath
	onAdded := w.onIndexAdded
	w.refreshIndexHealthLocked()

	// By default, check newly added index
	for i, idx := range w.indexes {
		if idx.ID == newIdx.ID && i < len(w.checkHWnds) {
			procSendMessageW.Call(w.checkHWnds[i], uintptr(bmSetCheck), uintptr(bstChecked), 0)
			break
		}
	}
	w.mu.Unlock()

	if configPath != "" {
		if cfg, err := LoadConfig(configPath); err == nil {
			alreadyInCfg := false
			for _, idx := range cfg.Indexes {
				if idx.Path == newIdx.Path || idx.ID == newIdx.ID {
					alreadyInCfg = true
					break
				}
			}
			if !alreadyInCfg {
				cfg.Indexes = append(cfg.Indexes, newIdx)
				_ = SaveConfig(configPath, cfg)
			}
		}
	}

	if onAdded != nil {
		go onAdded(newIdx)
	}
}

// SetOnIndexAdded registers a callback to be invoked when a new index is added.
func (w *SearchWindow) SetOnIndexAdded(cb IndexAddedCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onIndexAdded = cb
}

// SetConfigPath sets the config file path for automatic persistence.
func (w *SearchWindow) SetConfigPath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.configPath = path
}

// TriggerAddIndex simulates the user clicking the add index button and completing the dialog.
func (w *SearchWindow) TriggerAddIndex() (string, bool, error) {
	w.mu.RLock()
	hwnd := w.hwnd
	w.mu.RUnlock()

	path, ok, err := OpenIndexDialog(hwnd)
	if err != nil || !ok || path == "" {
		return "", ok, err
	}
	newIdx, err := GenerateIndexConfigFromPath(path)
	if err != nil {
		return "", false, err
	}
	w.AddIndex(newIdx)
	return path, true, nil
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
		windowMapMu.Lock()
		delete(windowMap, hwnd)
		windowMapMu.Unlock()

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
