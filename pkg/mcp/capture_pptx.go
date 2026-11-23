package mcp

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"doctools-cli/pkg/pptx"
)

// pptxExtractImages is a package-level variable to allow mocking in unit tests.
var pptxExtractImages = pptx.ExtractImagesCOM

// handleCapturePPTX captures the specified slides of a PowerPoint presentation as images.
func handleCapturePPTX(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
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

	width, height := 0, 0
	if dpi > 0 {
		width = 1920 * dpi / 150
		height = 1080 * dpi / 150
	}

	paths, err := pptxExtractImages(filePath, outputDir, pages, width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pptx slides: %w", err)
	}

	results := make([]CapturedImage, 0, len(paths))
	for idx, imgPath := range paths {
		pageNum := 0
		if idx < len(pages) {
			pageNum = pages[idx]
		}

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
