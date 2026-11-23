package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureExcel(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "test.xlsx")
	if err := os.WriteFile(xlsxPath, []byte("dummy xlsx content"), 0644); err != nil {
		t.Fatalf("failed to write dummy xlsx: %v", err)
	}
	outDir := filepath.Join(tmpDir, "out_excel_images")

	// Save original functions and restore after tests
	origListSheets := excelListSheets
	origExtractImages := excelExtractImages
	defer func() {
		excelListSheets = origListSheets
		excelExtractImages = origExtractImages
	}()

	t.Run("Valid capture with mocked extractor and sheets", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return []string{"Sheet1", "Sheet2"}, nil
		}

		var createdPDFs []string
		excelExtractImages = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
			var pdfs []string
			for i := range sheetNames {
				pdfPath := filepath.Join(outputDir, fmt.Sprintf("sheet_%d.pdf", i+1))
				createTestPDF(t, pdfPath)
				pdfs = append(pdfs, pdfPath)
				createdPDFs = append(createdPDFs, pdfPath)
			}
			return pdfs, nil
		}

		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{1, 2}, 150, outDir)
		if err != nil {
			t.Fatalf("handleCaptureExcel failed: %v", err)
		}

		if len(images) != 2 {
			t.Fatalf("expected 2 images, got %d", len(images))
		}

		if images[0].PageNumber != 1 || images[1].PageNumber != 2 {
			t.Errorf("expected page numbers [1, 2], got [%d, %d]", images[0].PageNumber, images[1].PageNumber)
		}

		for _, img := range images {
			if img.Width <= 0 || img.Height <= 0 {
				t.Errorf("expected positive dimensions, got width=%d, height=%d", img.Width, img.Height)
			}
			if img.FileSize <= 0 {
				t.Errorf("expected positive file size, got %d", img.FileSize)
			}
			if _, err := os.Stat(img.FilePath); err != nil {
				t.Errorf("expected output image file to exist at %s: %v", img.FilePath, err)
			}
		}

		// Verify temporary PDFs are deleted
		for _, pdfPath := range createdPDFs {
			if _, err := os.Stat(pdfPath); !os.IsNotExist(err) {
				t.Errorf("expected temporary pdf %s to be deleted, but it still exists", pdfPath)
			}
		}
	})

	t.Run("Valid capture with partial valid pages and default dpi", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return []string{"Sheet1", "Sheet2", "Sheet3"}, nil
		}

		var requestedSheets []string
		excelExtractImages = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
			requestedSheets = sheetNames
			pdfPath := filepath.Join(outputDir, "sheet_2.pdf")
			createTestPDF(t, pdfPath)
			return []string{pdfPath}, nil
		}

		// pages contains 2 (valid Sheet2) and 99 (invalid)
		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{2, 99}, 0, outDir)
		if err != nil {
			t.Fatalf("handleCaptureExcel failed: %v", err)
		}

		if len(requestedSheets) != 1 || requestedSheets[0] != "Sheet2" {
			t.Errorf("expected requestedSheets [Sheet2], got %v", requestedSheets)
		}

		if len(images) != 1 {
			t.Fatalf("expected 1 image, got %d", len(images))
		}
		if images[0].PageNumber != 2 {
			t.Errorf("expected page number 2, got %d", images[0].PageNumber)
		}
	})

	t.Run("Empty pages list returns empty result", func(t *testing.T) {
		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{}, 150, outDir)
		if err != nil {
			t.Fatalf("expected nil error on empty pages, got: %v", err)
		}
		if len(images) != 0 {
			t.Errorf("expected 0 images, got %d", len(images))
		}
	})

	t.Run("Non-existent file returns error", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "does_not_exist.xlsx")
		images, err := handleCaptureExcel(context.Background(), nonExistent, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for non-existent file, got nil")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("expected 'file not found' error message, got: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Mock excelListSheets returns error", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return nil, errors.New("cannot open workbook")
		}

		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error from handleCaptureExcel, got nil")
		}
		if !strings.Contains(err.Error(), "failed to list excel sheets: cannot open workbook") {
			t.Errorf("unexpected error message: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Out of range pages with no valid sheets returns error", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return []string{"Sheet1"}, nil
		}

		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{0, 5, 10}, 150, outDir)
		if err == nil {
			t.Errorf("expected error when no valid sheets, got nil")
		}
		if !strings.Contains(err.Error(), "no valid sheets for requested pages") {
			t.Errorf("unexpected error message: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Mock excelExtractImages returns error", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return []string{"Sheet1"}, nil
		}
		excelExtractImages = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
			return nil, errors.New("COM export failed")
		}

		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error from handleCaptureExcel, got nil")
		}
		if !strings.Contains(err.Error(), "failed to export excel sheets as PDF: COM export failed") {
			t.Errorf("unexpected error message: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Rasterize error cleans up temporary PDF", func(t *testing.T) {
		excelListSheets = func(inputPath string) ([]string, error) {
			return []string{"Sheet1"}, nil
		}
		var tempPDF string
		excelExtractImages = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
			tempPDF = filepath.Join(outputDir, "invalid.pdf")
			if err := os.WriteFile(tempPDF, []byte("not a pdf"), 0644); err != nil {
				return nil, err
			}
			return []string{tempPDF}, nil
		}

		images, err := handleCaptureExcel(context.Background(), xlsxPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for invalid pdf rasterization, got nil")
		}
		if !strings.Contains(err.Error(), "failed to rasterize sheet PDF") {
			t.Errorf("unexpected error message: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
		if _, err := os.Stat(tempPDF); !os.IsNotExist(err) {
			t.Errorf("expected temp PDF to be cleaned up, but it still exists at %s", tempPDF)
		}
	})

	t.Run("Context canceled before execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		images, err := handleCaptureExcel(ctx, xlsxPath, []int{1}, 150, outDir)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on context cancel, got %v", images)
		}
	})
}
