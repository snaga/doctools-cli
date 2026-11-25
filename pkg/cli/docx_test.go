package cli

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createSampleDocxForCLI generates a minimal valid docx archive fixture for CLI tests.
func createSampleDocxForCLI(t *testing.T, path string, xmlContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("failed to create entry in zip: %v", err)
	}
	if _, err := w.Write([]byte(xmlContent)); err != nil {
		t.Fatalf("failed to write xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
}

// TestCLIDocxExtractText verifies the CLI docx extract-text command for stdout output,
// file output with --output, and error handling for invalid/missing files.
func TestCLIDocxExtractText(t *testing.T) {
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "test.docx")
	sampleXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading1"/>
      </w:pPr>
      <w:r><w:t>Test Heading</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Test Paragraph Content</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`
	createSampleDocxForCLI(t, docxPath, sampleXML)

	t.Run("stdout with --json", func(t *testing.T) {
		testCLIDocxExtractTextStdout(t, docxPath)
	})

	t.Run("with --output flag", func(t *testing.T) {
		testCLIDocxExtractTextOutput(t, tmpDir, docxPath)
	})

	t.Run("error: non-existent file", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "missing.docx")
		_, _, exit := runCLI("docx", "extract-text", nonExistent)
		if exit == nil {
			t.Fatalf("expected error exit for non-existent file")
		}
	})

	t.Run("error: corrupt file", func(t *testing.T) {
		corruptPath := filepath.Join(tmpDir, "corrupt.docx")
		if err := os.WriteFile(corruptPath, []byte("not a zip file"), 0644); err != nil {
			t.Fatalf("failed to write corrupt file: %v", err)
		}
		_, _, exit := runCLI("docx", "extract-text", corruptPath)
		if exit == nil {
			t.Fatalf("expected error exit for corrupt file")
		}
	})

	t.Run("error: legacy .doc file", func(t *testing.T) {
		docPath := filepath.Join(tmpDir, "legacy.doc")
		if err := os.WriteFile(docPath, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, 0644); err != nil {
			t.Fatalf("failed to write legacy doc file: %v", err)
		}
		_, _, exit := runCLI("docx", "extract-text", docPath)
		if exit == nil {
			t.Fatalf("expected error exit for legacy doc file")
		}
	})
}

// testCLIDocxExtractTextStdout verifies stdout extraction and JSON structure.
func testCLIDocxExtractTextStdout(t *testing.T, docxPath string) {
	t.Helper()
	out, cmdErr, exit := runCLI("docx", "extract-text", docxPath, "--json")
	if exit != nil {
		t.Fatalf("unexpected exit: %v", exit.err)
	}
	if cmdErr != nil {
		t.Fatalf("unexpected cmdErr: %v", cmdErr)
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			OutputPath string `json:"output_path"`
			Content    string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("failed to parse json output %q: %v", out, err)
	}
	if res.Status != "success" {
		t.Errorf("expected status 'success', got %q", res.Status)
	}
	if !strings.Contains(res.Data.Content, "# Test Heading") {
		t.Errorf("expected heading in content, got %q", res.Data.Content)
	}
	if !strings.Contains(res.Data.Content, "Test Paragraph Content") {
		t.Errorf("expected paragraph in content, got %q", res.Data.Content)
	}
}

// testCLIDocxExtractTextOutput verifies saving to a specified output file path.
func testCLIDocxExtractTextOutput(t *testing.T, tmpDir, docxPath string) {
	t.Helper()
	outputPath := filepath.Join(tmpDir, "output.md")
	out, cmdErr, exit := runCLI("docx", "extract-text", docxPath, "-o", outputPath, "--json")
	if exit != nil {
		t.Fatalf("unexpected exit: %v", exit.err)
	}
	if cmdErr != nil {
		t.Fatalf("unexpected cmdErr: %v", cmdErr)
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			OutputPath string `json:"output_path"`
			Content    string `json:"content"`
			Length     int    `json:"length"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("failed to parse json output %q: %v", out, err)
	}
	if res.Status != "success" {
		t.Errorf("expected status 'success', got %q", res.Status)
	}
	if res.Data.OutputPath != outputPath {
		t.Errorf("expected output_path %q, got %q", outputPath, res.Data.OutputPath)
	}
	if res.Data.Content != "" {
		t.Errorf("expected empty content when output is specified, got %q", res.Data.Content)
	}
	if res.Data.Length == 0 {
		t.Errorf("expected non-zero length")
	}

	contentBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	contentStr := string(contentBytes)
	if !strings.Contains(contentStr, "# Test Heading") {
		t.Errorf("expected heading in file, got %q", contentStr)
	}
}
