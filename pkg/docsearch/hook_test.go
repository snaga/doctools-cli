package docsearch

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoubleTapDetector_NormalDoubleTap(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Tap 1: Press Ctrl
	triggered := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	if triggered {
		t.Errorf("First tap should not trigger double-tap")
	}

	// Release Ctrl
	triggered = detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(50*time.Millisecond))
	if triggered {
		t.Errorf("Ctrl release should not trigger double-tap")
	}

	// Tap 2: Press Ctrl after 200ms (within 400ms interval)
	triggered = detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(200*time.Millisecond))
	if !triggered {
		t.Errorf("Second tap within interval should trigger double-tap")
	}

	// Release Ctrl
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(250*time.Millisecond))

	// Allow goroutine callback to execute
	time.Sleep(50 * time.Millisecond)
	if count := atomic.LoadInt32(&triggerCount); count != 1 {
		t.Errorf("Expected trigger count 1, got %d", count)
	}
}

func TestDoubleTapDetector_Timeout(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Tap 1: Press and release Ctrl
	detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(50*time.Millisecond))

	// Tap 2: Press Ctrl after 500ms (> 400ms interval)
	triggered := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(500*time.Millisecond))
	if triggered {
		t.Errorf("Tap after timeout should not trigger double-tap")
	}
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(550*time.Millisecond))

	// Tap 3: Press Ctrl after 700ms (200ms after Tap 2 - should trigger as Tap 2 was the new first tap)
	triggered = detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(700*time.Millisecond))
	if !triggered {
		t.Errorf("Tap 3 within 200ms of Tap 2 should trigger double-tap")
	}

	time.Sleep(50 * time.Millisecond)
	if count := atomic.LoadInt32(&triggerCount); count != 1 {
		t.Errorf("Expected trigger count 1, got %d", count)
	}
}

func TestDoubleTapDetector_KeyRepeatIgnored(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Press Ctrl (initial down)
	triggered := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	if triggered {
		t.Errorf("Initial press should not trigger")
	}

	// Holding Ctrl generates auto-repeat down events
	triggered = detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(100*time.Millisecond))
	if triggered {
		t.Errorf("Auto-repeat press while holding should be ignored")
	}
	triggered = detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(200*time.Millisecond))
	if triggered {
		t.Errorf("Auto-repeat press while holding should be ignored")
	}

	// Release Ctrl after long press
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(600*time.Millisecond))

	time.Sleep(50 * time.Millisecond)
	if count := atomic.LoadInt32(&triggerCount); count != 0 {
		t.Errorf("Holding Ctrl should not trigger, got count %d", count)
	}
}

func TestDoubleTapDetector_InterruptedByOtherKey(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Tap 1: Press Ctrl
	detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)

	// User presses 'C' key (0x43) while holding Ctrl (e.g. Ctrl+C)
	const vkKeyC = 0x43
	interrupted := detector.ProcessKeyEvent(vkKeyC, true, baseTime.Add(50*time.Millisecond))
	if interrupted {
		t.Errorf("Non-Ctrl key press should not trigger")
	}
	detector.ProcessKeyEvent(vkKeyC, false, baseTime.Add(100*time.Millisecond))
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(150*time.Millisecond))

	// User presses Ctrl again after 250ms (within 400ms of baseTime, but interrupted by 'C')
	triggered := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(250*time.Millisecond))
	if triggered {
		t.Errorf("Ctrl press following an interrupted sequence should not trigger double-tap")
	}

	// Non-ctrl key release does not disrupt
	detector.ProcessKeyEvent(vkKeyC, false, baseTime.Add(300*time.Millisecond))

	time.Sleep(50 * time.Millisecond)
	if count := atomic.LoadInt32(&triggerCount); count != 0 {
		t.Errorf("Interrupted sequence should not trigger, got count %d", count)
	}

	// Nil callback test
	detectorNil := NewDoubleTapDetector(400*time.Millisecond, nil)
	detectorNil.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	detectorNil.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(50*time.Millisecond))
	triggered = detectorNil.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(100*time.Millisecond))
	if !triggered {
		t.Errorf("Nil callback detector should still return true on double-tap")
	}
}

