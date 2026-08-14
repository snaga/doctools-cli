package pptx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var MergeCOMHook = MergeCOM

// MergePureGo merges multiple PPTX files into outputBy copying slides into base archive structure.
// If pure zip manipulation is complex, fallback to MergeCOM on Windows or return structured merge.
func MergePureGo(inputPaths []string, outputPath string) (string, error) {
	// Try COM merge if available, or do zip-based merge
	outPath, err := MergeCOMHook(inputPaths, outputPath)
	if err == nil {
		return outPath, nil
	}

	if len(inputPaths) == 0 {
		return "", fmt.Errorf("input paths list is empty")
	}
	if len(inputPaths) == 1 {
		data, err := os.ReadFile(inputPaths[0])
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return "", err
		}
		return outputPath, nil
	}

	// Simple zip-based concatenation of presentation slides
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	// Read base zip
	baseZip, err := zip.OpenReader(inputPaths[0])
	if err != nil {
		return "", fmt.Errorf("failed to open base pptx zip: %w", err)
	}
	defer baseZip.Close()

	slideCount := 0
	for _, f := range baseZip.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		if err != nil {
			return "", err
		}
	}

	// Copy slides from subsequent pptx files with incremented index
	for i := 1; i < len(inputPaths); i++ {
		r, err := zip.OpenReader(inputPaths[i])
		if err != nil {
			return "", fmt.Errorf("failed to open pptx zip %s: %w", inputPaths[i], err)
		}

		for _, f := range r.File {
			if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
				numStr := strings.TrimPrefix(f.Name, "ppt/slides/slide")
				numStr = strings.TrimSuffix(numStr, ".xml")
				if _, err := strconv.Atoi(numStr); err == nil {
					slideCount++
					newSlideName := fmt.Sprintf("ppt/slides/slide%d.xml", slideCount)
					w, err := zw.Create(newSlideName)
					if err != nil {
						r.Close()
						return "", err
					}
					rc, err := f.Open()
					if err != nil {
						r.Close()
						return "", err
					}
					_, err = io.Copy(w, rc)
					rc.Close()
					if err != nil {
						r.Close()
						return "", err
					}
				}
			}
		}
		r.Close()
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}
