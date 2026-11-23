package mcp

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/pdf"
)

// excelExtractImages is a package-level variable to allow mocking in unit tests.
var excelExtractImages = excel.ExtractImagesCOM

// excelListSheets is a package-level variable to allow mocking in unit tests.
var excelListSheets = excel.ListSheets

// handleCaptureExcel captures the specified sheets of an Excel document as images.
func handleCaptureExcel(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	if len(pages) == 0 {
		return []CapturedImage{}, nil
	}

	if outputDir == "" {
		outputDir = ".tmp"
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	if dpi <= 0 {
		dpi = 150
	}

	// 1. シート一覧取得
	sheets, err := excelListSheets(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list excel sheets: %w", err)
	}

	// 2. 対象シート名特定
	targetSheets := make([]string, 0, len(pages))
	sheetPages := make([]int, 0, len(pages))
	for _, p := range pages {
		if 1 <= p && p <= len(sheets) {
			targetSheets = append(targetSheets, sheets[p-1])
			sheetPages = append(sheetPages, p)
		}
	}

	if len(targetSheets) == 0 {
		return nil, fmt.Errorf("no valid sheets for requested pages %v", pages)
	}

	// 3. Excel COM で PDF 生成
	pdfPaths, err := excelExtractImages(filePath, outputDir, targetSheets)
	if err != nil {
		return nil, fmt.Errorf("failed to export excel sheets as PDF: %w", err)
	}
	// Ensure all generated temporary PDFs are cleaned up upon function exit
	defer func() {
		for _, p := range pdfPaths {
			_ = os.Remove(p)
		}
	}()

	// 4. PNG 変換
	results := make([]CapturedImage, 0, len(pdfPaths))
	for i, pdfPath := range pdfPaths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		pngPaths, err := pdf.ExtractPages(pdfPath, outputDir, dpi, "png", 1, 1, true)
		if err != nil {
			return nil, fmt.Errorf("failed to rasterize sheet PDF %s: %w", pdfPath, err)
		}
		if len(pngPaths) == 0 {
			continue
		}

		imgPath := pngPaths[0]

		f, err := os.Open(imgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open generated image %s: %w", imgPath, err)
		}
		cfg, _, decodeErr := image.DecodeConfig(f)
		f.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode image config for %s: %w", imgPath, decodeErr)
		}

		stat, err := os.Stat(imgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat generated image %s: %w", imgPath, err)
		}

		pageNum := 0
		if i < len(sheetPages) {
			pageNum = sheetPages[i]
		}

		results = append(results, CapturedImage{
			PageNumber: pageNum,
			FilePath:   imgPath,
			Width:      cfg.Width,
			Height:     cfg.Height,
			FileSize:   stat.Size(),
		})
	}

	return results, nil
}
