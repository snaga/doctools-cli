package html

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtractText extracts text / markdown content from HTML.
func ExtractText(inputPath string, outputPath string) (string, error) {
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
	content := strings.Join(cleanLines, "\n\n")

	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := inputPath[:len(inputPath)-len(ext)]
		outputPath = base + ".md"
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	_ = os.WriteFile(outputPath, []byte(content), 0644)

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}
