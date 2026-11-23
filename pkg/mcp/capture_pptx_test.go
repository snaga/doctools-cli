package mcp

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create test png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
}

func TestCapturePPTX(t *testing.T) {
	tmpDir := t.TempDir()
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy pptx content"), 0644); err != nil {
		t.Fatalf("failed to write dummy pptx: %v", err)
	}
	outDir := filepath.Join(tmpDir, "out_pptx_images")

	// Save original function and restore after tests
	originalExtract := pptxExtractImages
	defer func() {
		pptxExtractImages = originalExtract
	}()

	t.Run("Valid capture with mocked extractor and dpi resolution calculation", func(t *testing.T) {
		var capturedWidth, capturedHeight int
		pptxExtractImages = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
			capturedWidth = width
			capturedHeight = height
			var resultPaths []string
			for _, s := range slides {
				imgPath := filepath.Join(outputDir, fmt.Sprintf("slide_%d.png", s))
				createTestPNG(t, imgPath, 1, 1)
				resultPaths = append(resultPaths, imgPath)
			}
			return resultPaths, nil
		}

		images, err := handleCapturePPTX(context.Background(), pptxPath, []int{1, 3}, 150, outDir)
		if err != nil {
			t.Fatalf("handleCapturePPTX failed: %v", err)
		}

		if capturedWidth != 1920 || capturedHeight != 1080 {
			t.Errorf("expected resolution 1920x1080 for dpi 150, got %dx%d", capturedWidth, capturedHeight)
		}

		if len(images) != 2 {
			t.Fatalf("expected 2 images, got %d", len(images))
		}

		if images[0].PageNumber != 1 || images[1].PageNumber != 3 {
			t.Errorf("expected page numbers [1, 3], got [%d, %d]", images[0].PageNumber, images[1].PageNumber)
		}

		for _, img := range images {
			if img.Width != 1 || img.Height != 1 {
				t.Errorf("expected dimensions 1x1, got %dx%d", img.Width, img.Height)
			}
			if img.FileSize <= 0 {
				t.Errorf("expected positive file size, got %d", img.FileSize)
			}
			if _, err := os.Stat(img.FilePath); err != nil {
				t.Errorf("expected output image file to exist at %s: %v", img.FilePath, err)
			}
		}
	})

	t.Run("Valid capture with zero dpi", func(t *testing.T) {
		var capturedWidth, capturedHeight int
		pptxExtractImages = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
			capturedWidth = width
			capturedHeight = height
			imgPath := filepath.Join(outputDir, "slide_0.png")
			createTestPNG(t, imgPath, 10, 20)
			return []string{imgPath}, nil
		}

		images, err := handleCapturePPTX(context.Background(), pptxPath, []int{1}, 0, outDir)
		if err != nil {
			t.Fatalf("handleCapturePPTX failed: %v", err)
		}

		if capturedWidth != 0 || capturedHeight != 0 {
			t.Errorf("expected resolution 0x0 for dpi 0, got %dx%d", capturedWidth, capturedHeight)
		}

		if len(images) != 1 {
			t.Fatalf("expected 1 image, got %d", len(images))
		}
		if images[0].Width != 10 || images[0].Height != 20 {
			t.Errorf("expected dimensions 10x20, got %dx%d", images[0].Width, images[0].Height)
		}
	})

	t.Run("Empty pages list returns empty result", func(t *testing.T) {
		images, err := handleCapturePPTX(context.Background(), pptxPath, []int{}, 150, outDir)
		if err != nil {
			t.Fatalf("expected nil error on empty pages, got: %v", err)
		}
		if len(images) != 0 {
			t.Errorf("expected 0 images, got %d", len(images))
		}
	})

	t.Run("Non-existent file returns error", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "does_not_exist.pptx")
		images, err := handleCapturePPTX(context.Background(), nonExistent, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error for non-existent file, got nil")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("expected 'file not found' error message, got: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Mock extractor returns error", func(t *testing.T) {
		pptxExtractImages = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
			return nil, errors.New("COM export failure")
		}

		images, err := handleCapturePPTX(context.Background(), pptxPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error from handleCapturePPTX, got nil")
		}
		if !strings.Contains(err.Error(), "failed to extract pptx slides: COM export failure") {
			t.Errorf("unexpected error message: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})

	t.Run("Context canceled before execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		images, err := handleCapturePPTX(ctx, pptxPath, []int{1}, 150, outDir)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on context cancel, got %v", images)
		}
	})

	t.Run("Corrupted image returned by extractor returns decode error", func(t *testing.T) {
		pptxExtractImages = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
			corruptedPath := filepath.Join(outputDir, "corrupted.png")
			if err := os.WriteFile(corruptedPath, []byte("not an image"), 0644); err != nil {
				return nil, err
			}
			return []string{corruptedPath}, nil
		}

		images, err := handleCapturePPTX(context.Background(), pptxPath, []int{1}, 150, outDir)
		if err == nil {
			t.Errorf("expected error when decoding corrupted image, got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode image config") {
			t.Errorf("expected decode image config error, got: %v", err)
		}
		if images != nil {
			t.Errorf("expected nil images on error, got %v", images)
		}
	})
}
