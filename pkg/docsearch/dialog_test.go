package docsearch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestGenerateIndexConfigFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid bleve directory", func(t *testing.T) {
		bleveDir := filepath.Join(tmpDir, "reports.bleve")
		if err := os.Mkdir(bleveDir, 0755); err != nil {
			t.Fatal(err)
		}

		cfg, err := GenerateIndexConfigFromPath(bleveDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "reports" {
			t.Errorf("expected ID 'reports', got %q", cfg.ID)
		}
		if cfg.Name != "reports" {
			t.Errorf("expected Name 'reports', got %q", cfg.Name)
		}
		if cfg.Path != filepath.Clean(bleveDir) {
			t.Errorf("expected Path %q, got %q", filepath.Clean(bleveDir), cfg.Path)
		}
		if !cfg.DefaultSelected {
			t.Errorf("expected DefaultSelected true")
		}
	})

	t.Run("valid directory without bleve extension", func(t *testing.T) {
		plainDir := filepath.Join(tmpDir, "CompanyDocs")
		if err := os.Mkdir(plainDir, 0755); err != nil {
			t.Fatal(err)
		}

		cfg, err := GenerateIndexConfigFromPath(plainDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "CompanyDocs" {
			t.Errorf("expected ID 'CompanyDocs', got %q", cfg.ID)
		}
		if cfg.Name != "CompanyDocs" {
			t.Errorf("expected Name 'CompanyDocs', got %q", cfg.Name)
		}
	})

	t.Run("directory named .bleve fallback", func(t *testing.T) {
		dotBleveDir := filepath.Join(tmpDir, "parent_dir", ".bleve")
		if err := os.MkdirAll(dotBleveDir, 0755); err != nil {
			t.Fatal(err)
		}

		cfg, err := GenerateIndexConfigFromPath(dotBleveDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Name != "parent_dir" {
			t.Errorf("expected fallback name 'parent_dir', got %q", cfg.Name)
		}
	})

	t.Run("non-existent path fails", func(t *testing.T) {
		missingPath := filepath.Join(tmpDir, "does_not_exist.bleve")
		_, err := GenerateIndexConfigFromPath(missingPath)
		if err == nil {
			t.Fatalf("expected error for non-existent path")
		}
	})

	t.Run("regular file fails", func(t *testing.T) {
		regularFile := filepath.Join(tmpDir, "file.txt")
		_ = os.WriteFile(regularFile, []byte("content"), 0644)
		_, err := GenerateIndexConfigFromPath(regularFile)
		if err == nil {
			t.Fatalf("expected error for regular file")
		}
	})

	t.Run("empty path fails", func(t *testing.T) {
		_, err := GenerateIndexConfigFromPath("")
		if err == nil {
			t.Fatalf("expected error for empty path")
		}
	})
}

func TestOpenIndexDialog_Mocked(t *testing.T) {
	oldFn := openIndexDialogFn
	defer func() { openIndexDialogFn = oldFn }()

	t.Run("user confirms dialog", func(t *testing.T) {
		openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
			return "C:/indexes/sample.bleve", true, nil
		}
		path, ok, err := OpenIndexDialog(0)
		if err != nil || !ok || path != "C:/indexes/sample.bleve" {
			t.Errorf("unexpected dialog result: path=%q, ok=%v, err=%v", path, ok, err)
		}
	})

	t.Run("user cancels dialog", func(t *testing.T) {
		openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
			return "", false, nil
		}
		path, ok, err := OpenIndexDialog(0)
		if err != nil || ok || path != "" {
			t.Errorf("unexpected dialog cancel result: path=%q, ok=%v, err=%v", path, ok, err)
		}
	})

	t.Run("dialog returns error", func(t *testing.T) {
		openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
			return "", false, errors.New("dialog failed")
		}
		_, ok, err := OpenIndexDialog(0)
		if err == nil || ok {
			t.Errorf("expected error from dialog, got err=%v, ok=%v", err, ok)
		}
	})
}

func TestOpenIndexDialogImpl_WindowsHooks(t *testing.T) {
	oldSHBrowse := callSHBrowseForFolderW
	oldSHGetPath := callSHGetPathFromIDListW
	oldCoTaskMemFree := callCoTaskMemFree
	oldDialogFn := openIndexDialogFn
	defer func() {
		callSHBrowseForFolderW = oldSHBrowse
		callSHGetPathFromIDListW = oldSHGetPath
		callCoTaskMemFree = oldCoTaskMemFree
		openIndexDialogFn = oldDialogFn
	}()

	openIndexDialogFn = nil // Test actual openIndexDialogImpl path

	t.Run("user cancels (pidl == 0)", func(t *testing.T) {
		callSHBrowseForFolderW = func(bi *browseInfoW) uintptr {
			return 0
		}
		path, ok, err := OpenIndexDialog(0)
		if err != nil || ok || path != "" {
			t.Errorf("expected cancel, got path=%q, ok=%v, err=%v", path, ok, err)
		}
	})

	t.Run("user confirms and path retrieved", func(t *testing.T) {
		memFreed := false
		callSHBrowseForFolderW = func(bi *browseInfoW) uintptr {
			return 0x1234
		}
		callSHGetPathFromIDListW = func(pidl uintptr, pszPath *uint16) uint32 {
			copy((*[1024]uint16)(unsafe.Pointer(pszPath))[:], windows.StringToUTF16("C:\\MyDocs\\idx.bleve"))
			return 1
		}
		callCoTaskMemFree = func(pv uintptr) {
			memFreed = true
		}

		path, ok, err := OpenIndexDialog(0)
		if err != nil || !ok || path != "C:\\MyDocs\\idx.bleve" {
			t.Errorf("unexpected result: path=%q, ok=%v, err=%v", path, ok, err)
		}
		if !memFreed {
			t.Errorf("expected CoTaskMemFree to be called")
		}
	})

	t.Run("path retrieval fails (ret == 0)", func(t *testing.T) {
		callSHBrowseForFolderW = func(bi *browseInfoW) uintptr {
			return 0x1234
		}
		callSHGetPathFromIDListW = func(pidl uintptr, pszPath *uint16) uint32 {
			return 0
		}
		callCoTaskMemFree = func(pv uintptr) {}

		_, ok, err := OpenIndexDialog(0)
		if err == nil || ok {
			t.Errorf("expected error from SHGetPathFromIDListW failure, got ok=%v, err=%v", ok, err)
		}
	})
}

