//go:build windows

package excel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// COM entry points are indirected through variables so that tests can drive
// defaultExtractImagesCOM without a real Excel installation. Exercising these
// paths for real lives behind the comtest build tag.
var (
	coInitializeEx  = ole.CoInitializeEx
	createObject    = oleutil.CreateObject
	callMethod      = oleutil.CallMethod
	getProperty     = oleutil.GetProperty
	mustGetProperty = oleutil.MustGetProperty
	putProperty     = oleutil.PutProperty
	queryInterface  = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
		return unk.QueryInterface(iid)
	}
	releaseDispatch = func(disp *ole.IDispatch) {
		if disp != nil {
			disp.Release()
		}
	}
	releaseUnknown = func(unk *ole.IUnknown) {
		if unk != nil {
			unk.Release()
		}
	}
)

var ExtractImagesCOMImpl = defaultExtractImagesCOM

func ExtractImagesCOM(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
	return ExtractImagesCOMImpl(inputPath, outputDir, sheetNames)
}

func defaultExtractImagesCOM(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid input path: %w", err)
	}
	if _, err := os.Stat(absInput); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", absInput)
	}

	if outputDir == "" {
		baseName := filepath.Base(absInput)
		ext := filepath.Ext(baseName)
		outputDir = filepath.Join(filepath.Dir(absInput), baseName[:len(baseName)-len(ext)]+"_images")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		absOutputDir = outputDir
	}

	// COM apartments are per OS thread, so the goroutine must stay on the
	// thread it initialized. Without this the runtime is free to move it and
	// the following OLE calls land on a thread where COM was never set up.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (code 1) only means COM was already initialized on this thread,
	// which is harmless. Any other failure leaves the apartment unusable, so it
	// must be reported instead of silently running OLE calls against it.
	if err := coInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return nil, fmt.Errorf("failed to initialize COM apartment: %w", err)
		}
	}
	defer ole.CoUninitialize()

	unknown, err := createObject("Excel.Application")
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel.Application COM object: %w", err)
	}
	defer releaseUnknown(unknown)

	excel, err := queryInterface(unknown, ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDispatch for Excel: %w", err)
	}
	defer releaseDispatch(excel)

	// EXCEL.EXE is running from here on. Registering the shutdown before
	// touching any property guarantees it is torn down on every exit path,
	// including an early error return or a panic out of mustGetProperty.
	defer callMethod(excel, "Quit")

	putProperty(excel, "Visible", false)
	putProperty(excel, "DisplayAlerts", false)

	workbooks := mustGetProperty(excel, "Workbooks").ToIDispatch()
	defer releaseDispatch(workbooks)

	wb, err := callMethod(workbooks, "Open", absInput)
	if err != nil {
		return nil, fmt.Errorf("failed to open workbook in Excel: %w", err)
	}
	workbook := wb.ToIDispatch()
	defer releaseDispatch(workbook)
	defer callMethod(workbook, "Close", false)

	var targetSheets []string
	if len(sheetNames) == 0 {
		activeSheet := mustGetProperty(workbook, "ActiveSheet").ToIDispatch()
		name := mustGetProperty(activeSheet, "Name").ToString()
		releaseDispatch(activeSheet)
		targetSheets = []string{name}
	} else {
		targetSheets = sheetNames
	}

	var outputPaths []string
	sheets := mustGetProperty(workbook, "Sheets").ToIDispatch()
	defer releaseDispatch(sheets)

	for _, sName := range targetSheets {
		sheetVariant, err := getProperty(sheets, "Item", sName)
		if err != nil {
			return nil, fmt.Errorf("sheet '%s' not found: %w", sName, err)
		}
		sheet := sheetVariant.ToIDispatch()

		pageSetup := mustGetProperty(sheet, "PageSetup").ToIDispatch()
		putProperty(pageSetup, "Zoom", false)
		putProperty(pageSetup, "FitToPagesWide", 1)
		putProperty(pageSetup, "FitToPagesTall", false)
		releaseDispatch(pageSetup)

		pdfPath := filepath.Join(absOutputDir, fmt.Sprintf("temp_%s.pdf", sName))
		// xlTypePDF = 0
		_, err = callMethod(sheet, "ExportAsFixedFormat", 0, pdfPath)
		releaseDispatch(sheet)
		if err != nil {
			return nil, fmt.Errorf("failed to export sheet '%s' as PDF: %w", sName, err)
		}

		outputPaths = append(outputPaths, pdfPath)
	}

	return outputPaths, nil
}
