//go:build windows

package pptx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var (
	coInitialize = ole.CoInitialize
	coUninitialize = ole.CoUninitialize
	createObject = oleutil.CreateObject
	callMethod = oleutil.CallMethod
	getProperty = oleutil.GetProperty
	mustGetProperty = oleutil.MustGetProperty
	queryInterface = func(unk *ole.IUnknown, iid *ole.GUID) (*ole.IDispatch, error) {
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

func ExtractImagesCOM(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
	return ExtractImagesCOMImpl(inputPath, outputDir, slides, width, height)
}

func defaultExtractImagesCOM(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
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
	absOutputDir, _ := filepath.Abs(outputDir)

	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}

	// COM apartments are per OS thread, so the goroutine must stay on the
	// thread it initialized. Without this the runtime is free to move it and
	// the following OLE calls land on a thread where COM was never set up.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (code 1) only means COM was already initialized on this thread,
	// which is harmless. Any other failure leaves the apartment unusable, so it
	// must be reported instead of silently running OLE calls against it.
	if err := coInitialize(0); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return nil, fmt.Errorf("failed to initialize COM apartment: %w", err)
		}
	}
	defer coUninitialize()

	unknown, err := createObject("PowerPoint.Application")
	if err != nil {
		return nil, fmt.Errorf("failed to create PowerPoint.Application COM object: %w", err)
	}
	defer releaseUnknown(unknown)

	ppt, err := queryInterface(unknown, ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDispatch for PowerPoint: %w", err)
	}
	defer releaseDispatch(ppt)

	// POWERPNT.EXE is running from here on. Registering the shutdown before
	// touching any property guarantees it is torn down on every exit path,
	// including an early error return or a panic out of mustGetProperty.
	defer callMethod(ppt, "Quit")

	presVal, err := getProperty(ppt, "Presentations")
	if err != nil {
		return nil, fmt.Errorf("failed to get Presentations property: %w", err)
	}
	presentations := presVal.ToIDispatch()
	defer releaseDispatch(presentations)

	// Open(FileName, ReadOnly, Untitled, WithWindow)
	presVar, err := callMethod(presentations, "Open", absInput, true, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to open presentation: %w", err)
	}
	pres := presVar.ToIDispatch()
	defer releaseDispatch(pres)
	defer callMethod(pres, "Close")

	slidesColl := mustGetProperty(pres, "Slides").ToIDispatch()
	defer releaseDispatch(slidesColl)

	countVar := mustGetProperty(slidesColl, "Count")
	totalSlides := int(countVar.Val)

	var targetSlides []int
	if len(slides) == 0 {
		for i := 1; i <= totalSlides; i++ {
			targetSlides = append(targetSlides, i)
		}
	} else {
		for _, s := range slides {
			if s >= 1 && s <= totalSlides {
				targetSlides = append(targetSlides, s)
			}
		}
	}

	var outputPaths []string
	for _, sNum := range targetSlides {
		slideVar, err := callMethod(slidesColl, "Item", sNum)
		if err != nil {
			return nil, fmt.Errorf("failed to get slide %d: %w", sNum, err)
		}
		slide := slideVar.ToIDispatch()

		imgPath := filepath.Join(absOutputDir, fmt.Sprintf("slide_%03d.png", sNum))
		_, err = callMethod(slide, "Export", imgPath, "PNG", width, height)
		releaseDispatch(slide)
		if err != nil {
			return nil, fmt.Errorf("failed to export slide %d: %w", sNum, err)
		}
		outputPaths = append(outputPaths, imgPath)
	}

	return outputPaths, nil
}

var MergeCOMImpl = defaultMergeCOM

// MergeCOM merges multiple PPTX files into outputPath using PowerPoint COM.
func MergeCOM(inputPaths []string, outputPath string) (string, error) {
	return MergeCOMImpl(inputPaths, outputPath)
}

func defaultMergeCOM(inputPaths []string, outputPath string) (string, error) {
	if len(inputPaths) == 0 {
		return "", fmt.Errorf("input paths list is empty")
	}

	absInputs := make([]string, len(inputPaths))
	for i, p := range inputPaths {
		abs, err := filepath.Abs(p)
		if err != nil || os.IsNotExist(err) {
			return "", fmt.Errorf("input file not found: %s", p)
		}
		absInputs[i] = abs
	}

	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("invalid output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	// COM apartments are per OS thread, so the goroutine must stay on the
	// thread it initialized. Without this the runtime is free to move it and
	// the following OLE calls land on a thread where COM was never set up.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (code 1) only means COM was already initialized on this thread,
	// which is harmless. Any other failure leaves the apartment unusable, so it
	// must be reported instead of silently running OLE calls against it.
	if err := coInitialize(0); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return "", fmt.Errorf("failed to initialize COM apartment: %w", err)
		}
	}
	defer coUninitialize()

	unknown, err := createObject("PowerPoint.Application")
	if err != nil {
		return "", fmt.Errorf("failed to create PowerPoint COM object: %w", err)
	}
	defer releaseUnknown(unknown)

	ppt, err := queryInterface(unknown, ole.IID_IDispatch)
	if err != nil {
		return "", fmt.Errorf("failed to query IDispatch: %w", err)
	}
	defer releaseDispatch(ppt)

	// POWERPNT.EXE is running from here on. Registering the shutdown before
	// touching any property guarantees it is torn down on every exit path,
	// including an early error return or a panic out of mustGetProperty.
	defer callMethod(ppt, "Quit")

	presentations := mustGetProperty(ppt, "Presentations").ToIDispatch()
	defer releaseDispatch(presentations)

	basePresVar, err := callMethod(presentations, "Open", absInputs[0], false, false, false)
	if err != nil {
		return "", fmt.Errorf("failed to open base presentation %s: %w", absInputs[0], err)
	}
	basePres := basePresVar.ToIDispatch()
	defer releaseDispatch(basePres)
	defer callMethod(basePres, "Close")

	slidesColl := mustGetProperty(basePres, "Slides").ToIDispatch()
	defer releaseDispatch(slidesColl)

	for i := 1; i < len(absInputs); i++ {
		countVar := mustGetProperty(slidesColl, "Count")
		count := int(countVar.Val)
		_, err := callMethod(slidesColl, "InsertFromFile", absInputs[i], count)
		if err != nil {
			return "", fmt.Errorf("failed to insert slides from %s: %w", absInputs[i], err)
		}
	}

	_, err = callMethod(basePres, "SaveAs", absOutput)
	if err != nil {
		return "", fmt.Errorf("failed to save merged presentation to %s: %w", absOutput, err)
	}

	return absOutput, nil
}
