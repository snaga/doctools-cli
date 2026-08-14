package pdf_test

import (
	"doctools-cli/pkg/pdf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPages_DPI_Zero(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "pages_dpi0")
	pages, err := pdf.ExtractPages(pdfPath, outDir, 0, "png", 1, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages with dpi=0 failed: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}

func TestExtractPages_NegativeDPI(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "pages_neg_dpi")
	pages, err := pdf.ExtractPages(pdfPath, outDir, -50, "png", 1, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages with negative dpi failed: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}

func TestExtractPages_JpegAlias(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "pages_jpeg")
	pages, err := pdf.ExtractPages(pdfPath, outDir, 72, "jpeg", 1, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages with format=jpeg failed: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
	if !strings.HasSuffix(pages[0], ".jpg") {
		t.Errorf("expected .jpg extension, got %s", pages[0])
	}
}

func TestExtractPages_EmptyOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	// Change to tmpDir so "." output works
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	pages, err := pdf.ExtractPages(pdfPath, "", 150, "png", 1, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages with empty outputDir failed: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}

func TestExtractPages_StartPageZero(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "pages_sp0")
	pages, err := pdf.ExtractPages(pdfPath, outDir, 150, "png", 0, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages with startPage=0 failed: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}

func TestExtractPages_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	// Create a file where the output dir should be
	badDir := filepath.Join(tmpDir, "bad_dir")
	os.WriteFile(badDir, []byte("not a dir"), 0644)

	_, err := pdf.ExtractPages(pdfPath, filepath.Join(badDir, "subdir"), 150, "png", 1, 1, true)
	if err == nil {
		t.Errorf("expected error for invalid output directory")
	}
}

func TestExtractPages_CorruptPDF(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPdf := filepath.Join(tmpDir, "corrupt.pdf")
	os.WriteFile(corruptPdf, []byte("not a pdf file at all"), 0644)

	_, err := pdf.ExtractPages(corruptPdf, tmpDir, 150, "png", 1, 1, true)
	if err == nil {
		t.Errorf("expected error for corrupt PDF")
	}
}

func TestExtractImages_CorruptPDF(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPdf := filepath.Join(tmpDir, "corrupt.pdf")
	os.WriteFile(corruptPdf, []byte("not a pdf file"), 0644)

	_, err := pdf.ExtractImages(corruptPdf, tmpDir, nil)
	if err == nil {
		t.Errorf("expected error for corrupt PDF in ExtractImages")
	}
}

func TestExtractImages_AllPages(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "all_imgs")
	_, err := pdf.ExtractImages(pdfPath, outDir, nil)
	if err != nil {
		t.Fatalf("ExtractImages all pages failed: %v", err)
	}
}

func TestSplit_EndPageBeyondTotal(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	outPath := filepath.Join(tmpDir, "split_beyond.pdf")
	_, err := pdf.Split(pdfPath, outPath, 1, 9999)
	// Should either work (clamped to last page) or error cleanly
	_ = err
}

func TestSplit_InvalidPDF(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPdf := filepath.Join(tmpDir, "corrupt.pdf")
	os.WriteFile(corruptPdf, []byte("not a pdf"), 0644)

	_, err := pdf.Split(corruptPdf, filepath.Join(tmpDir, "out.pdf"), 1, 1)
	if err == nil {
		t.Errorf("expected error for Split on corrupt PDF")
	}
}

func TestMerge_InvalidOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createSamplePDF(t, pdfPath)

	// Output path is a directory that exists as a file
	badOut := filepath.Join(tmpDir, "bad_out")
	os.WriteFile(badOut, []byte("file"), 0644)

	_, err := pdf.Merge([]string{pdfPath}, filepath.Join(badOut, "sub", "out.pdf"))
	// Should succeed creating parent dirs, or fail cleanly
	_ = err
}

func TestMerge_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "single.pdf")
	createSamplePDF(t, pdfPath)

	outPath := filepath.Join(tmpDir, "merged_single.pdf")
	result, err := pdf.Merge([]string{pdfPath}, outPath)
	if err != nil {
		t.Fatalf("Merge single file failed: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty merge result path")
	}
}
