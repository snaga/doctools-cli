package excel

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ListSheets returns a slice of sheet names from an Excel file.

func ListSheets(inputPath string) ([]string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	return f.GetSheetList(), nil
}

// ExtractCSV extracts specified sheet_names (or all if empty) to CSV files in outputDir.
func ExtractCSV(inputPath string, outputDir string, sheetNames []string, encoding string) ([]string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	allSheets := f.GetSheetList()
	sheetMap := make(map[string]bool)
	for _, s := range allSheets {
		sheetMap[s] = true
	}

	var targetSheets []string
	if len(sheetNames) == 0 {
		targetSheets = allSheets
	} else {
		for _, s := range sheetNames {
			if !sheetMap[s] {
				return nil, fmt.Errorf("sheet '%s' not found", s)
			}
			targetSheets = append(targetSheets, s)
		}
	}

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	} else {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	baseWithoutExt := baseName[:len(baseName)-len(ext)]

	var outputPaths []string

	for _, sheetName := range targetSheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("failed to read rows from sheet %s: %w", sheetName, err)
		}

		outFileName := fmt.Sprintf("%s_%s.csv", baseWithoutExt, sheetName)
		outPath := filepath.Join(outputDir, outFileName)

		outFile, err := os.Create(outPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output csv file %s: %w", outPath, err)
		}

		writer := csv.NewWriter(outFile)
		for _, row := range rows {
			if err := writer.Write(row); err != nil {
				outFile.Close()
				return nil, fmt.Errorf("failed to write row to csv: %w", err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			outFile.Close()
			return nil, fmt.Errorf("failed to flush csv writer: %w", err)
		}
		outFile.Close()

		absPath, err := filepath.Abs(outPath)
		if err == nil {
			outPath = absPath
		}
		outputPaths = append(outputPaths, outPath)
	}

	return outputPaths, nil
}

// CellDiff represents a single difference in an Excel cell between two files.
type CellDiff struct {
	Sheet       string `json:"sheet"`
	Cell        string `json:"cell"`
	MergedRange string `json:"merged_range,omitempty"`
	Type        string `json:"type"` // "value_change", "formula_change", "merge_change"
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
	OldFormula  string `json:"old_formula,omitempty"`
	NewFormula  string `json:"new_formula,omitempty"`
}

// ExcelDiffResult holds all cell differences between fileA and fileB.
type ExcelDiffResult struct {
	FileA        string     `json:"file_a"`
	FileB        string     `json:"file_b"`
	TotalChanges int        `json:"total_changes"`
	Differences  []CellDiff `json:"differences"`
}

type mergeInfo struct {
	rangeStr    string
	isTopLeft   bool
	topLeftCell string
}

func buildMergeMap(f *excelize.File, sheet string) (map[string]mergeInfo, error) {
	mergeMap := make(map[string]mergeInfo)
	mergeCells, err := f.GetMergeCells(sheet)
	if err != nil {
		return nil, err
	}

	for _, mc := range mergeCells {
		startAxis := mc.GetStartAxis()
		endAxis := mc.GetEndAxis()
		rangeStr := startAxis + ":" + endAxis

		sCol, sRow, err1 := excelize.CellNameToCoordinates(startAxis)
		eCol, eRow, err2 := excelize.CellNameToCoordinates(endAxis)
		if err1 != nil || err2 != nil {
			continue
		}

		for r := sRow; r <= eRow; r++ {
			for c := sCol; c <= eCol; c++ {
				cellName, err := excelize.CoordinatesToCellName(c, r)
				if err != nil {
					continue
				}
				mergeMap[cellName] = mergeInfo{
					rangeStr:    rangeStr,
					isTopLeft:   (cellName == startAxis),
					topLeftCell: startAxis,
				}
			}
		}
	}

	return mergeMap, nil
}

