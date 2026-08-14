package pptx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-ole/go-ole"
)

type dummyCompressor struct{ io.Writer }
func (c dummyCompressor) Close() error { return nil }

func createSamplePPTXInternal(t *testing.T, path string) {
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

	wOther, _ := zw.Create("ppt/presentation.xml")
	wOther.Write([]byte("<xml></xml>"))

	zw.Close()
}

func TestExtractTextPureGo_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-existent file
	_, err := ExtractTextPureGo(filepath.Join(tmpDir, "nonexistent.pptx"), "", 1, 0)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	// Invalid zip file
	invalidZip := filepath.Join(tmpDir, "invalid.pptx")
	os.WriteFile(invalidZip, []byte("not a zip"), 0644)
	_, err = ExtractTextPureGo(invalidZip, "", 1, 0)
	if err == nil {
		t.Errorf("expected error for invalid zip file")
	}

	// Empty pptx (no slides)
	emptyZip := filepath.Join(tmpDir, "empty.pptx")
	zf, _ := os.Create(emptyZip)
	zw := zip.NewWriter(zf)
	zw.Close()
	zf.Close()
	_, err = ExtractTextPureGo(emptyZip, "", 1, 0)
	if err == nil {
		t.Errorf("expected error for pptx with no slides")
	}

	// Sample file with startSlide / endSlide bounds and output file creation
	samplePath := filepath.Join(tmpDir, "sample.pptx")
	createSamplePPTXInternal(t, samplePath)

	outPath := filepath.Join(tmpDir, "sub/dir/out.md")
	content, err := ExtractTextPureGo(samplePath, outPath, 0, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == "" {
		t.Errorf("expected extracted text")
	}
}

func TestParseTokens(t *testing.T) {
	xmlData := []byte(`<p:sld><a:t>Hello Token</a:t><a:t>World Token</a:t></p:sld>`)
	tokens := parseTokens(xmlData)
	if len(tokens) != 2 || tokens[0] != "Hello Token" || tokens[1] != "World Token" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

func TestMergePureGo_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. MergeCOMHook success branch test
	oldHook := MergeCOMHook
	defer func() { MergeCOMHook = oldHook }()
	MergeCOMHook = func(inputPaths []string, outputPath string) (string, error) {
		return outputPath, nil
	}

	p1 := filepath.Join(tmpDir, "p1.pptx")
	createSamplePPTXInternal(t, p1)
	outSuccess := filepath.Join(tmpDir, "com_success.pptx")
	resCOM, err := MergePureGo([]string{p1}, outSuccess)
	if err != nil || resCOM != outSuccess {
		t.Fatalf("expected MergePureGo to succeed via MergeCOMHook")
	}

	// 2. Mock MergeCOMHook to fail so pure zip fallback runs
	MergeCOMHook = func(inputPaths []string, outputPath string) (string, error) {
		return "", fmt.Errorf("mock COM error")
	}


	// Empty input paths
	_, err = MergePureGo([]string{}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error for empty input paths")
	}

	// Single input path
	p1 = filepath.Join(tmpDir, "p1.pptx")
	createSamplePPTXInternal(t, p1)
	out1 := filepath.Join(tmpDir, "single_out.pptx")
	res, err := MergePureGo([]string{p1}, out1)
	if err != nil {
		t.Fatalf("unexpected error for single input: %v", err)
	}
	if res == "" {
		t.Errorf("expected output path")
	}

	// Single input error (nonexistent file)
	_, err = MergePureGo([]string{filepath.Join(tmpDir, "nonexistent.pptx")}, out1)
	if err == nil {
		t.Errorf("expected error for nonexistent single file")
	}

	outDir := filepath.Join(tmpDir, "dir_out")
	os.MkdirAll(outDir, 0755)

	// Single input error (write fail)
	_, err = MergePureGo([]string{p1}, outDir)
	if err == nil {
		t.Errorf("expected error when output is a directory for single input")
	}

	// Define p2
	p2 := filepath.Join(tmpDir, "p2.pptx")
	createSamplePPTXInternal(t, p2)

	// Multiple inputs: create output fail
	_, err = MergePureGo([]string{p1, p2}, outDir)
	if err == nil {
		t.Errorf("expected error when output is a directory for multiple inputs")
	}

	// Multiple inputs: base zip error
	_, err = MergePureGo([]string{filepath.Join(tmpDir, "nonexistent1.pptx"), p1}, out1)
	if err == nil {
		t.Errorf("expected error for nonexistent base zip")
	}

	// Multiple inputs: second zip error
	_, err = MergePureGo([]string{p1, filepath.Join(tmpDir, "nonexistent2.pptx")}, out1)
	if err == nil {
		t.Errorf("expected error for nonexistent second zip")
	}

	// Multiple inputs success with zip merge
	outMerged := filepath.Join(tmpDir, "merged_zip.pptx")
	resMerged, err := MergePureGo([]string{p1, p2}, outMerged)
	if err != nil {
		t.Fatalf("unexpected error merging zip files: %v", err)
	}
	if resMerged == "" {
		t.Errorf("expected merged output path")
	}

	// Multiple inputs: f.Open fail (unsupported compression method)
	p3 := filepath.Join(tmpDir, "p3.pptx")
	f3, _ := os.Create(p3)
	zw3 := zip.NewWriter(f3)
	zw3.RegisterCompressor(99, func(out io.Writer) (io.WriteCloser, error) {
		return dummyCompressor{out}, nil
	})
	w3, _ := zw3.CreateHeader(&zip.FileHeader{
		Name:   "ppt/slides/slide1.xml",
		Method: 99,
	})
	if w3 != nil {
		w3.Write([]byte("data"))
	}
	zw3.Close()
	f3.Close()

	_, err = MergePureGo([]string{p1, p3}, filepath.Join(tmpDir, "out3.pptx"))
	if err == nil {
		t.Errorf("expected error for unsupported compression in second zip")
	}
}

