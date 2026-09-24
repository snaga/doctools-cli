package docsearch

import (
	"testing"
)

func TestActivateDocSearchWindow_Mock(t *testing.T) {
	t.Run("returns true when window found", func(t *testing.T) {
		cleanup := SetActivateDocSearchWindowFnForTesting(func() bool {
			return true
		})
		defer cleanup()

		if !ActivateDocSearchWindow() {
			t.Errorf("expected ActivateDocSearchWindow() to return true")
		}
	})

	t.Run("returns false when window not found", func(t *testing.T) {
		cleanup := SetActivateDocSearchWindowFnForTesting(func() bool {
			return false
		})
		defer cleanup()

		if ActivateDocSearchWindow() {
			t.Errorf("expected ActivateDocSearchWindow() to return false")
		}
	})
}

func TestActivateDocSearchWindow_Default(t *testing.T) {
	// Call default implementation (should execute without crash/panic)
	_ = ActivateDocSearchWindow()
}
