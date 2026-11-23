package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createTestPDF(t *testing.T, path string) {
	t.Helper()
	// Minimal valid PDF-1.4 for testing
	body := "1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n" +
		"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
		"5 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello MCP Capture) Tj\nET\nendstream\nendobj\n"

	header := "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"
	o1 := len(header)
	o2 := o1 + len("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	o3 := o2 + len("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	o4 := o3 + len("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n")
	o5 := o4 + len("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	xrefOffset := len(header) + len(body)
	xref := "xref\n0 6\n0000000000 65535 f \n" +
		fmt.Sprintf("%010d 00000 n \n", o1) +
		fmt.Sprintf("%010d 00000 n \n", o2) +
		fmt.Sprintf("%010d 00000 n \n", o3) +
		fmt.Sprintf("%010d 00000 n \n", o4) +
		fmt.Sprintf("%010d 00000 n \n", o5)
	trailer := fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	fullPDF := []byte(header + body + xref + trailer)
	if err := os.WriteFile(path, fullPDF, 0644); err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}
}

func TestCapturePDF(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createTestPDF(t, pdfPath)

	outDir := filepath.Join(tmpDir, "out_images")

	t.Run("Valid capture page 1 with defaults", func(t *testing.T) {
		images, err := handleCapturePDF(context.Background(), pdfPath, []int{1}, 0, outDir)
		if err != nil {
			t.Fatalf("handleCapturePDF failed: %v", err)
		}

		if len(images) != 1 {
			t.Fatalf("expected 1 image, got %d", len(images))
		}

		img := images[0]
		if img.PageNumber != 1 {
			t.Errorf("expected PageNumber 1, got %d", img.PageNumber)
		}
		if img.Width <= 0 || img.Height <= 0 {
			t.Errorf("expected positive dimensions, got width=%d, height=%d", img.Width, img.Height)
		}
		if img.FileSize <= 0 {
			t.Errorf("expected positive file size, got %d", img.FileSize)
		}
		if _, err := os.Stat(img.FilePath); err != nil {
			t.Errorf("expected generated file to exist at %s: %v", img.FilePath, err)
		}
	})

	t.Run("Empty pages list", func(t *testing.T) {
		images, err := handleCapturePDF(context.Background(), pdfPath, []int{}, 150, outDir)
		if err != nil {
			t.Fatalf("expected nil error on empty pages, got: %v", err)
		}
		if len(images) != 0 {
			t.Errorf("expected 0 images, got %d", len(images))
		}
	})

	t.Run("Non-existent file", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "does_not_exist.pdf")
		images, err := handleCapturePDF(context.Background(), nonExistent, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for non-existent file, got nil")
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Invalid page number out of range", func(t *testing.T) {
		images, err := handleCapturePDF(context.Background(), pdfPath, []int{99}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for page number out of range, got nil")
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		images, err := handleCapturePDF(ctx, pdfPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for cancelled context, got nil")
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})
}
