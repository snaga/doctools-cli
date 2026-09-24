package docsearch

import (
	"sync"
)

// TrayCallbacks defines callback handlers for system tray interactions.
type TrayCallbacks struct {
	OnOpen     func()
	OnSettings func()
	OnExit     func()
}

// TrayIcon represents a system tray notification icon and its context menu.
type TrayIcon struct {
	cbs TrayCallbacks

	mu        sync.Mutex
	running   bool
	iconAdded bool
	hWnd      uintptr
	threadID  uint32
	done      chan struct{}
}

// NewTrayIcon creates a new TrayIcon instance with the provided callbacks.
func NewTrayIcon(cbs TrayCallbacks) *TrayIcon {
	return &TrayIcon{
		cbs: cbs,
	}
}

// IsRunning returns whether the tray icon is currently active.
func (t *TrayIcon) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// Callbacks returns a copy of the configured TrayCallbacks.
func (t *TrayIcon) Callbacks() TrayCallbacks {
	return t.cbs
}

// handleMenuAction dispatches context menu command IDs to corresponding callbacks asynchronously.
func (t *TrayIcon) handleMenuAction(id int) {
	switch id {
	case 1:
		if t.cbs.OnOpen != nil {
			go t.cbs.OnOpen()
		}
	case 2:
		if t.cbs.OnSettings != nil {
			go t.cbs.OnSettings()
		}
	case 3:
		if t.cbs.OnExit != nil {
			go t.cbs.OnExit()
		}
	}
}
