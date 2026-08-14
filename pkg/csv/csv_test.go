package csv_test

import (
	"doctools-cli/pkg/csv"
	"os"
	"path/filepath"
	"testing"
)

func createSampleCSV(t *testing.T, path string) {
	t.Helper()
	content := "Name,Age,City\nAlice,30,Tokyo\nBob,25,Osaka\n"
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test csv: %v", err)
	}
}

func TestCSV(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	createSampleCSV(t, csvPath)

	// Test GetMetadata
	meta, err := csv.GetMetadata(csvPath)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if meta.TotalRows != 3 || meta.MaxColumns != 3 {
		t.Errorf("unexpected metadata: %+v", meta)
	}

	// Test SearchValues
	results, err := csv.SearchValues(csvPath, "Tokyo")
	if err != nil {
		t.Fatalf("SearchValues failed: %v", err)
	}
	if len(results) != 1 || results[0].Row != 2 {
		t.Errorf("unexpected search results: %+v", results)
	}

	// Test ReadCells with headerRow and explicit column selection
	cells, err := csv.ReadCells(csvPath, 2, 3, []int{0, 2}, 1)
	if err != nil {
		t.Fatalf("ReadCells failed: %v", err)
	}
	if len(cells) != 3 { // header + 2 data rows
		t.Errorf("unexpected cells count: %d", len(cells))
	}

	// Test ReadCells without headerRow and empty columns
	cellsNoHeader, err := csv.ReadCells(csvPath, 1, 0, nil, 0)
	if err != nil {
		t.Fatalf("ReadCells no header failed: %v", err)
	}
	if len(cellsNoHeader) != 4 { // header label row + 3 data rows
		t.Errorf("unexpected cells count: %d", len(cellsNoHeader))
	}

	// Test Extract with custom outputPath
	outPath := filepath.Join(tmpDir, "out.csv")
	extracted, err := csv.Extract(csvPath, outPath, 2, 3, []int{0, 2})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if _, err := os.Stat(extracted); os.IsNotExist(err) {
		t.Errorf("expected output csv file to exist")
	}

	// Test Extract with default outputPath and all columns
	extractedDefault, err := csv.Extract(csvPath, "", 0, 0, nil)
	if err != nil {
		t.Fatalf("Extract default path failed: %v", err)
	}
	if _, err := os.Stat(extractedDefault); os.IsNotExist(err) {
		t.Errorf("expected default extracted file to exist")
	}
	defer os.Remove(extractedDefault)
}

func TestCSVErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.csv")

	// DetectEncoding non-existent
	_, err := csv.DetectEncoding(nonExistent)
	if err == nil {
		t.Errorf("expected error for DetectEncoding on nonexistent file")
	}

	// GetMetadata non-existent
	_, err = csv.GetMetadata(nonExistent)
	if err == nil {
		t.Errorf("expected error for GetMetadata on nonexistent file")
	}

	// ReadCells non-existent
	_, err = csv.ReadCells(nonExistent, 1, 1, nil, 0)
	if err == nil {
		t.Errorf("expected error for ReadCells on nonexistent file")
	}

	// SearchValues non-existent
	_, err = csv.SearchValues(nonExistent, "query")
	if err == nil {
		t.Errorf("expected error for SearchValues on nonexistent file")
	}

	// Extract non-existent
	_, err = csv.Extract(nonExistent, "", 1, 1, nil)
	if err == nil {
		t.Errorf("expected error for Extract on nonexistent file")
	}

	// Extract invalid output path (writing to directory)
	validCsv := filepath.Join(tmpDir, "valid.csv")
	createSampleCSV(t, validCsv)
	dirOut := filepath.Join(tmpDir, "dir_out")
	os.MkdirAll(dirOut, 0755)
	_, err = csv.Extract(validCsv, dirOut, 1, 1, nil)
	if err == nil {
		t.Errorf("expected error for Extract writing to directory")
	}
}
