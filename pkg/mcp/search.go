package mcp

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/html"
	"doctools-cli/pkg/pdf"
	"doctools-cli/pkg/pptx"
)

// MaxSearchHits defines the maximum number of matches returned for a single search.
const MaxSearchHits = 50

// SearchHit represents a single matched location and context snippet.
type SearchHit struct {
	Location string `json:"location"`
	Snippet  string `json:"snippet"`
}

// RegisterDefaultSearchHandlers registers production search handlers to the given dispatcher.
func RegisterDefaultSearchHandlers(d *Dispatcher) {
	d.RegisterSearchHandler(FormatPDF, handleSearchPDF)
	d.RegisterSearchHandler(FormatExcel, handleSearchExcel)
	d.RegisterSearchHandler(FormatPPTX, handleSearchPPTX)
	d.RegisterSearchHandler(FormatCSV, handleSearchCSV)
	d.RegisterSearchHandler(FormatText, handleSearchText)
	d.RegisterSearchHandler(FormatHTML, handleSearchHTML)
}

func handleSearchPDF(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	pageTexts, err := pdf.ExtractPageTexts(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to open PDF file %s: %v", params.FilePath, err),
		}, nil
	}

	totalPages := len(pageTexts)
	startPage := 1
	endPage := totalPages
	if params.Options != nil && strings.TrimSpace(params.Options.Pages) != "" {
		sp, ep, err := parseRange(params.Options.Pages)
		if err != nil {
			return &DoctoolsRunResult{
				Status:   "error",
				FilePath: params.FilePath,
				Message:  fmt.Sprintf("invalid pages option %q: %v", params.Options.Pages, err),
			}, nil
		}
		if sp >= 1 {
			startPage = sp
		}
		if ep >= 1 && ep <= totalPages {
			endPage = ep
		}
	}

	var hits []SearchHit
	lowerQuery := strings.ToLower(params.Query)

	for p := startPage; p <= endPage; p++ {
		text := pageTexts[p]
		if strings.Contains(strings.ToLower(text), lowerQuery) {
			snippet := makeSnippet(text, params.Query, 60)
			hits = append(hits, SearchHit{
				Location: fmt.Sprintf("Page %d", p),
				Snippet:  snippet,
			})
			if len(hits) >= MaxSearchHits {
				break
			}
		}
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:     "success",
		Content:    formatted,
		FilePath:   params.FilePath,
		CharCount:  utf8.RuneCountInString(formatted),
		TotalPages: totalPages,
	}, nil
}

func handleSearchExcel(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	var sheets []string
	if params.Options != nil && strings.TrimSpace(params.Options.Sheet) != "" {
		sheets = []string{strings.TrimSpace(params.Options.Sheet)}
	}

	result, err := excel.SearchCell(params.FilePath, params.Query, sheets)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to search Excel file %s: %v", params.FilePath, err),
		}, nil
	}

	var hits []SearchHit
	for _, match := range result.Matches {
		snippet := makeSnippet(match.Value, params.Query, 60)
		if snippet == "" {
			snippet = fmt.Sprintf("[%s]", match.Value)
		}
		loc := fmt.Sprintf("%s!%s", match.Sheet, match.Cell)
		hits = append(hits, SearchHit{
			Location: loc,
			Snippet:  snippet,
		})
		if len(hits) >= MaxSearchHits {
			break
		}
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   formatted,
		FilePath:  params.FilePath,
		CharCount: utf8.RuneCountInString(formatted),
	}, nil
}

func handleSearchPPTX(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	text, err := pptx.ExtractTextPureGo(params.FilePath, "", 1, 0)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to extract PPTX text from %s: %v", params.FilePath, err),
		}, nil
	}

	var hits []SearchHit
	lowerQuery := strings.ToLower(params.Query)
	slides := strings.Split(text, "## Slide ")

	for _, s := range slides {
		sTrim := strings.TrimSpace(s)
		if sTrim == "" {
			continue
		}
		lines := strings.SplitN(sTrim, "\n", 2)
		slideNumStr := strings.TrimSpace(lines[0])
		slideContent := ""
		if len(lines) > 1 {
			slideContent = lines[1]
		}
		if strings.Contains(strings.ToLower(slideContent), lowerQuery) || strings.Contains(strings.ToLower(slideNumStr), lowerQuery) {
			snippet := makeSnippet(slideContent, params.Query, 60)
			hits = append(hits, SearchHit{
				Location: fmt.Sprintf("Slide %s", slideNumStr),
				Snippet:  snippet,
			})
			if len(hits) >= MaxSearchHits {
				break
			}
		}
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   formatted,
		FilePath:  params.FilePath,
		CharCount: utf8.RuneCountInString(formatted),
	}, nil
}