// DiffExcel compares two Excel files and returns cell-level differences.
func DiffExcel(fileA, fileB string, sheetFilter []string) (*ExcelDiffResult, error) {
	if _, err := os.Stat(fileA); os.IsNotExist(err) {
		return nil, fmt.Errorf("file A not found: %s", fileA)
	}
	if _, err := os.Stat(fileB); os.IsNotExist(err) {
		return nil, fmt.Errorf("file B not found: %s", fileB)
	}

	fA, err := excelize.OpenFile(fileA)
	if err != nil {
		return nil, fmt.Errorf("failed to open file A: %w", err)
	}
	defer fA.Close()

	fB, err := excelize.OpenFile(fileB)
	if err != nil {
		return nil, fmt.Errorf("failed to open file B: %w", err)
	}
	defer fB.Close()

	sheetsA := fA.GetSheetList()
	sheetsB := fB.GetSheetList()

	mapA := make(map[string]bool)
	for _, s := range sheetsA {
		mapA[s] = true
	}
	mapB := make(map[string]bool)
	for _, s := range sheetsB {
		mapB[s] = true
	}

	var targetSheets []string
	if len(sheetFilter) > 0 {
		for _, s := range sheetFilter {
			if !mapA[s] || !mapB[s] {
				return nil, fmt.Errorf("sheet '%s' not found in both files", s)
			}
			targetSheets = append(targetSheets, s)
		}
	} else {
		for _, s := range sheetsA {
			if mapB[s] {
				targetSheets = append(targetSheets, s)
			}
		}
	}

	result := &ExcelDiffResult{
		FileA:       fileA,
		FileB:       fileB,
		Differences: []CellDiff{},
	}

	for _, sheet := range targetSheets {
		mergeMapA, err := buildMergeMap(fA, sheet)
		if err != nil {
			return nil, fmt.Errorf("failed to get merge cells for sheet %s in file A: %w", sheet, err)
		}
		mergeMapB, err := buildMergeMap(fB, sheet)
		if err != nil {
			return nil, fmt.Errorf("failed to get merge cells for sheet %s in file B: %w", sheet, err)
		}

		cellSet := make(map[string]bool)

		rowsA, _ := fA.GetRows(sheet)
		for rIdx, row := range rowsA {
			for cIdx := range row {
				cellName, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
				if err == nil {
					cellSet[cellName] = true
				}
			}
		}

		rowsB, _ := fB.GetRows(sheet)
		for rIdx, row := range rowsB {
			for cIdx := range row {
				cellName, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
				if err == nil {
					cellSet[cellName] = true
				}
			}
		}

		for cellName := range mergeMapA {
			cellSet[cellName] = true
		}
		for cellName := range mergeMapB {
			cellSet[cellName] = true
		}

		sortedCells := make([]string, 0, len(cellSet))
		for cellName := range cellSet {
			sortedCells = append(sortedCells, cellName)
		}

		sortCells(sortedCells)

		for _, cellName := range sortedCells {
			mInfoA, inA := mergeMapA[cellName]
			mInfoB, inB := mergeMapB[cellName]

			// Skip non-top-left cells of merged ranges
			if (inA && !mInfoA.isTopLeft) || (inB && !mInfoB.isTopLeft) {
				continue
			}

			rangeA := ""
			if inA {
				rangeA = mInfoA.rangeStr
			}
			rangeB := ""
			if inB {
				rangeB = mInfoB.rangeStr
			}

			mergedRange := ""
			if rangeB != "" {
				mergedRange = rangeB
			} else if rangeA != "" {
				mergedRange = rangeA
			}

			if rangeA != rangeB {
				result.Differences = append(result.Differences, CellDiff{
					Sheet:       sheet,
					Cell:        cellName,
					MergedRange: mergedRange,
					Type:        "merge_change",
					OldValue:    rangeA,
					NewValue:    rangeB,
				})
				continue
			}

			valA, _ := fA.GetCellValue(sheet, cellName)
			valB, _ := fB.GetCellValue(sheet, cellName)
			formulaA, _ := fA.GetCellFormula(sheet, cellName)
			formulaB, _ := fB.GetCellFormula(sheet, cellName)

			if formulaA != formulaB {
				result.Differences = append(result.Differences, CellDiff{
					Sheet:       sheet,
					Cell:        cellName,
					MergedRange: mergedRange,
					Type:        "formula_change",
					OldFormula:  formulaA,
					NewFormula:  formulaB,
					OldValue:    valA,
					NewValue:    valB,
				})
			} else if valA != valB {
				result.Differences = append(result.Differences, CellDiff{
					Sheet:       sheet,
					Cell:        cellName,
					MergedRange: mergedRange,
					Type:        "value_change",
					OldValue:    valA,
					NewValue:    valB,
				})
			}
		}
	}

	result.TotalChanges = len(result.Differences)
	return result, nil
}

