package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"doctools-cli/pkg/pdf"
	"doctools-cli/pkg/pptx"
)

// Package-level variables to allow mocking in unit tests.
var (
	pdfGetPageCount   = pdf.GetPageCount
	pptxGetSlideCount = pptx.GetSlideCount
	capturePDF        = handleCapturePDF
	capturePPTX       = handleCapturePPTX
	captureExcel      = handleCaptureExcel
)

// CaptureHandlerFunc handles capture commands for a specific file format.
type CaptureHandlerFunc func(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error)

// RegisterDefaultCaptureHandlers registers the default capture handlers on the dispatcher.
func RegisterDefaultCaptureHandlers(d *Dispatcher) {
	d.RegisterCaptureHandler(FormatPDF, handleCapture)
	d.RegisterCaptureHandler(FormatPPTX, handleCapture)
	d.RegisterCaptureHandler(FormatExcel, handleCapture)
}

// handleCapture is the unified capture dispatch function for PDF, PowerPoint, and Excel.
func handleCapture(ctx context.Context, format FileFormat, params DoctoolsRunParams) (*DoctoolsRunResult, error) {
	var totalPages int
	switch format {
	case FormatPDF:
		count, err := pdfGetPageCount(params.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get page count for %s: %w", params.FilePath, err)
		}
		totalPages = count
	case FormatPPTX:
		count, err := pptxGetSlideCount(params.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get slide count for %s: %w", params.FilePath, err)
		}
		totalPages = count
	case FormatExcel:
		sheets, err := excelListSheets(params.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to list sheets for %s: %w", params.FilePath, err)
		}
		totalPages = len(sheets)
	default:
		return nil, fmt.Errorf("capture is not supported for format %q. Available formats for capture: PDF (.pdf), PowerPoint (.pptx), Excel (.xlsx, .xlsm, .xls)", format)
	}

	pagesStr := ""
	if params.Options != nil {
		if params.Options.Pages != "" {
			pagesStr = params.Options.Pages
		} else if params.Options.Slides != "" {
			pagesStr = params.Options.Slides
		}
	}

	pages, skipped, err := parsePageSelection(pagesStr, totalPages)
	if err != nil {
		return nil, err
	}

	outputDir := ".tmp"
	if params.Options != nil && params.Options.OutputPath != "" {
		outputDir = params.Options.OutputPath
	}

	dpi := 150
	if params.Options != nil && params.Options.DPI > 0 {
		dpi = params.Options.DPI
	}

	var images []CapturedImage
	switch format {
	case FormatPDF:
		images, err = capturePDF(ctx, params.FilePath, pages, dpi, outputDir)
	case FormatPPTX:
		images, err = capturePPTX(ctx, params.FilePath, pages, dpi, outputDir)
	case FormatExcel:
		images, err = captureExcel(ctx, params.FilePath, pages, dpi, outputDir)
	default:
		return nil, fmt.Errorf("capture is not supported for format %q. Available formats for capture: PDF (.pdf), PowerPoint (.pptx), Excel (.xlsx, .xlsm, .xls)", format)
	}
	if err != nil {
		return nil, err
	}

	inline := true
	if params.Options != nil && params.Options.Inline != nil {
		inline = *params.Options.Inline
	}

	var (
		status      string
		content     string
		imageBase64 string
		imageMIME   string
		resultPath  string
	)

	if len(images) == 1 && inline {
		// Case A: Single page inline Base64 injection
		data, err := os.ReadFile(images[0].FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read captured image %s: %w", images[0].FilePath, err)
		}
		base64Str := base64.StdEncoding.EncodeToString(data)
		status = "success"
		content = fmt.Sprintf("Captured page %d of %s to %s (%dx%d, %d bytes)",
			images[0].PageNumber, filepath.Base(params.FilePath), images[0].FilePath, images[0].Width, images[0].Height, images[0].FileSize)
		imageBase64 = base64Str
		imageMIME = "image/png"
		resultPath = images[0].FilePath
	} else {
		// Case B: Multi-page or Base64 opt-out (context protection mode)
		status = "saved_to_file"
		imageBase64 = ""
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Captured %d pages of %s to directory:\n", len(images), filepath.Base(params.FilePath)))
		for _, img := range images {
			sb.WriteString(fmt.Sprintf("- Page %d: %s (%dx%d, %d bytes)\n", img.PageNumber, img.FilePath, img.Width, img.Height, img.FileSize))
		}
		content = sb.String()
		if len(images) == 1 {
			resultPath = images[0].FilePath
		} else {
			resultPath = outputDir
		}
	}

	var warning string
	if len(skipped) > 0 {
		warning = fmt.Sprintf("Warning: skipped out-of-range pages: %v", skipped)
	}

	return &DoctoolsRunResult{
		Status:      status,
		Content:     content,
		FilePath:    resultPath,
		TotalPages:  totalPages,
		CharCount:   len(content),
		Message:     warning,
		Images:      images,
		ImageBase64: imageBase64,
		ImageMIME:   imageMIME,
	}, nil
}
