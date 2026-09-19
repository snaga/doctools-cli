package docsearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckIndexHealth(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Valid index directory
	validDir := filepath.Join(tmpDir, "docs.bleve")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid index dir: %v", err)
	}

	// 2. Regular file (not a directory)
	regularFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	// 3. Non-existent path
	missingPath := filepath.Join(tmpDir, "non_existent.bleve")

	// 4. Empty path
	emptyPath := ""

	indexes := []IndexConfig{
		{ID: "valid", Name: "Valid Docs", Path: validDir, DefaultSelected: true},
		{ID: "file", Name: "File Not Dir", Path: regularFile, DefaultSelected: true},
		{ID: "missing", Name: "Missing Docs", Path: missingPath, DefaultSelected: false},
		{ID: "empty", Name: "Empty Path", Path: emptyPath, DefaultSelected: false},
	}

	statuses := CheckIndexHealth(indexes)
	if len(statuses) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(statuses))
	}

	if !statuses[0].Exists {
		t.Errorf("expected validDir to exist, got false")
	}
	if statuses[1].Exists {
		t.Errorf("expected regularFile to be marked false (not a dir), got true")
	}
	if statuses[2].Exists {
		t.Errorf("expected missingPath to be false, got true")
	}
	if statuses[3].Exists {
		t.Errorf("expected emptyPath to be false, got true")
	}
}

func TestValidateIndexes(t *testing.T) {
	tmpDir := t.TempDir()

	validDir1 := filepath.Join(tmpDir, "idx1.bleve")
	_ = os.Mkdir(validDir1, 0755)
	validDir2 := filepath.Join(tmpDir, "idx2.bleve")
	_ = os.Mkdir(validDir2, 0755)

	missingDir := filepath.Join(tmpDir, "missing.bleve")

	indexes := []IndexConfig{
		{ID: "1", Name: "Index 1", Path: validDir1},
		{ID: "2", Name: "Index 2", Path: validDir2},
		{ID: "3", Name: "Index 3", Path: missingDir},
	}

	valid, missing := ValidateIndexes(indexes)

	if len(valid) != 2 {
		t.Fatalf("expected 2 valid indexes, got %d", len(valid))
	}
	if valid[0].ID != "1" || valid[1].ID != "2" {
		t.Errorf("unexpected valid indexes: %+v", valid)
	}

	if len(missing) != 1 {
		t.Fatalf("expected 1 missing index, got %d", len(missing))
	}
	if missing[0].ID != "3" {
		t.Errorf("unexpected missing index: %+v", missing[0])
	}
}

func TestFormatMissingIndexLabel(t *testing.T) {
	got := FormatMissingIndexLabel("社内マニュアル")
	expected := "⚠️ 社内マニュアル (見つかりません)"
	if got != expected {
		t.Errorf("FormatMissingIndexLabel() = %q; expected %q", got, expected)
	}

	if MissingIndexWarningText != "⚠️ 有効なインデックスがありません。[＋ 追加] から登録してください" {
		t.Errorf("unexpected MissingIndexWarningText: %q", MissingIndexWarningText)
	}
}
