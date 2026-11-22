package html

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtractText extracts text / markdown content from HTML.
// ExtractContent reads an HTML file and returns the extracted Markdown-like text content in-memory without creating any disk files.
func ExtractContent(inputPath string) (string, error) {
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read html file: %w", err)
	}

	htmlStr := string(b)
	// Simple HTML tag removal & line break formatting
	reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	htmlStr = reScript.ReplaceAllString(htmlStr, "")

	reStyle := regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	htmlStr = reStyle.ReplaceAllString(htmlStr, "")

	reTags := regexp.MustCompile(`<[^>]*>`)
	text := reTags.ReplaceAllString(htmlStr, "\n")

	// Collapse multiple newlines
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}
	return strings.Join(cleanLines, "\n\n"), nil
}

// ExtractText extracts text from an HTML file and saves it to outputPath.
// If outputPath is empty, it writes to a file with the same basename and .md extension.
func ExtractText(inputPath string, outputPath string) (string, error) {
	content, err := ExtractContent(inputPath)
	if err != nil {
		return "", err
	}

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = base + ".md"
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %w", err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}