func TestExtractImagesCOM_Mock(t *testing.T) {
	oldImpl := ExtractImagesCOMImpl
	defer func() { ExtractImagesCOMImpl = oldImpl }()

	ExtractImagesCOMImpl = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
		return []string{"slide_001.png"}, nil
	}

	imgs, err := ExtractImagesCOM("test.pptx", "out", []int{1}, 100, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 1 || imgs[0] != "slide_001.png" {
		t.Errorf("unexpected image paths: %v", imgs)
	}
}

func TestMergeCOM_Mock(t *testing.T) {
	oldImpl := MergeCOMImpl
	defer func() { MergeCOMImpl = oldImpl }()

	MergeCOMImpl = func(inputPaths []string, outputPath string) (string, error) {
		return outputPath, nil
	}

	res, err := MergeCOM([]string{"a.pptx", "b.pptx"}, "out.pptx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "out.pptx" {
		t.Errorf("unexpected result: %s", res)
	}
}

func TestDefaultExtractImagesCOM_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	// Non-existent pptx
	_, err := defaultExtractImagesCOM(filepath.Join(tmpDir, "nonexistent.pptx"), tmpDir, nil, 0, 0)
	if err == nil {
		t.Errorf("expected error for nonexistent pptx in defaultExtractImagesCOM")
	}

	// We don't want to call defaultExtractImagesCOM with validPPTX and empty outputDir because it reaches COM and blocks!
	// So we stop here.
	
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	os.WriteFile(validPPTX, []byte("dummy"), 0644)

	// outputDir creation fail
	fileAsDir := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(fileAsDir, []byte("test"), 0644)
	_, err = defaultExtractImagesCOM(validPPTX, fileAsDir, nil, 0, 0)
	if err == nil {
		t.Errorf("expected error for output directory creation failure")
	}
}

func TestDefaultMergeCOM_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty input
	_, err := defaultMergeCOM([]string{}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error for empty inputs")
	}

	// Non-existent input
	_, err = defaultMergeCOM([]string{filepath.Join(tmpDir, "nonexistent.pptx")}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error for nonexistent input in defaultMergeCOM")
	}

	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	os.WriteFile(validPPTX, []byte("dummy"), 0644)

	// outputDir creation fail
	fileAsDir := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(fileAsDir, []byte("test"), 0644)
	_, err = defaultMergeCOM([]string{validPPTX}, filepath.Join(fileAsDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error for output directory creation failure")
	}
}



