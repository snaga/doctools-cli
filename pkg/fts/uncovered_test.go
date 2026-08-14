package fts_test

import (
	"doctools-cli/pkg/fts"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. parseXlsx error path
	badXlsx := filepath.Join(tmpDir, "bad.xlsx")
	os.WriteFile(badXlsx, []byte("bad"), 0644)
	fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{Force: true, IncludeExts: []string{".xlsx"}})

	// 2. parsePptx error path
	badPptx := filepath.Join(tmpDir, "bad.pptx")
	os.WriteFile(badPptx, []byte("bad"), 0644)
	fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{Force: true, IncludeExts: []string{".pptx"}})

	// 3. parsePdf error path
	badPdf := filepath.Join(tmpDir, "bad.pdf")
	os.WriteFile(badPdf, []byte("bad"), 0644)
	fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{Force: true, IncludeExts: []string{".pdf"}})
}

func TestQueryIndex_Uncovered(t *testing.T) {
	tmpDir := t.TempDir()

	validIdx, _ := fts.BuildIndex(tmpDir, filepath.Join(tmpDir, "test.bleve"))

	// limit <= 0
	fts.QueryIndex(validIdx, "test", -1)
}

func TestQueryIndex_NonExistentIndex(t *testing.T) {
	_, err := fts.QueryIndex("/nonexistent/path/index.bleve", "test", 10)
	if err == nil {
		t.Errorf("expected error for non-existent index path")
	}
}

func TestNormalizeExt_Uncovered(t *testing.T) {
	// Various normalization edge cases
	tests := []struct {
		input    string
		expected string
	}{
		{"txt", ".txt"},
		{".TXT", ".txt"},
		{"  .Pdf  ", ".pdf"},
		{".xlsx", ".xlsx"},
		{"", ""},
		{".", "."},
	}
	for _, tt := range tests {
		got := fts.NormalizeExt(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeExt(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildIndexWithOptions_ExcludeExts(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	// Create files with different extensions
	os.WriteFile(filepath.Join(sourceDir, "include.txt"), []byte("included content"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "exclude.csv"), []byte("excluded content"), 0644)

	indexPath := filepath.Join(tmpDir, "exclude_test.bleve")
	res, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt", "csv"},
		ExcludeExts: []string{"csv"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file (only .txt), got %d", res.IndexedFiles)
	}
}

func TestBuildIndexWithOptions_Verbose(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "verbose_test.txt"), []byte("verbose content"), 0644)

	indexPath := filepath.Join(tmpDir, "verbose.bleve")
	res, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		Verbose:     true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions with Verbose failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}

func TestBuildIndexWithOptions_DiffUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "update.txt"), []byte("original content"), 0644)

	indexPath := filepath.Join(tmpDir, "diff.bleve")

	// First build
	res1, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	if res1.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file on first build, got %d", res1.IndexedFiles)
	}

	// Second build without Force - same file should be skipped (unchanged)
	res2, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       false,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	if res2.SkippedFiles != 1 {
		t.Errorf("expected 1 skipped file on second build, got %d", res2.SkippedFiles)
	}

	// Modify file and rebuild - should re-index
	time.Sleep(1100 * time.Millisecond) // ensure modtime differs
	os.WriteFile(filepath.Join(sourceDir, "update.txt"), []byte("updated content"), 0644)

	res3, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       false,
		Verbose:     true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("Third build failed: %v", err)
	}
	if res3.IndexedFiles != 1 {
		t.Errorf("expected 1 re-indexed file on third build, got %d", res3.IndexedFiles)
	}
}

func TestBuildIndexWithOptions_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "timeout.txt"), []byte("timeout test"), 0644)

	indexPath := filepath.Join(tmpDir, "timeout.bleve")

	// Use extremely short timeout to trigger timeout path
	res, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		Timeout:     1 * time.Nanosecond,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions with timeout failed: %v", err)
	}
	if res.TimeoutFiles != 1 {
		t.Errorf("expected 1 timeout file, got %d", res.TimeoutFiles)
	}
}

func TestBuildIndexWithOptions_NonExistentSource(t *testing.T) {
	_, err := fts.BuildIndexWithOptions("/nonexistent/dir", fts.BuildOptions{
		Force: true,
	})
	if err == nil {
		t.Errorf("expected error for non-existent source dir")
	}
}

func TestBuildIndexWithOptions_EmptyIndexPath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("content"), 0644)

	// Empty IndexPath should default to sourceDir/fts.bleve
	res, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions with empty IndexPath failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}

func TestFTS_OtherUncovered(t *testing.T) {
	// Querying something that matches file_path only to hit snippet logic without content fragment
	tmpDir := t.TempDir()
	validIdx, _ := fts.BuildIndex(tmpDir, filepath.Join(tmpDir, "test.bleve"))
	_, _ = fts.QueryIndex(validIdx, "file_path:uncovered", 10)
}
