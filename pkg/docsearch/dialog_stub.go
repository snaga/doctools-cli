//go:build !windows

package docsearch

func openIndexDialogImpl(parentHWnd uintptr) (string, bool, error) {
	return "", false, nil
}
