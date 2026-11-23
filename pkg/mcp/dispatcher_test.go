package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/mcp"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected mcp.FileFormat
	}{
		{"pdf lower", "sample.pdf", mcp.FormatPDF},
		{"pdf upper", "SAMPLE.PDF", mcp.FormatPDF},
		{"xlsx", "data.xlsx", mcp.FormatExcel},
		{"xlsm", "data.xlsm", mcp.FormatExcel},
		{"xls", "data.xls", mcp.FormatExcel},
		{"pptx", "presentation.pptx", mcp.FormatPPTX},
		{"csv", "records.csv", mcp.FormatCSV},
		{"txt", "notes.txt", mcp.FormatText},
		{"md", "README.md", mcp.FormatText},
		{"markdown", "spec.markdown", mcp.FormatText},
		{"json", "config.json", mcp.FormatText},
		{"yaml", "config.yaml", mcp.FormatText},
		{"yml", "config.yml", mcp.FormatText},
		{"html", "index.html", mcp.FormatHTML},
		{"htm", "index.htm", mcp.FormatHTML},
		{"docx", "doc.docx", mcp.FormatWord},
		{"unknown", "binary.bin", mcp.FormatUnknown},
		{"no extension", "file_without_ext", mcp.FormatUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mcp.DetectFormat(tc.filePath)
			if got != tc.expected {
				t.Errorf("DetectFormat(%q) = %q, expected %q", tc.filePath, got, tc.expected)
			}
		})
	}
}

func TestDispatcher_Handle_Validation(t *testing.T) {
	tempDir := t.TempDir()
	validPDF := filepath.Join(tempDir, "test.pdf")
	if err := os.WriteFile(validPDF, []byte("%PDF-1.4 sample"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ctx := context.Background()

	t.Run("empty file_path", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: "   ",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "file_path is required") {
			t.Errorf("expected error status for empty file_path, got: %+v", res)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: filepath.Join(tempDir, "does_not_exist.pdf"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "file not found") {
			t.Errorf("expected file not found error, got: %+v", res)
		}
	})

	t.Run("directory path", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: tempDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "directory, not a file") {
			t.Errorf("expected directory error, got: %+v", res)
		}
	})

	t.Run("invalid command", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "delete",
			FilePath: validPDF,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "invalid command") {
			t.Errorf("expected invalid command error, got: %+v", res)
		}
	})

	t.Run("search missing query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: validPDF,
			Query:    "   ",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "query is required") {
			t.Errorf("expected query required error, got: %+v", res)
		}
	})

	t.Run("unsupported extension", func(t *testing.T) {
		unsupportedFile := filepath.Join(tempDir, "test.exe")
		_ = os.WriteFile(unsupportedFile, []byte("binary"), 0644)

		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: unsupportedFile,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "unsupported file extension") || !strings.Contains(res.Message, "Supported formats:") {
			t.Errorf("expected unsupported extension error with supported formats list, got: %+v", res)
		}
	})

	t.Run("unsupported Word docx", func(t *testing.T) {
		docxFile := filepath.Join(tempDir, "test.docx")
		_ = os.WriteFile(docxFile, []byte("docx"), 0644)

		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: docxFile,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "Word (.docx) format is not yet supported") || !strings.Contains(res.Message, "Supported formats:") {
			t.Errorf("expected Word unsupported error with supported formats list, got: %+v", res)
		}
	})
}

