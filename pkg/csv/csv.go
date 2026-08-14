package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saintfish/chardet"
)

// DetectEncoding detects the character encoding of a file.
func DetectEncoding(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "utf-8", err
	}
	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(b)
	if err != nil {
		return "utf-8", nil
	}
	return result.Charset, nil
}

// CSVMetadata holds summary information of a CSV file.
type CSVMetadata struct {
	Encoding   string `json:"encoding"`
	TotalRows  int    `json:"total_rows"`
	MaxColumns int    `json:"max_columns"`
}

// GetMetadata returns CSV file metadata.
func GetMetadata(path string) (*CSVMetadata, error) {
	enc, _ := DetectEncoding(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	totalRows := 0
	maxCols := 0

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		totalRows++
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	return &CSVMetadata{
		Encoding:   enc,
		TotalRows:  totalRows,
		MaxColumns: maxCols,
	}, nil
}

// ReadCells reads specific rows/columns from a CSV file.
func ReadCells(path string, startRow int, endRow int, columns []int, headerRow int) ([][]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var allRows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		allRows = append(allRows, row)
	}

	var headerLabels []string
	if headerRow > 0 && headerRow <= len(allRows) {
		headerLabels = allRows[headerRow-1]
	}

	if startRow < 1 {
		startRow = 1
	}
	if endRow == 0 || endRow > len(allRows) {
		endRow = len(allRows)
	}

	var colIndices []int
	if len(columns) > 0 {
		colIndices = columns
	} else if len(allRows) > 0 {
		for i := 0; i < len(allRows[0]); i++ {
			colIndices = append(colIndices, i)
		}
	}

	var result [][]interface{}
	headers := []interface{}{"row_num"}
	for _, ci := range colIndices {
		label := fmt.Sprintf("col_%d", ci)
		if headerRow > 0 && ci < len(headerLabels) {
			label = headerLabels[ci]
		}
		headers = append(headers, label)
	}
	result = append(result, headers)

	for r := startRow; r <= endRow; r++ {
		if r == headerRow {
			continue
		}
		if r > len(allRows) {
			break
		}
		rowContent := allRows[r-1]
		rowArr := []interface{}{r}
		for _, ci := range colIndices {
			val := ""
			if ci < len(rowContent) {
				val = rowContent[ci]
			}
			rowArr = append(rowArr, val)
		}
		result = append(result, rowArr)
	}

	return result, nil
}

// SearchResult represents a match in CSV.
type SearchResult struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"`
}

// SearchValues searches string in CSV cells.
func SearchValues(path string, query string) ([]SearchResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var results []SearchResult
	qLower := strings.ToLower(query)

	rowIdx := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rowIdx++
		for colIdx, val := range row {
			if strings.Contains(strings.ToLower(val), qLower) {
				results = append(results, SearchResult{
					Row:   rowIdx,
					Col:   colIdx,
					Value: val,
				})
			}
		}
	}

	return results, nil
}

// Extract extracts specific rows and columns to a new CSV file.
func Extract(inputPath string, outputPath string, startRow int, endRow int, columns []int) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source csv file: %w", err)
	}
	defer f.Close()

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = base + "_extracted.csv"
	}

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var allRows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		allRows = append(allRows, row)
	}

	if startRow < 1 {
		startRow = 1
	}
	if endRow == 0 || endRow > len(allRows) {
		endRow = len(allRows)
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output csv file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	for r := startRow; r <= endRow; r++ {
		if r > len(allRows) {
			break
		}
		rowContent := allRows[r-1]
		var outRow []string
		if len(columns) > 0 {
			for _, ci := range columns {
				if ci < len(rowContent) {
					outRow = append(outRow, rowContent[ci])
				} else {
					outRow = append(outRow, "")
				}
			}
		} else {
			outRow = rowContent
		}
		if err := writer.Write(outRow); err != nil {
			return "", err
		}
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}
