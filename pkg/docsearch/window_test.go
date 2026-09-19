package docsearch

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func sampleIndexes() []IndexConfig {
	return []IndexConfig{
		{ID: "rules", Name: "社内規程", Path: "C:/docs/indexes/rules.bleve", DefaultSelected: true},
		{ID: "project_a", Name: "案件A", Path: "C:/docs/indexes/project_a.bleve", DefaultSelected: false},
	}
}

func TestSearchWindow_Lifecycle(t *testing.T) {
	indexes := sampleIndexes()
	var searchCalled bool
	w, err := NewSearchWindow(indexes, func(query string, selectedIndexes []string) {
		searchCalled = true
	})
	if err != nil {
		t.Fatalf("Failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	if w.IsVisible() {
		t.Errorf("Window should initially be invisible")
	}

	// Show window
	if err := w.Show(); err != nil {
		t.Fatalf("Failed to show window: %v", err)
	}
	if !w.IsVisible() {
		t.Errorf("Window should be visible after Show()")
	}

	// Hide window
	if err := w.Hide(); err != nil {
		t.Fatalf("Failed to hide window: %v", err)
	}
	if w.IsVisible() {
		t.Errorf("Window should be invisible after Hide()")
	}

	// Close window
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close window: %v", err)
	}
	if err := w.Show(); err == nil {
		t.Errorf("Expected error showing closed window")
	}
	_ = searchCalled
}

func TestSearchWindow_QueryAndIndexes(t *testing.T) {
	indexes := sampleIndexes()
	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("Failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	// Initial query is empty
	if q := w.GetQuery(); q != "" {
		t.Errorf("Expected empty initial query, got '%s'", q)
	}

	// Set and Get query
	w.SetQuery("有給 申請")
	if q := w.GetQuery(); q != "有給 申請" {
		t.Errorf("Expected query '有給 申請', got '%s'", q)
	}

	// Default selected indexes
	selected := w.GetSelectedIndexes()
	expected := []string{"rules"}
	if !reflect.DeepEqual(selected, expected) {
		t.Errorf("Expected default selected %v, got %v", expected, selected)
	}

	// Update selected indexes
	w.SetSelectedIndexes([]string{"rules", "project_a"})
	selected = w.GetSelectedIndexes()
	expectedBoth := []string{"rules", "project_a"}
	if !reflect.DeepEqual(selected, expectedBoth) {
		t.Errorf("Expected updated selected %v, got %v", expectedBoth, selected)
	}

	// Set to only project_a
	w.SetSelectedIndexes([]string{"project_a"})
	selected = w.GetSelectedIndexes()
	expectedProjectA := []string{"project_a"}
	if !reflect.DeepEqual(selected, expectedProjectA) {
		t.Errorf("Expected updated selected %v, got %v", expectedProjectA, selected)
	}
}

func TestSearchWindow_KeyHandling(t *testing.T) {
	indexes := sampleIndexes()
	var (
		mu           sync.Mutex
		receivedQ    string
		receivedIdxs []string
		calledCh     = make(chan struct{}, 1)
	)

	w, err := NewSearchWindow(indexes, func(query string, selectedIndexes []string) {
		mu.Lock()
		receivedQ = query
		receivedIdxs = selectedIndexes
		mu.Unlock()
		select {
		case calledCh <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("Failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	_ = w.Show()
	w.SetQuery("就業規則")

	// 1. Tab key handling
	handled := w.handleKeyDown(vkTab)
	if !handled {
		t.Errorf("Tab key should be handled")
	}

	// 2. Escape key hides window
	handled = w.handleKeyDown(vkEscape)
	if !handled {
		t.Errorf("Escape key should be handled")
	}
	if w.IsVisible() {
		t.Errorf("Window should be hidden after Escape")
	}

	// Reshow
	_ = w.Show()
	if !w.IsVisible() {
		t.Errorf("Window should be visible after reshow")
	}

	// 3. Return key submits search and hides window
	handled = w.handleKeyDown(vkReturn)
	if !handled {
		t.Errorf("Return key should be handled")
	}
	if w.IsVisible() {
		t.Errorf("Window should be hidden after Return")
	}

	select {
	case <-calledCh:
		mu.Lock()
		defer mu.Unlock()
		if receivedQ != "就業規則" {
			t.Errorf("Expected query '就業規則', got '%s'", receivedQ)
		}
		if !reflect.DeepEqual(receivedIdxs, []string{"rules"}) {
			t.Errorf("Expected indexes ['rules'], got %v", receivedIdxs)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Search callback was not invoked within timeout")
	}

	// Other key not handled
	if w.handleKeyDown(0x41) { // 'A' key
		t.Errorf("Key 'A' should not be handled globally by window controller")
	}
}

func TestShowSearchWindow_Helper(t *testing.T) {
	indexes := sampleIndexes()
	w, err := ShowSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("ShowSearchWindow failed: %v", err)
	}
	defer w.Close()

	if !w.IsVisible() {
		t.Errorf("Expected window to be visible after ShowSearchWindow")
	}

	// Calling again should return active window
	w2, err := ShowSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("Second ShowSearchWindow failed: %v", err)
	}
	if w != w2 {
		t.Errorf("Expected same window instance to be returned")
	}
}

func TestSearchWindow_EdgeCases(t *testing.T) {
	opts := DefaultWindowOptions()
	if opts.Width <= 0 || opts.Height <= 0 {
		t.Errorf("invalid default window options: %v", opts)
	}

	// Empty controls
	wEmpty := &SearchWindow{}
	wEmpty.focusNextControl()
	if q := wEmpty.GetQuery(); q != "" {
		t.Errorf("expected empty query from empty window, got %s", q)
	}
	wEmpty.SetQuery("test") // should safely no-op

	// Closed window operations
	wEmpty.closed = true
	if err := wEmpty.Hide(); err != nil {
		t.Errorf("hide closed window should not error")
	}
	if err := wEmpty.Close(); err != nil {
		t.Errorf("close closed window should not error")
	}
}
