//go:build !windows

package docsearch

// Start is a stub for non-Windows platforms.
func (t *TrayIcon) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = true
	return nil
}

// Stop is a stub for non-Windows platforms.
func (t *TrayIcon) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	return nil
}

// handleTrayEvent is a stub for non-Windows platforms.
func (t *TrayIcon) handleTrayEvent(event uint32) {
	switch event {
	case 0x0202, 0x0203: // WM_LBUTTONUP, WM_LBUTTONDBLCLK
		if t.cbs.OnOpen != nil {
			go t.cbs.OnOpen()
		}
	}
}
