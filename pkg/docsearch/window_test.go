package docsearch

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func sampleIndexes(t *testing.T) []IndexConfig {
	t.Helper()
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "rules.bleve")
	projDir := filepath.Join(dir, "project_a.bleve")
	if err := os.Mkdir(rulesDir, 0755); err != nil {
		t.Fatalf("failed to create rules index dir: %v", err)
	}
	if err := os.Mkdir(projDir, 0755); err != nil {
		t.Fatalf("failed to create proj index dir: %v", err)
	}
	return []IndexConfig{
		{ID: "rules", Name: "社内規程", Path: rulesDir, DefaultSelected: true},
		{ID: "project_a", Name: "案件A", Path: projDir, DefaultSelected: false},
	}
}

func TestSearchWindow_Lifecycle(t *testing.T) {
	indexes := sampleIndexes(t)
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
	indexes := sampleIndexes(t)
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
	indexes := sampleIndexes(t)
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
	indexes := sampleIndexes(t)
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
	if opts.Width != 620 || opts.Height != 124 {
		t.Errorf("expected default window options 620x124, got %dx%d", opts.Width, opts.Height)
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

func TestSearchWindow_HealthCheck_MissingIndex(t *testing.T) {
	tmpDir := t.TempDir()
	validDir := filepath.Join(tmpDir, "valid.bleve")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatal(err)
	}

	missingPath := filepath.Join(tmpDir, "non_existent.bleve")

	indexes := []IndexConfig{
		{ID: "valid", Name: "Valid Docs", Path: validDir, DefaultSelected: true},
		{ID: "missing", Name: "Missing Docs", Path: missingPath, DefaultSelected: true},
	}

	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("NewSearchWindow failed: %v", err)
	}
	defer w.Close()

	// Should not have full warning because 1 valid index exists
	if w.HasWarning() {
		t.Errorf("expected HasWarning to be false when 1 valid index exists")
	}

	// Missing index should NOT be in selected indexes even if DefaultSelected was true
	selected := w.GetSelectedIndexes()
	if len(selected) != 1 || selected[0] != "valid" {
		t.Errorf("expected only valid index to be selected, got: %v", selected)
	}

	// Attempting to select missing index should be ignored
	w.SetSelectedIndexes([]string{"valid", "missing"})
	selected = w.GetSelectedIndexes()
	if len(selected) != 1 || selected[0] != "valid" {
		t.Errorf("expected missing index to be excluded after SetSelectedIndexes, got: %v", selected)
	}
}

func TestSearchWindow_HealthCheck_NoValidIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "non_existent.bleve")

	// Test 1: All missing
	indexes := []IndexConfig{
		{ID: "m1", Name: "Missing 1", Path: missingPath, DefaultSelected: true},
	}
	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("NewSearchWindow failed: %v", err)
	}
	defer w.Close()

	if !w.HasWarning() {
		t.Errorf("expected HasWarning true when all indexes are missing")
	}

	if len(w.GetSelectedIndexes()) != 0 {
		t.Errorf("expected 0 selected indexes when all missing")
	}

	// Test 2: Empty indexes
	wEmpty, err := NewSearchWindow([]IndexConfig{}, nil)
	if err != nil {
		t.Fatalf("NewSearchWindow empty failed: %v", err)
	}
	defer wEmpty.Close()

	if !wEmpty.HasWarning() {
		t.Errorf("expected HasWarning true when index slice is empty")
	}
}

func TestSearchWindow_HealthCheck_DynamicUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "non_existent.bleve")

	// Initially all missing
	indexes := []IndexConfig{
		{ID: "m1", Name: "Missing 1", Path: missingPath, DefaultSelected: true},
	}
	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if !w.HasWarning() {
		t.Errorf("expected warning initially")
	}

	// Dynamically create an index directory and update via SetIndexes
	validDir := filepath.Join(tmpDir, "now_valid.bleve")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatal(err)
	}

	newIndexes := []IndexConfig{
		{ID: "m1", Name: "Missing 1", Path: missingPath, DefaultSelected: false},
		{ID: "valid1", Name: "Now Valid", Path: validDir, DefaultSelected: true},
	}

	w.SetIndexes(newIndexes)

	if w.HasWarning() {
		t.Errorf("expected warning to be cleared after adding valid index")
	}

	selected := w.GetSelectedIndexes()
	if len(selected) != 1 || selected[0] != "valid1" {
		t.Errorf("expected 'valid1' to be selected, got: %v", selected)
	}
}

func TestSearchWindow_ModernStyling(t *testing.T) {
	indexes := sampleIndexes(t)
	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("Failed to create SearchWindow: %v", err)
	}

	// 1. Verify custom modern font was generated
	hFont := w.CustomFont()
	if hFont == 0 {
		t.Errorf("expected non-zero customFont handle")
	}

	// 2. Verify window options has default modern placeholder
	if w.options.Placeholder != "🔍 ドキュメントを検索... (Escで閉じる)" {
		t.Errorf("unexpected placeholder: %q", w.options.Placeholder)
	}

	// 3. Close window and verify font cleanup
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if w.CustomFont() != 0 {
		t.Errorf("expected customFont to be reset to 0 after Close")
	}
}

