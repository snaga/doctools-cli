package excel_test

import (
	"os"
	"path/filepath"
	"testing"

	"doctools-cli/pkg/excel"

	"github.com/xuri/excelize/v2"
)

func createTestExcel(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	index, err := f.NewSheet("Sheet2")
	if err != nil {
		t.Fatalf("failed to create sheet: %v", err)
	}
	f.SetCellValue("Sheet1", "A1", "Hello")
	f.SetCellValue("Sheet1", "B1", "World")
	f.SetCellValue("Sheet2", "A1", "Foo")
	f.SetCellValue("Sheet2", "B1", "Bar")
	f.SetActiveSheet(index)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("failed to save excel: %v", err)
	}
	return path
}

func TestListSheets(t *testing.T) {
	path := createTestExcel(t)
	sheets, err := excel.ListSheets(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %d", len(sheets))
	}
	if sheets[0] != "Sheet1" || sheets[1] != "Sheet2" {
		t.Errorf("unexpected sheet names: %v", sheets)
	}

	// Error case: file not found
	_, err = excel.ListSheets(filepath.Join(t.TempDir(), "nonexistent.xlsx"))
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	// Error case: invalid file format
	invalidFile := filepath.Join(t.TempDir(), "invalid.xlsx")
	os.WriteFile(invalidFile, []byte("not an excel file"), 0644)
	_, err = excel.ListSheets(invalidFile)
	if err == nil {
		t.Errorf("expected error for invalid excel file")
	}
}

func TestExtractCSV(t *testing.T) {
	path := createTestExcel(t)
	outDir := t.TempDir()

	// Extract all sheets with default outputDir
	paths, err := excel.ExtractCSV(path, outDir, nil, "utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 output csv files, got %d", len(paths))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", p)
		}
	}

	// Extract specific sheet
	paths, err = excel.ExtractCSV(path, "", []string{"Sheet1"}, "utf-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output csv file, got %d", len(paths))
	}
	defer os.Remove(paths[0])

	// Error case: non-existent file
	_, err = excel.ExtractCSV(filepath.Join(t.TempDir(), "nonexistent.xlsx"), outDir, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	// Error case: non-existent sheet
	_, err = excel.ExtractCSV(path, outDir, []string{"NonExistentSheet"}, "utf-8")
	if err == nil {
		t.Errorf("expected error for non-existent sheet")
	}

	// Error case: invalid file content
	invalidFile := filepath.Join(t.TempDir(), "invalid.xlsx")
	os.WriteFile(invalidFile, []byte("not excel"), 0644)
	_, err = excel.ExtractCSV(invalidFile, outDir, nil, "utf-8")
	if err == nil {
		t.Errorf("expected error for invalid excel file")
	}
}

func TestExtractImagesCOM(t *testing.T) {
	oldImpl := excel.ExtractImagesCOMImpl
	defer func() { excel.ExtractImagesCOMImpl = oldImpl }()

	excel.ExtractImagesCOMImpl = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
		if inputPath == "nonexistent.xlsx" {
			return nil, os.ErrNotExist
		}
		return []string{filepath.Join(outputDir, "temp_Sheet1.pdf")}, nil
	}

	tmpDir := t.TempDir()
	_, err := excel.ExtractImagesCOM("nonexistent.xlsx", tmpDir, nil)
	if err == nil {
		t.Errorf("expected error for non-existent file in ExtractImagesCOM")
	}

	path := createTestExcel(t)
	res, err := excel.ExtractImagesCOM(path, tmpDir, []string{"Sheet1"})
	if err != nil {
		t.Fatalf("unexpected error with mock COM: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 output path, got %d", len(res))
	}
}

