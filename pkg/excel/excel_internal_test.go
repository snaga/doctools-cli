package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ole/go-ole"
	"github.com/xuri/excelize/v2"
)

func TestExtractImagesCOM_Mock(t *testing.T) {
	oldImpl := ExtractImagesCOMImpl
	defer func() { ExtractImagesCOMImpl = oldImpl }()

	ExtractImagesCOMImpl = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
		if inputPath == "error.xlsx" {
			return nil, fmt.Errorf("mock error")
		}
		return []string{"mock_out.pdf"}, nil
	}

	paths, err := ExtractImagesCOM("test.xlsx", "out", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "mock_out.pdf" {
		t.Errorf("unexpected output paths: %v", paths)
	}

	_, err = ExtractImagesCOM("error.xlsx", "out", nil)
	if err == nil {
		t.Errorf("expected mock error")
	}
}

func TestDefaultExtractImagesCOM_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Invalid input path (file does not exist)
	_, err := defaultExtractImagesCOM("nonexistent.xlsx", tmpDir, nil)
	if err == nil {
		t.Errorf("expected error for nonexistent file")
	}

}

func TestExtractCSV_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-existent file
	_, err := ExtractCSV(filepath.Join(tmpDir, "nonexistent.xlsx"), tmpDir, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error for nonexistent file")
	}

	// Corrupt file
	corruptFile := filepath.Join(tmpDir, "corrupt.xlsx")
	_ = os.WriteFile(corruptFile, []byte("invalid content"), 0644)
	_, err = ExtractCSV(corruptFile, tmpDir, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error for corrupt excel file")
	}

	// Output dir is a file (MkdirAll failure)
	dummyFile := filepath.Join(tmpDir, "file_as_dir")
	_ = os.WriteFile(dummyFile, []byte("file"), 0644)
	validExcel := filepath.Join(tmpDir, "valid.xlsx")
	f := excelize.NewFile()
	_ = f.SaveAs(validExcel)
	f.Close()

	_, err = ExtractCSV(validExcel, dummyFile, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error when outputDir is a file")
	}

	// Output CSV file creation error (create dir named valid_Sheet1.csv inside outDir)
	outDir2 := filepath.Join(tmpDir, "out2")
	_ = os.MkdirAll(outDir2, 0755)
	_ = os.MkdirAll(filepath.Join(outDir2, "valid_Sheet1.csv"), 0755)

	_, err = ExtractCSV(validExcel, outDir2, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error when CSV output file cannot be created")
	}
}


// TestDefaultExtractImagesCOM_CoInitializeFailure checks that an unusable COM
// apartment is reported instead of being swallowed and running OLE calls anyway.
func TestDefaultExtractImagesCOM_CoInitializeFailure(t *testing.T) {
	old := coInitializeEx
	defer func() { coInitializeEx = old }()
	coInitializeEx = func(p uintptr, coinit uint32) error {
		return ole.NewError(ole.E_UNEXPECTED)
	}

	tmpDir := t.TempDir()
	path := newTestWorkbook(t, tmpDir)

	_, err := defaultExtractImagesCOM(path, tmpDir, nil)
	if err == nil {
		t.Fatal("expected an error when the COM apartment cannot be initialized")
	}
	if !strings.Contains(err.Error(), "failed to initialize COM apartment") {
		t.Errorf("unexpected error: %v", err)
	}
}

// withMockedCOM replaces every COM entry point with an in-memory stub and
// restores them when the test ends.
func withMockedCOM(t *testing.T) {
	t.Helper()

	oldCreate, oldQuery := createObject, queryInterface
	oldCall, oldGet, oldMustGet, oldPut := callMethod, getProperty, mustGetProperty, putProperty
	oldRelDisp, oldRelUnk := releaseDispatch, releaseUnknown
	t.Cleanup(func() {
		createObject, queryInterface = oldCreate, oldQuery
		callMethod, getProperty, mustGetProperty, putProperty = oldCall, oldGet, oldMustGet, oldPut
		releaseDispatch, releaseUnknown = oldRelDisp, oldRelUnk
	})

	createObject = func(programID string) (*ole.IUnknown, error) { return &ole.IUnknown{}, nil }
	queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
		return &ole.IDispatch{}, nil
	}
	callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	getProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	mustGetProperty = func(disp *ole.IDispatch, name string, params ...interface{}) *ole.VARIANT {
		return &ole.VARIANT{}
	}
	putProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
		return &ole.VARIANT{}, nil
	}
	releaseDispatch = func(disp *ole.IDispatch) {}
	releaseUnknown = func(unk *ole.IUnknown) {}
}

