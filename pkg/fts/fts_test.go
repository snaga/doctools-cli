package fts_test

import (
	"archive/zip"
	"doctools-cli/pkg/fts"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func createTestFiles(t *testing.T, dir string) {
	t.Helper()

	// 1. Text/MD files
	os.WriteFile(filepath.Join(dir, "doc1.md"), []byte("# Bleve Search\nBleve search engine implementation in Go."), 0644)
	os.WriteFile(filepath.Join(dir, "doc2.txt"), []byte("Search plain text content"), 0644)
	os.WriteFile(filepath.Join(dir, "doc3.csv"), []byte("header1,header2\nvalue1,value2"), 0644)
	os.WriteFile(filepath.Join(dir, "doc4.json"), []byte(`{"title":"JSON Search Key"}`), 0644)
	os.WriteFile(filepath.Join(dir, "doc5.html"), []byte(`<html><body><p>HTML search term paragraph</p></body></html>`), 0644)

	// 2. Excel file (.xlsx)
	excelPath := filepath.Join(dir, "sample.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Excel search keyword")
	_, _ = f.NewSheet("Sheet2")
	_ = f.SetCellValue("Sheet2", "A1", "Data in second sheet")
	if err := f.SaveAs(excelPath); err != nil {
		t.Fatalf("failed to create excel file: %v", err)
	}
	f.Close()

	// 3. PPTX file (.pptx)
	pptxPath := filepath.Join(dir, "sample.pptx")
	pf, err := os.Create(pptxPath)
	if err != nil {
		t.Fatalf("failed to create pptx file: %v", err)
	}
	zw := zip.NewWriter(pf)
	w, err := zw.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("failed to create slide xml in zip: %v", err)
	}
	_, _ = w.Write([]byte(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Presentation slide search phrase</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`))
	_ = zw.Close()
	_ = pf.Close()

	// 4. PDF file (.pdf)
	pdfPath := filepath.Join(dir, "sample.pdf")
	pdfDummy := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>endobj 4 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj 5 0 obj<</Length 62>>stream\nBT /F1 24 Tf 100 700 Td (PDF Page search content) Tj ET\nendstream\nendobj\nxref\n0 6\n0000000000 65535 f \n0000000009 00000 n \n0000000052 00000 n \n0000000101 00000 n \n0000000212 00000 n \n0000000287 00000 n \ntrailer<</Size 6/Root 1 0 R>>\nstartxref\n400\n%%EOF\n")
	if err := os.WriteFile(pdfPath, pdfDummy, 0644); err != nil {
		t.Fatalf("failed to create pdf file: %v", err)
	}
}

func TestNormalizeExt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"md", ".md"},
		{".md", ".md"},
		{" XLSX ", ".xlsx"},
		{".PDF", ".pdf"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := fts.NormalizeExt(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeExt(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// 1. On-the-fly チャンク化: Text/MD, Excel, PPTX, PDF のテストファイルに対するインデックス化と検索。
func TestFTS_OnTheFlyChunking(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFiles(t, tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")
	res, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     10 * time.Second,
		Force:       true,
		IncludeExts: []string{".xlsx", ".pptx", ".pdf", ".csv", ".txt", ".md", ".json", ".html"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}

	if res.IndexedFiles < 7 {
		t.Errorf("expected at least 7 indexed files (md, txt, csv, json, html, xlsx, pptx, pdf), got %d", res.IndexedFiles)
	}

	testCases := []struct {
		name          string
		query         string
		wantFileType  string
		wantUnitType  string
		wantUnitName  string
		wantPageOrIdx int
		wantLocator   string
	}{
		{
			name:          "Text/MD file search",
			query:         "Bleve",
			wantFileType:  ".md",
			wantUnitType:  "section",
			wantUnitName:  "",
			wantPageOrIdx: 0,
			wantLocator:   "",
		},
		{
			name:          "Excel file search",
			query:         "Excel",
			wantFileType:  ".xlsx",
			wantUnitType:  "sheet",
			wantUnitName:  "Sheet1",
			wantPageOrIdx: 0,
			wantLocator:   "sheet=Sheet1",
		},
		{
			name:          "PPTX file search",
			query:         "Presentation",
			wantFileType:  ".pptx",
			wantUnitType:  "slide",
			wantUnitName:  "",
			wantPageOrIdx: 1,
			wantLocator:   "slide=1",
		},
		{
			name:          "PDF file search",
			query:         "Page",
			wantFileType:  ".pdf",
			wantUnitType:  "page",
			wantUnitName:  "",
			wantPageOrIdx: 1,
			wantLocator:   "page=1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qRes, err := fts.QueryIndex(indexPath, tc.query, 10)
			if err != nil {
				t.Fatalf("QueryIndex failed for query %q: %v", tc.query, err)
			}
			if qRes.TotalHits < 1 {
				t.Fatalf("expected at least 1 hit for query %q, got 0", tc.query)
			}

			hit := qRes.Hits[0]
			if hit.Source.FileType != tc.wantFileType {
				t.Errorf("FileType = %q, want %q", hit.Source.FileType, tc.wantFileType)
			}
			if hit.Target.UnitType != tc.wantUnitType {
				t.Errorf("UnitType = %q, want %q", hit.Target.UnitType, tc.wantUnitType)
			}
			if tc.wantUnitName != "" && hit.Target.UnitName != tc.wantUnitName {
				t.Errorf("UnitName = %q, want %q", hit.Target.UnitName, tc.wantUnitName)
			}
			if tc.wantPageOrIdx > 0 && hit.Target.PageOrIndex != tc.wantPageOrIdx {
				t.Errorf("PageOrIndex = %d, want %d", hit.Target.PageOrIndex, tc.wantPageOrIdx)
			}
			if tc.wantLocator != "" && hit.Target.Locator != tc.wantLocator {
				t.Errorf("Locator = %q, want %q", hit.Target.Locator, tc.wantLocator)
			}
			if hit.Snippet == "" {
				t.Errorf("expected non-empty snippet")
			}
		})
	}
}

// 2. 差分インデックス化: 2回目の BuildIndex 実行時に skipped_files > 0 になること、force=true 時に再インデックス化されること。
func TestFTS_DifferentialIndexing(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFiles(t, tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")

	// 1st run (force = true)
	res1, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     10 * time.Second,
		Force:       true,
		IncludeExts: []string{".xlsx", ".pptx", ".pdf", ".csv", ".txt", ".md", ".json", ".html"},
	})
	if err != nil {
		t.Fatalf("1st BuildIndexWithOptions failed: %v", err)
	}
	if res1.IndexedFiles == 0 {
		t.Fatalf("expected indexed files > 0 on first build, got %d", res1.IndexedFiles)
	}

	// 2nd run (force = false, unchanged files)
	res2, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     10 * time.Second,
		Force:       false,
		IncludeExts: []string{".xlsx", ".pptx", ".pdf", ".csv", ".txt", ".md", ".json", ".html"},
	})
	if err != nil {
		t.Fatalf("2nd BuildIndexWithOptions failed: %v", err)
	}
	if res2.SkippedFiles == 0 {
		t.Errorf("expected SkippedFiles > 0 on 2nd build, got %d", res2.SkippedFiles)
	}
	if res2.IndexedFiles != 0 {
		t.Errorf("expected IndexedFiles == 0 on 2nd unchanged build, got %d", res2.IndexedFiles)
	}

	// 3rd run (force = true) -> Re-index all files
	res3, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     10 * time.Second,
		Force:       true,
		IncludeExts: []string{".xlsx", ".pptx", ".pdf", ".csv", ".txt", ".md", ".json", ".html"},
	})
	if err != nil {
		t.Fatalf("3rd BuildIndexWithOptions failed: %v", err)
	}
	if res3.SkippedFiles != 0 {
		t.Errorf("expected SkippedFiles == 0 on 3rd forced build, got %d", res3.SkippedFiles)
	}
	if res3.IndexedFiles == 0 {
		t.Errorf("expected IndexedFiles > 0 on 3rd forced build, got %d", res3.IndexedFiles)
	}
}

// 3. ファイルタイムアウト: タイムアウト時間を極小に設定した際、タイムアウトで安全にスキップ (timeout_files > 0) されること。
func TestFTS_FileTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFiles(t, tmpDir)

	indexPath := filepath.Join(tmpDir, "test.bleve")

	// Extremely short timeout (1 nanosecond)
	res, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     1 * time.Nanosecond,
		Force:       true,
		IncludeExts: []string{".xlsx", ".pptx", ".pdf", ".csv", ".txt", ".md", ".json", ".html"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}

	if res.TimeoutFiles == 0 {
		t.Errorf("expected TimeoutFiles > 0 with nanosecond timeout, got %d", res.TimeoutFiles)
	}
}

// 4. 構造化メタデータ・スニペット出力: QueryIndex の返り値で source (file_path, file_name 等) と target (unit_type, unit_name, page_or_index, locator 等) および snippet が正しく取得できること。
func TestFTS_StructuredMetadataAndSnippet(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file with known content
	fileName := "metadata_test.md"
	filePath := filepath.Join(tmpDir, fileName)
	content := "# Metadata Test Document\nThis document verifies structured metadata and snippet extraction in Bleve index."
	os.WriteFile(filePath, []byte(content), 0644)

	indexPath := filepath.Join(tmpDir, "meta.bleve")
	_, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Timeout:     5 * time.Second,
		Force:       true,
		IncludeExts: []string{"md"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}

	qRes, err := fts.QueryIndex(indexPath, "structured", 10)
	if err != nil {
		t.Fatalf("QueryIndex failed: %v", err)
	}

	if qRes.TotalHits != 1 {
		t.Fatalf("expected 1 hit, got %d", qRes.TotalHits)
	}

	hit := qRes.Hits[0]

	// Verify Source Metadata
	absFilePath, _ := filepath.Abs(filePath)
	if !strings.EqualFold(filepath.ToSlash(hit.Source.FilePath), filepath.ToSlash(absFilePath)) {
		t.Errorf("Source.FilePath = %q, want %q", hit.Source.FilePath, absFilePath)
	}
	if hit.Source.FileName != fileName {
		t.Errorf("Source.FileName = %q, want %q", hit.Source.FileName, fileName)
	}
	if hit.Source.FileType != ".md" {
		t.Errorf("Source.FileType = %q, want %q", hit.Source.FileType, ".md")
	}
	if hit.Source.UpdatedAt.IsZero() {
		t.Errorf("Source.UpdatedAt should not be zero")
	}

	// Verify Target Metadata
	if hit.Target.UnitType != "section" {
		t.Errorf("Target.UnitType = %q, want %q", hit.Target.UnitType, "section")
	}

	// Verify Snippet
	if hit.Snippet == "" {
		t.Errorf("Snippet should not be empty")
	}
	if !strings.Contains(hit.Snippet, "structured") && !strings.Contains(hit.Snippet, "Metadata") {
		t.Errorf("Snippet %q expected to contain query match text", hit.Snippet)
	}
}

func TestFTSErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	// BuildIndex non-existent source directory
	_, err := fts.BuildIndex(filepath.Join(tmpDir, "nonexistent"), "")
	if err == nil {
		t.Errorf("expected error for nonexistent source dir")
	}

	validIdx, _ := fts.BuildIndex(tmpDir, filepath.Join(tmpDir, "valid.bleve"))
	_, err = fts.QueryIndex(validIdx, "+/invalid syntax logic error query range +++:::", 10)
	if err == nil {
		t.Errorf("expected query error for invalid syntax query")
	}

	// QueryIndex non-existent index path
	_, err = fts.QueryIndex(filepath.Join(tmpDir, "nonexistent.bleve"), "query", 10)
	if err == nil {
		t.Errorf("expected error for nonexistent bleve index")
	}

	// QueryIndex invalid bleve index path (a plain file)
	invalidIndex := filepath.Join(tmpDir, "invalid.bleve")
	os.WriteFile(invalidIndex, []byte("not a bleve index"), 0644)
	_, err = fts.QueryIndex(invalidIndex, "query", 10)
	if err == nil {
		t.Errorf("expected error opening invalid bleve index")
	}
}

func TestFTSDefaultBuildIndex(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("default build index test"), 0644)

	defaultIdx, err := fts.BuildIndex(tmpDir, "")
	if err != nil {
		t.Fatalf("BuildIndex default path failed: %v", err)
	}
	defer os.RemoveAll(defaultIdx)

	if !strings.HasSuffix(defaultIdx, "fts.bleve") {
		t.Errorf("expected default index path to end with fts.bleve, got %q", defaultIdx)
	}
}

func TestFTS_ExtensionFiltering(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte("Markdown text content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "doc.txt"), []byte("Plain text content"), 0644)

	excelPath := filepath.Join(tmpDir, "doc.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Excel text content")
	_ = f.SaveAs(excelPath)
	f.Close()

	// 1. Default IncludeExts (should index doc.xlsx, skip doc.md and doc.txt)
	idxDefault := filepath.Join(tmpDir, "default.bleve")
	resDef, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath: idxDefault,
		Force:     true,
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions default failed: %v", err)
	}
	if resDef.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file (.xlsx) by default, got %d", resDef.IndexedFiles)
	}

	// 2. Custom IncludeExts (IncludeExts = ["md", "txt"])
	idxInc := filepath.Join(tmpDir, "inc.bleve")
	resInc, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxInc,
		Force:       true,
		IncludeExts: []string{"md", ".txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions include failed: %v", err)
	}
	if resInc.IndexedFiles != 2 {
		t.Errorf("expected 2 indexed files (.md, .txt), got %d", resInc.IndexedFiles)
	}

	// 3. ExcludeExts (IncludeExts = ["md", "txt", "xlsx"], ExcludeExts = ["xlsx"])
	idxExc := filepath.Join(tmpDir, "exc.bleve")
	resExc, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxExc,
		Force:       true,
		IncludeExts: []string{"md", "txt", "xlsx"},
		ExcludeExts: []string{"xlsx"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions exclude failed: %v", err)
	}
	if resExc.IndexedFiles != 2 {
		t.Errorf("expected 2 indexed files (.md, .txt), got %d", resExc.IndexedFiles)
	}
}

func TestNormalizeDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"docs", "docs"},
		{"docs/sub", "docs/sub"},
		{"docs\\sub", "docs/sub"},
		{"./docs", "docs"},
		{"/docs/", "docs"},
		{"  docs/sub  ", "docs/sub"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := fts.NormalizeDir(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFTS_DirectoryPruningAndFiltering(t *testing.T) {
	tmpDir := t.TempDir()

	// Structure:
	// tmpDir/
	//   doc_root.md
	//   docs/
	//     doc_docs.md
	//     sub/
	//       doc_sub.md
	//   ignored/
	//     doc_ignored.md
	//   .trash/
	//     doc_trash.md
	//   .obsidian/
	//     doc_obsidian.md

	_ = os.WriteFile(filepath.Join(tmpDir, "doc_root.md"), []byte("Root markdown file"), 0644)

	docsDir := filepath.Join(tmpDir, "docs")
	_ = os.MkdirAll(filepath.Join(docsDir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(docsDir, "doc_docs.md"), []byte("Docs markdown file"), 0644)
	_ = os.WriteFile(filepath.Join(docsDir, "sub", "doc_sub.md"), []byte("Sub docs markdown file"), 0644)

	ignoredDir := filepath.Join(tmpDir, "ignored")
	_ = os.MkdirAll(ignoredDir, 0755)
	_ = os.WriteFile(filepath.Join(ignoredDir, "doc_ignored.md"), []byte("Ignored markdown file"), 0644)

	trashDir := filepath.Join(tmpDir, ".trash")
	_ = os.MkdirAll(trashDir, 0755)
	_ = os.WriteFile(filepath.Join(trashDir, "doc_trash.md"), []byte("Trash markdown file"), 0644)

	obsidianDir := filepath.Join(tmpDir, ".obsidian")
	_ = os.MkdirAll(obsidianDir, 0755)
	_ = os.WriteFile(filepath.Join(obsidianDir, "doc_obsidian.md"), []byte("Obsidian markdown file"), 0644)

	// 1. Default run (hidden directories .trash and .obsidian should be skipped, doc_root, docs/*, ignored/* indexed)
	idxDefault := filepath.Join(tmpDir, "default.bleve")
	resDef, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxDefault,
		Force:       true,
		IncludeExts: []string{"md"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions default failed: %v", err)
	}
	// doc_root, doc_docs, doc_sub, doc_ignored = 4 files (.trash and .obsidian skipped)
	if resDef.IndexedFiles != 4 {
		t.Errorf("expected 4 indexed files by default (skipping hidden dirs), got %d", resDef.IndexedFiles)
	}

	// 2. ExcludeDirs run (exclude "ignored" and "docs/sub")
	idxExc := filepath.Join(tmpDir, "exc.bleve")
	resExc, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxExc,
		Force:       true,
		IncludeExts: []string{"md"},
		ExcludeDirs: []string{"ignored", "docs/sub"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions exclude dirs failed: %v", err)
	}
	// doc_root, doc_docs = 2 files
	if resExc.IndexedFiles != 2 {
		t.Errorf("expected 2 indexed files with ExcludeDirs, got %d", resExc.IndexedFiles)
	}

	// 3. IncludeDirs run (include only "docs")
	idxInc := filepath.Join(tmpDir, "inc.bleve")
	resInc, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxInc,
		Force:       true,
		IncludeExts: []string{"md"},
		IncludeDirs: []string{"docs"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions include dirs failed: %v", err)
	}
	// doc_docs, doc_sub = 2 files
	if resInc.IndexedFiles != 2 {
		t.Errorf("expected 2 indexed files with IncludeDirs=['docs'], got %d", resInc.IndexedFiles)
	}

	// 4. IncludeDirs allowing hidden folder explicitly (IncludeDirs = [".obsidian"])
	idxHidden := filepath.Join(tmpDir, "hidden.bleve")
	resHidden, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   idxHidden,
		Force:       true,
		IncludeExts: []string{"md"},
		IncludeDirs: []string{".obsidian"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions include hidden dir failed: %v", err)
	}
	// doc_obsidian = 1 file
	if resHidden.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file with IncludeDirs=['.obsidian'], got %d", resHidden.IndexedFiles)
	}
}

func TestBuildIndex_JapanesePDF(t *testing.T) {
	pdfName := "240920 IDC CIO Summit ホワイトカラーの生産性はなぜ低いのか R01.pdf"
	candidates := []string{
		filepath.Join("..", "..", pdfName),
		pdfName,
	}

	var targetPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			targetPath = c
			break
		}
	}

	if targetPath == "" {
		t.Skip("Japanese PDF not found, skipping TestBuildIndex_JapanesePDF")
	}

	tmpDir := t.TempDir()
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read japanese pdf: %v", err)
	}

	copiedPath := filepath.Join(tmpDir, pdfName)
	if err := os.WriteFile(copiedPath, data, 0644); err != nil {
		t.Fatalf("failed to copy japanese pdf: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "japanese_pdf.bleve")
	res, err := fts.BuildIndexWithOptions(tmpDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{".pdf"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Fatalf("expected 1 indexed file, got %d", res.IndexedFiles)
	}

	qRes, err := fts.QueryIndex(indexPath, "ホワイトカラー", 10)
	if err != nil {
		t.Fatalf("QueryIndex failed: %v", err)
	}

	if qRes.TotalHits < 1 {
		t.Fatalf("expected at least 1 hit for 'ホワイトカラー', got %d", qRes.TotalHits)
	}

	foundMark := false
	for _, hit := range qRes.Hits {
		if strings.Contains(hit.Snippet, "<mark>") {
			foundMark = true
		}
		if strings.Contains(hit.Snippet, "/Artifact BMC") {
			t.Errorf("hit.Snippet contains drawing operator '/Artifact BMC': %q", hit.Snippet)
		}
	}

	if !foundMark {
		t.Errorf("expected hit.Snippet to contain '<mark>' highlight, hits: %+v", qRes.Hits)
	}
}