func TestExtractTextPureGo_MalformedXML(t *testing.T) {
	tmpDir := t.TempDir()
	pptxPath := filepath.Join(tmpDir, "malformed.pptx")
	f, err := os.Create(pptxPath)
	if err != nil {
		t.Fatalf("failed to create pptx file: %v", err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("ppt/slides/slide1.xml")
	// Malformed XML that triggers fallback parseTokens
	w.Write([]byte(`<invalid><a:t>Fallback Token Text</a:t></unclosed>`))
	zw.Close()
	f.Close()

	content, err := ExtractTextPureGo(pptxPath, "", 1, 1)
	if err != nil {
		t.Fatalf("ExtractTextPureGo failed on malformed XML: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("expected non-empty content for malformed XML")
	}
}

func TestDefaultCOM_CreateObjectError(t *testing.T) {
	oldCreate := createObject
	defer func() { createObject = oldCreate }()
	createObject = func(programID string) (*ole.IUnknown, error) {
		return nil, fmt.Errorf("mock create object error")
	}

	tmpDir := t.TempDir()
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	createSamplePPTXInternal(t, validPPTX)

	_, err := defaultExtractImagesCOM(validPPTX, tmpDir, nil, 0, 0)
	if err == nil {
		t.Errorf("expected error from createObject mock in extract")
	}

	_, err = defaultMergeCOM([]string{validPPTX}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error from createObject mock in merge")
	}
}

func TestDefaultCOM_FullMockSuccess(t *testing.T) {
	oldCreate := createObject
	oldCall := callMethod
	oldGet := getProperty
	oldMustGet := mustGetProperty
	oldQuery := queryInterface
	oldRelease := releaseDispatch
	oldReleaseUnk := releaseUnknown
	defer func() {
		createObject = oldCreate
		callMethod = oldCall
		getProperty = oldGet
		mustGetProperty = oldMustGet
		queryInterface = oldQuery
		releaseDispatch = oldRelease
		releaseUnknown = oldReleaseUnk
	}()

	releaseDispatch = func(disp *ole.IDispatch) {}
	releaseUnknown = func(unk *ole.IUnknown) {}
	createObject = func(programID string) (*ole.IUnknown, error) {
		return &ole.IUnknown{}, nil
	}
	queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
		return &ole.IDispatch{}, nil
	}
	getProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	mustGetProperty = func(disp *ole.IDispatch, name string, params ...interface{}) *ole.VARIANT {
		return &ole.VARIANT{Val: 2} // Say, 2 slides
	}
	callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}

	tmpDir := t.TempDir()
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	createSamplePPTXInternal(t, validPPTX)
	outPath := filepath.Join(tmpDir, "out")

	_, err := defaultExtractImagesCOM(validPPTX, outPath, []int{1}, 0, 0)
	if err != nil {
		t.Errorf("expected success in mocked extract: %v", err)
	}
	// Call without slides to hit the empty slides array branch
	_, _ = defaultExtractImagesCOM(validPPTX, outPath, []int{}, 0, 0)

	_, err = defaultMergeCOM([]string{validPPTX, validPPTX}, filepath.Join(tmpDir, "out.pptx"))
	if err != nil {
		t.Errorf("expected success in mocked merge: %v", err)
	}
}

func TestDefaultCOM_CallMethodError(t *testing.T) {
	oldCreate := createObject
	oldCall := callMethod
	oldGet := getProperty
	oldMustGet := mustGetProperty
	oldQuery := queryInterface
	oldRelease := releaseDispatch
	oldReleaseUnk := releaseUnknown
	defer func() {
		createObject = oldCreate
		callMethod = oldCall
		getProperty = oldGet
		mustGetProperty = oldMustGet
		queryInterface = oldQuery
		releaseDispatch = oldRelease
		releaseUnknown = oldReleaseUnk
	}()

	releaseDispatch = func(disp *ole.IDispatch) {}
	releaseUnknown = func(unk *ole.IUnknown) {}
	createObject = func(programID string) (*ole.IUnknown, error) {
		return &ole.IUnknown{}, nil
	}
	queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
		return &ole.IDispatch{}, nil
	}
	getProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	mustGetProperty = func(disp *ole.IDispatch, name string, params ...interface{}) *ole.VARIANT {
		return &ole.VARIANT{Val: 2} // Say, 2 slides
	}
	callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		if name == "Item" || name == "InsertFromFile" {
			return nil, fmt.Errorf("mock %s error", name)
		}
		return &ole.VARIANT{}, nil
	}

	tmpDir := t.TempDir()
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	createSamplePPTXInternal(t, validPPTX)
	outPath := filepath.Join(tmpDir, "out")

	_, err := defaultExtractImagesCOM(validPPTX, outPath, []int{1}, 0, 0)
	if err == nil {
		t.Errorf("expected error in mocked extract")
	}

	_, err = defaultMergeCOM([]string{validPPTX, validPPTX}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error in mocked merge")
	}
}