func sortCells(cells []string) {
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			c1, r1, err1 := excelize.CellNameToCoordinates(cells[i])
			c2, r2, err2 := excelize.CellNameToCoordinates(cells[j])
			if err1 == nil && err2 == nil {
				if r1 > r2 || (r1 == r2 && c1 > c2) {
					cells[i], cells[j] = cells[j], cells[i]
				}
			} else if cells[i] > cells[j] {
				cells[i], cells[j] = cells[j], cells[i]
			}
		}
	}
}

type ExcelPatchItem struct {
	Sheet            string  `json:"sheet"`
	Cell             string  `json:"cell"`
	OldValue         *string `json:"old_value,omitempty"`
	ExpectedOldValue *string `json:"expected_old_value,omitempty"`
	NewValue         string  `json:"new_value"`
	Formula          string  `json:"formula,omitempty"`

	OldValuePresent bool `json:"-"`
	OldValueIsNull  bool `json:"-"`
}

func (e *ExcelPatchItem) UnmarshalJSON(data []byte) error {
	type Alias ExcelPatchItem
	aux := struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	rawOld, oldPresent := rawMap["old_value"]
	if !oldPresent {
		rawOld, oldPresent = rawMap["expected_old_value"]
	}

	if oldPresent {
		e.OldValuePresent = true
		if string(bytes.TrimSpace(rawOld)) == "null" {
			e.OldValueIsNull = true
		}
	}
	return nil
}

type PatchResultItem struct {
	Sheet    string `json:"sheet"`
	Cell     string `json:"cell"`
	Status   string `json:"status"` // "success", "error"
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
	Message  string `json:"message,omitempty"`
}

