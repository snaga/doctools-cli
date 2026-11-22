package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/mcp"
)

func TestSearch_PDF(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "search_sample.pdf")
	createSamplePDF(t, pdfPath) // Contains "Hello PDF"

	ctx := context.Background()

	t.Run("matching query case-insensitive", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: pdfPath,
			Query:    "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success status, got %+v", res)
		}
		if !strings.Contains(res.Content, "Found 1 match(es)") || !strings.Contains(res.Content, "Page 1") {
			t.Errorf("expected match on Page 1, got %q", res.Content)
		}
		if !strings.Contains(res.Content, "[Hello]") {
			t.Errorf("expected highlighted snippet with brackets, got %q", res.Content)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: pdfPath,
			Query:    "nonexistent_term",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success status, got %+v", res)
		}
		if !strings.Contains(res.Content, `No matches found for "nonexistent_term"`) {
			t.Errorf("expected 'No matches found' message, got %q", res.Content)
		}
	})
}

func TestSearch_Excel(t *testing.T) {
	tempDir := t.TempDir()
	excelPath := filepath.Join(tempDir, "search_sample.xlsx")
	createSampleExcel(t, excelPath) // Contains "Book" in Sheet1!A2

	ctx := context.Background()

	t.Run("matching query across sheets", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: excelPath,
			Query:    "book",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "Sheet1!A2") || !strings.Contains(res.Content, "[Book]") {
			t.Errorf("expected match at Sheet1!A2, got %q", res.Content)
		}
	})

	t.Run("specific sheet option", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: excelPath,
			Query:    "book",
			Options: &mcp.DoctoolsRunOptions{
				Sheet: "Sheet1",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" || !strings.Contains(res.Content, "Sheet1!A2") {
			t.Errorf("expected match in Sheet1, got %+v", res)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: excelPath,
			Query:    "zebra",
		})
		if err != nil || !strings.Contains(res.Content, "No matches found") {
			t.Errorf("expected no matches found, got: %+v, err: %v", res, err)
		}
	})
}

func TestSearch_PPTX(t *testing.T) {
	tempDir := t.TempDir()
	pptxPath := filepath.Join(tempDir, "search_sample.pptx")
	createSamplePPTX(t, pptxPath) // Contains "Slide 1 Intro"

	ctx := context.Background()

	t.Run("matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: pptxPath,
			Query:    "intro",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "Slide 1") || !strings.Contains(res.Content, "[Intro]") {
			t.Errorf("expected match in Slide 1, got %q", res.Content)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: pptxPath,
			Query:    "nonexistent",
		})
		if err != nil || !strings.Contains(res.Content, "No matches found") {
			t.Errorf("expected no matches found, got %+v", res)
		}
	})
}

func TestSearch_CSV(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "search_sample.csv")
	createSampleCSV(t, csvPath) // Contains Widget, Gadget

	ctx := context.Background()

	t.Run("matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: csvPath,
			Query:    "gadget",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "Row 3") || !strings.Contains(res.Content, "[Gadget]") {
			t.Errorf("expected match on Row 3, got %q", res.Content)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: csvPath,
			Query:    "spaceship",
		})
		if err != nil || !strings.Contains(res.Content, "No matches found") {
			t.Errorf("expected no matches found, got %+v", res)
		}
	})
}

func TestSearch_Text(t *testing.T) {
	tempDir := t.TempDir()
	txtPath := filepath.Join(tempDir, "search_sample.txt")
	content := "Line 1: Introduction\nLine 2: Important Keyword Here\nLine 3: Conclusion\n"
	_ = os.WriteFile(txtPath, []byte(content), 0644)

	ctx := context.Background()

	t.Run("matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: txtPath,
			Query:    "keyword",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "Line 2") || !strings.Contains(res.Content, "[Keyword]") {
			t.Errorf("expected match on Line 2, got %q", res.Content)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: txtPath,
			Query:    "missing_word",
		})
		if err != nil || !strings.Contains(res.Content, "No matches found") {
			t.Errorf("expected no matches found, got %+v", res)
		}
	})
}

func TestSearch_HTML(t *testing.T) {
	tempDir := t.TempDir()
	htmlPath := filepath.Join(tempDir, "search_sample.html")
	createSampleHTML(t, htmlPath) // Contains "Sample Heading", "Sample paragraph body text"

	ctx := context.Background()

	t.Run("matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: htmlPath,
			Query:    "paragraph",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("expected success, got %+v", res)
		}
		if !strings.Contains(res.Content, "[paragraph]") {
			t.Errorf("expected highlighted snippet for paragraph, got %q", res.Content)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: htmlPath,
			Query:    "extraterrestrial",
		})
		if err != nil || !strings.Contains(res.Content, "No matches found") {
			t.Errorf("expected no matches found, got %+v", res)
		}
	})
}

func TestSearch_Errors(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	t.Run("corrupt pdf search", func(t *testing.T) {
		corruptPDF := filepath.Join(tempDir, "bad.pdf")
		_ = os.WriteFile(corruptPDF, []byte("broken pdf"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: corruptPDF,
			Query:    "term",
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to open PDF") {
			t.Errorf("expected error on corrupt pdf search, got %+v", res)
		}
	})

	t.Run("corrupt pptx search", func(t *testing.T) {
		corruptPPTX := filepath.Join(tempDir, "bad.pptx")
		_ = os.WriteFile(corruptPPTX, []byte("broken pptx"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: corruptPPTX,
			Query:    "term",
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to extract PPTX") {
			t.Errorf("expected error on corrupt pptx search, got %+v", res)
		}
	})

	t.Run("corrupt excel search", func(t *testing.T) {
		corruptExcel := filepath.Join(tempDir, "bad.xlsx")
		_ = os.WriteFile(corruptExcel, []byte("broken excel"), 0644)
		res, err := mcp.HandleDoctoolsRun(ctx, mcp.DoctoolsRunParams{
			Command:  "search",
			FilePath: corruptExcel,
			Query:    "term",
		})
		if err != nil || res.Status != "error" || !strings.Contains(res.Message, "failed to search Excel") {
			t.Errorf("expected error on corrupt excel search, got %+v", res)
		}
	})
}

func TestSearch_MakeSnippetAndFormat(t *testing.T) {
	// Search in a text with long context before and after
	tempDir := t.TempDir()
	txtPath := filepath.Join(tempDir, "long_context.txt")
	longContent := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam."
	_ = os.WriteFile(txtPath, []byte(longContent), 0644)

	res, err := mcp.HandleDoctoolsRun(context.Background(), mcp.DoctoolsRunParams{
		Command:  "search",
		FilePath: txtPath,
		Query:    "tempor",
	})
	if err != nil || res.Status != "success" {
		t.Fatalf("unexpected search result: %+v, err: %v", res, err)
	}
	if !strings.Contains(res.Content, "[tempor]") {
		t.Errorf("expected snippet containing [tempor], got %q", res.Content)
	}
	if !strings.Contains(res.Content, "...") {
		t.Errorf("expected ellipsis in long snippet, got %q", res.Content)
	}
}
