//go:build !windows

package docsearch

import (
	"errors"
)

// Start returns an error on non-Windows platforms as low-level keyboard hooks are Windows-only.
func (h *KeyboardHook) Start() error {
	return errors.New("keyboard hook is only supported on Windows")
}

// Stop is a no-op on non-Windows platforms.
func (h *KeyboardHook) Stop() error {
	return nil
}
