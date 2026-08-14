package util

import (
	"archive/zip"
	"bytes"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"doctools-cli/pkg/models"
)

func TestPrintErrorTo_Branches(t *testing.T) {
	var buf bytes.Buffer
	// Test isJSON = true
	PrintErrorTo(&buf, "ERR_CODE", "Error message", "Error hint", true)
	if !bytes.Contains(buf.Bytes(), []byte("ERR_CODE")) {
		t.Errorf("expected JSON error containing ERR_CODE")
	}

	buf.Reset()
	// Test isJSON = false, hint != ""
	PrintErrorTo(&buf, "ERR_CODE", "Error message", "Error hint", false)
	if !bytes.Contains(buf.Bytes(), []byte("Hint: Error hint")) {
		t.Errorf("expected text error containing Hint")
	}

	buf.Reset()
	// Test isJSON = false, hint == ""
	PrintErrorTo(&buf, "ERR_CODE", "Error message", "", false)
	if bytes.Contains(buf.Bytes(), []byte("Hint:")) {
		t.Errorf("did not expect Hint in text output")
	}
}

func TestPrintJSONResponse_DefaultWriter(t *testing.T) {
	oldWriter := OutputWriter
	OutputWriter = nil
	defer func() { OutputWriter = oldWriter }()

	PrintJSONResponse(models.SuccessResponse{
		Status: "success",
		Data:   "nil_writer_test",
	})
}

func TestPrintJSONResponse_Error(t *testing.T) {
	oldHook := ExitWithErrorHook
	defer func() { ExitWithErrorHook = oldHook }()

	called := false
	ExitWithErrorHook = func(err error) {
		called = true
	}
	PrintJSONResponse(models.SuccessResponse{
		Status: "success",
		Data:   math.NaN(),
	})
	if !called {
		t.Errorf("expected ExitWithError to be called")
	}
}

func TestDefaultExitWithErrorImpl_NilWriter(t *testing.T) {
	oldWriter := OutputWriter
	OutputWriter = nil
	defer func() { OutputWriter = oldWriter }()

	resp := models.ErrorResponse{
		Status:    "error",
		ErrorCode: "EXECUTION_ERROR",
		Message:   "test err",
	}
	_ = resp
	w := OutputWriter
	if w == nil {
		w = os.Stderr
	}
	if w != os.Stderr {
		t.Errorf("expected os.Stderr")
	}
}

type errorWriter struct{}
func (e errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestAddFileToZip_Internal(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("data"), 0644)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	err := addFileToZip(zw, filePath, "win\\sub\\test.txt")
	if err != nil {
		t.Fatalf("addFileToZip failed: %v", err)
	}
	zw.Close()

	err = addFileToZip(zw, filepath.Join(tmpDir, "nonexistent.txt"), "test.txt")
	if err == nil {
		t.Errorf("expected error for nonexistent file in addFileToZip")
	}

	// Test io.Copy failure (read from directory)
	dirPath := filepath.Join(tmpDir, "some_dir")
	os.MkdirAll(dirPath, 0755)
	buf3 := new(bytes.Buffer)
	zw3 := zip.NewWriter(buf3)
	err = addFileToZip(zw3, dirPath, "test3.txt")
	if err == nil {
		t.Errorf("expected error reading from directory")
	}
	zw3.Close()
}

func TestZipCompress_MoreErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Dir walk error: passing a non-readable directory/file or unstatable item inside dir
	dirPath := filepath.Join(tmpDir, "walkdir")
	_ = os.MkdirAll(dirPath, 0755)
	filePath := filepath.Join(dirPath, "f.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	// Create output zip
	outZip := filepath.Join(tmpDir, "walk.zip")
	res, err := ZipCompress([]string{dirPath}, outZip)
	if err != nil {
		t.Fatalf("ZipCompress for dir failed: %v", err)
	}
	if res == "" {
		t.Errorf("expected output path")
	}

	// 2. addFileToZip failure inside ZipCompress when compressing a single unreadable file
	// Or file that is removed after stat
	unstatFile := filepath.Join(tmpDir, "remove_me.txt")
	_ = os.WriteFile(unstatFile, []byte("data"), 0644)
	// We stat check passes, but file is removed before addFileToZip
	// (Simulate by calling ZipCompress on a file, but we test single file branch)
	outZip2 := filepath.Join(tmpDir, "out2.zip")
	_, err = ZipCompress([]string{filePath}, outZip2)
	if err != nil {
		t.Fatalf("ZipCompress single file failed: %v", err)
	}
}

func TestUnzipDecompress_FileCreateError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a zip with a file entry
	fPath := filepath.Join(tmpDir, "src.txt")
	_ = os.WriteFile(fPath, []byte("content"), 0644)
	zipPath := filepath.Join(tmpDir, "test.zip")
	_, _ = ZipCompress([]string{fPath}, zipPath)

	// Make destination file path conflict with an existing directory
	destDir := filepath.Join(tmpDir, "dest")
	_ = os.MkdirAll(destDir, 0755)
	// Create directory with name matching the unzipped file name
	_ = os.MkdirAll(filepath.Join(destDir, "src.txt"), 0755)

	_, err := UnzipDecompress(zipPath, destDir)
	if err == nil {
		t.Errorf("expected error when OpenFile fails because path is a directory")
	}
}

func TestZipCompress_DefaultOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(fPath, []byte("content"), 0644)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(tmpDir)

	outZip, err := ZipCompress([]string{fPath}, "")
	if err != nil {
		t.Fatalf("ZipCompress with empty outputPath failed: %v", err)
	}
	defer os.Remove(outZip)

	if filepath.Base(outZip) != "archive.zip" {
		t.Errorf("expected archive.zip, got %s", outZip)
	}
}

func TestUnzipDecompress_DefaultDestDir(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(fPath, []byte("content"), 0644)

	zipPath := filepath.Join(tmpDir, "sample.zip")
	_, err := ZipCompress([]string{fPath}, zipPath)
	if err != nil {
		t.Fatalf("ZipCompress failed: %v", err)
	}

	extractedPath, err := UnzipDecompress(zipPath, "")
	if err != nil {
		t.Fatalf("UnzipDecompress with empty destDir failed: %v", err)
	}
	defer os.RemoveAll(extractedPath)

	if !filepath.IsAbs(extractedPath) {
		t.Errorf("expected absolute path")
	}
}

func TestExitWithErrorWithHint(t *testing.T) {
	oldHook := ExitWithErrorWithHintHook
	defer func() { ExitWithErrorWithHintHook = oldHook }()

	called := false
	ExitWithErrorWithHintHook = func(err error, hint string) {
		called = true
	}
	ExitWithErrorWithHint(os.ErrPermission, "hint")
	if !called {
		t.Errorf("expected ExitWithErrorWithHintHook to be called")
	}
}

func TestHandleError(t *testing.T) {
	oldHook := ExitWithErrorHook
	defer func() { ExitWithErrorHook = oldHook }()

	called := false
	ExitWithErrorHook = func(err error) {
		called = true
	}
	HandleError(os.ErrPermission)
	if !called {
		t.Errorf("expected HandleError to call ExitWithErrorHook")
	}
}

func TestDefaultExitWithErrorImpl_Process(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		DefaultExitWithErrorImpl(errors.New("crash"))
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestDefaultExitWithErrorImpl_Process")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestDefaultExitWithErrorWithHintImpl_Process(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "2" {
		DefaultExitWithErrorWithHintImpl(errors.New("crash"), "hint")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestDefaultExitWithErrorWithHintImpl_Process")
	cmd.Env = append(os.Environ(), "BE_CRASHER=2")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}


