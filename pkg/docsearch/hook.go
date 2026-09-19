package docsearch

import (
	"sync"
	"time"
)

// Virtual key codes for Ctrl keys (Win32 compatible).
const (
	VK_CONTROL  = 0x11
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
)

// DoubleTapDetector detects double-tap of Ctrl keys within a specified interval.
// It is designed to be pure Go and OS-independent for deterministic unit testing.
type DoubleTapDetector struct {
	interval            time.Duration
	onTrigger           func()
	mu                  sync.Mutex
	lastCtrlDownTime    time.Time
	ctrlIsPressed       bool
	waitingForSecondTap bool
}

// NewDoubleTapDetector creates a new detector with the given interval and trigger callback.
// If interval <= 0, the default 400ms is used.
func NewDoubleTapDetector(interval time.Duration, onTrigger func()) *DoubleTapDetector {
	if interval <= 0 {
		interval = 400 * time.Millisecond
	}
	return &DoubleTapDetector{
		interval:  interval,
		onTrigger: onTrigger,
	}
}

// IsCtrlKey returns true if the virtual key code corresponds to a Control key.
func IsCtrlKey(vkCode uint32) bool {
	return vkCode == VK_CONTROL || vkCode == VK_LCONTROL || vkCode == VK_RCONTROL
}

// ProcessKeyEvent processes a single key event with an explicit timestamp.
// Returns true if this event triggered a double-tap callback.
func (d *DoubleTapDetector) ProcessKeyEvent(vkCode uint32, isDown bool, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !IsCtrlKey(vkCode) {
		// Any non-Ctrl key press cancels the double-tap sequence
		if isDown {
			d.waitingForSecondTap = false
		}
		return false
	}

	if isDown {
		if d.ctrlIsPressed {
			// Auto-repeat event while holding Ctrl: ignore
			return false
		}
		d.ctrlIsPressed = true

		if d.waitingForSecondTap && !d.lastCtrlDownTime.IsZero() && now.Sub(d.lastCtrlDownTime) <= d.interval {
			// Double-tap condition met!
			d.waitingForSecondTap = false
			if d.onTrigger != nil {
				go d.onTrigger()
			}
			return true
		}

		// First Ctrl tap or timed out previous tap
		d.lastCtrlDownTime = now
		d.waitingForSecondTap = true
		return false
	}

	// Key release (Up)
	d.ctrlIsPressed = false
	if !d.lastCtrlDownTime.IsZero() && now.Sub(d.lastCtrlDownTime) > d.interval {
		d.waitingForSecondTap = false
	}
	return false
}

// Process processes a key event using the current time.
func (d *DoubleTapDetector) Process(vkCode uint32, isDown bool) bool {
	return d.ProcessKeyEvent(vkCode, isDown, time.Now())
}

// Reset clears the internal state of the detector.
func (d *DoubleTapDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastCtrlDownTime = time.Time{}
	d.ctrlIsPressed = false
	d.waitingForSecondTap = false
}

// Interval returns the configured double-tap interval duration.
func (d *DoubleTapDetector) Interval() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.interval
}

// KeyboardHook manages a low-level keyboard hook to detect Ctrl double-tap shortcuts.
type KeyboardHook struct {
	detector   *DoubleTapDetector
	intervalMs int
	onTrigger  func()

	mu       sync.Mutex
	running  bool
	hookH    uintptr
	threadID uint32
	done     chan struct{}
}

// NewKeyboardHook creates a new KeyboardHook with the specified interval in milliseconds
// and trigger callback. If intervalMs <= 0, the default 400ms is used.
func NewKeyboardHook(intervalMs int, onTrigger func()) *KeyboardHook {
	if intervalMs <= 0 {
		intervalMs = 400
	}
	detector := NewDoubleTapDetector(time.Duration(intervalMs)*time.Millisecond, onTrigger)
	return &KeyboardHook{
		detector:   detector,
		intervalMs: intervalMs,
		onTrigger:  onTrigger,
	}
}

// Detector returns the underlying DoubleTapDetector.
func (h *KeyboardHook) Detector() *DoubleTapDetector {
	return h.detector
}

// IsRunning returns whether the keyboard hook is currently active.
func (h *KeyboardHook) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// StartKeyboardHook creates and starts a KeyboardHook with default 400ms interval.
func StartKeyboardHook(onTrigger func()) (*KeyboardHook, error) {
	hook := NewKeyboardHook(400, onTrigger)
	if err := hook.Start(); err != nil {
		return nil, err
	}
	return hook, nil
}
