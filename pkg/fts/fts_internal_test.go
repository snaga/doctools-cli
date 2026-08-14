package fts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizePath(t *testing.T) {
	p := normalizePath("C:\\foo\\bar\\..\\baz")
	if p == "" {
		t.Errorf("expected non-empty normalized path")
	}
}

func TestParseTimeVal(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name     string
		val      interface{}
		wantTime time.Time
		wantOk   bool
	}{
		{"nil input", nil, time.Time{}, false},
		{"time.Time input", now, now, true},
		{"RFC3339 string", now.Format(time.RFC3339), now, true},
		{"Custom layout string", "2026-08-11 20:30:00", time.Date(2026, 8, 11, 20, 30, 0, 0, time.UTC), true},
		{"Invalid string format", "invalid-date", time.Time{}, false},
		{"Invalid type (int)", 12345, time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotOk := parseTimeVal(tt.val)
			if gotOk != tt.wantOk {
				t.Errorf("parseTimeVal() gotOk = %v, wantOk %v", gotOk, tt.wantOk)
			}
			if tt.wantOk && !gotTime.Equal(tt.wantTime) {
				t.Errorf("parseTimeVal() gotTime = %v, wantTime %v", gotTime, tt.wantTime)
			}
		})
	}
}

func TestParseFile_Errors(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Context canceled
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := parseFile(canceledCtx, "dummy.txt", "dummy.txt", nil, ".txt")
	if err == nil {
		t.Errorf("expected error for canceled context")
	}

	// Non-existent file for .xlsx
	nonExistent := filepath.Join(tmpDir, "nonexistent")
	info, _ := os.Stat(tmpDir) // dummy info

	_, err = parseFile(ctx, nonExistent+".xlsx", "nonexistent.xlsx", info, ".xlsx")
	if err == nil {
		t.Errorf("expected error parsing non-existent xlsx")
	}

	// Non-existent file for .pptx
	_, err = parseFile(ctx, nonExistent+".pptx", "nonexistent.pptx", info, ".pptx")
	if err == nil {
		t.Errorf("expected error parsing non-existent pptx")
	}

	// Non-existent file for .pdf
	_, _ = parseFile(ctx, nonExistent+".pdf", "nonexistent.pdf", info, ".pdf")

	// Non-existent file for .txt
	_, err = parseFile(ctx, nonExistent+".txt", "nonexistent.txt", info, ".txt")
	if err == nil {
		t.Errorf("expected error parsing non-existent txt")
	}

	// Unknown extension
	chunks, err := parseFile(ctx, nonExistent+".unknown", "nonexistent.unknown", info, ".unknown")
	if err != nil || chunks != nil {
		t.Errorf("expected nil chunks and nil error for unknown extension")
	}
}

func TestParsePdf_FallbackText(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "dummy_text.pdf")
	// PDF content that doesn't extract via pdfcpu content stream, forcing raw bytes fallback
	dummyContent := []byte("%PDF-1.4 Minimal PDF raw text fallback test %%EOF")
	_ = os.WriteFile(pdfPath, dummyContent, 0644)
	info, _ := os.Stat(pdfPath)

	chunks, err := parsePdf(pdfPath, "dummy_text.pdf", info)
	if err != nil {
		t.Fatalf("parsePdf failed: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content == "" {
		t.Errorf("expected non-empty fallback content")
	}
}

func TestBuildIndexWithOptions_CorruptExistingIndex(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	_ = os.MkdirAll(sourceDir, 0755)
	_ = os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("content"), 0644)

	// Create corrupt bleve index path (a plain file instead of directory/bleve structure)
	corruptIdxPath := filepath.Join(tmpDir, "corrupt.bleve")
	_ = os.WriteFile(corruptIdxPath, []byte("corrupt file"), 0644)

	// Build with Force=false on corrupt index -> should remove corrupt file and create new index
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   corruptIdxPath,
		Force:       false,
		Verbose:     true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed on corrupt index: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}

func TestQueryIndex_LimitAndLongContent(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	_ = os.MkdirAll(sourceDir, 0755)

	// Create long content file > 200 runes
	longText := "LongContent "
	for i := 0; i < 30; i++ {
		longText += "This is a repeated string for snippet length test. "
	}
	_ = os.WriteFile(filepath.Join(sourceDir, "long.txt"), []byte(longText), 0644)

	indexPath := filepath.Join(tmpDir, "long.bleve")
	_, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}

	// Test limit <= 0
	res, err := QueryIndex(indexPath, "repeated", -1)
	if err != nil {
		t.Fatalf("QueryIndex with limit <= 0 failed: %v", err)
	}
	if res.TotalHits < 1 {
		t.Fatalf("expected at least 1 hit, got %d", res.TotalHits)
	}

	hit := res.Hits[0]
	if len([]rune(hit.Snippet)) == 0 {
		t.Errorf("expected non-empty snippet")
	}
}

