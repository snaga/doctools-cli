package pptx_test

import (
	"archive/zip"
	"doctools-cli/pkg/pptx"
	"os"
	"path/filepath"
	"testing"
)

func createSamplePPTX(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create sample file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	slide1XML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Slide 1 Title</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`

	slide2XML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Slide 2 Content</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`

	w1, _ := zw.Create("ppt/slides/slide1.xml")
	w1.Write([]byte(slide1XML))

	w2, _ := zw.Create("ppt/slides/slide2.xml")
	w2.Write([]byte(slide2XML))

	zw.Close()
}

func TestExtractTextPureGo(t *testing.T) {
	samplePath := filepath.Join(t.TempDir(), "sample.pptx")
	createSamplePPTX(t, samplePath)

	outPath := filepath.Join(t.TempDir(), "out.md")
	content, err := pptx.ExtractTextPureGo(samplePath, outPath, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == "" {
		t.Errorf("expected extracted text, got empty")
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("expected output md file to exist")
	}
}

func TestMergePureGo(t *testing.T) {
	p1 := filepath.Join(t.TempDir(), "p1.pptx")
	p2 := filepath.Join(t.TempDir(), "p2.pptx")
	createSamplePPTX(t, p1)
	createSamplePPTX(t, p2)

	outPath := filepath.Join(t.TempDir(), "merged.pptx")
	res, err := pptx.MergePureGo([]string{p1, p2}, outPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == "" {
		t.Errorf("expected output path")
	}
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("expected merged file to exist")
	}
}
