package excel_test

import (
	"doctools-cli/pkg/excel"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLoadPatchItems_Error(t *testing.T) {
	_, err := excel.LoadPatchItems([]byte("invalid json"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestLoadPatchItems_BOM(t *testing.T) {
	// UTF-8 BOM prefix
	bom := []byte{0xEF, 0xBB, 0xBF}
	jsonData := `[{"sheet":"Sheet1","cell":"A1","new_value":"X"}]`
	data := append(bom, []byte(jsonData)...)
	items, err := excel.LoadPatchItems(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Sheet != "Sheet1" {
		t.Errorf("unexpected items: %v", items)
	}
}

func TestLoadPatchItems_ExpectedOldValue(t *testing.T) {
	jsonData := `[{"sheet":"Sheet1","cell":"A1","expected_old_value":"OldVal","new_value":"NewVal"}]`
	items, err := excel.LoadPatchItems([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].OldValuePresent {
		t.Errorf("expected OldValuePresent to be true when expected_old_value is used")
	}
}

func TestLoadPatchItems_OldValueNull(t *testing.T) {
	jsonData := `[{"sheet":"Sheet1","cell":"A1","old_value":null,"new_value":"NewVal"}]`
	items, err := excel.LoadPatchItems([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].OldValuePresent {
		t.Errorf("expected OldValuePresent to be true")
	}
	if !items[0].OldValueIsNull {
		t.Errorf("expected OldValueIsNull to be true")
	}
}

func TestUnmarshalJSON_AllFieldCombinations(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		wantPresent     bool
		wantNull        bool
		wantOldValue    *string
		wantExpectedOld *string
	}{
		{
			name:        "no old_value key",
			json:        `{"sheet":"S","cell":"A1","new_value":"X"}`,
			wantPresent: false,
			wantNull:    false,
		},
		{
			name:        "old_value as string",
			json:        `{"sheet":"S","cell":"A1","old_value":"hello","new_value":"X"}`,
			wantPresent: true,
			wantNull:    false,
		},
		{
			name:        "old_value as null",
			json:        `{"sheet":"S","cell":"A1","old_value":null,"new_value":"X"}`,
			wantPresent: true,
			wantNull:    true,
		},
		{
			name:        "expected_old_value as string",
			json:        `{"sheet":"S","cell":"A1","expected_old_value":"world","new_value":"X"}`,
			wantPresent: true,
			wantNull:    false,
		},
		{
			name:        "expected_old_value as null",
			json:        `{"sheet":"S","cell":"A1","expected_old_value":null,"new_value":"X"}`,
			wantPresent: true,
			wantNull:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item excel.ExcelPatchItem
			err := json.Unmarshal([]byte(tt.json), &item)
			if err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if item.OldValuePresent != tt.wantPresent {
				t.Errorf("OldValuePresent = %v, want %v", item.OldValuePresent, tt.wantPresent)
			}
			if item.OldValueIsNull != tt.wantNull {
				t.Errorf("OldValueIsNull = %v, want %v", item.OldValueIsNull, tt.wantNull)
			}
		})
	}
}

func TestPatchExcel_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	fPath := filepath.Join(tmpDir, "normal.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Val")
	f.SaveAs(fPath)

	// test saving read-only file
	roFile := filepath.Join(tmpDir, "ro.xlsx")
	f2 := excelize.NewFile()
	f2.SetCellValue("Sheet1", "A1", "Val")
	f2.SaveAs(roFile)
	os.Chmod(roFile, 0444)

	patchItems := []excel.ExcelPatchItem{
		{Sheet: "Sheet1", Cell: "A1", NewValue: "NewValA1"},
	}
	_, err := excel.PatchExcel(roFile, patchItems, false, false)
	if err == nil {
		t.Errorf("expected error when saving to read only file")
	}

	// target missing sheet
	patchItemsBadSheet := []excel.ExcelPatchItem{
		{Sheet: "NonExistent", Cell: "A1", NewValue: "X"},
	}
	_, err = excel.PatchExcel(fPath, patchItemsBadSheet, false, false)
	if err == nil {
		t.Errorf("expected error when patching non-existent sheet")
	}

	// target un-parsable cell
	patchItemsBadCell := []excel.ExcelPatchItem{
		{Sheet: "Sheet1", Cell: "A!1", NewValue: "X"},
	}
	_, err = excel.PatchExcel(fPath, patchItemsBadCell, false, false)
	if err == nil {
		t.Errorf("expected error for bad cell")
	}
}

func TestPatchExcel_Backup(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "backup_test.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Original")
	f.SaveAs(fPath)

	patchItems := []excel.ExcelPatchItem{
		{Sheet: "Sheet1", Cell: "A1", NewValue: "Updated"},
	}
	result, err := excel.PatchExcel(fPath, patchItems, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BackupFile == "" {
		t.Errorf("expected backup file to be created")
	}
	if _, err := os.Stat(result.BackupFile); os.IsNotExist(err) {
		t.Errorf("backup file does not exist: %s", result.BackupFile)
	}
}

func TestPatchExcel_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "dryrun_test.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Original")
	f.SaveAs(fPath)

	patchItems := []excel.ExcelPatchItem{
		{Sheet: "Sheet1", Cell: "A1", NewValue: "Updated"},
	}
	result, err := excel.PatchExcel(fPath, patchItems, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DryRun {
		t.Errorf("expected DryRun to be true")
	}
}

func TestDiffExcel_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "fileA.xlsx")
	fileB := filepath.Join(tmpDir, "fileB.xlsx")
	f := excelize.NewFile()
	f.SaveAs(fileA)

	os.WriteFile(fileB, []byte("bad"), 0644)

	// A is ok, B is bad
	_, err := excel.DiffExcel(fileA, fileB, nil)
	if err == nil {
		t.Errorf("expected error for invalid fileB")
	}

	// A does not exist
	_, err = excel.DiffExcel("nonexistent.xlsx", fileA, nil)
	if err == nil {
		t.Errorf("expected error for nonexistent fileA")
	}
}

func TestDiffExcel_WithSheetFilter(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "diffA.xlsx")
	fileB := filepath.Join(tmpDir, "diffB.xlsx")

	fA := excelize.NewFile()
	fA.SetCellValue("Sheet1", "A1", "Hello")
	fA.SaveAs(fileA)

	fB := excelize.NewFile()
	fB.SetCellValue("Sheet1", "A1", "World")
	fB.SaveAs(fileB)

	result, err := excel.DiffExcel(fileA, fileB, []string{"Sheet1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChanges == 0 {
		t.Errorf("expected at least one change")
	}

	// Non-existent sheet filter
	_, err = excel.DiffExcel(fileA, fileB, []string{"NonExistent"})
	if err == nil {
		t.Errorf("expected error for non-existent sheet filter")
	}
}

func TestSortCells(t *testing.T) {
	cells := []string{"B2", "A1", "Invalid2", "Invalid1"}
	excel.SortCells(cells)
	if cells[0] != "A1" || cells[1] != "B2" || cells[2] != "Invalid1" || cells[3] != "Invalid2" {
		t.Errorf("unexpected sort order: %v", cells)
	}
}

func TestSearchCell_SheetFilter(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "search_filter.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "SearchMe")
	f.NewSheet("Sheet2")
	f.SetCellValue("Sheet2", "A1", "SearchMe")
	f.SaveAs(fPath)

	// Search with sheet filter
	result, err := excel.SearchCell(fPath, "SearchMe", []string{"Sheet1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalMatches != 1 {
		t.Errorf("expected 1 match, got %d", result.TotalMatches)
	}

	// Non-existent sheet filter
	_, err = excel.SearchCell(fPath, "SearchMe", []string{"NonExistent"})
	if err == nil {
		t.Errorf("expected error for non-existent sheet filter")
	}
}

func TestSearchCell_MergedCells(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "search_merged.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "MergedValue")
	f.MergeCell("Sheet1", "A1", "B2")
	f.SaveAs(fPath)

	result, err := excel.SearchCell(fPath, "MergedValue", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalMatches == 0 {
		t.Errorf("expected at least 1 match")
	}
	// Check that merged range is populated
	found := false
	for _, m := range result.Matches {
		if m.MergedRange != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one match with merged_range populated")
	}
}

func TestSearchCell_NonexistentFile(t *testing.T) {
	_, err := excel.SearchCell("nonexistent.xlsx", "test", nil)
	if err == nil {
		t.Errorf("expected error for nonexistent file")
	}
}

func TestSearchCell_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "corrupt.xlsx")
	os.WriteFile(fPath, []byte("not excel"), 0644)
	_, err := excel.SearchCell(fPath, "test", nil)
	if err == nil {
		t.Errorf("expected error for corrupt file")
	}
}

func TestExtractCSV_SheetFilter(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "csv_filter.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Data1")
	f.NewSheet("Sheet2")
	f.SetCellValue("Sheet2", "A1", "Data2")
	f.SaveAs(fPath)

	outDir := filepath.Join(tmpDir, "out")

	// Extract specific sheet
	paths, err := excel.ExtractCSV(fPath, outDir, []string{"Sheet1"}, "utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}

	// Non-existent sheet filter
	_, err = excel.ExtractCSV(fPath, "", []string{"NonExistent"}, "utf-8")
	if err == nil {
		t.Errorf("expected error for non-existent sheet filter")
	}
}

func TestExtractCSV_EmptyOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "empty_outdir.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Data")
	f.SaveAs(fPath)

	// Empty output dir should default to input file's directory
	paths, err := excel.ExtractCSV(fPath, "", nil, "utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}