func TestDispatcher_Handle_SupportedExtensions(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	d := mcp.NewDispatcher()
	// Override read and search handlers with stubs confirming format routing
	for _, fmtType := range []mcp.FileFormat{
		mcp.FormatPDF, mcp.FormatExcel, mcp.FormatPPTX, mcp.FormatCSV, mcp.FormatText, mcp.FormatHTML,
	} {
		f := fmtType
		d.RegisterReadHandler(f, func(ctx context.Context, format mcp.FileFormat, params mcp.DoctoolsRunParams) (*mcp.DoctoolsRunResult, error) {
			return &mcp.DoctoolsRunResult{
				Status:  "success",
				Content: string(format),
			}, nil
		})
		d.RegisterSearchHandler(f, func(ctx context.Context, format mcp.FileFormat, params mcp.DoctoolsRunParams) (*mcp.DoctoolsRunResult, error) {
			return &mcp.DoctoolsRunResult{
				Status:  "success",
				Content: fmt.Sprintf("%s:%s", format, params.Query),
			}, nil
		})
	}

	supportedExts := []struct {
		ext    string
		format mcp.FileFormat
	}{
		{".pdf", mcp.FormatPDF},
		{".xlsx", mcp.FormatExcel},
		{".xlsm", mcp.FormatExcel},
		{".xls", mcp.FormatExcel},
		{".pptx", mcp.FormatPPTX},
		{".csv", mcp.FormatCSV},
		{".txt", mcp.FormatText},
		{".md", mcp.FormatText},
		{".markdown", mcp.FormatText},
		{".json", mcp.FormatText},
		{".yaml", mcp.FormatText},
		{".yml", mcp.FormatText},
		{".html", mcp.FormatHTML},
		{".htm", mcp.FormatHTML},
	}

	for _, item := range supportedExts {
		filePath := filepath.Join(tempDir, "sample"+item.ext)
		if err := os.WriteFile(filePath, []byte("dummy content"), 0644); err != nil {
			t.Fatalf("failed to create file for %s: %v", item.ext, err)
		}

		// Test read command
		t.Run("read "+item.ext, func(t *testing.T) {
			res, err := d.Handle(ctx, mcp.DoctoolsRunParams{
				Command:  "read",
				FilePath: filePath,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Status != "success" {
				t.Errorf("expected success status for %s, got %+v", item.ext, res)
			}
			if !strings.Contains(res.Content, string(item.format)) {
				t.Errorf("expected content to mention format %q, got %q", item.format, res.Content)
			}
		})

		// Test search command
		t.Run("search "+item.ext, func(t *testing.T) {
			res, err := d.Handle(ctx, mcp.DoctoolsRunParams{
				Command:  "search",
				FilePath: filePath,
				Query:    "sample",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Status != "success" {
				t.Errorf("expected success status for search %s, got %+v", item.ext, res)
			}
			if !strings.Contains(res.Content, "sample") {
				t.Errorf("expected content to contain query 'sample', got %q", res.Content)
			}
		})
	}
}

func TestDispatcher_CustomHandlers(t *testing.T) {
	tempDir := t.TempDir()
	samplePDF := filepath.Join(tempDir, "sample.pdf")
	_ = os.WriteFile(samplePDF, []byte("%PDF-1.4 sample"), 0644)

	d := mcp.NewDispatcher()

	readCalled := false
	d.RegisterReadHandler(mcp.FormatPDF, func(ctx context.Context, format mcp.FileFormat, params mcp.DoctoolsRunParams) (*mcp.DoctoolsRunResult, error) {
		readCalled = true
		return &mcp.DoctoolsRunResult{
			Status:  "success",
			Content: "custom pdf read executed",
		}, nil
	})

	searchCalled := false
	d.RegisterSearchHandler(mcp.FormatPDF, func(ctx context.Context, format mcp.FileFormat, params mcp.DoctoolsRunParams) (*mcp.DoctoolsRunResult, error) {
		searchCalled = true
		return &mcp.DoctoolsRunResult{
			Status:  "success",
			Content: "custom pdf search executed",
		}, nil
	})

	ctx := context.Background()

	// Test read handler registration
	resRead, err := d.Handle(ctx, mcp.DoctoolsRunParams{
		Command:  "read",
		FilePath: samplePDF,
	})
	if err != nil || !readCalled || resRead.Content != "custom pdf read executed" {
		t.Errorf("custom read handler failed: readCalled=%v, err=%v, res=%+v", readCalled, err, resRead)
	}

	// Test search handler registration
	resSearch, err := d.Handle(ctx, mcp.DoctoolsRunParams{
		Command:  "search",
		FilePath: samplePDF,
		Query:    "find me",
	})
	if err != nil || !searchCalled || resSearch.Content != "custom pdf search executed" {
		t.Errorf("custom search handler failed: searchCalled=%v, err=%v, res=%+v", searchCalled, err, resSearch)
	}
}

func TestDispatcher_HandleToolCall(t *testing.T) {
	tempDir := t.TempDir()
	sampleFile := filepath.Join(tempDir, "data.csv")
	_ = os.WriteFile(sampleFile, []byte("a,b,c\n1,2,3"), 0644)

	d := mcp.NewDispatcher()
	ctx := context.Background()

	t.Run("valid doctools_run", func(t *testing.T) {
		args, _ := json.Marshal(mcp.DoctoolsRunParams{
			Command:  "read",
			FilePath: sampleFile,
		})
		callRes, err := d.HandleToolCall(ctx, "doctools_run", args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callRes.IsError {
			t.Errorf("expected isError to be false, got true: %+v", callRes)
		}
		if len(callRes.Content) == 0 {
			t.Fatalf("expected non-empty content")
		}

		var runRes mcp.DoctoolsRunResult
		if err := json.Unmarshal([]byte(callRes.Content[0].Text), &runRes); err != nil {
			t.Fatalf("failed to unmarshal content: %v", err)
		}
		if runRes.Status != "success" {
			t.Errorf("expected success status, got %q", runRes.Status)
		}
	})

	t.Run("unknown tool name", func(t *testing.T) {
		callRes, err := d.HandleToolCall(ctx, "other_tool", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !callRes.IsError {
			t.Errorf("expected isError=true for unknown tool")
		}
		if !strings.Contains(callRes.Content[0].Text, "unknown tool") {
			t.Errorf("expected 'unknown tool' text, got %q", callRes.Content[0].Text)
		}
	})

	t.Run("invalid arguments JSON", func(t *testing.T) {
		callRes, err := d.HandleToolCall(ctx, "doctools_run", json.RawMessage(`{invalid`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !callRes.IsError {
			t.Errorf("expected isError=true for invalid JSON")
		}
		if !strings.Contains(callRes.Content[0].Text, "invalid arguments JSON") {
			t.Errorf("expected invalid arguments message, got %q", callRes.Content[0].Text)
		}
	})
}

func createTestPDFForCapture(t *testing.T, path string) {
	t.Helper()
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

func TestDispatcher_Capture(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	createTestPDFForCapture(t, pdfPath)

	outDir := filepath.Join(tempDir, "out_capture")
	d := mcp.NewDispatcher()
	ctx := context.Background()

	t.Run("single page capture inline base64 and HandleToolCall", func(t *testing.T) {
		params := mcp.DoctoolsRunParams{
			Command:  "capture",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages:      "1",
				OutputPath: outDir,
			},
		}

		res, err := d.Handle(ctx, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected status 'success', got %q", res.Status)
		}
		if res.ImageBase64 == "" {
			t.Errorf("expected non-empty ImageBase64 for single page inline capture")
		}
		if res.ImageMIME != "image/png" {
			t.Errorf("expected image/png MIME, got %q", res.ImageMIME)
		}
		if len(res.Images) != 1 {
			t.Fatalf("expected 1 captured image, got %d", len(res.Images))
		}
		if !strings.Contains(res.Content, "Captured page 1") {
			t.Errorf("expected content to mention Captured page 1, got %q", res.Content)
		}

		// HandleToolCall verification
		argsJSON, _ := json.Marshal(params)
		callRes, err := d.HandleToolCall(ctx, "doctools_run", argsJSON)
		if err != nil {
			t.Fatalf("HandleToolCall failed: %v", err)
		}
		if callRes.IsError {
			t.Errorf("expected IsError=false")
		}

		// Verify Blocker-1 prevention: text content chunk MUST NOT contain Base64 data
		for _, c := range callRes.Content {
			if c.Type == "text" {
				if strings.Contains(c.Text, "image_base64") {
					t.Errorf("expected text content JSON NOT to include image_base64 field, but found it")
				}
				if strings.Contains(c.Text, res.ImageBase64) {
					t.Errorf("expected text content JSON NOT to leak huge Base64 data into text chunk")
				}
			}
		}

		var hasImageContent bool
		var imageContent mcp.ToolContent
		for _, c := range callRes.Content {
			if c.Type == "image" {
				hasImageContent = true
				imageContent = c
			}
		}
		if !hasImageContent {
			t.Fatalf("expected CallToolResult to contain ToolContent with type 'image'")
		}
		if imageContent.Data != res.ImageBase64 {
			t.Errorf("expected image content data to match ImageBase64")
		}
		if imageContent.MIMEType != "image/png" {
			t.Errorf("expected image MIMEType 'image/png', got %q", imageContent.MIMEType)
		}
	})

	t.Run("multi-page capture saved to file", func(t *testing.T) {
		// Mock capture handler returning 2 images to simulate multi-page
		multiD := mcp.NewDispatcher()
		dummyImg1 := filepath.Join(tempDir, "page_1.png")
		dummyImg2 := filepath.Join(tempDir, "page_2.png")
		_ = os.WriteFile(dummyImg1, []byte("png1"), 0644)
		_ = os.WriteFile(dummyImg2, []byte("png2"), 0644)

		multiD.RegisterCaptureHandler(mcp.FormatPDF, func(ctx context.Context, format mcp.FileFormat, params mcp.DoctoolsRunParams) (*mcp.DoctoolsRunResult, error) {
			imgs := []mcp.CapturedImage{
				{PageNumber: 1, FilePath: dummyImg1, Width: 100, Height: 100, FileSize: 4},
				{PageNumber: 2, FilePath: dummyImg2, Width: 100, Height: 100, FileSize: 4},
			}
			return &mcp.DoctoolsRunResult{
				Status:      "saved_to_file",
				Content:     "Captured 2 pages to directory",
				FilePath:    outDir,
				TotalPages:  2,
				Images:      imgs,
				ImageBase64: "",
			}, nil
		})

		params := mcp.DoctoolsRunParams{
			Command:  "capture",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages:      "1-2",
				OutputPath: outDir,
			},
		}

		res, err := multiD.Handle(ctx, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "saved_to_file" {
			t.Errorf("expected status 'saved_to_file', got %q", res.Status)
		}
		if res.ImageBase64 != "" {
			t.Errorf("expected empty ImageBase64 for multi-page, got %q", res.ImageBase64)
		}

		// Tool call verification
		argsJSON, _ := json.Marshal(params)
		callRes, err := multiD.HandleToolCall(ctx, "doctools_run", argsJSON)
		if err != nil {
			t.Fatalf("HandleToolCall failed: %v", err)
		}
		for _, c := range callRes.Content {
			if c.Type == "image" {
				t.Errorf("did not expect 'image' content for multi-page result: %+v", c)
			}
		}
	})

	t.Run("inline false opt-out for single page", func(t *testing.T) {
		inlineFalse := false
		params := mcp.DoctoolsRunParams{
			Command:  "capture",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages:      "1",
				OutputPath: outDir,
				Inline:     &inlineFalse,
			},
		}

		res, err := d.Handle(ctx, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "saved_to_file" {
			t.Errorf("expected status 'saved_to_file', got %q", res.Status)
		}
		if res.ImageBase64 != "" {
			t.Errorf("expected empty ImageBase64 when inline=false, got %q", res.ImageBase64)
		}
		if !strings.Contains(res.Content, "Captured 1 pages") {
			t.Errorf("expected file listing content, got %q", res.Content)
		}

		argsJSON, _ := json.Marshal(params)
		callRes, err := d.HandleToolCall(ctx, "doctools_run", argsJSON)
		if err != nil {
			t.Fatalf("HandleToolCall failed: %v", err)
		}
		for _, c := range callRes.Content {
			if c.Type == "image" {
				t.Errorf("did not expect 'image' content when inline=false")
			}
		}
	})

	t.Run("unsupported formats error", func(t *testing.T) {
		unsupportedExts := []string{".csv", ".txt", ".docx"}
		for _, ext := range unsupportedExts {
			filePath := filepath.Join(tempDir, "dummy"+ext)
			_ = os.WriteFile(filePath, []byte("sample"), 0644)

			params := mcp.DoctoolsRunParams{
				Command:  "capture",
				FilePath: filePath,
			}

			res, err := d.Handle(ctx, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Status != "error" {
				t.Errorf("expected status 'error' for unsupported format %s, got %q", ext, res.Status)
			}
			if !strings.Contains(res.Message, "capture is not supported for format") {
				t.Errorf("expected error message to contain format support info, got %v", res.Message)
			}

			argsJSON, _ := json.Marshal(params)
			callRes, err := d.HandleToolCall(ctx, "doctools_run", argsJSON)
			if err != nil {
				t.Fatalf("unexpected error from HandleToolCall: %v", err)
			}
			if !callRes.IsError {
				t.Errorf("expected IsError=true for unsupported format %s", ext)
			}
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		nonExistent := filepath.Join(tempDir, "does_not_exist.pdf")
		params := mcp.DoctoolsRunParams{
			Command:  "capture",
			FilePath: nonExistent,
		}

		res, err := d.Handle(ctx, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" || !strings.Contains(res.Message, "file not found") {
			t.Errorf("expected file not found error, got %+v", res)
		}
	})

	t.Run("out of range pages error", func(t *testing.T) {
		params := mcp.DoctoolsRunParams{
			Command:  "capture",
			FilePath: pdfPath,
			Options: &mcp.DoctoolsRunOptions{
				Pages: "99",
			},
		}

		res, err := d.Handle(ctx, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "error" {
			t.Errorf("expected status 'error' for out of range pages, got %q", res.Status)
		}
		if !strings.Contains(res.Message, "no valid pages in range") {
			t.Errorf("expected 'no valid pages in range' in message, got %v", res.Message)
		}

		argsJSON, _ := json.Marshal(params)
		callRes, err := d.HandleToolCall(ctx, "doctools_run", argsJSON)
		if err != nil {
			t.Fatalf("unexpected error from HandleToolCall: %v", err)
		}
		if !callRes.IsError {
			t.Errorf("expected IsError=true for out of range pages")
		}
	})
}
