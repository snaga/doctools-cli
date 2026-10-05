package pdf_test

import (
	"doctools-cli/pkg/pdf"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createSamplePDF(t *testing.T, path string) {
	t.Helper()
	// Fully valid minimal PDF 1.4 with correct byte offsets for pdfcpu validation
	body := "1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n" +
		"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
		"5 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello PDF) Tj\nET\nendstream\nendobj\n"
	
	header := "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"
	// Offsets
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

func TestPDFOperations(t *testing.T) {
	tmpDir := t.TempDir()
	pdf1 := filepath.Join(tmpDir, "pdf1.pdf")
	pdf2 := filepath.Join(tmpDir, "pdf2.pdf")

	createSamplePDF(t, pdf1)
	createSamplePDF(t, pdf2)

	// Test ExtractText with outputPath and page range
	txtOut := filepath.Join(tmpDir, "out.txt")
	txtContent, err := pdf.ExtractText(pdf1, txtOut, 1, 1)
	if err != nil {
		t.Fatalf("ExtractText failed: %v", err)
	}
	if !strings.Contains(txtContent, "Hello PDF") {
		t.Errorf("expected text content to contain 'Hello PDF', got %q", txtContent)
	}
	if !strings.Contains(txtContent, "--- Page 1 ---") {
		t.Errorf("expected text content to contain '--- Page 1 ---', got %q", txtContent)
	}

	savedBytes, err := os.ReadFile(txtOut)
	if err != nil {
		t.Fatalf("failed to read saved text file: %v", err)
	}
	if string(savedBytes) != txtContent {
		t.Errorf("saved file content does not match returned text")
	}

	// Test ExtractText with startPage > 0 and endPage 0
	txtContent2, err := pdf.ExtractText(pdf1, "", 1, 0)
	if err != nil {
		t.Fatalf("ExtractText page 1- failed: %v", err)
	}
	if !strings.Contains(txtContent2, "Hello PDF") {
		t.Errorf("expected text content to contain 'Hello PDF', got %q", txtContent2)
	}

	// Test ExtractText with startPage 0 and endPage 0 (all pages)
	txtContent3, err := pdf.ExtractText(pdf1, "", 0, 0)
	if err != nil {
		t.Fatalf("ExtractText all pages failed: %v", err)
	}
	if !strings.Contains(txtContent3, "Hello PDF") {
		t.Errorf("expected text content to contain 'Hello PDF', got %q", txtContent3)
	}

	// Test Merge
	mergedOut := filepath.Join(tmpDir, "merged.pdf")
	_, err = pdf.Merge([]string{pdf1, pdf2}, mergedOut)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	if _, err := os.Stat(mergedOut); os.IsNotExist(err) {
		t.Errorf("expected merged pdf to exist")
	}

	// Test Split with custom outputPath
	splitOut := filepath.Join(tmpDir, "split.pdf")
	_, err = pdf.Split(mergedOut, splitOut, 1, 1)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Test Split with empty outputPath (default name)
	_, err = pdf.Split(mergedOut, "", 1, 1)
	if err != nil {
		t.Fatalf("Split with default path failed: %v", err)
	}

	// Test ExtractImages with explicit outputDir
	imgs, err := pdf.ExtractImages(pdf1, filepath.Join(tmpDir, "extracted_imgs"), []int{1})
	if err != nil {
		t.Fatalf("ExtractImages failed: %v", err)
	}
	_ = imgs

	// Test ExtractImages with empty outputDir
	imgs2, err := pdf.ExtractImages(pdf1, "", nil)
	if err != nil {
		t.Fatalf("ExtractImages default dir failed: %v", err)
	}
	_ = imgs2

	// Test ExtractPages PNG
	pageOutDir := filepath.Join(tmpDir, "pages_png")
	pagesPNG, err := pdf.ExtractPages(pdf1, pageOutDir, 150, "png", 1, 0, true)
	if err != nil {
		t.Fatalf("ExtractPages PNG failed: %v", err)
	}
	if len(pagesPNG) == 0 {
		t.Errorf("expected rendered page images")
	}

	// Test ExtractPages JPG with force false error on existing file
	_, err = pdf.ExtractPages(pdf1, pageOutDir, 150, "png", 1, 0, false)
	if err == nil {
		t.Errorf("expected error when output file exists and force is false")
	}

	// Test ExtractPages JPG with force true
	pagesJPG, err := pdf.ExtractPages(pdf1, filepath.Join(tmpDir, "pages_jpg"), 72, "jpg", 1, 1, true)
	if err != nil {
		t.Fatalf("ExtractPages JPG failed: %v", err)
	}
	if len(pagesJPG) != 1 {
		t.Errorf("expected 1 page image, got %d", len(pagesJPG))
	}
}

func TestPDFErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.pdf")

	// ExtractText non-existent
	_, err := pdf.ExtractText(nonExistent, "", 0, 0)
	if err == nil {
		t.Errorf("expected error for ExtractText on nonexistent file")
	}

	// ExtractText on invalid PDF content
	invalidPdf := filepath.Join(tmpDir, "invalid.pdf")
	os.WriteFile(invalidPdf, []byte("not a pdf"), 0644)
	_, err = pdf.ExtractText(invalidPdf, "", 0, 0)
	if err == nil {
		t.Errorf("expected error for ExtractText on invalid pdf")
	}

	// ExtractText on invalid page range
	validPdf := filepath.Join(tmpDir, "valid.pdf")
	createSamplePDF(t, validPdf)
	_, err = pdf.ExtractText(validPdf, "", 5, 2)
	if err == nil {
		t.Errorf("expected error for ExtractText with invalid page range")
	}

	// Split non-existent
	_, err = pdf.Split(nonExistent, "", 1, 1)
	if err == nil {
		t.Errorf("expected error for Split on nonexistent file")
	}

	// Split invalid range
	pdf1 := filepath.Join(tmpDir, "pdf1.pdf")
	createSamplePDF(t, pdf1)
	_, err = pdf.Split(pdf1, "", 0, 0)
	if err == nil {
		t.Errorf("expected error for Split with invalid page range")
	}

	// Split with read-only/invalid output path
	invalidOutDir := filepath.Join(tmpDir, "invalid_out_file")
	os.WriteFile(invalidOutDir, []byte("is a file"), 0644)
	invalidOutPath := filepath.Join(invalidOutDir, "out.pdf")
	_, err = pdf.Split(pdf1, invalidOutPath, 1, 1)
	if err == nil {
		t.Errorf("expected error for Split writing to invalid output path")
	}

	// Merge empty input
	_, err = pdf.Merge([]string{}, filepath.Join(tmpDir, "out.pdf"))
	if err == nil {
		t.Errorf("expected error for Merge with empty input")
	}

	// Merge non-existent input
	_, err = pdf.Merge([]string{nonExistent}, filepath.Join(tmpDir, "out.pdf"))
	if err == nil {
		t.Errorf("expected error for Merge with nonexistent file")
	}

	// Merge invalid PDF input
	_, err = pdf.Merge([]string{invalidPdf}, filepath.Join(tmpDir, "out.pdf"))
	if err == nil {
		t.Errorf("expected error for Merge with invalid pdf")
	}

	// ExtractImages non-existent input
	_, err = pdf.ExtractImages(nonExistent, "", []int{1})
	if err == nil {
		t.Errorf("expected error for ExtractImages on nonexistent file")
	}

	// ExtractImages on invalid pdf file
	_, err = pdf.ExtractImages(invalidPdf, tmpDir, []int{1})
	if err == nil {
		t.Errorf("expected error for ExtractImages on invalid pdf")
	}

	// ExtractPages non-existent input
	_, err = pdf.ExtractPages(nonExistent, "", 150, "png", 1, 0, true)
	if err == nil {
		t.Errorf("expected error for ExtractPages on nonexistent file")
	}

	// ExtractPages unsupported format
	_, err = pdf.ExtractPages(pdf1, tmpDir, 150, "gif", 1, 0, true)
	if err == nil {
		t.Errorf("expected error for unsupported format")
	}

	// ExtractPages invalid page range
	_, err = pdf.ExtractPages(pdf1, tmpDir, 150, "png", 100, 200, true)
	if err == nil {
		t.Errorf("expected error for invalid page range")
	}
}

func TestPDFExtractText_Japanese(t *testing.T) {
	candidates := []string{
		"../../240920 IDC CIO Summit ホワイトカラーの生産性はなぜ低いのか R01.pdf",
		"../240920 IDC CIO Summit ホワイトカラーの生産性はなぜ低いのか R01.pdf",
		"240920 IDC CIO Summit ホワイトカラーの生産性はなぜ低いのか R01.pdf",
	}

	var samplePDF string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			samplePDF = c
			break
		}
	}

	if samplePDF == "" {
		t.Skip("Japanese sample PDF not found, skipping")
	}

	txt, err := pdf.ExtractText(samplePDF, "", 0, 0)
	if err != nil {
		t.Fatalf("ExtractText failed on Japanese PDF: %v", err)
	}

	if !strings.Contains(txt, "ホワイトカラーの生産性はなぜ低いのか") {
		t.Errorf("expected extracted text to contain 'ホワイトカラーの生産性はなぜ低いのか'")
	}
	if !strings.Contains(txt, "SAPジャパン") {
		t.Errorf("expected extracted text to contain 'SAPジャパン'")
	}
	if strings.Contains(txt, "/Artifact BMC") {
		t.Errorf("extracted text should not contain raw drawing operator '/Artifact BMC'")
	}
}