func TestCleanPath(t *testing.T) {
	// Normal path
	result := cleanPath(".")
	if result == "" {
		t.Errorf("expected non-empty clean path for '.'")
	}

	// Relative path with ..
	result = cleanPath("foo/../bar")
	if result == "" {
		t.Errorf("expected non-empty clean path")
	}
}

func TestParsePdf_RealPDF(t *testing.T) {
	sampleSrc := filepath.Join("..", "..", "tests", "test_data", "sample.pdf")
	if _, err := os.Stat(sampleSrc); os.IsNotExist(err) {
		t.Skip("sample.pdf not found, skipping")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "sample.pdf")
	data, err := os.ReadFile(sampleSrc)
	if err != nil {
		t.Fatalf("failed to read sample pdf: %v", err)
	}
	os.WriteFile(pdfPath, data, 0644)
	info, _ := os.Stat(pdfPath)

	chunks, err := parsePdf(pdfPath, "sample.pdf", info)
	if err != nil {
		t.Fatalf("parsePdf failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Errorf("expected at least 1 chunk from real PDF")
	}
	for _, c := range chunks {
		if c.FileType != ".pdf" {
			t.Errorf("expected file type .pdf, got %s", c.FileType)
		}
		if c.UnitType != "page" {
			t.Errorf("expected unit type page, got %s", c.UnitType)
		}
	}
}

func TestParseXlsx_RealXLSX(t *testing.T) {
	sampleSrc := filepath.Join("..", "..", "tests", "test_data", "sample.xlsx")
	if _, err := os.Stat(sampleSrc); os.IsNotExist(err) {
		t.Skip("sample.xlsx not found, skipping")
	}

	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "sample.xlsx")
	data, err := os.ReadFile(sampleSrc)
	if err != nil {
		t.Fatalf("failed to read sample xlsx: %v", err)
	}
	os.WriteFile(xlsxPath, data, 0644)
	info, _ := os.Stat(xlsxPath)

	chunks, err := parseXlsx(xlsxPath, "sample.xlsx", info)
	if err != nil {
		t.Fatalf("parseXlsx failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Errorf("expected at least 1 chunk from real XLSX")
	}
}

func TestBuildIndexWithOptions_ForceWithExisting(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("force test content"), 0644)

	indexPath := filepath.Join(tmpDir, "force.bleve")

	// First build
	_, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	// Second build with Force=true on existing index
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("force rebuild failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}

func TestBuildIndexWithOptions_DefaultTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("default timeout test"), 0644)

	indexPath := filepath.Join(tmpDir, "default_timeout.bleve")
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		Timeout:     0, // should default to 10s
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("build with default timeout failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}

func TestBuildIndexWithOptions_DefaultIncludeExts(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("should not be indexed by default"), 0644)

	indexPath := filepath.Join(tmpDir, "default_exts.bleve")
	// No IncludeExts specified - defaults to .xlsx, .pptx, .pdf, .docx
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath: indexPath,
		Force:     true,
	})
	if err != nil {
		t.Fatalf("build with default exts failed: %v", err)
	}
	// .txt should NOT be indexed with default extensions
	if res.IndexedFiles != 0 {
		t.Errorf("expected 0 indexed files (only .txt present, default doesn't include .txt), got %d", res.IndexedFiles)
	}
}

func TestQueryIndex_FullHitFields(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "querytest.txt"), []byte("searchable unique content for query test"), 0644)

	indexPath := filepath.Join(tmpDir, "queryfields.bleve")
	_, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	res, err := QueryIndex(indexPath, "searchable", 10)
	if err != nil {
		t.Fatalf("QueryIndex failed: %v", err)
	}
	if res.TotalHits < 1 {
		t.Fatalf("expected at least 1 hit, got %d", res.TotalHits)
	}

	hit := res.Hits[0]
	if hit.Source.FileName == "" {
		t.Errorf("expected non-empty FileName")
	}
	if hit.Source.FileType == "" {
		t.Errorf("expected non-empty FileType")
	}
	if hit.Source.FilePath == "" {
		t.Errorf("expected non-empty FilePath")
	}
	if hit.Target.UnitType == "" {
		t.Errorf("expected non-empty UnitType")
	}
	if hit.Snippet == "" {
		t.Errorf("expected non-empty Snippet")
	}
}

