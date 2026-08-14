package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipCompress compresses input files/dirs into zip file.
func ZipCompress(inputPaths []string, outputPath string) (string, error) {
	if len(inputPaths) == 0 {
		return "", fmt.Errorf("input paths list is empty")
	}

	if outputPath == "" {
		outputPath = "archive.zip"
	}
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}

	// The archive is created before the inputs are read, so any failure below
	// would otherwise leave a truncated file on disk - including in the caller's
	// working directory when outputPath fell back to the bare default.
	succeeded := false
	defer func() {
		outFile.Close()
		if !succeeded {
			_ = os.Remove(outputPath)
		}
	}()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	for _, inputPath := range inputPaths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return "", fmt.Errorf("input path not found: %s", inputPath)
		}

		if info.IsDir() {
			err = filepath.Walk(inputPath, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Dir(inputPath), path)
				if err != nil {
					return err
				}
				if fi.IsDir() {
					return nil
				}
				return addFileToZip(zw, path, rel)
			})
			if err != nil {
				return "", err
			}
		} else {
			if err := addFileToZip(zw, inputPath, filepath.Base(inputPath)); err != nil {
				return "", err
			}
		}
	}

	succeeded = true

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

func addFileToZip(zw *zip.Writer, srcPath string, zipPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := zw.Create(strings.ReplaceAll(zipPath, "\\", "/"))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// UnzipDecompress decompresses zip file into destDir.
func UnzipDecompress(zipPath string, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	if destDir == "" {
		ext := filepath.Ext(zipPath)
		base := zipPath[:len(zipPath)-len(ext)]
		destDir = base + "_extracted"
	}
	_ = os.MkdirAll(destDir, 0755)

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return "", err
		}
	}

	absPath, err := filepath.Abs(destDir)
	if err != nil {
		return destDir, nil
	}
	return absPath, nil
}
