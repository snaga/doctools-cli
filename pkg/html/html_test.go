package html_test

import (
	"doctools-cli/pkg/html"
	"os"
	"path/filepath"
	"testing"
)

func TestHTML(t *testing.T) {
	tmpDir := t.TempDir()
	htmlPath := filepath.Join(tmpDir, "test.html")
	content := "<html><head><script>var x=1;</script><style>body{color:red;}</style></head><body><h1>Header</h1><p>Paragraph text</p></body></html>"
	os.WriteFile(htmlPath, []byte(content), 0644)

	// ExtractText with custom outputPath
	outPath := filepath.Join(tmpDir, "out.md")
	resPath, err := html.ExtractText(htmlPath, outPath)
	if err != nil {
		t.Fatalf("ExtractText failed: %v", err)
	}
	if _, err := os.Stat(resPath); os.IsNotExist(err) {
		t.Errorf("expected extracted file to exist")
	}

	// ExtractText with empty outputPath (defaults to inputPath.md)
	resDefault, err := html.ExtractText(htmlPath, "")
	if err != nil {
		t.Fatalf("ExtractText default path failed: %v", err)
	}
	if _, err := os.Stat(resDefault); os.IsNotExist(err) {
		t.Errorf("expected default extracted file to exist")
	}
	defer os.Remove(resDefault)
}

func TestHTMLErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.html")

	_, err := html.ExtractText(nonExistent, "")
	if err == nil {
		t.Errorf("expected error for ExtractText on nonexistent file")
	}
}