func handleSearchCSV(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	f, err := os.Open(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to open CSV file %s: %v", params.FilePath, err),
		}, nil
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var hits []SearchHit
	lowerQuery := strings.ToLower(params.Query)
	rowNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &DoctoolsRunResult{
				Status:   "error",
				FilePath: params.FilePath,
				Message:  fmt.Sprintf("failed to read CSV file %s: %v", params.FilePath, err),
			}, nil
		}
		rowNum++
		lineStr := strings.Join(record, ", ")
		if strings.Contains(strings.ToLower(lineStr), lowerQuery) {
			snippet := makeSnippet(lineStr, params.Query, 60)
			hits = append(hits, SearchHit{
				Location: fmt.Sprintf("Row %d", rowNum),
				Snippet:  snippet,
			})
			if len(hits) >= MaxSearchHits {
				break
			}
		}
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   formatted,
		FilePath:  params.FilePath,
		CharCount: utf8.RuneCountInString(formatted),
	}, nil
}

func handleSearchText(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	f, err := os.Open(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to open text file %s: %v", params.FilePath, err),
		}, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var hits []SearchHit
	lowerQuery := strings.ToLower(params.Query)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			snippet := makeSnippet(line, params.Query, 60)
			hits = append(hits, SearchHit{
				Location: fmt.Sprintf("Line %d", lineNum),
				Snippet:  snippet,
			})
			if len(hits) >= MaxSearchHits {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to read text file %s: %v", params.FilePath, err),
		}, nil
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   formatted,
		FilePath:  params.FilePath,
		CharCount: utf8.RuneCountInString(formatted),
	}, nil
}

func handleSearchHTML(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	content, err := html.ExtractContent(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to extract HTML from %s: %v", params.FilePath, err),
		}, nil
	}

	lines := strings.Split(content, "\n")
	var hits []SearchHit
	lowerQuery := strings.ToLower(params.Query)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), lowerQuery) {
			snippet := makeSnippet(trimmed, params.Query, 60)
			hits = append(hits, SearchHit{
				Location: fmt.Sprintf("Paragraph %d", i+1),
				Snippet:  snippet,
			})
			if len(hits) >= MaxSearchHits {
				break
			}
		}
	}

	formatted := formatSearchResults(params.FilePath, params.Query, hits)
	return &DoctoolsRunResult{
		Status:    "success",
		Content:   formatted,
		FilePath:  params.FilePath,
		CharCount: utf8.RuneCountInString(formatted),
	}, nil
}

// makeSnippet extracts a snippet around the matched query with surrounding context.
func makeSnippet(text string, query string, maxRadius int) string {
	runes := []rune(text)
	lowerRunes := []rune(strings.ToLower(text))
	lowerQueryRunes := []rune(strings.ToLower(query))

	if len(lowerQueryRunes) == 0 {
		return ""
	}

	// Find the matching rune index in rune space
	matchStart := -1
	for i := 0; i <= len(lowerRunes)-len(lowerQueryRunes); i++ {
		match := true
		for j := 0; j < len(lowerQueryRunes); j++ {
			if lowerRunes[i+j] != lowerQueryRunes[j] {
				match = false
				break
			}
		}
		if match {
			matchStart = i
			break
		}
	}

	if matchStart == -1 {
		return ""
	}

	matchLen := len(lowerQueryRunes)
	matchEnd := matchStart + matchLen

	start := matchStart - maxRadius
	prefix := "..."
	if start <= 0 {
		start = 0
		prefix = ""
	}

	end := matchEnd + maxRadius
	suffix := "..."
	if end >= len(runes) {
		end = len(runes)
		suffix = ""
	}

	before := strings.TrimSpace(strings.ReplaceAll(string(runes[start:matchStart]), "\n", " "))
	matched := string(runes[matchStart:matchEnd])
	after := strings.TrimSpace(strings.ReplaceAll(string(runes[matchEnd:end]), "\n", " "))

	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(prefix)
		sb.WriteString(" ")
	}
	if before != "" {
		sb.WriteString(before)
		sb.WriteString(" ")
	}
	sb.WriteString("[")
	sb.WriteString(matched)
	sb.WriteString("]")
	if after != "" {
		sb.WriteString(" ")
		sb.WriteString(after)
	}
	if suffix != "" {
		sb.WriteString(" ")
		sb.WriteString(suffix)
	}

	return sb.String()
}

func formatSearchResults(filePath string, query string, hits []SearchHit) string {
	baseName := filepath.Base(filePath)
	if len(hits) == 0 {
		return fmt.Sprintf("No matches found for %q in %s.", query, baseName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d match(es) for %q in %s:\n\n", len(hits), query, baseName))
	for _, hit := range hits {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", hit.Location, hit.Snippet))
	}
	return sb.String()
}