func TestDoubleTapDetector_LeftAndRightCtrl(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Tap 1: Left Ctrl
	detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(40*time.Millisecond))

	// Tap 2: Right Ctrl within interval
	triggered := detector.ProcessKeyEvent(VK_RCONTROL, true, baseTime.Add(150*time.Millisecond))
	if !triggered {
		t.Errorf("Left Ctrl followed by Right Ctrl within interval should trigger")
	}
	detector.ProcessKeyEvent(VK_RCONTROL, false, baseTime.Add(200*time.Millisecond))

	// Generic VK_CONTROL test
	detector.Reset()
	detector.ProcessKeyEvent(VK_CONTROL, true, baseTime.Add(500*time.Millisecond))
	detector.ProcessKeyEvent(VK_CONTROL, false, baseTime.Add(550*time.Millisecond))
	triggered = detector.ProcessKeyEvent(VK_CONTROL, true, baseTime.Add(650*time.Millisecond))
	if !triggered {
		t.Errorf("Generic VK_CONTROL double-tap should trigger")
	}
}

func TestDoubleTapDetector_ConsecutiveTaps(t *testing.T) {
	var triggerCount int32
	detector := NewDoubleTapDetector(400*time.Millisecond, func() {
		atomic.AddInt32(&triggerCount, 1)
	})

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Tap 1
	detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(30*time.Millisecond))

	// Tap 2: triggers double-tap
	t2 := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(150*time.Millisecond))
	if !t2 {
		t.Errorf("Tap 2 should trigger")
	}
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(180*time.Millisecond))

	// Tap 3: immediate 3rd tap at 250ms (100ms after Tap 2)
	// Should NOT trigger because Tap 2 consumed the sequence, Tap 3 is a new first tap
	t3 := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(250*time.Millisecond))
	if t3 {
		t.Errorf("Tap 3 should not trigger immediately after Tap 2")
	}
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(280*time.Millisecond))

	// Tap 4: at 380ms (130ms after Tap 3) -> should trigger as Tap 3 + Tap 4
	t4 := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(380*time.Millisecond))
	if !t4 {
		t.Errorf("Tap 4 should trigger with Tap 3")
	}
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(410*time.Millisecond))

	time.Sleep(50 * time.Millisecond)
	if count := atomic.LoadInt32(&triggerCount); count != 2 {
		t.Errorf("Expected 2 total triggers for 4 rapid taps, got %d", count)
	}
}

func TestDoubleTapDetector_Reset(t *testing.T) {
	detector := NewDoubleTapDetector(400*time.Millisecond, nil)
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime)
	detector.ProcessKeyEvent(VK_LCONTROL, false, baseTime.Add(50*time.Millisecond))

	detector.Reset()

	// Tap after reset within 400ms should NOT trigger because state was reset
	triggered := detector.ProcessKeyEvent(VK_LCONTROL, true, baseTime.Add(100*time.Millisecond))
	if triggered {
		t.Errorf("Press after Reset should not trigger double-tap")
	}
}

func TestDoubleTapDetector_DefaultInterval(t *testing.T) {
	d := NewDoubleTapDetector(0, nil)
	if d.Interval() != 400*time.Millisecond {
		t.Errorf("Expected default interval 400ms, got %v", d.Interval())
	}

	d2 := NewDoubleTapDetector(-100*time.Millisecond, nil)
	if d2.Interval() != 400*time.Millisecond {
		t.Errorf("Expected default interval 400ms for negative value, got %v", d2.Interval())
	}
}

