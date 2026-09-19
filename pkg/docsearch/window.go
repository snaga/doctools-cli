package docsearch

import (
	"sync"
)

// SearchCallback is called when the user submits a search from the search window.
type SearchCallback func(query string, selectedIndexes []string)

// SearchWindowController defines the interface for interacting with the desktop search floating window.
type SearchWindowController interface {
	Show() error
	Hide() error
	IsVisible() bool
	GetQuery() string
	SetQuery(q string)
	GetSelectedIndexes() []string
	SetSelectedIndexes(ids []string)
	Close() error
}

// WindowOptions holds configuration for creating the search floating window.
type WindowOptions struct {
	Width       int
	Height      int
	Placeholder string
}

// DefaultWindowOptions returns standard dimensions and texts for the search window.
func DefaultWindowOptions() WindowOptions {
	return WindowOptions{
		Width:       560,
		Height:      110,
		Placeholder: "Search documents...",
	}
}

// Global window reference for singleton management if needed
var (
	windowMu       sync.Mutex
	activeWindow   *SearchWindow
)

// ShowSearchWindow is a helper function to create and show a SearchWindow.
func ShowSearchWindow(indexes []IndexConfig, onSearch SearchCallback) (*SearchWindow, error) {
	windowMu.Lock()
	defer windowMu.Unlock()

	if activeWindow != nil && !activeWindow.isClosed() {
		activeWindow.indexes = indexes
		activeWindow.onSearch = onSearch
		_ = activeWindow.Show()
		return activeWindow, nil
	}

	w, err := NewSearchWindow(indexes, onSearch)
	if err != nil {
		return nil, err
	}
	activeWindow = w
	_ = w.Show()
	return w, nil
}