func TestSearchWindow_AddIndex_DynamicAndPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	initialDir := filepath.Join(tmpDir, "initial.bleve")
	_ = os.Mkdir(initialDir, 0755)

	cfgPath := filepath.Join(tmpDir, "docsearch.json")
	initialCfg := &Config{
		Server: ServerConfig{Port: 18080, Host: "127.0.0.1"},
		Hotkey: HotkeyConfig{Enabled: true, IntervalMS: 400},
		Indexes: []IndexConfig{
			{ID: "initial", Name: "Initial", Path: initialDir, DefaultSelected: true},
		},
	}
	if err := SaveConfig(cfgPath, initialCfg); err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	w, err := NewSearchWindow(initialCfg.Indexes, nil)
	if err != nil {
		t.Fatalf("failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	w.SetConfigPath(cfgPath)

	addedCh := make(chan IndexConfig, 1)
	w.SetOnIndexAdded(func(newIdx IndexConfig) {
		addedCh <- newIdx
	})

	// Add dynamic index
	newDir := filepath.Join(tmpDir, "manuals.bleve")
	_ = os.Mkdir(newDir, 0755)
	newIdx := IndexConfig{
		ID:              "manuals",
		Name:            "Manuals",
		Path:            newDir,
		DefaultSelected: true,
	}

	w.AddIndex(newIdx)

	select {
	case added := <-addedCh:
		if added.ID != "manuals" {
			t.Errorf("expected added ID 'manuals', got %q", added.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for OnIndexAdded callback")
	}

	// Verify it is in selected indexes
	selected := w.GetSelectedIndexes()
	if !reflect.DeepEqual(selected, []string{"initial", "manuals"}) {
		t.Errorf("expected selected ['initial', 'manuals'], got %v", selected)
	}

	// Verify persistence in docsearch.json
	savedCfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if len(savedCfg.Indexes) != 2 {
		t.Fatalf("expected 2 indexes in config, got %d", len(savedCfg.Indexes))
	}
	if savedCfg.Indexes[1].ID != "manuals" {
		t.Errorf("expected second index ID 'manuals', got %q", savedCfg.Indexes[1].ID)
	}

	// Adding duplicate index should safely no-op
	w.AddIndex(newIdx)
	if len(w.indexes) != 2 {
		t.Errorf("duplicate index was added, expected count 2, got %d", len(w.indexes))
	}
}

func TestSearchWindow_TriggerAddIndex(t *testing.T) {
	oldFn := openIndexDialogFn
	defer func() { openIndexDialogFn = oldFn }()

	tmpDir := t.TempDir()
	w, err := NewSearchWindow([]IndexConfig{}, nil)
	if err != nil {
		t.Fatalf("failed to create SearchWindow: %v", err)
	}
	defer w.Close()

	// 1. User confirms dialog with valid directory
	addedBleve := filepath.Join(tmpDir, "knowledge.bleve")
	_ = os.Mkdir(addedBleve, 0755)

	openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
		return addedBleve, true, nil
	}

	path, ok, err := w.TriggerAddIndex()
	if err != nil || !ok || path != addedBleve {
		t.Fatalf("TriggerAddIndex failed: path=%q, ok=%v, err=%v", path, ok, err)
	}

	if w.HasWarning() {
		t.Errorf("expected warning to be cleared after adding valid index")
	}
	selected := w.GetSelectedIndexes()
	if len(selected) != 1 || selected[0] != "knowledge" {
		t.Errorf("expected 'knowledge' in selected, got %v", selected)
	}

	// 2. User cancels dialog
	openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
		return "", false, nil
	}

	path, ok, err = w.TriggerAddIndex()
	if err != nil || ok || path != "" {
		t.Fatalf("expected cancel, got path=%q, ok=%v, err=%v", path, ok, err)
	}
	// Index count should not have changed
	if len(w.GetSelectedIndexes()) != 1 {
		t.Errorf("expected index count to remain 1, got %d", len(w.GetSelectedIndexes()))
	}
}

func TestSearchWindow_HandleAddIndexClicked(t *testing.T) {
	oldFn := openIndexDialogFn
	defer func() { openIndexDialogFn = oldFn }()

	tmpDir := t.TempDir()
	w, err := NewSearchWindow([]IndexConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// 1. Valid path adds index
	idxDir := filepath.Join(tmpDir, "clicked.bleve")
	_ = os.Mkdir(idxDir, 0755)

	openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
		return idxDir, true, nil
	}
	w.handleAddIndexClicked()

	selected := w.GetSelectedIndexes()
	if len(selected) != 1 || selected[0] != "clicked" {
		t.Errorf("expected index added via handleAddIndexClicked, got %v", selected)
	}

	// 2. Error / cancel from dialog safely returns
	openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
		return "", false, errors.New("dialog error")
	}
	w.handleAddIndexClicked()

	// 3. Invalid directory returned from dialog safely returns
	openIndexDialogFn = func(parentHWnd uintptr) (string, bool, error) {
		return filepath.Join(tmpDir, "missing_dir.bleve"), true, nil
	}
	w.handleAddIndexClicked()
}