func TestKeyboardHook_Lifecycle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("KeyboardHook native lifecycle test requires Windows")
	}

	var triggered int32
	hook := NewKeyboardHook(400, func() {
		atomic.AddInt32(&triggered, 1)
	})

	if hook.IsRunning() {
		t.Errorf("Hook should not be running before Start")
	}

	// Start hook
	err := hook.Start()
	if err != nil {
		t.Fatalf("Failed to start KeyboardHook: %v", err)
	}

	if !hook.IsRunning() {
		t.Errorf("Hook should be running after Start")
	}

	// Duplicate Start should return error
	if err := hook.Start(); err == nil {
		t.Errorf("Duplicate Start should return error, got nil")
	}

	// Verify detector is accessible and can process events directly
	detector := hook.Detector()
	if detector == nil {
		t.Fatalf("Detector() returned nil")
	}
	detector.ProcessKeyEvent(VK_LCONTROL, true, time.Now())
	detector.ProcessKeyEvent(VK_LCONTROL, false, time.Now().Add(20*time.Millisecond))
	detector.ProcessKeyEvent(VK_LCONTROL, true, time.Now().Add(100*time.Millisecond))
	detector.ProcessKeyEvent(VK_LCONTROL, false, time.Now().Add(120*time.Millisecond))

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&triggered) == 0 {
		t.Errorf("Expected detector trigger via hook")
	}

	// Stop hook
	err = hook.Stop()
	if err != nil {
		t.Fatalf("Failed to stop KeyboardHook: %v", err)
	}

	if hook.IsRunning() {
		t.Errorf("Hook should not be running after Stop")
	}

	// Duplicate Stop should be safe
	if err := hook.Stop(); err != nil {
		t.Errorf("Duplicate Stop should succeed without error, got %v", err)
	}
}

func TestStartKeyboardHook_Helper(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("StartKeyboardHook test requires Windows")
	}

	hook, err := StartKeyboardHook(func() {})
	if err != nil {
		t.Fatalf("StartKeyboardHook failed: %v", err)
	}

	if !hook.IsRunning() {
		t.Errorf("Hook should be running")
	}

	err = hook.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestKeyboardHook_NewDefaultInterval(t *testing.T) {
	hook := NewKeyboardHook(0, nil)
	if hook.Detector().Interval() != 400*time.Millisecond {
		t.Errorf("Expected 400ms default interval, got %v", hook.Detector().Interval())
	}
}

func TestHookCallback_Simulation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("hookCallback simulation requires Windows")
	}

	var triggered int32
	hook := NewKeyboardHook(400, func() {
		atomic.AddInt32(&triggered, 1)
	})

	activeHookMu.Lock()
	primaryHook = hook
	activeHookMu.Unlock()
	defer func() {
		activeHookMu.Lock()
		primaryHook = nil
		activeHookMu.Unlock()
	}()

	kbd := kbdLLHookStruct{
		VkCode: VK_LCONTROL,
	}

	// 1st press
	processHookEvent(wmKeyDown, &kbd)
	// 1st release
	processHookEvent(wmKeyUp, &kbd)
	// 2nd press
	processHookEvent(wmKeyDown, &kbd)
	// 2nd release
	processHookEvent(wmKeyUp, &kbd)

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&triggered) == 0 {
		t.Errorf("Expected processHookEvent simulation to trigger callback")
	}

	// wParam not keydown/keyup should bypass
	processHookEvent(0x0200, &kbd)

	// kbd == nil should safely bypass
	processHookEvent(wmKeyDown, nil)

	// primaryHook == nil should safely bypass
	activeHookMu.Lock()
	primaryHook = nil
	activeHookMu.Unlock()
	processHookEvent(wmKeyDown, &kbd)

	// Process method test
	detector := hook.Detector()
	detector.Reset()
	detector.Process(VK_LCONTROL, true)
	detector.Process(VK_LCONTROL, false)
	detector.Process(VK_LCONTROL, true)
	time.Sleep(50 * time.Millisecond)
}