func TestComputeCheckboxLayouts_GuardRail(t *testing.T) {
	opts := DefaultWindowOptions() // Width = 620
	// maxChkX is opts.Width - 16 - 80 - 12 = 512
	// Add button is at opts.Width - 16 - 80 = 524
	expectedMaxX := opts.Width - 16 - 80 - 12
	addBtnX := opts.Width - 16 - 80

	t.Run("Normal indexes fit within layout", func(t *testing.T) {
		indexes := []IndexConfig{
			{ID: "idx1", Name: "社内規程"},
			{ID: "idx2", Name: "開発資料"},
		}
		statuses := []IndexHealthStatus{
			{Index: indexes[0], Exists: true},
			{Index: indexes[1], Exists: true},
		}

		layouts := ComputeCheckboxLayouts(opts.Width, indexes, statuses)
		if len(layouts) != 2 {
			t.Fatalf("expected 2 layouts, got %d", len(layouts))
		}

		for i, l := range layouts {
			if !l.Visible {
				t.Errorf("layout %d should be visible", i)
			}
			if l.Y != 64 {
				t.Errorf("layout %d Y should be 64, got %d", i, l.Y)
			}
			if l.Height != 26 {
				t.Errorf("layout %d Height should be 26, got %d", i, l.Height)
			}
			if l.X+l.Width > expectedMaxX {
				t.Errorf("layout %d right edge (%d) exceeds maxChkX (%d)", i, l.X+l.Width, expectedMaxX)
			}
			if l.X+l.Width >= addBtnX {
				t.Errorf("layout %d right edge (%d) overlaps add button (%d)", i, l.X+l.Width, addBtnX)
			}
		}
	})

	t.Run("Very long index name is clipped to maxChkX", func(t *testing.T) {
		indexes := []IndexConfig{
			{ID: "long1", Name: "これは非常に長い名前を持つインデックスであり確実に右端の追加ボタンと衝突する可能性があるもの"},
		}
		statuses := []IndexHealthStatus{
			{Index: indexes[0], Exists: true},
		}

		layouts := ComputeCheckboxLayouts(opts.Width, indexes, statuses)
		if len(layouts) != 1 {
			t.Fatalf("expected 1 layout, got %d", len(layouts))
		}

		l := layouts[0]
		if !l.Visible {
			t.Errorf("clipped checkbox should still be visible")
		}
		if l.X+l.Width > expectedMaxX {
			t.Errorf("clipped checkbox right edge (%d) exceeds maxChkX (%d)", l.X+l.Width, expectedMaxX)
		}
		if l.X+l.Width >= addBtnX {
			t.Errorf("clipped checkbox overlaps add button (%d)", addBtnX)
		}
		if l.Width != expectedMaxX-l.X {
			t.Errorf("expected clipped width %d, got %d", expectedMaxX-l.X, l.Width)
		}
	})

	t.Run("Many indexes hide overflow beyond maxChkX", func(t *testing.T) {
		var indexes []IndexConfig
		var statuses []IndexHealthStatus
		for i := 0; i < 10; i++ {
			idx := IndexConfig{
				ID:   string(rune('a' + i)),
				Name: "ドキュメント_" + string(rune('A'+i)),
			}
			indexes = append(indexes, idx)
			statuses = append(statuses, IndexHealthStatus{
				Index:  idx,
				Exists: true,
			})
		}

		layouts := ComputeCheckboxLayouts(opts.Width, indexes, statuses)
		visibleCount := 0
		hiddenCount := 0

		for i, l := range layouts {
			if l.Visible {
				visibleCount++
				if l.X+l.Width > expectedMaxX {
					t.Errorf("visible layout %d right edge (%d) exceeds maxChkX (%d)", i, l.X+l.Width, expectedMaxX)
				}
				if l.X+l.Width >= addBtnX {
					t.Errorf("visible layout %d overlaps add button at %d", i, addBtnX)
				}
			} else {
				hiddenCount++
			}
		}

		if visibleCount == 0 {
			t.Errorf("expected at least some checkboxes to be visible")
		}
		if hiddenCount == 0 {
			t.Errorf("expected trailing checkboxes to be hidden")
		}
	})
}

func TestSearchWindow_LongAndManyIndexes_NoOverlap(t *testing.T) {
	dir := t.TempDir()
	var indexes []IndexConfig
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, string(rune('a'+i)))
		_ = os.Mkdir(p, 0755)
		indexes = append(indexes, IndexConfig{
			ID:              string(rune('a' + i)),
			Name:            "長いプロジェクト資料名_テスト_" + string(rune('A'+i)),
			Path:            p,
			DefaultSelected: i == 0,
		})
	}

	w, err := NewSearchWindow(indexes, nil)
	if err != nil {
		t.Fatalf("Failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	if err := w.Show(); err != nil {
		t.Fatalf("Failed to show SearchWindow: %v", err)
	}

	// Verify add button handle is present
	if w.addBtnHWnd == 0 {
		t.Errorf("expected addBtnHWnd to be created")
	}

	// Window should not be in warning state since indexes exist
	if w.HasWarning() {
		t.Errorf("expected HasWarning to be false")
	}
}
