package image_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	imgpkg "doctools-cli/pkg/image"
)

func createTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create image: %v", err)
	}
	defer f.Close()
	png.Encode(f, img)
}

func TestImageOperations(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	createTestPNG(t, imgPath)

	meta, err := imgpkg.GetMetadata(imgPath)
	if err != nil || meta.Width != 100 || meta.Height != 100 {
		t.Errorf("GetMetadata failed: %v, meta=%+v", err, meta)
	}

	cropPath := filepath.Join(tmpDir, "crop.png")
	res, err := imgpkg.Crop(imgPath, cropPath, 10, 10, 50, 50)
	if err != nil {
		t.Fatalf("Crop failed: %v", err)
	}
	if _, err := os.Stat(res); os.IsNotExist(err) {
		t.Errorf("expected cropped image to exist")
	}

	// Crop with out-of-bounds cropRect (tests rect intersection logic)
	resIntersect, err := imgpkg.Crop(imgPath, cropPath, -50, -50, 200, 200)
	if err != nil {
		t.Fatalf("Crop out of bounds failed: %v", err)
	}
	if _, err := os.Stat(resIntersect); os.IsNotExist(err) {
		t.Errorf("expected cropped image to exist")
	}

	// Crop with empty outputPath (default path)
	resDefault, err := imgpkg.Crop(imgPath, "", 10, 10, 50, 50)
	if err != nil {
		t.Fatalf("Crop with default path failed: %v", err)
	}
	defer os.Remove(resDefault)

	// SaveClipboard tests
	cb1, err := imgpkg.SaveClipboard("")
	if err != nil || cb1 == "" {
		t.Errorf("SaveClipboard with empty path failed: %v", err)
	}
	cb2, err := imgpkg.SaveClipboard(filepath.Join(tmpDir, "cb.png"))
	if err != nil || cb2 == "" {
		t.Errorf("SaveClipboard with custom path failed: %v", err)
	}
}

func TestImageErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.png")

	// GetMetadata non-existent
	_, err := imgpkg.GetMetadata(nonExistent)
	if err == nil {
		t.Errorf("expected error for GetMetadata on nonexistent file")
	}

	// GetMetadata invalid image format
	invalidImg := filepath.Join(tmpDir, "invalid.png")
	os.WriteFile(invalidImg, []byte("not an image"), 0644)
	_, err = imgpkg.GetMetadata(invalidImg)
	if err == nil {
		t.Errorf("expected error for GetMetadata on invalid file")
	}

	// Crop non-existent file
	_, err = imgpkg.Crop(nonExistent, "", 0, 0, 10, 10)
	if err == nil {
		t.Errorf("expected error for Crop on nonexistent file")
	}

	// Crop invalid image file
	_, err = imgpkg.Crop(invalidImg, "", 0, 0, 10, 10)
	if err == nil {
		t.Errorf("expected error for Crop on invalid file")
	}

	// Crop with invalid output path (e.g. dir as out file)
	imgPath := filepath.Join(tmpDir, "test.png")
	createTestPNG(t, imgPath)
	invalidOut := filepath.Join(tmpDir, "dir_as_out")
	os.MkdirAll(invalidOut, 0755)
	_, err = imgpkg.Crop(imgPath, invalidOut, 0, 0, 10, 10)
	if err == nil {
		t.Errorf("expected error for Crop writing to a directory")
	}
}