func TestDiffExcel(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "fileA.xlsx")
	fileB := filepath.Join(tmpDir, "fileB.xlsx")

	// File A
	fA := excelize.NewFile()
	fA.SetCellValue("Sheet1", "A1", "Val1")
	fA.SetCellFormula("Sheet1", "A2", "SUM(1,2)")
	fA.SetCellValue("Sheet1", "B2", "MergedVal")
	fA.MergeCell("Sheet1", "B2", "D4")
	fA.SetCellValue("Sheet1", "E1", "SplitMe")
	fA.MergeCell("Sheet1", "E1", "E2")

	if err := fA.SaveAs(fileA); err != nil {
		t.Fatalf("failed to save fileA: %v", err)
	}

	// File B
	fB := excelize.NewFile()
	fB.SetCellValue("Sheet1", "A1", "Val2")         // value_change
	fB.SetCellFormula("Sheet1", "A2", "SUM(2,3)")   // formula_change
	fB.SetCellValue("Sheet1", "B2", "MergedValNew") // value_change on merged cell top-left
	fB.MergeCell("Sheet1", "B2", "D4")
	fB.SetCellValue("Sheet1", "E1", "SplitMe")
	// E1:E2 merge cell removed in File B -> merge_change

	if err := fB.SaveAs(fileB); err != nil {
		t.Fatalf("failed to save fileB: %v", err)
	}

	res, err := excel.DiffExcel(fileA, fileB, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalChanges != 4 {
		t.Errorf("expected 4 changes, got %d", res.TotalChanges)
	}

	diffMap := make(map[string]excel.CellDiff)
	for _, d := range res.Differences {
		diffMap[d.Cell] = d
	}

	// Check A1
	if diff, ok := diffMap["A1"]; ok {
		if diff.Type != "value_change" || diff.OldValue != "Val1" || diff.NewValue != "Val2" {
			t.Errorf("unexpected diff for A1: %+v", diff)
		}
	} else {
		t.Errorf("expected diff for A1")
	}

	// Check A2
	if diff, ok := diffMap["A2"]; ok {
		if diff.Type != "formula_change" || diff.OldFormula != "SUM(1,2)" || diff.NewFormula != "SUM(2,3)" {
			t.Errorf("unexpected diff for A2: %+v", diff)
		}
	} else {
		t.Errorf("expected diff for A2")
	}

	// Check B2 (Merged cell top-left)
	if diff, ok := diffMap["B2"]; ok {
		if diff.Type != "value_change" || diff.OldValue != "MergedVal" || diff.NewValue != "MergedValNew" {
			t.Errorf("unexpected diff for B2: %+v", diff)
		}
		if diff.MergedRange != "B2:D4" {
			t.Errorf("expected MergedRange B2:D4, got %s", diff.MergedRange)
		}
	} else {
		t.Errorf("expected diff for B2")
	}

	// Check E1 (Merge change)
	if diff, ok := diffMap["E1"]; ok {
		if diff.Type != "merge_change" || diff.OldValue != "E1:E2" || diff.NewValue != "" {
			t.Errorf("unexpected diff for E1: %+v", diff)
		}
	} else {
		t.Errorf("expected diff for E1")
	}

	// Non-top-left cells in merged range (e.g. B3, C2, D4) must NOT be reported as individual diffs
	if _, ok := diffMap["B3"]; ok {
		t.Errorf("non-top-left cell B3 should be skipped")
	}

	// Error case: file not found
	_, err = excel.DiffExcel("nonexistentA.xlsx", fileB, nil)
	if err == nil {
		t.Errorf("expected error for non-existent fileA")
	}

	// Error case: invalid sheet filter
	_, err = excel.DiffExcel(fileA, fileB, []string{"NonExistentSheet"})
	if err == nil {
		t.Errorf("expected error for non-existent sheet filter")
	}
}

