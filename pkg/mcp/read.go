package mcp

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"doctools-cli/pkg/docx"
	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/html"
	"doctools-cli/pkg/pdf"
	"doctools-cli/pkg/pptx"
)

// RegisterDefaultReadHandlers registers production read handlers to the given dispatcher.
func RegisterDefaultReadHandlers(d *Dispatcher) {
	d.RegisterReadHandler(FormatPDF, handleReadPDF)
	d.RegisterReadHandler(FormatExcel, handleReadExcel)
	d.RegisterReadHandler(FormatPPTX, handleReadPPTX)
	d.RegisterReadHandler(FormatCSV, handleReadCSV)
	d.RegisterReadHandler(FormatText, handleReadText)
	d.RegisterReadHandler(FormatHTML, handleReadHTML)
	d.RegisterReadHandler(FormatWord, handleReadDocx)
}

// handleReadDocx extracts formatted text/markdown from Word (.docx) files using pure Go docx parser.
func handleReadDocx(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	text, err := docx.ExtractText(params.FilePath, "")
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to extract Word text from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(text, params, 0)
}

func handleReadPDF(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	startPage := 1
	endPage := 0
	if params.Options != nil && strings.TrimSpace(params.Options.Pages) != "" {
		var err error
		startPage, endPage, err = parseRange(params.Options.Pages)
		if err != nil {
			return &DoctoolsRunResult{
				Status:   "error",
				FilePath: params.FilePath,
				Message:  fmt.Sprintf("invalid pages option %q: %v", params.Options.Pages, err),
			}, nil
		}
	}

	totalPages, _ := pdf.GetPageCount(params.FilePath)

	text, err := pdf.ExtractText(params.FilePath, "", startPage, endPage)
	if err != nil {
		return &DoctoolsRunResult{
			Status:     "error",
			FilePath:   params.FilePath,
			TotalPages: totalPages,
			Message:    fmt.Sprintf("failed to extract PDF text from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(text, params, totalPages)
}

func handleReadExcel(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	var sheets []string
	if params.Options != nil && strings.TrimSpace(params.Options.Sheet) != "" {
		sheets = []string{strings.TrimSpace(params.Options.Sheet)}
	}

	allSheets, _ := excel.ListSheets(params.FilePath)
	totalSheets := len(allSheets)

	text, err := excel.ExtractMarkdown(params.FilePath, sheets, false)
	if err != nil {
		return &DoctoolsRunResult{
			Status:     "error",
			FilePath:   params.FilePath,
			TotalPages: totalSheets,
			Message:    fmt.Sprintf("failed to extract Excel markdown from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(text, params, totalSheets)
}

func handleReadPPTX(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	startSlide := 1
	endSlide := 0
	if params.Options != nil && strings.TrimSpace(params.Options.Slides) != "" {
		var err error
		startSlide, endSlide, err = parseRange(params.Options.Slides)
		if err != nil {
			return &DoctoolsRunResult{
				Status:   "error",
				FilePath: params.FilePath,
				Message:  fmt.Sprintf("invalid slides option %q: %v", params.Options.Slides, err),
			}, nil
		}
	}

	totalSlides, _ := pptx.GetSlideCount(params.FilePath)

	text, err := pptx.ExtractTextPureGo(params.FilePath, "", startSlide, endSlide)
	if err != nil {
		return &DoctoolsRunResult{
			Status:     "error",
			FilePath:   params.FilePath,
			TotalPages: totalSlides,
			Message:    fmt.Sprintf("failed to extract PPTX text from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(text, params, totalSlides)
}

func handleReadCSV(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	text, err := extractCSVToMarkdown(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to extract CSV table from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(text, params, 1)
}

func handleReadText(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	data, err := os.ReadFile(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to read text file %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(string(data), params, 1)
}

func handleReadHTML(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	content, err := html.ExtractContent(params.FilePath)
	if err != nil {
		return &DoctoolsRunResult{
			Status:   "error",
			FilePath: params.FilePath,
			Message:  fmt.Sprintf("failed to extract HTML text from %s: %v", params.FilePath, err),
		}, nil
	}

	return HandleSmartDynamicResponse(content, params, 1)
}

// HandleSmartDynamicResponse implements threshold-based inline return vs file-saving logic.
func HandleSmartDynamicResponse(text string, params DoctoolsRunParams, totalPages int) (*DoctoolsRunResult, error) {
	maxInlineChars := DefaultMaxInlineChars
	if params.Options != nil && params.Options.MaxInlineChars > 0 {
		maxInlineChars = params.Options.MaxInlineChars
	}

	charCount := utf8.RuneCountInString(text)

	if charCount <= maxInlineChars {
		return &DoctoolsRunResult{
			Status:     "success",
			Content:    text,
			FilePath:   params.FilePath,
			CharCount:  charCount,
			TotalPages: totalPages,
		}, nil
	}

	// Threshold exceeded: write to file to protect model context
	savePath := ""
	if params.Options != nil && strings.TrimSpace(params.Options.OutputPath) != "" {
		savePath = strings.TrimSpace(params.Options.OutputPath)
	} else {
		base := strings.TrimSuffix(filepath.Base(params.FilePath), filepath.Ext(params.FilePath))
		savePath = filepath.Join(".tmp", base+"_extracted.md")
	}

	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return &DoctoolsRunResult{
			Status:     "error",
			FilePath:   params.FilePath,
			TotalPages: totalPages,
			Message:    fmt.Sprintf("failed to create directory for %s: %v", savePath, err),
		}, nil
	}

	if err := os.WriteFile(savePath, []byte(text), 0644); err != nil {
		return &DoctoolsRunResult{
			Status:     "error",
			FilePath:   params.FilePath,
			TotalPages: totalPages,
			Message:    fmt.Sprintf("failed to save extracted content to %s: %v", savePath, err),
		}, nil
	}

	return &DoctoolsRunResult{
		Status:     "saved_to_file",
		FilePath:   savePath,
		CharCount:  charCount,
		TotalPages: totalPages,
		Message:    fmt.Sprintf("Content exceeded inline threshold (%d chars). To protect context, it was safely saved to file '%s'. Total pages/sheets: %d. Please inspect specific sections with read_file or grep.", charCount, savePath, totalPages),
	}, nil
}

// parseRange parses range strings like "1-5", "3", "3-", "-5".
func parseRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1, 0, nil
	}

	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		startStr := strings.TrimSpace(parts[0])
		endStr := strings.TrimSpace(parts[1])

		start := 1
		end := 0
		if startStr != "" {
			var err error
			start, err = strconv.Atoi(startStr)
			if err != nil || start < 1 {
				return 0, 0, fmt.Errorf("invalid start range %q", s)
			}
		}
		if endStr != "" {
			var err error
			end, err = strconv.Atoi(endStr)
			if err != nil || end < 1 {
				return 0, 0, fmt.Errorf("invalid end range %q", s)
			}
			if startStr != "" && end < start {
				return 0, 0, fmt.Errorf("end range (%d) cannot be less than start (%d)", end, start)
			}
		}
		return start, end, nil
	}

	num, err := strconv.Atoi(s)
	if err != nil || num < 1 {
		return 0, 0, fmt.Errorf("invalid range number %q", s)
	}
	return num, num, nil
}

// extractCSVToMarkdown parses a CSV file and formats its content as a Markdown table.
func extractCSVToMarkdown(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		rows = append(rows, record)
	}

	if len(rows) == 0 {
		return "", nil
	}

	header := rows[0]
	var sb strings.Builder

	// Header row
	sb.WriteString("| ")
	for i, col := range header {
		sb.WriteString(strings.ReplaceAll(col, "|", "\\|"))
		if i < len(header)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(" |\n")

	// Separator row
	sb.WriteString("|")
	for range header {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range rows[1:] {
		sb.WriteString("| ")
		for i := 0; i < len(header); i++ {
			val := ""
			if i < len(row) {
				val = strings.ReplaceAll(row[i], "|", "\\|")
			}
			sb.WriteString(val)
			if i < len(header)-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString(" |\n")
	}

	return sb.String(), nil
}
