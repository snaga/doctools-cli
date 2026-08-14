package text_test

import (
	"doctools-cli/pkg/text"
	"os"
	"path/filepath"
	"testing"
)

func TestTextOperations(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")
	content := "Line 1: Hello\nLine 2: World\nLine 3: Go\nLine 4: Doctools\n"
	os.WriteFile(filePath, []byte(content), 0644)

	head, err := text.ReadHead(filePath, 2)
	if err != nil || len(head) != 2 {
		t.Errorf("ReadHead failed: %v, len=%d", err, len(head))
	}

	// ReadTail nLines >= total lines
	tailAll, err := text.ReadTail(filePath, 10)
	if err != nil || len(tailAll) != 4 {
		t.Errorf("ReadTail all lines failed: %v, len=%d", err, len(tailAll))
	}

	tail, err := text.ReadTail(filePath, 2)
	if err != nil || len(tail) != 2 {
		t.Errorf("ReadTail failed: %v, len=%d", err, len(tail))
	}

	matches, err := text.Grep(filePath, "World")
	if err != nil || len(matches) != 1 || matches[0].Line != 2 {
		t.Errorf("Grep failed: %v, matches=%+v", err, matches)
	}

	meta, err := text.GetMetadata(filePath)
	if err != nil || meta.Lines != 4 {
		t.Errorf("GetMetadata failed: %v, meta=%+v", err, meta)
	}

	// ConvertEncoding with custom output path
	outConv := filepath.Join(tmpDir, "conv.txt")
	resConv, err := text.ConvertEncoding(filePath, "utf-8", outConv)
	if err != nil || resConv != outConv {
		t.Errorf("ConvertEncoding custom path failed: %v, res=%s", err, resConv)
	}

	// ConvertEncoding with default output path
	resConvDef, err := text.ConvertEncoding(filePath, "utf-8", "")
	if err != nil {
		t.Errorf("ConvertEncoding default path failed: %v", err)
	}
	defer os.Remove(resConvDef)
}

func TestTextErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.txt")

	// ReadHead non-existent
	_, err := text.ReadHead(nonExistent, 5)
	if err == nil {
		t.Errorf("expected error for ReadHead on nonexistent file")
	}

	// ReadTail non-existent
	_, err = text.ReadTail(nonExistent, 5)
	if err == nil {
		t.Errorf("expected error for ReadTail on nonexistent file")
	}

	// Grep non-existent
	_, err = text.Grep(nonExistent, "pattern")
	if err == nil {
		t.Errorf("expected error for Grep on nonexistent file")
	}

	// Grep invalid regex pattern
	_, err = text.Grep(filepath.Join(tmpDir, "valid.txt"), "[invalid regex")
	if err == nil {
		t.Errorf("expected error for Grep with invalid regex")
	}

	// ConvertEncoding non-existent
	_, err = text.ConvertEncoding(nonExistent, "utf-8", "")
	if err == nil {
		t.Errorf("expected error for ConvertEncoding on nonexistent file")
	}

	// ConvertEncoding invalid output path (writing to dir)
	validFile := filepath.Join(tmpDir, "valid.txt")
	os.WriteFile(validFile, []byte("valid content"), 0644)
	dirOut := filepath.Join(tmpDir, "dir_out")
	os.MkdirAll(dirOut, 0755)
	_, err = text.ConvertEncoding(validFile, "utf-8", dirOut)
	if err == nil {
		t.Errorf("expected error for ConvertEncoding writing to directory")
	}

	// GetMetadata non-existent
	_, err = text.GetMetadata(nonExistent)
	if err == nil {
		t.Errorf("expected error for GetMetadata on nonexistent file")
	}
}

func TestCopyClipboard(t *testing.T) {
	// Execute CopyClipboard
	res, err := text.CopyClipboard("Hello Clipboard Test")
	if err != nil {
		t.Errorf("CopyClipboard returned error: %v", err)
	}
	if res == "" {
		t.Errorf("expected response string from CopyClipboard")
	}
}
