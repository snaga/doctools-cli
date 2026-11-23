package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleCapture_PPTX(t *testing.T) {
	tmpDir := t.TempDir()
	dummyPPTX := filepath.Join(tmpDir, "presentation.pptx")
	_ = os.WriteFile(dummyPPTX, []byte("fake pptx"), 0644)

	dummyImg := filepath.Join(tmpDir, "slide_1.png")
	_ = os.WriteFile(dummyImg, []byte("dummy image content"), 0644)

	// Save and restore original mocks
	origSlideCount := pptxGetSlideCount
	origCapturePPTX := capturePPTX
	defer func() {
		pptxGetSlideCount = origSlideCount
		capturePPTX = origCapturePPTX
	}()

	pptxGetSlideCount = func(inputPath string) (int, error) {
		return 3, nil
	}

	var capturedPages []int
	var capturedDPI int
	var capturedOutputDir string

	capturePPTX = func(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
		capturedPages = pages
		capturedDPI = dpi
		capturedOutputDir = outputDir
		return []CapturedImage{
			{PageNumber: 2, FilePath: dummyImg, Width: 1920, Height: 1080, FileSize: int64(len("dummy image content"))},
		}, nil
	}

	ctx := context.Background()
	params := DoctoolsRunParams{
		Command:  "capture",
		FilePath: dummyPPTX,
		Options: &DoctoolsRunOptions{
			Slides:     "2",
			DPI:        300,
			OutputPath: tmpDir,
		},
	}

	res, err := handleCapture(ctx, FormatPPTX, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "success" {
		t.Errorf("expected status 'success', got %q", res.Status)
	}
	if len(capturedPages) != 1 || capturedPages[0] != 2 {
		t.Errorf("expected captured pages [2], got %v", capturedPages)
	}
	if capturedDPI != 300 {
		t.Errorf("expected dpi 300, got %d", capturedDPI)
	}
	if capturedOutputDir != tmpDir {
		t.Errorf("expected outputDir %s, got %s", tmpDir, capturedOutputDir)
	}
	if res.ImageBase64 == "" {
		t.Errorf("expected non-empty ImageBase64 for single slide")
	}
}

func TestHandleCapture_Excel(t *testing.T) {
	tmpDir := t.TempDir()
	dummyExcel := filepath.Join(tmpDir, "data.xlsx")
	_ = os.WriteFile(dummyExcel, []byte("fake excel"), 0644)

	dummyImg := filepath.Join(tmpDir, "sheet_1.png")
	_ = os.WriteFile(dummyImg, []byte("dummy image content"), 0644)

	origListSheets := excelListSheets
	origCaptureExcel := captureExcel
	defer func() {
		excelListSheets = origListSheets
		captureExcel = origCaptureExcel
	}()

	excelListSheets = func(filePath string) ([]string, error) {
		return []string{"Sheet1", "Sheet2"}, nil
	}

	captureExcel = func(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
		return []CapturedImage{
			{PageNumber: 1, FilePath: dummyImg, Width: 800, Height: 600, FileSize: int64(len("dummy image content"))},
		}, nil
	}

	ctx := context.Background()
	params := DoctoolsRunParams{
		Command:  "capture",
		FilePath: dummyExcel,
		Options: &DoctoolsRunOptions{
			Pages: "1",
		},
	}

	res, err := handleCapture(ctx, FormatExcel, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "success" {
		t.Errorf("expected status 'success', got %q", res.Status)
	}
	if res.TotalPages != 2 {
		t.Errorf("expected TotalPages=2, got %d", res.TotalPages)
	}
}

func TestHandleCapture_SkippedPagesWarning(t *testing.T) {
	tmpDir := t.TempDir()
	dummyPDF := filepath.Join(tmpDir, "test.pdf")
	_ = os.WriteFile(dummyPDF, []byte("fake pdf"), 0644)

	dummyImg := filepath.Join(tmpDir, "page_1.png")
	_ = os.WriteFile(dummyImg, []byte("dummy img"), 0644)

	origPageCount := pdfGetPageCount
	origCapturePDF := capturePDF
	defer func() {
		pdfGetPageCount = origPageCount
		capturePDF = origCapturePDF
	}()

	pdfGetPageCount = func(inputPath string) (int, error) {
		return 2, nil
	}

	capturePDF = func(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
		return []CapturedImage{
			{PageNumber: 1, FilePath: dummyImg, Width: 100, Height: 100, FileSize: int64(len("dummy img"))},
		}, nil
	}

	ctx := context.Background()
	params := DoctoolsRunParams{
		Command:  "capture",
		FilePath: dummyPDF,
		Options: &DoctoolsRunOptions{
			Pages: "1,5", // 5 is out-of-range for totalPages=2
		},
	}

	res, err := handleCapture(ctx, FormatPDF, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res.Message, "Warning: skipped out-of-range pages: [5]") {
		t.Errorf("expected skipped warning message, got %q", res.Message)
	}
}

func TestHandleCapture_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("unsupported format error", func(t *testing.T) {
		_, err := handleCapture(ctx, FormatCSV, DoctoolsRunParams{FilePath: "sample.csv"})
		if err == nil || !strings.Contains(err.Error(), "capture is not supported for format") {
			t.Errorf("expected unsupported format error, got %v", err)
		}
	})

	t.Run("pdf page count error", func(t *testing.T) {
		orig := pdfGetPageCount
		defer func() { pdfGetPageCount = orig }()
		pdfGetPageCount = func(inputPath string) (int, error) {
			return 0, errors.New("read pdf error")
		}

		_, err := handleCapture(ctx, FormatPDF, DoctoolsRunParams{FilePath: "bad.pdf"})
		if err == nil || !strings.Contains(err.Error(), "failed to get page count") {
			t.Errorf("expected pdf page count error, got %v", err)
		}
	})

	t.Run("pptx slide count error", func(t *testing.T) {
		orig := pptxGetSlideCount
		defer func() { pptxGetSlideCount = orig }()
		pptxGetSlideCount = func(inputPath string) (int, error) {
			return 0, errors.New("read pptx error")
		}

		_, err := handleCapture(ctx, FormatPPTX, DoctoolsRunParams{FilePath: "bad.pptx"})
		if err == nil || !strings.Contains(err.Error(), "failed to get slide count") {
			t.Errorf("expected pptx slide count error, got %v", err)
		}
	})

	t.Run("excel list sheets error", func(t *testing.T) {
		orig := excelListSheets
		defer func() { excelListSheets = orig }()
		excelListSheets = func(filePath string) ([]string, error) {
			return nil, errors.New("read excel error")
		}

		_, err := handleCapture(ctx, FormatExcel, DoctoolsRunParams{FilePath: "bad.xlsx"})
		if err == nil || !strings.Contains(err.Error(), "failed to list sheets") {
			t.Errorf("expected excel list sheets error, got %v", err)
		}
	})

	t.Run("invalid page range error", func(t *testing.T) {
		orig := pdfGetPageCount
		defer func() { pdfGetPageCount = orig }()
		pdfGetPageCount = func(inputPath string) (int, error) {
			return 3, nil
		}

		_, err := handleCapture(ctx, FormatPDF, DoctoolsRunParams{
			FilePath: "sample.pdf",
			Options:  &DoctoolsRunOptions{Pages: "abc"},
		})
		if err == nil {
			t.Errorf("expected invalid page number error, got nil")
		}
	})

	t.Run("capture handler error", func(t *testing.T) {
		origPageCount := pdfGetPageCount
		origCapturePDF := capturePDF
		defer func() {
			pdfGetPageCount = origPageCount
			capturePDF = origCapturePDF
		}()

		pdfGetPageCount = func(inputPath string) (int, error) {
			return 2, nil
		}
		capturePDF = func(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
			return nil, errors.New("extract error")
		}

		_, err := handleCapture(ctx, FormatPDF, DoctoolsRunParams{
			FilePath: "sample.pdf",
			Options:  &DoctoolsRunOptions{Pages: "1"},
		})
		if err == nil || !strings.Contains(err.Error(), "extract error") {
			t.Errorf("expected extract error, got %v", err)
		}
	})

	t.Run("single image file read error", func(t *testing.T) {
		origPageCount := pdfGetPageCount
		origCapturePDF := capturePDF
		defer func() {
			pdfGetPageCount = origPageCount
			capturePDF = origCapturePDF
		}()

		pdfGetPageCount = func(inputPath string) (int, error) {
			return 2, nil
		}
		capturePDF = func(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
			return []CapturedImage{
				{PageNumber: 1, FilePath: "non_existent_image.png", Width: 100, Height: 100, FileSize: 100},
			}, nil
		}

		_, err := handleCapture(ctx, FormatPDF, DoctoolsRunParams{
			FilePath: "sample.pdf",
			Options:  &DoctoolsRunOptions{Pages: "1"},
		})
		if err == nil || !strings.Contains(err.Error(), "failed to read captured image") {
			t.Errorf("expected failed to read captured image error, got %v", err)
		}
	})
}
