//go:build !windows

package excel

import (
	"errors"
)

func ExtractImagesCOM(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
	return nil, errors.New("COM export is only supported on Windows")
}
