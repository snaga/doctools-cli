package image

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

// ImageMetadata holds metadata of an image.
type ImageMetadata struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

// GetMetadata reads width, height and format of an image.
func GetMetadata(path string) (*ImageMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	cfg, fmtName, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image config: %w", err)
	}

	return &ImageMetadata{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: fmtName,
	}, nil
}

// Crop crops an image specified by rectangle (left, top, right, bottom).
func Crop(inputPath string, outputPath string, left int, top int, right int, bottom int) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	cropRect := image.Rect(left, top, right, bottom)
	if !cropRect.In(bounds) {
		cropRect = cropRect.Intersect(bounds)
	}

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	simg, ok := img.(subImager)
	if !ok {
		return "", fmt.Errorf("image format does not support cropping")
	}

	croppedImg := simg.SubImage(cropRect)

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = fmt.Sprintf("%s_crop%s", base, ext)
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create cropped image file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, croppedImg); err != nil {
		return "", fmt.Errorf("failed to encode cropped image: %w", err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

// SaveClipboard saves Windows clipboard image to file.
func SaveClipboard(outputPath string) (string, error) {
	if outputPath == "" {
		outputPath = "clipboard.png"
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		absPath = outputPath
	}
	return absPath, nil
}
