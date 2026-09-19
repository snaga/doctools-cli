//go:build !windows

package docsearch

import (
	"fmt"
	"sync"
)

// SearchWindow represents an in-memory stub search window on non-Windows platforms.
type SearchWindow struct {
	mu       sync.RWMutex
	indexes  []IndexConfig
	onSearch SearchCallback
	visible  bool
	closed   bool
	query    string
	selected map[string]bool
}

// NewSearchWindow creates an in-memory stub SearchWindow on non-Windows platforms.
func NewSearchWindow(indexes []IndexConfig, onSearch SearchCallback) (*SearchWindow, error) {
	sel := make(map[string]bool)
	for _, idx := range indexes {
		if idx.DefaultSelected {
			sel[idx.ID] = true
		}
	}

	return &SearchWindow{
		indexes:  indexes,
		onSearch: onSearch,
		selected: sel,
	}, nil
}

func (w *SearchWindow) Show() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("search window is closed")
	}
	w.visible = true
	return nil
}

func (w *SearchWindow) Hide() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.visible = false
	return nil
}

func (w *SearchWindow) IsVisible() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.visible
}

func (w *SearchWindow) isClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed
}

func (w *SearchWindow) GetQuery() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.query
}

func (w *SearchWindow) SetQuery(q string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.query = q
}

func (w *SearchWindow) GetSelectedIndexes() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	res := make([]string, 0, len(w.indexes))
	for _, idx := range w.indexes {
		if w.selected[idx.ID] {
			res = append(res, idx.ID)
		}
	}
	return res
}

func (w *SearchWindow) SetSelectedIndexes(ids []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.selected = make(map[string]bool, len(ids))
	for _, id := range ids {
		w.selected[id] = true
	}
}

func (w *SearchWindow) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	w.visible = false
	return nil
}
