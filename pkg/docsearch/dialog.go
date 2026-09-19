package docsearch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IndexAddedCallback is called when a new index is added dynamically.
type IndexAddedCallback func(newIndex IndexConfig)

// openIndexDialogFn is a hookable function for testing OpenIndexDialog.
var openIndexDialogFn func(parentHWnd uintptr) (string, bool, error)

// OpenIndexDialog displays the native directory selection dialog to choose a Bleve index.
// Returns the selected directory path, a boolean indicating whether the user confirmed, and any error.
func OpenIndexDialog(parentHWnd uintptr) (string, bool, error) {
	if openIndexDialogFn != nil {
		return openIndexDialogFn(parentHWnd)
	}
	return openIndexDialogImpl(parentHWnd)
}

// GenerateIndexConfigFromPath creates an IndexConfig from a given filesystem path.
// It automatically derives a clean Name and ID from the directory name.
func GenerateIndexConfigFromPath(path string) (IndexConfig, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return IndexConfig{}, fmt.Errorf("index path cannot be empty")
	}

	cleanPath := filepath.Clean(trimmed)
	fi, err := os.Stat(cleanPath)
	if err != nil {
		return IndexConfig{}, fmt.Errorf("path does not exist: %w", err)
	}
	if !fi.IsDir() {
		return IndexConfig{}, fmt.Errorf("selected path is not a directory: %s", cleanPath)
	}

	base := filepath.Base(cleanPath)
	name := strings.TrimSuffix(base, ".bleve")
	if name == "" || name == "." {
		name = filepath.Base(filepath.Dir(cleanPath))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "CustomIndex"
	}

	id := name

	return IndexConfig{
		ID:              id,
		Name:            name,
		Path:            cleanPath,
		DefaultSelected: true,
	}, nil
}