func TestDefaultCOM_CallMethodExportError(t *testing.T) {
	oldCreate := createObject
	oldCall := callMethod
	oldGet := getProperty
	oldMustGet := mustGetProperty
	oldQuery := queryInterface
	oldRelease := releaseDispatch
	oldReleaseUnk := releaseUnknown
	defer func() {
		createObject = oldCreate
		callMethod = oldCall
		getProperty = oldGet
		mustGetProperty = oldMustGet
		queryInterface = oldQuery
		releaseDispatch = oldRelease
		releaseUnknown = oldReleaseUnk
	}()

	releaseDispatch = func(disp *ole.IDispatch) {}
	releaseUnknown = func(unk *ole.IUnknown) {}
	createObject = func(programID string) (*ole.IUnknown, error) {
		return &ole.IUnknown{}, nil
	}
	queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
		return &ole.IDispatch{}, nil
	}
	getProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	mustGetProperty = func(disp *ole.IDispatch, name string, params ...interface{}) *ole.VARIANT {
		return &ole.VARIANT{Val: 1} // 1 slide
	}
	callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		if name == "Export" {
			return nil, fmt.Errorf("mock Export error")
		}
		if name == "SaveAs" {
			return nil, fmt.Errorf("mock SaveAs error")
		}
		return &ole.VARIANT{}, nil
	}

	tmpDir := t.TempDir()
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	createSamplePPTXInternal(t, validPPTX)
	outPath := filepath.Join(tmpDir, "out")

	_, err := defaultExtractImagesCOM(validPPTX, outPath, []int{1}, 0, 0)
	if err == nil {
		t.Errorf("expected error in mocked extract (Export)")
	}

	_, err = defaultMergeCOM([]string{validPPTX}, filepath.Join(tmpDir, "out.pptx"))
	if err == nil {
		t.Errorf("expected error in mocked merge (SaveAs)")
	}
}

// TestDefaultCOM_CoInitializeFailure checks that an unusable COM apartment is
// reported instead of being swallowed and running OLE calls anyway.
func TestDefaultCOM_CoInitializeFailure(t *testing.T) {
	old := coInitialize
	defer func() { coInitialize = old }()
	coInitialize = func(p uintptr) error {
		return ole.NewError(ole.E_UNEXPECTED)
	}

	tmpDir := t.TempDir()
	validPPTX := filepath.Join(tmpDir, "valid.pptx")
	createSamplePPTXInternal(t, validPPTX)

	if _, err := defaultExtractImagesCOM(validPPTX, tmpDir, nil, 0, 0); err == nil {
		t.Error("expected an error from extract when the COM apartment cannot be initialized")
	}
	if _, err := defaultMergeCOM([]string{validPPTX}, filepath.Join(tmpDir, "out.pptx")); err == nil {
		t.Error("expected an error from merge when the COM apartment cannot be initialized")
	}
}