type ExcelPatchResult struct {
	TargetFile   string            `json:"target_file"`
	BackupFile   string            `json:"backup_file,omitempty"`
	DryRun       bool              `json:"dry_run"`
	TotalPatches int               `json:"total_patches"`
	AppliedCount int               `json:"applied_count"`
	AuditLog     []PatchResultItem `json:"audit_log"`
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

// LoadPatchItems parses raw patch data into a slice of ExcelPatchItem, handling UTF-8 BOM.
func LoadPatchItems(data []byte) ([]ExcelPatchItem, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var patchItems []ExcelPatchItem
	if err := json.Unmarshal(data, &patchItems); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	return patchItems, nil
}

// PatchExcel applies a set of patch items to an Excel file with guardrails.
func PatchExcel(targetPath string, patchItems []ExcelPatchItem, createBackup bool, dryRun bool) (*ExcelPatchResult, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("target file not found: %s", targetPath)
	}

	result := &ExcelPatchResult{
		TargetFile:   targetPath,
		DryRun:       dryRun,
		TotalPatches: len(patchItems),
		AuditLog:     make([]PatchResultItem, 0, len(patchItems)),
	}

	if createBackup && !dryRun {
		ext := filepath.Ext(targetPath)
		baseWithoutExt := targetPath[:len(targetPath)-len(ext)]
		timestamp := time.Now().Format("20060102_150405")
		backupPath := fmt.Sprintf("%s_backup_%s%s", baseWithoutExt, timestamp, ext)
		if err := copyFile(targetPath, backupPath); err != nil {
			return nil, fmt.Errorf("failed to create backup file: %w", err)
		}
		result.BackupFile = backupPath
	}

	f, err := excelize.OpenFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open target excel file: %w", err)
	}
	defer f.Close()

	type preparedOp struct {
		item        ExcelPatchItem
		targetCell  string
		oldVal      string
		mergedRange string
	}

	prepared := make([]preparedOp, 0, len(patchItems))

	// --- Validation Phase ---
	for i, item := range patchItems {
		if item.Sheet == "" || item.Cell == "" {
			return nil, fmt.Errorf("item #%d: sheet and cell must be specified", i+1)
		}

		mergeMap, err := buildMergeMap(f, item.Sheet)
		if err != nil {
			return nil, fmt.Errorf("item #%d: failed to get merge cells for sheet %s: %w", i+1, item.Sheet, err)
		}

		targetCell := item.Cell
		actualMergedRange := ""
		if mInfo, inMerge := mergeMap[item.Cell]; inMerge {
			targetCell = mInfo.topLeftCell
			actualMergedRange = mInfo.rangeStr
		}

		curVal, err := f.GetCellValue(item.Sheet, targetCell)
		if err != nil {
			return nil, fmt.Errorf("item #%d: failed to get cell value for %s!%s: %w", i+1, item.Sheet, targetCell, err)
		}

		var expectedOld *string
		if item.OldValue != nil {
			expectedOld = item.OldValue
		} else if item.ExpectedOldValue != nil {
			expectedOld = item.ExpectedOldValue
		}

		if expectedOld != nil {
			if curVal != *expectedOld {
				return nil, fmt.Errorf("validation failed for item #%d (%s!%s): expected old_value '%s', got '%s'", i+1, item.Sheet, item.Cell, *expectedOld, curVal)
			}
		} else if item.OldValuePresent && item.OldValueIsNull {
			fmt.Printf("[WARN] Cell %s: Guardrail explicitly bypassed (old_value is null).\n", item.Cell)
		} else {
			fmt.Printf("[WARN] Cell %s: Guardrail omitted. Consider specifying \"old_value\" for safe execution.\n", item.Cell)
		}

		prepared = append(prepared, preparedOp{
			item:        item,
			targetCell:  targetCell,
			oldVal:      curVal,
			mergedRange: actualMergedRange,
		})
	}

	// --- Apply Phase ---
	for _, pop := range prepared {
		item := pop.item
		targetCell := pop.targetCell
		newVal := item.NewValue

		if !dryRun {
			if item.Formula != "" {
				if err := f.SetCellFormula(item.Sheet, targetCell, item.Formula); err != nil {
					return nil, fmt.Errorf("failed to set cell formula for %s!%s: %w", item.Sheet, targetCell, err)
				}
				if item.NewValue != "" {
					_ = f.SetCellValue(item.Sheet, targetCell, item.NewValue)
				}
			} else {
				if err := f.SetCellValue(item.Sheet, targetCell, item.NewValue); err != nil {
					return nil, fmt.Errorf("failed to set cell value for %s!%s: %w", item.Sheet, targetCell, err)
				}
			}
		}

		if item.Formula != "" && newVal == "" {
			newVal = "=" + item.Formula
		}

		result.AuditLog = append(result.AuditLog, PatchResultItem{
			Sheet:    item.Sheet,
			Cell:     item.Cell,
			Status:   "success",
			OldValue: pop.oldVal,
			NewValue: newVal,
		})
		result.AppliedCount++
	}

	if !dryRun {
		if err := f.Save(); err != nil {
			return nil, fmt.Errorf("failed to save excel file: %w", err)
		}
	}

	return result, nil
}

