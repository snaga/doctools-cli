package docsearch

var (
	activateDocSearchWindowFn = activateDocSearchWindowImpl
)

// ActivateDocSearchWindow searches for an existing visible window with "DocSearch"
// in its title, restores and brings it to the foreground, and returns true.
// If no such window is found, it returns false.
func ActivateDocSearchWindow() bool {
	if activateDocSearchWindowFn != nil {
		return activateDocSearchWindowFn()
	}
	return false
}

// SetActivateDocSearchWindowFnForTesting allows tests to mock window activation.
// It returns a cleanup function that restores the previous implementation.
func SetActivateDocSearchWindowFnForTesting(fn func() bool) func() {
	prev := activateDocSearchWindowFn
	activateDocSearchWindowFn = fn
	return func() {
		activateDocSearchWindowFn = prev
	}
}
