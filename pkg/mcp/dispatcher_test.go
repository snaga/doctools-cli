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
		if res.Status != "error" || !strings.Contains(res.Message, "unsupported file extension") {
			t.Errorf("expected unsupported extension error, got: %+v", res)
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
		if res.Status != "error" || !strings.Contains(res.Message, "Word (.docx) format is not yet supported") {
			t.Errorf("expected Word unsupported error, got: %+v", res)
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
