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
	HasWarning() bool
	GetQuery() string
	SetQuery(q string)
	GetSelectedIndexes() []string
	SetSelectedIndexes(ids []string)
	SetIndexes(indexes []IndexConfig)
	AddIndex(newIdx IndexConfig)
	SetOnIndexAdded(cb IndexAddedCallback)
	SetConfigPath(path string)
	CustomFont() uintptr
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
		Width:       620,
		Height:      124,
		Placeholder: "🔍 ドキュメントを検索... (Escで閉じる)",
	}
}

// CheckboxLayout contains layout positioning information for an index checkbox control.
type CheckboxLayout struct {
	Index   int
	X       int
	Y       int
	Width   int
	Height  int
	Visible bool
	Label   string
	Enabled bool
}

// ComputeCheckboxLayouts calculates positions and bounds for search index checkboxes
// ensuring they do not overlap with the Add button (guard-rail protection).
func ComputeCheckboxLayouts(windowWidth int, indexes []IndexConfig, statuses []IndexHealthStatus) []CheckboxLayout {
	layouts := make([]CheckboxLayout, len(indexes))
	chkX := 16
	chkY := 64
	chkHeight := 26
	maxChkX := windowWidth - 16 - 80 - 12

	for i := range indexes {
		s := statuses[i]
		var label string
		var chkWidth int
		if s.Exists {
			label = s.Index.Name
			chkWidth = len([]rune(label))*16 + 32
			if chkWidth < 80 {
				chkWidth = 80
			}
		} else {
			label = FormatMissingIndexLabel(s.Index.Name)
			chkWidth = len([]rune(label))*14 + 32
			if chkWidth < 120 {
				chkWidth = 120
			}
		}

		if chkX+chkWidth > maxChkX {
			if chkX < maxChkX-40 {
				chkWidth = maxChkX - chkX
			} else {
				layouts[i] = CheckboxLayout{
					Index:   i,
					Visible: false,
					Label:   label,
					Enabled: s.Exists,
				}
				continue
			}
		}

		layouts[i] = CheckboxLayout{
			Index:   i,
			X:       chkX,
			Y:       chkY,
			Width:   chkWidth,
			Height:  chkHeight,
			Visible: true,
			Label:   label,
			Enabled: s.Exists,
		}
		chkX += chkWidth + 8
	}
	return layouts
}

// Global window reference for singleton management if needed
var (
	windowMu     sync.Mutex
	activeWindow *SearchWindow
)

// ShowSearchWindow is a helper function to create and show a SearchWindow.
func ShowSearchWindow(indexes []IndexConfig, onSearch SearchCallback) (*SearchWindow, error) {
	windowMu.Lock()
	defer windowMu.Unlock()

	if activeWindow != nil && !activeWindow.isClosed() {
		activeWindow.SetIndexes(indexes)
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
