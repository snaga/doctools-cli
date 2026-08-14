package text

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/saintfish/chardet"
)

// ReadHead reads first n lines from file.
func ReadHead(path string, nLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= nLines {
			break
		}
	}
	return lines, scanner.Err()
}

// ReadTail reads last n lines from file.
func ReadTail(path string, nLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) <= nLines {
		return lines, nil
	}
	return lines[len(lines)-nLines:], nil
}

// MatchItem represents line match.
type MatchItem struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Grep searches pattern in file.
func Grep(path string, pattern string) ([]MatchItem, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var matches []MatchItem
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		if re.MatchString(text) {
			matches = append(matches, MatchItem{Line: lineNum, Text: text})
		}
	}
	return matches, scanner.Err()
}

// ConvertEncoding converts file from inputEncoding to outputEncoding.
func ConvertEncoding(inputPath string, outputEncoding string, outputPath string) (string, error) {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = fmt.Sprintf("%s_%s%s", base, outputEncoding, ext)
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write converted file: %w", err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

// TextMetadata represents file metadata.
type TextMetadata struct {
	Encoding string `json:"encoding"`
	Size     int64  `json:"size"`
	Lines    int    `json:"lines"`
}

// GetMetadata returns file size, encoding and line count.
func GetMetadata(path string) (*TextMetadata, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	b, _ := os.ReadFile(path)
	detector := chardet.NewTextDetector()
	res, _ := detector.DetectBest(b)
	enc := "utf-8"
	if res != nil {
		enc = res.Charset
	}

	lines, _ := ReadTail(path, 1000000)

	return &TextMetadata{
		Encoding: enc,
		Size:     st.Size(),
		Lines:    len(lines),
	}, nil
}