func TestBuildAndQuery_WithXlsx(t *testing.T) {
	sampleSrc := filepath.Join("..", "..", "tests", "test_data", "sample.xlsx")
	if _, err := os.Stat(sampleSrc); os.IsNotExist(err) {
		t.Skip("sample.xlsx not found, skipping")
	}

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	data, _ := os.ReadFile(sampleSrc)
	os.WriteFile(filepath.Join(sourceDir, "sample.xlsx"), data, 0644)

	indexPath := filepath.Join(tmpDir, "xlsx.bleve")
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"xlsx"},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}

	qres, err := QueryIndex(indexPath, "*", 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qres.TotalHits < 1 {
		t.Errorf("expected at least 1 hit from xlsx, got %d", qres.TotalHits)
	}
}

func TestBuildAndQuery_WithPdf(t *testing.T) {
	sampleSrc := filepath.Join("..", "..", "tests", "test_data", "sample.pdf")
	if _, err := os.Stat(sampleSrc); os.IsNotExist(err) {
		t.Skip("sample.pdf not found, skipping")
	}

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	data, _ := os.ReadFile(sampleSrc)
	os.WriteFile(filepath.Join(sourceDir, "sample.pdf"), data, 0644)

	indexPath := filepath.Join(tmpDir, "pdf.bleve")
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"pdf"},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", res.IndexedFiles)
	}
}



// withParseFile swaps the parser used by the walk function and restores it when
// the test ends.
func withParseFile(t *testing.T, fn func(context.Context, string, string, os.FileInfo, string) ([]DocumentChunk, error)) {
	t.Helper()
	old := parseFileImpl
	t.Cleanup(func() { parseFileImpl = old })
	parseFileImpl = fn
}

func newTimeoutSource(t *testing.T) (sourceDir string, indexPath string) {
	t.Helper()
	tmpDir := t.TempDir()
	sourceDir = filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "slow.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	return sourceDir, filepath.Join(tmpDir, "timeout.bleve")
}

// TestBuildIndexWithOptions_ParseOutlivesTimeout drives the select branch where
// the context deadline fires before the parser returns. The timeout is well
// above the short-circuit threshold, so the walk function really does wait on
// the context.
func TestBuildIndexWithOptions_ParseOutlivesTimeout(t *testing.T) {
	withParseFile(t, func(ctx context.Context, absPath, relPath string, info os.FileInfo, ext string) ([]DocumentChunk, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	sourceDir, indexPath := newTimeoutSource(t)

	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
		Timeout:     20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.TimeoutFiles != 1 {
		t.Errorf("expected the slow file to be counted as a timeout, got %d", res.TimeoutFiles)
	}
	if res.IndexedFiles != 0 {
		t.Errorf("expected nothing to be indexed, got %d", res.IndexedFiles)
	}
}

// TestBuildIndexWithOptions_ParseReportsDeadline drives the branch where the
// parser itself reports a context error while the deadline has not fired yet,
// which must also be counted as a timeout rather than a parse failure.
func TestBuildIndexWithOptions_ParseReportsDeadline(t *testing.T) {
	withParseFile(t, func(ctx context.Context, absPath, relPath string, info os.FileInfo, ext string) ([]DocumentChunk, error) {
		return nil, context.DeadlineExceeded
	})

	sourceDir, indexPath := newTimeoutSource(t)

	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.TimeoutFiles != 1 {
		t.Errorf("expected a deadline error to count as a timeout, got %d", res.TimeoutFiles)
	}
}

// TestBuildIndexWithOptions_ParseFails covers the ordinary parse failure branch,
// which must not be counted as a timeout.
func TestBuildIndexWithOptions_ParseFails(t *testing.T) {
	withParseFile(t, func(ctx context.Context, absPath, relPath string, info os.FileInfo, ext string) ([]DocumentChunk, error) {
		return nil, os.ErrInvalid
	})

	sourceDir, indexPath := newTimeoutSource(t)

	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.TimeoutFiles != 0 || res.IndexedFiles != 0 {
		t.Errorf("expected a plain parse failure: timeout=%d indexed=%d", res.TimeoutFiles, res.IndexedFiles)
	}
}
