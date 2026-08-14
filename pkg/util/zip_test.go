package util_test

import (
	"archive/zip"
	"doctools-cli/pkg/util"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type dummyCompressor struct{ io.Writer }
func (c dummyCompressor) Close() error { return nil }

func TestZipUnzip(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.txt")
	os.WriteFile(file1, []byte("Hello Zip"), 0644)

	zipOut := filepath.Join(tmpDir, "out.zip")
	resZip, err := util.ZipCompress([]string{file1}, zipOut)
	if err != nil {
		t.Fatalf("ZipCompress failed: %v", err)
	}
	if _, err := os.Stat(resZip); os.IsNotExist(err) {
		t.Errorf("expected zip file to exist")
	}

	unzipDir := filepath.Join(tmpDir, "unzipped")
	resUnzip, err := util.UnzipDecompress(resZip, unzipDir)
	if err != nil {
		t.Fatalf("UnzipDecompress failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resUnzip, "f1.txt")); os.IsNotExist(err) {
		t.Errorf("expected unzipped file to exist")
	}
}

func TestZipCompress_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Non-existent path in ZipCompress
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")
	_, err := util.ZipCompress([]string{nonExistentPath}, filepath.Join(tmpDir, "out.zip"))
	if err == nil {
		t.Errorf("expected error compressing non-existent file")
	}

	// 2. Empty inputPaths
	_, err = util.ZipCompress([]string{}, filepath.Join(tmpDir, "out.zip"))
	if err == nil {
		t.Errorf("expected error compressing empty input paths")
	}
}

func TestUnzipDecompress_CorruptZip(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Non-existent zip file
	nonExistentZip := filepath.Join(tmpDir, "nonexistent.zip")
	_, err := util.UnzipDecompress(nonExistentZip, filepath.Join(tmpDir, "out"))
	if err == nil {
		t.Errorf("expected error decompressing non-existent zip file")
	}

	// 2. Corrupt zip file content
	corruptZip := filepath.Join(tmpDir, "corrupt.zip")
	_ = os.WriteFile(corruptZip, []byte("this is not a zip file content"), 0644)
	_, err = util.UnzipDecompress(corruptZip, filepath.Join(tmpDir, "out"))
	if err == nil {
		t.Errorf("expected error decompressing corrupt zip file")
	}

	// 3. Dest path error (cannot create file because dir name collision)
	validZip := filepath.Join(tmpDir, "valid.zip")
	zf, err := os.Create(validZip)
	if err == nil {
		zw := zip.NewWriter(zf)
		w, _ := zw.Create("file.txt")
		w.Write([]byte("data"))
		zw.Close()
		zf.Close()

		// Create a file where unzipped file should be created as directory
		conflictDir := filepath.Join(tmpDir, "conflict")
		_ = os.WriteFile(conflictDir, []byte("is file"), 0644)

		_, err = util.UnzipDecompress(validZip, filepath.Join(conflictDir, "sub"))
		if err == nil {
			t.Errorf("expected error when destDir cannot be created")
		}
	}

	// 4. os.OpenFile error (file is a directory)
	zf2, _ := os.Create(filepath.Join(tmpDir, "valid2.zip"))
	zw2 := zip.NewWriter(zf2)
	w2, _ := zw2.Create("file2.txt")
	w2.Write([]byte("data"))
	zw2.Close()
	zf2.Close()

	dest2 := filepath.Join(tmpDir, "dest2")
	os.MkdirAll(filepath.Join(dest2, "file2.txt"), 0755) // This is a directory!
	_, err = util.UnzipDecompress(filepath.Join(tmpDir, "valid2.zip"), dest2)
	if err == nil {
		t.Errorf("expected error when os.OpenFile fails")
	}

	// 5. Unsupported compression method
	zf3, _ := os.Create(filepath.Join(tmpDir, "valid3.zip"))
	zw3 := zip.NewWriter(zf3)
	zw3.RegisterCompressor(99, func(out io.Writer) (io.WriteCloser, error) {
		return dummyCompressor{out}, nil
	})
	w3, _ := zw3.CreateHeader(&zip.FileHeader{
		Name:   "file3.txt",
		Method: 99, // unsupported method
	})
	if w3 != nil {
		w3.Write([]byte("data"))
	}
	zw3.Close()
	zf3.Close()
	_, err = util.UnzipDecompress(filepath.Join(tmpDir, "valid3.zip"), filepath.Join(tmpDir, "dest3"))
	if err == nil {
		t.Errorf("expected error for unsupported compression method")
	}
}

