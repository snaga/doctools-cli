package text

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// resolveEncoding maps encoding names to encoding.Encoding implementations.
func resolveEncoding(name string) (encoding.Encoding, error) {
	norm := strings.ToLower(strings.TrimSpace(name))
	switch norm {
	case "utf-8", "utf8", "":
		return unicode.UTF8, nil
	case "sjis", "shift_jis", "shift-jis", "cp932", "windows-31j":
		return japanese.ShiftJIS, nil
	case "euc-jp", "eucjp":
		return japanese.EUCJP, nil
	case "iso-2022-jp":
		return japanese.ISO2022JP, nil
	case "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1, nil
	case "windows-1252", "cp1252":
		return charmap.Windows1252, nil
	default:
		enc, err := htmlindex.Get(norm)
		if err == nil {
			return enc, nil
		}
		return nil, fmt.Errorf("unsupported encoding: %s", name)
	}
}

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

	dstEnc, err := resolveEncoding(outputEncoding)
	if err != nil {
		return "", err
	}

	var outBytes []byte
	if len(content) == 0 {
		outBytes = []byte{}
	} else {
		// Detect source encoding
		detector := chardet.NewTextDetector()
		res, detErr := detector.DetectBest(content)
		srcEncName := "utf-8"
		if detErr == nil && res != nil && res.Charset != "" {
			srcEncName = res.Charset
		}

		srcEnc, err := resolveEncoding(srcEncName)
		if err != nil {
			srcEnc = unicode.UTF8
		}

		// Step 1: Decode input to UTF-8
		var utf8Bytes []byte
		if srcEnc == unicode.UTF8 {
			utf8Bytes = content
		} else {
			decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(content), srcEnc.NewDecoder()))
			if err != nil {
				return "", fmt.Errorf("failed to decode input text: %w", err)
			}
			utf8Bytes = decoded
		}

		// Step 2: Encode UTF-8 to target encoding
		if dstEnc == unicode.UTF8 {
			outBytes = utf8Bytes
		} else {
			encoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(utf8Bytes), dstEnc.NewEncoder()))
			if err != nil {
				return "", fmt.Errorf("failed to encode output text: %w", err)
			}
			outBytes = encoded
		}
	}

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = fmt.Sprintf("%s_%s%s", base, outputEncoding, ext)
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, outBytes, 0644); err != nil {
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
