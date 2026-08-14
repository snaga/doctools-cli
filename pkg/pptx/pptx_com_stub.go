//go:build !windows

package pptx

import "errors"

func ExtractImagesCOM(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
	return nil, errors.New("COM export is only supported on Windows")
}

func MergeCOM(inputPaths []string, outputPath string) (string, error) {
	return "", errors.New("COM merge is only supported on Windows")
}