// ExtractMarkdown converts Excel sheets to Markdown tables.
func ExtractMarkdown(inputPath string, sheetNames []string, withCoords bool) (string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputPath)
	}

	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	allSheets := f.GetSheetList()
	sheetMap := make(map[string]bool)
	for _, s := range allSheets {
		sheetMap[s] = true
	}

	var targetSheets []string
	if len(sheetNames) == 0 {
		targetSheets = allSheets
	} else {
		for _, s := range sheetNames {
			if !sheetMap[s] {
				return "", fmt.Errorf("sheet '%s' not found", s)
			}
			targetSheets = append(targetSheets, s)
		}
	}

	var sb strings.Builder

	for i, sheetName := range targetSheets {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", sheetName))

		rows, err := f.GetRows(sheetName)
		if err != nil {
			return "", fmt.Errorf("failed to read rows from sheet %s: %w", sheetName, err)
		}

		if len(rows) == 0 {
			continue
		}

		maxCols := 0
		for _, r := range rows {
			if len(r) > maxCols {
				maxCols = len(r)
			}
		}

		if maxCols == 0 {
			continue
		}

		for rIdx, row := range rows {
			rowNum := rIdx + 1
			sb.WriteString("|")
			for colIdx := 0; colIdx < maxCols; colIdx++ {
				colNum := colIdx + 1
				val := ""
				if colIdx < len(row) {
					val = row[colIdx]
				}

				if withCoords {
					cellName, _ := excelize.CoordinatesToCellName(colNum, rowNum)
					if val != "" {
						val = fmt.Sprintf("[%s] %s", cellName, val)
					} else {
						val = fmt.Sprintf("[%s]", cellName)
					}
				}

				val = strings.ReplaceAll(val, "|", "\\|")
				val = strings.ReplaceAll(val, "\n", " ")

				sb.WriteString(fmt.Sprintf(" %s |", val))
			}
			sb.WriteString("\n")

			if rIdx == 0 {
				sb.WriteString("|")
				for colIdx := 0; colIdx < maxCols; colIdx++ {
					sb.WriteString(" --- |")
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String(), nil
}

type CellSearchResult struct {
	Sheet       string `json:"sheet"`
	Cell        string `json:"cell"`
	MergedRange string `json:"merged_range,omitempty"`
	Value       string `json:"value"`
}

type ExcelSearchResult struct {
	TotalMatches int                `json:"total_matches"`
	Matches      []CellSearchResult `json:"matches"`
}

// SearchCell performs a case-insensitive partial match search on Excel cell values.
func SearchCell(inputPath string, query string, sheetNames []string) (*ExcelSearchResult, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	allSheets := f.GetSheetList()
	sheetMap := make(map[string]bool)
	for _, s := range allSheets {
		sheetMap[s] = true
	}

	var targetSheets []string
	if len(sheetNames) == 0 {
		targetSheets = allSheets
	} else {
		for _, s := range sheetNames {
			if !sheetMap[s] {
				return nil, fmt.Errorf("sheet '%s' not found", s)
			}
			targetSheets = append(targetSheets, s)
		}
	}

	result := &ExcelSearchResult{
		Matches: []CellSearchResult{},
	}

	lowerQuery := strings.ToLower(query)

	for _, sheetName := range targetSheets {
		mergeMap, err := buildMergeMap(f, sheetName)
		if err != nil {
			return nil, fmt.Errorf("failed to build merge map for sheet %s: %w", sheetName, err)
		}

		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("failed to read rows from sheet %s: %w", sheetName, err)
		}

		for rIdx, row := range rows {
			for cIdx, val := range row {
				if strings.Contains(strings.ToLower(val), lowerQuery) {
					cellName, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
					if err != nil {
						continue
					}

					mergedRange := ""
					if mInfo, inMerge := mergeMap[cellName]; inMerge {
						mergedRange = mInfo.rangeStr
					}

					result.Matches = append(result.Matches, CellSearchResult{
						Sheet:       sheetName,
						Cell:        cellName,
						MergedRange: mergedRange,
						Value:       val,
					})
				}
			}
		}
	}

	result.TotalMatches = len(result.Matches)
	return result, nil
}




