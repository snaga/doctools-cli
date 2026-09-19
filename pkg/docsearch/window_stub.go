//go:build !windows

package docsearch

import (
	"fmt"
	"sync"
)

// SearchWindow represents an in-memory stub search window on non-Windows platforms.
type SearchWindow struct {
	mu           sync.RWMutex
	indexes      []IndexConfig
	onSearch     SearchCallback
	onIndexAdded IndexAddedCallback
	configPath   string
	visible      bool
	closed       bool
	hasWarning   bool
	query        string
	selected     map[string]bool
}

// NewSearchWindow creates an in-memory stub SearchWindow on non-Windows platforms.
func NewSearchWindow(indexes []IndexConfig, onSearch SearchCallback) (*SearchWindow, error) {
	sel := make(map[string]bool)
	for _, idx := range indexes {
		if idx.DefaultSelected {
			sel[idx.ID] = true
		}
	}

	w := &SearchWindow{
		indexes:  indexes,
		onSearch: onSearch,
		selected: sel,
	}
	w.refreshIndexHealthLocked()
	return w, nil
}

func (w *SearchWindow) refreshIndexHealthLocked() {
	statuses := CheckIndexHealth(w.indexes)
	validCount := 0
	for _, s := range statuses {
		if s.Exists {
			validCount++
		} else {
			delete(w.selected, s.Index.ID)
		}
	}
	w.hasWarning = (validCount == 0)
}

func (w *SearchWindow) Show() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("search window is closed")
	}
	w.refreshIndexHealthLocked()
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

func (w *SearchWindow) HasWarning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.hasWarning
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
	statuses := CheckIndexHealth(w.indexes)
	res := make([]string, 0, len(w.indexes))
	for i, idx := range w.indexes {
		if i < len(statuses) && !statuses[i].Exists {
			continue
		}
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
	w.refreshIndexHealthLocked()
}

func (w *SearchWindow) SetIndexes(indexes []IndexConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.indexes = indexes
	w.selected = make(map[string]bool)
	for _, idx := range indexes {
		if idx.DefaultSelected {
			w.selected[idx.ID] = true
		}
	}
	w.refreshIndexHealthLocked()
}

// AddIndex adds a new index dynamically, updating selection, health, and persistence.
func (w *SearchWindow) AddIndex(newIdx IndexConfig) {
	w.mu.Lock()
	for _, idx := range w.indexes {
		if idx.Path == newIdx.Path || idx.ID == newIdx.ID {
			w.mu.Unlock()
			return
		}
	}
	w.indexes = append(w.indexes, newIdx)
	if newIdx.DefaultSelected {
		if w.selected == nil {
			w.selected = make(map[string]bool)
		}
		w.selected[newIdx.ID] = true
	}
	w.refreshIndexHealthLocked()
	configPath := w.configPath
	onAdded := w.onIndexAdded
	w.mu.Unlock()

	if configPath != "" {
		if cfg, err := LoadConfig(configPath); err == nil {
			alreadyInCfg := false
			for _, idx := range cfg.Indexes {
				if idx.Path == newIdx.Path || idx.ID == newIdx.ID {
					alreadyInCfg = true
					break
				}
			}
			if !alreadyInCfg {
				cfg.Indexes = append(cfg.Indexes, newIdx)
				_ = SaveConfig(configPath, cfg)
			}
		}
	}

	if onAdded != nil {
		go onAdded(newIdx)
	}
}

// SetOnIndexAdded registers a callback to be invoked when a new index is added.
func (w *SearchWindow) SetOnIndexAdded(cb IndexAddedCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onIndexAdded = cb
}

// SetConfigPath sets the config file path for automatic persistence.
func (w *SearchWindow) SetConfigPath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.configPath = path
}

// TriggerAddIndex simulates user clicking the add index button.
func (w *SearchWindow) TriggerAddIndex() (string, bool, error) {
	path, ok, err := OpenIndexDialog(0)
	if err != nil || !ok || path == "" {
		return "", ok, err
	}
	newIdx, err := GenerateIndexConfigFromPath(path)
	if err != nil {
		return "", false, err
	}
	w.AddIndex(newIdx)
	return path, true, nil
}

func (w *SearchWindow) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	w.visible = false
	return nil
}
