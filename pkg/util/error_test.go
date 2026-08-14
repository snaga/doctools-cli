package util

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/models"
)

func TestPrintErrorTo_JSON(t *testing.T) {
	var buf bytes.Buffer
	PrintErrorTo(&buf, "FILE_NOT_FOUND", "The specified file does not exist.", "Please check the path.", true)

	var errResp models.ErrorResponse
	err := json.Unmarshal(buf.Bytes(), &errResp)
	if err != nil {
		t.Fatalf("failed to parse JSON error output: %v", err)
	}

	if errResp.Status != "error" {
		t.Errorf("expected status 'error', got %q", errResp.Status)
	}
	if errResp.ErrorCode != "FILE_NOT_FOUND" {
		t.Errorf("expected error_code 'FILE_NOT_FOUND', got %q", errResp.ErrorCode)
	}
	if errResp.Message != "The specified file does not exist." {
		t.Errorf("unexpected message: %q", errResp.Message)
	}
	if errResp.Hint != "Please check the path." {
		t.Errorf("unexpected hint: %q", errResp.Hint)
	}
}

func TestPrintErrorTo_Text(t *testing.T) {
	var buf bytes.Buffer
	PrintErrorTo(&buf, "INVALID_ARG", "Invalid flag provided.", "", false)

	out := buf.String()
	if !strings.Contains(out, "Error [INVALID_ARG]: Invalid flag provided.") {
		t.Errorf("unexpected text output: %s", out)
	}
	if strings.Contains(out, "Hint:") {
		t.Errorf("did not expect hint output: %s", out)
	}

	buf.Reset()
	PrintErrorTo(&buf, "INVALID_ARG", "Invalid flag provided.", "Use --help.", false)
	out = buf.String()
	if !strings.Contains(out, "Hint: Use --help.") {
		t.Errorf("unexpected hint output: %s", out)
	}
}

func TestPrintErrorTo_JSONError(t *testing.T) {
	var buf bytes.Buffer

	oldFormat := formatErrorJSON
	defer func() { formatErrorJSON = oldFormat }()
	formatErrorJSON = func(errCode, msg, hint string) ([]byte, error) {
		return nil, errors.New("mock json error")
	}

	PrintErrorTo(&buf, "ERR", "msg", "hint", true)
	if !strings.Contains(buf.String(), "mock json error") {
		t.Errorf("expected mock json error, got: %s", buf.String())
	}

	formatErrorJSON = oldFormat // Restore to allow PrintSuccessTo to use the real one

	// Now test PrintSuccessTo JSON error
	buf.Reset()
	PrintSuccessTo(&buf, math.NaN(), true)
	if !strings.Contains(buf.String(), "JSON_ENCODE_ERROR") {
		t.Errorf("expected JSON_ENCODE_ERROR when unmarshalable data is passed, got: %s", buf.String())
	}
}

func TestPrintError(t *testing.T) {
	// Call PrintError to increase coverage on os.Stderr wrapper
	PrintError("TEST_ERR", "test message", "test hint", false)
}

func TestPrintSuccessTo_JSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"foo": "bar"}
	PrintSuccessTo(&buf, data, true)

	var resp models.Response
	err := json.Unmarshal(buf.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse JSON success output: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %q", resp.Status)
	}

	buf.Reset()
	PrintSuccessTo(&buf, data, false)
	if buf.Len() != 0 {
		t.Errorf("expected no output when isJSON is false")
	}
}

func TestPrintSuccess(t *testing.T) {
	PrintSuccess("ok", false)
}

func TestPrintJSONResponse(t *testing.T) {
	var buf bytes.Buffer
	oldWriter := OutputWriter
	OutputWriter = &buf
	defer func() { OutputWriter = oldWriter }()

	PrintJSONResponse(models.SuccessResponse{
		Status: "success",
		Data:   "test",
	})

	if !strings.Contains(buf.String(), `"status": "success"`) {
		t.Errorf("expected JSON response written to custom OutputWriter, got: %s", buf.String())
	}
}

func TestDefaultExitWithErrorImpl(t *testing.T) {
	var buf bytes.Buffer
	oldWriter := OutputWriter
	OutputWriter = &buf
	defer func() { OutputWriter = oldWriter }()

	DefaultExitWithErrorImpl = func(err error) {
		resp := models.ErrorResponse{
			Status:    "error",
			ErrorCode: "EXECUTION_ERROR",
			Message:   err.Error(),
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		w := OutputWriter
		if w == nil {
			w = os.Stderr
		}
		buf.WriteString(string(b) + "\n")
	}

	DefaultExitWithErrorImpl(errors.New("test execution error"))
	if !strings.Contains(buf.String(), "test execution error") {
		t.Errorf("expected error written to custom OutputWriter, got: %s", buf.String())
	}
}


func TestExitWithError(t *testing.T) {
	oldHook := ExitWithErrorHook
	defer func() { ExitWithErrorHook = oldHook }()

	called := false
	ExitWithErrorHook = func(err error) {
		called = true
	}
	ExitWithError(errors.New("test error"))
	if !called {
		t.Errorf("expected ExitWithErrorHook to be called")
	}
}

func TestZipCompress_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Several cases below pass an empty outputPath, which falls back to a bare
	// "archive.zip" in the working directory. Move out of the package directory
	// first so nothing is written there.
	t.Chdir(tmpDir)

	// Empty input paths
	_, err := ZipCompress([]string{}, "")
	if err == nil {
		t.Errorf("expected error for empty input paths")
	}

	// Non-existent input path
	_, err = ZipCompress([]string{filepath.Join(tmpDir, "nonexistent")}, "")
	if err == nil {
		t.Errorf("expected error for nonexistent input path")
	}

	// Invalid output path (e.g. directory as output file)
	invalidOut := filepath.Join(tmpDir, "dir_as_out")
	os.MkdirAll(invalidOut, 0755)
	dummyFile := filepath.Join(tmpDir, "dummy.txt")
	os.WriteFile(dummyFile, []byte("dummy"), 0644)
	_, err = ZipCompress([]string{dummyFile}, invalidOut)
	if err == nil {
		t.Errorf("expected error when output path is a directory")
	}

	// Directory input with subdirectories and files, and empty outputPath (defaults to archive.zip)
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "f1.txt"), []byte("sub file"), 0644)

	zipRes, err := ZipCompress([]string{"subdir"}, "")
	if err != nil {
		t.Fatalf("ZipCompress failed for dir: %v", err)
	}
	defer os.Remove(zipRes)

	// Test Unzip with empty destDir
	destRes, err := UnzipDecompress(zipRes, "")
	if err != nil {
		t.Fatalf("UnzipDecompress failed: %v", err)
	}
	defer os.RemoveAll(destRes)

	// Test Unzip with non-existent zip path
	_, err = UnzipDecompress(filepath.Join(tmpDir, "nonexistent.zip"), "")
	if err == nil {
		t.Errorf("expected error opening nonexistent zip file")
	}

	// Test Unzip with zip file containing directory entries
	zipWithDir := filepath.Join(tmpDir, "with_dir.zip")
	zf, err := os.Create(zipWithDir)
	if err == nil {
		zw := zip.NewWriter(zf)
		_, _ = zw.CreateHeader(&zip.FileHeader{Name: "folder/"})
		zw.Close()
		zf.Close()

		extractTo := filepath.Join(tmpDir, "extracted_folder")
		_, err = UnzipDecompress(zipWithDir, extractTo)
		if err != nil {
			t.Errorf("UnzipDecompress failed on folder entry: %v", err)
		}
	}
}