func TestPatchExcel(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.xlsx")

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "OldValA1")
	f.SetCellValue("Sheet1", "B2", "MergedVal")
	f.SetCellValue("Sheet1", "C1", "ValC1")
	f.SetCellValue("Sheet1", "D1", "ValD1")
	f.MergeCell("Sheet1", "B2", "D4")
	if err := f.SaveAs(targetPath); err != nil {
		t.Fatalf("failed to create target excel: %v", err)
	}

	// 1. Dry run test
	oldValA1 := "OldValA1"
	patchItemsDry := []excel.ExcelPatchItem{
		{
			Sheet:    "Sheet1",
			Cell:     "A1",
			OldValue: &oldValA1,
			NewValue: "NewValA1",
		},
	}

	resDry, err := excel.PatchExcel(targetPath, patchItemsDry, true, true)
	if err != nil {
		t.Fatalf("unexpected error during dry run: %v", err)
	}
	if !resDry.DryRun {
		t.Errorf("expected DryRun=true in result")
	}
	if resDry.BackupFile != "" {
		t.Errorf("expected BackupFile empty in dry run, got %s", resDry.BackupFile)
	}
	if resDry.AppliedCount != 1 {
		t.Errorf("expected AppliedCount=1, got %d", resDry.AppliedCount)
	}

	// Check file was not modified
	fCheck, _ := excelize.OpenFile(targetPath)
	valA1, _ := fCheck.GetCellValue("Sheet1", "A1")
	fCheck.Close()
	if valA1 != "OldValA1" {
		t.Errorf("file was modified during dry run, A1 = %s", valA1)
	}

	// 2. Expected old value mismatch test (Atomic abort)
	wrongVal := "WrongVal"
	patchItemsMismatch := []excel.ExcelPatchItem{
		{
			Sheet:    "Sheet1",
			Cell:     "A1",
			OldValue: &wrongVal,
			NewValue: "NewValA1",
		},
	}
	_, err = excel.PatchExcel(targetPath, patchItemsMismatch, false, false)
	if err == nil {
		t.Errorf("expected error on old value mismatch, got nil")
	}

	// 3. Guardrail Matrix: null & omitted test
	// C1 has old_value explicitly null, D1 omits old_value
	patchItemsGuardrails := []excel.ExcelPatchItem{
		{
			Sheet:           "Sheet1",
			Cell:            "C1",
			NewValue:        "BypassedC1",
			OldValuePresent: true,
			OldValueIsNull:  true,
		},
		{
			Sheet:    "Sheet1",
			Cell:     "D1",
			NewValue: "OmittedD1",
		},
	}
	resGuardrails, err := excel.PatchExcel(targetPath, patchItemsGuardrails, false, false)
	if err != nil {
		t.Fatalf("unexpected error during guardrails patch: %v", err)
	}
	if resGuardrails.AppliedCount != 2 {
		t.Errorf("expected AppliedCount=2, got %d", resGuardrails.AppliedCount)
	}

	// 4. Successful patch application with backup, formula, and merged cell auto-resolution
	oldMergedVal := "MergedVal"
	patchItemsSuccess := []excel.ExcelPatchItem{
		{
			Sheet:    "Sheet1",
			Cell:     "A1",
			OldValue: &oldValA1,
			NewValue: "UpdatedA1",
		},
		{
			Sheet:            "Sheet1",
			Cell:             "C3", // automatically resolves to top-left cell B2
			ExpectedOldValue: &oldMergedVal,
			NewValue:         "UpdatedMergedVal",
		},
		{
			Sheet:   "Sheet1",
			Cell:    "E1",
			Formula: "SUM(10,20)",
		},
	}

	resSuccess, err := excel.PatchExcel(targetPath, patchItemsSuccess, true, false)
	if err != nil {
		t.Fatalf("unexpected error during successful patch: %v", err)
	}
	if resSuccess.AppliedCount != 3 {
		t.Errorf("expected AppliedCount=3, got %d", resSuccess.AppliedCount)
	}
	if resSuccess.BackupFile == "" {
		t.Errorf("expected BackupFile to be created")
	} else if _, err := os.Stat(resSuccess.BackupFile); os.IsNotExist(err) {
		t.Errorf("backup file %s does not exist on disk", resSuccess.BackupFile)
	}

	// Verify modified content
	fPatched, _ := excelize.OpenFile(targetPath)
	defer fPatched.Close()

	patchedA1, _ := fPatched.GetCellValue("Sheet1", "A1")
	if patchedA1 != "UpdatedA1" {
		t.Errorf("expected A1='UpdatedA1', got '%s'", patchedA1)
	}

	patchedB2, _ := fPatched.GetCellValue("Sheet1", "B2")
	if patchedB2 != "UpdatedMergedVal" {
		t.Errorf("expected B2='UpdatedMergedVal', got '%s'", patchedB2)
	}

	formulaE1, _ := fPatched.GetCellFormula("Sheet1", "E1")
	if formulaE1 != "SUM(10,20)" {
		t.Errorf("expected E1 formula='SUM(10,20)', got '%s'", formulaE1)
	}

	// 5. Test LoadPatchItems with UTF-8 BOM and null / omitted JSON
	bomJSON := []byte("\xef\xbb\xbf" + `[
		{"sheet": "Sheet1", "cell": "A1", "old_value": "UpdatedA1", "new_value": "BOMA1"},
		{"sheet": "Sheet1", "cell": "B1", "old_value": null, "new_value": "BOMNullB1"},
		{"sheet": "Sheet1", "cell": "C1", "new_value": "BOMOmittedC1"}
	]`)
	loadedItems, err := excel.LoadPatchItems(bomJSON)
	if err != nil {
		t.Fatalf("failed to load patch items with BOM: %v", err)
	}
	if len(loadedItems) != 3 {
		t.Fatalf("expected 3 loaded items, got %d", len(loadedItems))
	}
	if !loadedItems[1].OldValueIsNull || !loadedItems[1].OldValuePresent {
		t.Errorf("expected item[1] OldValueIsNull and OldValuePresent to be true")
	}
	if loadedItems[2].OldValuePresent {
		t.Errorf("expected item[2] OldValuePresent to be false")
	}
}

