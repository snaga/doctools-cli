package mcp_test

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/mcp"
	"github.com/xuri/excelize/v2"
)

func createSamplePDF(t *testing.T, path string) {
	t.Helper()
	body := "1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n" +
		"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
		"5 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello PDF) Tj\nET\nendstream\nendobj\n"

	header := "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"
	o1 := len(header)
	o2 := o1 + len("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	o3 := o2 + len("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	o4 := o3 + len("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n")
	o5 := o4 + len("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	xrefOffset := len(header) + len(body)
	xref := fmt.Sprintf("xref\n0 6\n0000000000 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \ntrailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", o1, o2, o3, o4, o5, xrefOffset)
	if err := os.WriteFile(path, []byte(header+body+xref), 0644); err != nil {
		t.Fatalf("failed to create sample pdf: %v", err)
	}
}

func createSampleExcel(t *testing.T, path string) {
	t.Helper()
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Item")
	f.SetCellValue("Sheet1", "B1", "Price")
	f.SetCellValue("Sheet1", "A2", "Book")
	f.SetCellValue("Sheet1", "B2", 15)
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("failed to save excel: %v", err)
	}
}

func createSamplePPTX(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create pptx: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	slideXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Slide 1 Intro</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`
	w, err := zw.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("failed to write slide: %v", err)
	}
	w.Write([]byte(slideXML))
	zw.Close()
}

func createSampleCSV(t *testing.T, path string) {
	t.Helper()
	content := "Product,Count,Status\nWidget,100,InStock\nGadget,50,OutOfStock\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}
}

func createSampleHTML(t *testing.T, path string) {
	t.Helper()
	content := "<html><body><h1>Sample Heading</h1><p>Sample paragraph body text.</p></body></html>"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write html: %v", err)
	}
}

func TestRead_PDF(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	createSamplePDF(t, pdfPath)

	ctx := context.Background()

	t.Run("default read", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pdfPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if res.TotalPages <= 0 {
			t.Errorf("expected TotalPages > 0, got %d", res.TotalPages)
		}
		if !strings.Contains(res.Content, "Hello PDF") {
			t.Errorf("expected content to contain 'Hello PDF', got %q", res.Content)
		}
	})

	t.Run("valid pages option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages: "1",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
	})

	t.Run("invalid pages option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages: "invalid-range",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "invalid pages option") {
			t.Errorf("expected error for invalid pages, got %+v", res)
		}
	})
}

func TestRead_Excel(t *testing.T) {
	tempDir := t.TempDir()
	excelPath := filepath.Join(tempDir, "sample.xlsx")
	createSampleExcel(t, excelPath)

	ctx := context.Background()

	t.Run("default read", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: excelPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if res.TotalPages <= 0 {
			t.Errorf("expected TotalPages > 0, got %d", res.TotalPages)
		}
		if !strings.Contains(res.Content, "Book") || !strings.Contains(res.Content, "Price") {
			t.Errorf("expected markdown table with 'Book' and 'Price', got %q", res.Content)
		}
	})

	t.Run("specific sheet option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: excelPath,
			Options: &mcp.DoctoolsRunOptions{
				Sheet: "Sheet1",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
	})
}

func TestRead_PPTX(t *testing.T) {
	tempDir := t.TempDir()
	pptxPath := filepath.Join(tempDir, "sample.pptx")
	createSamplePPTX(t, pptxPath)

	ctx := context.Background()

	t.Run("default read", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pptxPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if res.TotalPages <= 0 {
			t.Errorf("expected TotalPages > 0, got %d", res.TotalPages)
		}
		if !strings.Contains(res.Content, "Slide 1 Intro") {
			t.Errorf("expected content to contain 'Slide 1 Intro', got %q", res.Content)
		}
	})

	t.Run("valid slides option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pptxPath,
			Options: &mcp.DoctoolsRunOptions{
				Slides: "1",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
	})

	t.Run("invalid slides option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pptxPath,
			Options: &mcp.DoctoolsRunOptions{
				Slides: "5-2",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "invalid slides option") {
			t.Errorf("expected error for invalid slides range, got %+v", res)
		}
	})
}

func TestRead_CSV(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "sample.csv")
	createSampleCSV(t, csvPath)

	ctx := context.Background()

	res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
		Command:  "read",
		FilePath: csvPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "success" {
		t.Errorf("expected success, got %+v", res)
	}
	if !strings.Contains(res.Content, "Widget") || !strings.Contains(res.Content, "| --- |") {
		t.Errorf("expected formatted markdown table, got %q", res.Content)
	}
}

func TestRead_Text(t *testing.T) {
	tempDir := t.TempDir()
	txtPath := filepath.Join(tempDir, "sample.md")
	_ = os.WriteFile(txtPath, []byte("# Markdown Title\nHello from text file!"), 0644)

	ctx := context.Background()

	res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
		Command:  "read",
		FilePath: txtPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "success" {
		t.Errorf("expected success, got %+v", res)
	}
	if !strings.Contains(res.Content, "Hello from text file!") {
		t.Errorf("expected content to match file text, got %q", res.Content)
	}
}

func TestRead_HTML(t *testing.T) {
	tempDir := t.TempDir()
	htmlPath := filepath.Join(tempDir, "sample.html")
	createSampleHTML(t, htmlPath)

	ctx := context.Background()

	res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
		Command:  "read",
		FilePath: htmlPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "success" {
		t.Errorf("expected success, got %+v", res)
	}
	if !strings.Contains(res.Content, "Sample Heading") {
		t.Errorf("expected extracted text from html, got %q", res.Content)
	}

	// Verify that no .md file was generated in the source directory
	unexpectedMd := filepath.Join(tempDir, "sample.md")
	if _, err := os.Stat(unexpectedMd); !os.IsNotExist(err) {
		t.Errorf("expected %s NOT to be created, but it exists", unexpectedMd)
	}
}

func TestSmartDynamicResponse_ThresholdHandling(t *testing.T) {
	tempDir := t.TempDir()
	txtPath := filepath.Join(tempDir, "large.txt")
	defer os.RemoveAll(".tmp")

	// Generate 1000 characters
	longText := strings.Repeat("ABCDEFGHIJ", 100) // 1000 chars
	_ = os.WriteFile(txtPath, []byte(longText), 0644)

	ctx := context.Background()

	t.Run("inline return below threshold", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: txtPath,
			Options: &mcp.DoctoolsRunOptions{
				MaxInlineChars: 2000,
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected status 'success', got %q", res.Status)
		}
		if res.Content != longText {
			t.Errorf("expected full inline content returned")
		}
		if res.CharCount != 1000 {
			t.Errorf("expected char_count 1000, got %d", res.CharCount)
		}
	})

	t.Run("saved to file when exceeding threshold", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: txtPath,
			Options: &mcp.DoctoolsRunOptions{
				MaxInlineChars: 500, // less than 1000
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "saved_to_file" {
			t.Errorf("expected status 'saved_to_file', got %q", res.Status)
		}
		if res.Content != "" {
			t.Errorf("expected content to be omitted/empty in saved_to_file response")
		}
		if res.CharCount != 1000 {
			t.Errorf("expected char_count 1000, got %d", res.CharCount)
		}
		if !strings.Contains(res.Message, "Content exceeded inline threshold") {
			t.Errorf("expected context protection message, got %q", res.Message)
		}

		// Verify saved file
		savedBytes, err := os.ReadFile(res.FilePath)
		if err != nil {
			t.Fatalf("failed to read saved file %s: %v", res.FilePath, err)
		}
		if string(savedBytes) != longText {
			t.Errorf("saved file content does not match original text")
		}
	})

	t.Run("explicit output_path option", func(t *testing.T) {
		customOut := filepath.Join(tempDir, "custom_extracted.md")
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: txtPath,
			Options: &mcp.DoctoolsRunOptions{
				MaxInlineChars: 100,
				OutputPath:     customOut,
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "saved_to_file" {
			t.Errorf("expected status 'saved_to_file', got %q", res.Status)
		}
		if res.FilePath != customOut {
			t.Errorf("expected saved file path %q, got %q", customOut, res.FilePath)
		}

		if _, err := os.Stat(customOut); os.IsNotExist(err) {
			t.Errorf("custom output file was not created: %s", customOut)
		}
	})
}

func TestRead_CSV_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	t.Run("empty csv", func(t *testing.T) {
		emptyPath := filepath.Join(tempDir, "empty.csv")
		_ = os.WriteFile(emptyPath, []byte(""), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: emptyPath,
		})
		if err != nil || res.Status != "success" || res.Content != "" {
			t.Errorf("expected empty success content, got: %+v, err: %v", res, err)
		}
	})

	t.Run("csv with escaped pipes", func(t *testing.T) {
		pipePath := filepath.Join(tempDir, "pipes.csv")
		_ = os.WriteFile(pipePath, []byte("Col|A,Col|B\nval|1,val|2\n"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: pipePath,
		})
		if err != nil || res.Status != "success" {
			t.Fatalf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "Col\\|A") || !strings.Contains(res.Content, "val\\|1") {
			t.Errorf("expected pipes to be escaped in markdown, got %q", res.Content)
		}
	})
}

func TestRead_ExtractionErrors(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	t.Run("corrupt excel", func(t *testing.T) {
		corruptExcel := filepath.Join(tempDir, "corrupt.xlsx")
		_ = os.WriteFile(corruptExcel, []byte("not a zip"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: corruptExcel,
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to extract Excel") {
			t.Errorf("expected excel extract error, got %+v", res)
		}
	})

	t.Run("corrupt pptx", func(t *testing.T) {
		corruptPPTX := filepath.Join(tempDir, "corrupt.pptx")
		_ = os.WriteFile(corruptPPTX, []byte("not a zip"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: corruptPPTX,
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to extract PPTX") {
			t.Errorf("expected pptx extract error, got %+v", res)
		}
	})

	t.Run("corrupt pdf", func(t *testing.T) {
		corruptPDF := filepath.Join(tempDir, "corrupt.pdf")
		_ = os.WriteFile(corruptPDF, []byte("not a pdf"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: corruptPDF,
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to extract PDF") {
			t.Errorf("expected pdf extract error, got %+v", res)
		}
	})
}
