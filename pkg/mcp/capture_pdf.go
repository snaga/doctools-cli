package mcp

import (
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"doctools-cli/pkg/pdf"
)

// handleCapturePDF captures the specified pages of a PDF document as images.
func handleCapturePDF(ctx context.Context, filePath string, pages []int, dpi int, outputDir string) ([]CapturedImage, error) {
	if dpi <= 0 {
		dpi = 150
	}
	if outputDir == "" {
		outputDir = ".tmp"
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	if len(pages) == 0 {
		return []CapturedImage{}, nil
	}

	results := make([]CapturedImage, 0, len(pages))
	for _, p := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		paths, err := pdf.ExtractPages(filePath, outputDir, dpi, "png", p, p, true)
		if err != nil {
			return nil, fmt.Errorf("failed to extract page %d from pdf: %w", p, err)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no image generated for page %d", p)
		}

		imgPath := paths[0]

		f, err := os.Open(imgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open generated image %s: %w", imgPath, err)
		}
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to decode image config for %s: %w", imgPath, err)
		}

		stat, err := os.Stat(imgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat generated image %s: %w", imgPath, err)
		}

		results = append(results, CapturedImage{
			PageNumber: p,
			FilePath:   imgPath,
			Width:      cfg.Width,
			Height:     cfg.Height,
			FileSize:   stat.Size(),
		})
	}

	return results, nil
}