func newTestWorkbook(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "input.xlsx")
	wb := excelize.NewFile()
	wb.SetCellValue("Sheet1", "A1", "value")
	if err := wb.SaveAs(path); err != nil {
		t.Fatalf("failed to create test workbook: %v", err)
	}
	wb.Close()
	return path
}

func TestDefaultExtractImagesCOM_MockedSuccess(t *testing.T) {
	withMockedCOM(t)

	tmpDir := t.TempDir()
	path := newTestWorkbook(t, tmpDir)

	paths, err := defaultExtractImagesCOM(path, tmpDir, []string{"Sheet1"})
	if err != nil {
		t.Fatalf("expected success with mocked COM: %v", err)
	}
	if len(paths) != 1 || !strings.HasSuffix(paths[0], "temp_Sheet1.pdf") {
		t.Errorf("unexpected export paths: %v", paths)
	}
}

// TestDefaultExtractImagesCOM_MockedDefaultOutputDir also covers the branch that
// derives the output directory from the input file name.
func TestDefaultExtractImagesCOM_MockedDefaultOutputDir(t *testing.T) {
	withMockedCOM(t)

	tmpDir := t.TempDir()
	path := newTestWorkbook(t, tmpDir)

	paths, err := defaultExtractImagesCOM(path, "", []string{"Sheet1"})
	if err != nil {
		t.Fatalf("expected success with mocked COM: %v", err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], "input_images") {
		t.Errorf("expected the derived output directory, got: %v", paths)
	}
}

func TestDefaultExtractImagesCOM_MockedFailures(t *testing.T) {
	tmpDir := t.TempDir()
	path := newTestWorkbook(t, tmpDir)

	t.Run("create object fails", func(t *testing.T) {
		withMockedCOM(t)
		createObject = func(programID string) (*ole.IUnknown, error) {
			return nil, fmt.Errorf("mock create failure")
		}
		if _, err := defaultExtractImagesCOM(path, tmpDir, nil); err == nil {
			t.Error("expected an error when the COM object cannot be created")
		}
	})

	t.Run("query interface fails", func(t *testing.T) {
		withMockedCOM(t)
		queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
			return nil, fmt.Errorf("mock query failure")
		}
		if _, err := defaultExtractImagesCOM(path, tmpDir, nil); err == nil {
			t.Error("expected an error when IDispatch cannot be obtained")
		}
	})

	t.Run("workbook open fails", func(t *testing.T) {
		withMockedCOM(t)
		callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
			if name == "Open" {
				return nil, fmt.Errorf("mock open failure")
			}
			return &ole.VARIANT{}, nil
		}
		_, err := defaultExtractImagesCOM(path, tmpDir, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to open workbook") {
			t.Errorf("expected the workbook open error, got: %v", err)
		}
	})

	t.Run("sheet lookup fails", func(t *testing.T) {
		withMockedCOM(t)
		getProperty = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
			return nil, fmt.Errorf("mock missing sheet")
		}
		_, err := defaultExtractImagesCOM(path, tmpDir, []string{"Missing"})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected the sheet lookup error, got: %v", err)
		}
	})

	t.Run("export fails", func(t *testing.T) {
		withMockedCOM(t)
		callMethod = func(disp *ole.IDispatch, name string, params ...interface{}) (*ole.VARIANT, error) {
			if name == "ExportAsFixedFormat" {
				return nil, fmt.Errorf("mock export failure")
			}
			return &ole.VARIANT{}, nil
		}
		_, err := defaultExtractImagesCOM(path, tmpDir, []string{"Sheet1"})
		if err == nil || !strings.Contains(err.Error(), "failed to export sheet") {
			t.Errorf("expected the export error, got: %v", err)
		}
	})
}