func TestExtractMarkdown(t *testing.T) {
	path := createTestExcel(t)

	// Standard extraction
	md, err := excel.ExtractMarkdown(path, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(md, "## Sheet1") || !containsStr(md, "| Hello | World |") {
		t.Errorf("unexpected markdown output:\n%s", md)
	}

	// Extraction with coords
	mdCoords, err := excel.ExtractMarkdown(path, []string{"Sheet1"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(mdCoords, "[A1] Hello") || !containsStr(mdCoords, "[B1] World") {
		t.Errorf("unexpected markdown with coords output:\n%s", mdCoords)
	}

	// Error cases
	_, err = excel.ExtractMarkdown("nonexistent.xlsx", nil, false)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	_, err = excel.ExtractMarkdown(path, []string{"NonExistentSheet"}, false)
	if err == nil {
		t.Errorf("expected error for non-existent sheet")
	}
}

func TestSearchCell(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "search_test.xlsx")

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "User Identification")
	f.SetCellValue("Sheet1", "C5", "USER_ID_COLUMN")
	f.SetCellValue("Sheet1", "B2", "Merged Keyword")
	f.MergeCell("Sheet1", "B2", "D4")

	if err := f.SaveAs(path); err != nil {
		t.Fatalf("failed to save excel: %v", err)
	}

	// Case-insensitive search for "user"
	res, err := excel.SearchCell(path, "user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalMatches != 2 {
		t.Fatalf("expected 2 matches for 'user', got %d", res.TotalMatches)
	}

	// Search for "merged" to test merged cell range info
	resMerged, err := excel.SearchCell(path, "merged", []string{"Sheet1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resMerged.TotalMatches != 1 {
		t.Fatalf("expected 1 match for 'merged', got %d", resMerged.TotalMatches)
	}
	if resMerged.Matches[0].MergedRange != "B2:D4" {
		t.Errorf("expected MergedRange 'B2:D4', got '%s'", resMerged.Matches[0].MergedRange)
	}

	// Error cases
	_, err = excel.SearchCell("nonexistent.xlsx", "query", nil)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	_, err = excel.SearchCell(path, "query", []string{"InvalidSheet"})
	if err == nil {
		t.Errorf("expected error for non-existent sheet")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || filepath.HasPrefix(s, substr) || (len(s) > 0 && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestDefaultExtractImagesCOM covers the checks that run before any COM object
// is created. Driving a real Excel instance lives behind the comtest build tag
// (see com_integration_test.go) because it depends on the installed Office.
func TestDefaultExtractImagesCOM(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := excel.DefaultExtractImagesCOM("nonexistent.xlsx", tmpDir, nil)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
}

func TestDiffExcel_Uncovered(t *testing.T) {
	// Cover merge changes when one is empty and another is set
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "fileA.xlsx")
	fileB := filepath.Join(tmpDir, "fileB.xlsx")

	fA := excelize.NewFile()
	fA.SetCellValue("Sheet1", "A1", "Val1")
	fA.MergeCell("Sheet1", "A1", "B2") // A1:B2

	fB := excelize.NewFile()
	fB.SetCellValue("Sheet1", "A1", "Val1")
	fB.MergeCell("Sheet1", "A1", "C3") // A1:C3
	
	fA.SaveAs(fileA)
	fB.SaveAs(fileB)

	res, err := excel.DiffExcel(fileA, fileB, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundMergeDiff := false
	for _, d := range res.Differences {
		if d.Type == "merge_change" && d.Cell == "A1" {
			foundMergeDiff = true
			if d.OldValue != "A1:B2" || d.NewValue != "A1:C3" {
				t.Errorf("unexpected merge diff: %+v", d)
			}
		}
	}
	if !foundMergeDiff {
		t.Errorf("expected merge diff for A1")
	}
}

func TestPatchExcel_Uncovered(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.xlsx")

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "OldValA1")
	f.SaveAs(targetPath)

	// Backup file error path
	// If we use a directory as targetPath, copyFile fails
	patchItems := []excel.ExcelPatchItem{
		{Sheet: "Sheet1", Cell: "A1", NewValue: "NewValA1"},
	}
	_, err := excel.PatchExcel(tmpDir, patchItems, true, false)
	if err == nil {
		t.Errorf("expected error for backup file creation failure")
	}
}




