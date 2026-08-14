package cli

import (
	"archive/zip"
	"bytes"
	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/pptx"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestCLICommands(t *testing.T) {
	// Mock Excel & PPTX COM hooks to avoid external COM dependency in CLI test
	oldExcelCOM := excel.ExtractImagesCOMImpl
	defer func() { excel.ExtractImagesCOMImpl = oldExcelCOM }()
	excel.ExtractImagesCOMImpl = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
		return []string{"mock_sheet.pdf"}, nil
	}

	oldPptxImgCOM := pptx.ExtractImagesCOMImpl
	defer func() { pptx.ExtractImagesCOMImpl = oldPptxImgCOM }()
	pptx.ExtractImagesCOMImpl = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
		return []string{"mock_slide.png"}, nil
	}

	oldPptxMergeCOM := pptx.MergeCOMHook
	defer func() { pptx.MergeCOMHook = oldPptxMergeCOM }()
	pptx.MergeCOMHook = func(inputs []string, output string) (string, error) {
		return "", os.ErrNotExist
	}

	tmpDir := t.TempDir()

	// 1. CSV Commands
	csvFile := filepath.Join(tmpDir, "sample.csv")
	os.WriteFile(csvFile, []byte("Name,Age\nAlice,30\nBob,25\n"), 0644)

	// csv metadata
	out, err, _ := runCLI("csv", "metadata", csvFile)
	if err != nil {
		t.Fatalf("csv metadata failed: %v", err)
	}
	_ = out

	// csv read-cells
	out, err, _ = runCLI("csv", "read-cells", csvFile, "--start-row", "1", "--end-row", "2", "--columns", "0", "--header-row", "1")
	if err != nil {
		t.Fatalf("csv read-cells failed: %v", err)
	}

	// csv search
	out, err, _ = runCLI("csv", "search", csvFile, "Alice")
	if err != nil {
		t.Fatalf("csv search failed: %v", err)
	}

	// csv extract
	outCsvExt := filepath.Join(tmpDir, "csv_ext.csv")
	out, err, _ = runCLI("csv", "extract", csvFile, "-o", outCsvExt)
	if err != nil {
		t.Fatalf("csv extract failed: %v", err)
	}

	// 2. Excel Commands
	excelFile := filepath.Join(tmpDir, "sample.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Hello Excel")
	f.SaveAs(excelFile)

	// excel list-sheets
	out, err, _ = runCLI("excel", "list-sheets", excelFile)
	if err != nil {
		t.Fatalf("excel list-sheets failed: %v", err)
	}

	// excel extract-csv
	out, err, _ = runCLI("excel", "extract-csv", excelFile, "-o", tmpDir)
	if err != nil {
		t.Fatalf("excel extract-csv failed: %v", err)
	}

	// excel extract-images (mocked hook)
	out, err, _ = runCLI("excel", "extract-images", excelFile)
	if err != nil {
		t.Fatalf("excel extract-images failed: %v", err)
	}

	// excel diff
	excelFile2 := filepath.Join(tmpDir, "sample2.xlsx")
	f2 := excelize.NewFile()
	f2.SetCellValue("Sheet1", "A1", "Hello Excel 2")
	f2.SaveAs(excelFile2)

	out, err, _ = runCLI("excel", "diff", excelFile, excelFile2)
	if err != nil {
		t.Fatalf("excel diff failed: %v", err)
	}

	out, err, _ = runCLI("excel", "diff", excelFile, excelFile2, "--json")
	if err != nil {
		t.Fatalf("excel diff --json failed: %v", err)
	}

	// excel patch --schema
	outSchema, err, _ := runCLI("excel", "patch", "--schema")
	if err != nil {
		t.Fatalf("excel patch --schema failed: %v", err)
	}
	if !strings.Contains(outSchema, "sheet") || !strings.Contains(outSchema, "cell") || !strings.Contains(outSchema, "new_value") {
		t.Errorf("unexpected schema output: %s", outSchema)
	}

	// excel patch
	patchFile := filepath.Join(tmpDir, "patch.json")
	os.WriteFile(patchFile, []byte(`[
		{"sheet": "Sheet1", "cell": "A1", "old_value": "Hello Excel", "new_value": "Patched Excel"}
	]`), 0644)

	out, err, _ = runCLI("excel", "patch", excelFile, "--patch-file", patchFile, "--dry-run")
	if err != nil {
		t.Fatalf("excel patch --dry-run failed: %v", err)
	}

	out, err, _ = runCLI("excel", "patch", excelFile, "-p", patchFile, "--json")
	if err != nil {
		t.Fatalf("excel patch --json failed: %v", err)
	}

	// excel extract-markdown
	out, err, _ = runCLI("excel", "extract-markdown", excelFile, "--with-coords")
	if err != nil {
		t.Fatalf("excel extract-markdown failed: %v", err)
	}

	mdOutFile := filepath.Join(tmpDir, "extracted.md")
	out, err, _ = runCLI("excel", "extract-markdown", excelFile, "-o", mdOutFile, "--json")
	if err != nil {
		t.Fatalf("excel extract-markdown -o --json failed: %v", err)
	}

	// excel extract-text (alias)
	out, err, _ = runCLI("excel", "extract-text", excelFile, "--with-coords")
	if err != nil {
		t.Fatalf("excel extract-text failed: %v", err)
	}

	// excel search-cell
	out, err, _ = runCLI("excel", "search-cell", excelFile, "Patched")
	if err != nil {
		t.Fatalf("excel search-cell failed: %v", err)
	}

	out, err, _ = runCLI("excel", "search-cell", excelFile, "Patched", "--json")
	if err != nil {
		t.Fatalf("excel search-cell --json failed: %v", err)
	}

	// 3. FTS Commands
	ftsDir := filepath.Join(tmpDir, "fts_data")
	os.MkdirAll(ftsDir, 0755)
	os.WriteFile(filepath.Join(ftsDir, "doc.txt"), []byte("searchable term in fts"), 0644)
	idxFile := filepath.Join(tmpDir, "test.bleve")

	// fts build
	out, err, _ = runCLI("fts", "build", ftsDir, "-i", idxFile, "--include-ext", "txt")
	if err != nil {
		t.Fatalf("fts build failed: %v", err)
	}

	// fts query
	out, err, _ = runCLI("fts", "query", idxFile, "searchable")
	if err != nil {
		t.Fatalf("fts query failed: %v", err)
	}

	// 4. HTML Commands
	htmlFile := filepath.Join(tmpDir, "sample.html")
	os.WriteFile(htmlFile, []byte("<html><body><h1>Title</h1></body></html>"), 0644)
	outMd := filepath.Join(tmpDir, "html_out.md")

	out, err, _ = runCLI("html", "extract-text", htmlFile, "-o", outMd)
	if err != nil {
		t.Fatalf("html extract-text failed: %v", err)
	}

	// 5. Image Commands
	imgFile := filepath.Join(tmpDir, "sample.png")
	imgObj := image.NewRGBA(image.Rect(0, 0, 50, 50))
	imgF, _ := os.Create(imgFile)
	png.Encode(imgF, imgObj)
	imgF.Close()

	// image metadata
	out, err, _ = runCLI("image", "metadata", imgFile)
	if err != nil {
		t.Fatalf("image metadata failed: %v", err)
	}

	// image crop
	cropOut := filepath.Join(tmpDir, "crop_out.png")
	out, err, _ = runCLI("image", "crop", imgFile, "-o", cropOut, "--left", "0", "--top", "0", "--right", "20", "--bottom", "20")
	if err != nil {
		t.Fatalf("image crop failed: %v", err)
	}

	// image save-clipboard
	cbOut := filepath.Join(tmpDir, "cb_out.png")
	out, err, _ = runCLI("image", "save-clipboard", "-o", cbOut)
	if err != nil {
		t.Fatalf("image save-clipboard failed: %v", err)
	}

	// 6. PageIndex Commands
	pdfFile := filepath.Join(tmpDir, "sample.pdf")
	os.WriteFile(pdfFile, []byte("dummy pdf"), 0644)
	jsonIdxFile := pdfFile + ".pageindex.json"
	os.WriteFile(jsonIdxFile, []byte(`{"node_id":"root","nodes":[{"node_id":"sec1"}]}`), 0644)

	// pageindex tree
	out, err, _ = runCLI("pageindex", "tree", pdfFile, "--node-id", "root", "-d", "2")
	if err != nil {
		t.Fatalf("pageindex tree failed: %v", err)
	}

	// pageindex content
	out, err, _ = runCLI("pageindex", "content", pdfFile, "-t", "pdf")
	if err != nil {
		t.Fatalf("pageindex content failed: %v", err)
	}

	// 7. PDF Commands (sample.pdf)
	samplePdfSrc := filepath.Join("..", "..", "tests", "test_data", "sample.pdf")
	if pdfData, err := os.ReadFile(samplePdfSrc); err == nil {
		realPdfFile := filepath.Join(tmpDir, "real_sample.pdf")
		os.WriteFile(realPdfFile, pdfData, 0644)

		out, err, _ = runCLI("pdf", "extract-text", realPdfFile)
		if err != nil {
			t.Fatalf("pdf extract-text failed: %v", err)
		}

		outSplit := filepath.Join(tmpDir, "pdf_split.pdf")
		out, err, _ = runCLI("pdf", "split", realPdfFile, "-o", outSplit, "--start-page", "1", "--end-page", "1")
		if err != nil {
			t.Fatalf("pdf split failed: %v", err)
		}

		outMerge := filepath.Join(tmpDir, "pdf_merge.pdf")
		out, err, _ = runCLI("pdf", "merge", realPdfFile, realPdfFile, "-o", outMerge)
		if err != nil {
			t.Fatalf("pdf merge failed: %v", err)
		}

		out, err, _ = runCLI("pdf", "extract-images", realPdfFile)
		if err != nil {
			t.Fatalf("pdf extract-images failed: %v", err)
		}

		outPages := filepath.Join(tmpDir, "pdf_pages_cli")
		out, err, _ = runCLI("pdf", "extract-pages", realPdfFile, "-o", outPages, "--dpi", "72", "--format", "jpg", "--force", "--json")
		if err != nil {
			t.Fatalf("pdf extract-pages failed: %v", err)
		}
		if !strings.Contains(out, `"total_pages"`) {
			t.Errorf("expected json response for extract-pages, got: %s", out)
		}
	}


	// 8. PPTX Commands
	pptxFile1 := filepath.Join(tmpDir, "sample1.pptx")
	pptxFile2 := filepath.Join(tmpDir, "sample2.pptx")
	createSamplePPTXCLI(t, pptxFile1)
	createSamplePPTXCLI(t, pptxFile2)

	out, err, _ = runCLI("pptx", "extract-text", pptxFile1)
	if err != nil {
		t.Fatalf("pptx extract-text failed: %v", err)
	}

	outPptxMerge := filepath.Join(tmpDir, "merged.pptx")
	out, err, _ = runCLI("pptx", "merge", pptxFile1, pptxFile2, "-o", outPptxMerge)
	if err != nil {
		t.Fatalf("pptx merge failed: %v", err)
	}

	out, err, _ = runCLI("pptx", "extract-images", pptxFile1)
	if err != nil {
		t.Fatalf("pptx extract-images failed: %v", err)
	}

	// 9. Text Commands
	txtFile := filepath.Join(tmpDir, "sample.txt")
	os.WriteFile(txtFile, []byte("Line 1\nLine 2\nLine 3\n"), 0644)

	out, err, _ = runCLI("text", "head", txtFile, "-n", "2")
	if err != nil {
		t.Fatalf("text head failed: %v", err)
	}

	out, err, _ = runCLI("text", "tail", txtFile, "-n", "2")
	if err != nil {
		t.Fatalf("text tail failed: %v", err)
	}

	out, err, _ = runCLI("text", "grep", txtFile, "Line")
	if err != nil {
		t.Fatalf("text grep failed: %v", err)
	}

	out, err, _ = runCLI("text", "convert", txtFile, "-e", "utf-8")
	if err != nil {
		t.Fatalf("text convert failed: %v", err)
	}

	out, err, _ = runCLI("text", "metadata", txtFile)
	if err != nil {
		t.Fatalf("text metadata failed: %v", err)
	}
	if !strings.Contains(out, `"encoding"`) || !strings.Contains(out, `"lines"`) {
		t.Errorf("expected encoding and lines in metadata output: %s", out)
	}

	out, err, _ = runCLI("text", "copy-clipboard", "Hello world")
	if err != nil {
		t.Fatalf("text copy-clipboard failed: %v", err)
	}

	// 10. Util Commands
	zipOut := filepath.Join(tmpDir, "arch.zip")
	out, err, _ = runCLI("util", "zip", txtFile, "-o", zipOut)
	if err != nil {
		t.Fatalf("util zip failed: %v", err)
	}

	unzipOut := filepath.Join(tmpDir, "unzipped")
	out, err, _ = runCLI("util", "unzip", zipOut, "-o", unzipOut)
	if err != nil {
		t.Fatalf("util unzip failed: %v", err)
	}

	// 11. JSON flag variants for each command to cover --json branches
	out, _, _ = runCLI("csv", "metadata", csvFile, "--json")
	out, _, _ = runCLI("csv", "read-cells", csvFile, "--start-row", "1", "--end-row", "2", "--columns", "0", "--header-row", "1", "--json")
	out, _, _ = runCLI("csv", "search", csvFile, "Alice", "--json")
	csvExtJson := filepath.Join(tmpDir, "csv_ext_json.csv")
	out, _, _ = runCLI("csv", "extract", csvFile, "-o", csvExtJson, "--json")

	out, _, _ = runCLI("excel", "list-sheets", excelFile, "--json")
	excelCsvJson := filepath.Join(tmpDir, "excel_csv_json")
	out, _, _ = runCLI("excel", "extract-csv", excelFile, "-o", excelCsvJson, "--json")
	out, _, _ = runCLI("excel", "extract-images", excelFile, "--json")
	out, _, _ = runCLI("excel", "extract-markdown", excelFile, "--json")
	out, _, _ = runCLI("excel", "search-cell", excelFile, "Patched", "--json")

	out, _, _ = runCLI("html", "extract-text", htmlFile, "--json")

	imgJsonOut := filepath.Join(tmpDir, "crop_json.png")
	out, _, _ = runCLI("image", "metadata", imgFile, "--json")
	out, _, _ = runCLI("image", "crop", imgFile, "-o", imgJsonOut, "--left", "0", "--top", "0", "--right", "20", "--bottom", "20", "--json")

	out, _, _ = runCLI("pageindex", "tree", pdfFile, "--node-id", "root", "--json")
	out, _, _ = runCLI("pageindex", "content", pdfFile, "-t", "pdf", "--json")

	out, _, _ = runCLI("text", "head", txtFile, "-n", "2", "--json")
	out, _, _ = runCLI("text", "tail", txtFile, "-n", "2", "--json")
	out, _, _ = runCLI("text", "grep", txtFile, "Line", "--json")
	out, _, _ = runCLI("text", "convert", txtFile, "-e", "utf-8", "--json")
	out, _, _ = runCLI("text", "metadata", txtFile, "--json")

	zipJsonOut := filepath.Join(tmpDir, "json_arch.zip")
	out, _, _ = runCLI("util", "zip", txtFile, "-o", zipJsonOut, "--json")
	unzipJsonOut := filepath.Join(tmpDir, "json_unzipped")
	out, _, _ = runCLI("util", "unzip", zipJsonOut, "-o", unzipJsonOut, "--json")

	// Execute RootCmd Execute wrapper
	Execute()
}


func TestCLIErrors(t *testing.T) {
	// Mock Excel & PPTX COM hooks for errors test
	oldExcelCOM := excel.ExtractImagesCOMImpl
	defer func() { excel.ExtractImagesCOMImpl = oldExcelCOM }()
	excel.ExtractImagesCOMImpl = func(inputPath string, outputDir string, sheetNames []string) ([]string, error) {
		return nil, os.ErrNotExist
	}

	oldPptxImgCOM := pptx.ExtractImagesCOMImpl
	defer func() { pptx.ExtractImagesCOMImpl = oldPptxImgCOM }()
	pptx.ExtractImagesCOMImpl = func(inputPath string, outputDir string, slides []int, width int, height int) ([]string, error) {
		return nil, os.ErrNotExist
	}

	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.file")

	// "util zip" and the merge commands default --output to a bare relative
	// path (archive.zip, merged.pptx, merged.pdf), so running from the package
	// directory would litter it. Every path below is absolute, so moving the
	// working directory only affects those defaults.
	t.Chdir(tmpDir)

	// Test error paths triggering ExitWithError
	runCLI("csv", "read-cells", nonExistent)
	runCLI("csv", "search", nonExistent, "q")
	runCLI("csv", "extract", nonExistent)
	runCLI("excel", "list-sheets", nonExistent)
	runCLI("excel", "extract-csv", nonExistent)
	runCLI("excel", "extract-images", nonExistent)
	runCLI("excel", "diff", nonExistent, nonExistent)
	runCLI("excel", "patch", nonExistent, "-p", nonExistent)


	runCLI("fts", "build", nonExistent)
	runCLI("fts", "query", nonExistent, "q")
	runCLI("html", "extract-text", nonExistent)
	runCLI("image", "metadata", nonExistent)
	runCLI("image", "crop", nonExistent)
	runCLI("pageindex", "tree", nonExistent)
	runCLI("pageindex", "content", nonExistent)
	runCLI("pdf", "extract-text", nonExistent)
	runCLI("pdf", "split", nonExistent)
	runCLI("pdf", "merge", nonExistent, nonExistent)
	runCLI("pdf", "extract-images", nonExistent)
	runCLI("pdf", "extract-pages", nonExistent)
	runCLI("pptx", "extract-text", nonExistent)
	runCLI("pptx", "merge", nonExistent, nonExistent)
	runCLI("pptx", "extract-images", nonExistent)
	runCLI("text", "head", nonExistent)
	runCLI("text", "tail", nonExistent)
	runCLI("text", "grep", nonExistent, "q")
	runCLI("text", "convert", nonExistent)
	runCLI("text", "metadata", nonExistent)
	runCLI("util", "zip", nonExistent)
	runCLI("util", "unzip", nonExistent)
}

// TestCSVMetadataError uses runCLI because the command dereferences its result
// right after the error check, so the test has to stop at ExitWithError the way
// os.Exit would.
func TestCSVMetadataError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.csv")

	_, _, exit := runCLI("csv", "metadata", missing)
	if exit == nil {
		t.Fatal("expected csv metadata to exit with an error for a missing file")
	}
}

func createSamplePPTXCLI(t *testing.T, path string) {
	t.Helper()
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("ppt/slides/slide1.xml")
	w.Write([]byte(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Slide 1</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`))
	zw.Close()
	f.Close()
}

func TestCLIFTSDetailed(t *testing.T) {
	execWithStdout := func(args ...string) (string, error) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		out, err, _ := runCLI(args...)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()

		if out != "" {
			return out + "\n" + buf.String(), err
		}
		return buf.String(), err
	}

	tmpDir := t.TempDir()
	ftsDir := filepath.Join(tmpDir, "fts_sources")
	os.MkdirAll(ftsDir, 0755)

	os.WriteFile(filepath.Join(ftsDir, "test_doc.md"), []byte("# CLI FTS Test Document\nTesting structured metadata response via CLI."), 0644)
	idxFile := filepath.Join(tmpDir, "cli_test.bleve")

	// 1. Initial build (-i, -f, --include-ext md)
	out, err := execWithStdout("fts", "build", ftsDir, "-i", idxFile, "-f", "--include-ext", "md")
	if err != nil {
		t.Fatalf("fts build -f failed: %v", err)
	}
	if !strings.Contains(out, `"status"`) || !strings.Contains(out, `"indexed_files": 1`) && !strings.Contains(out, `"indexed_files":1`) {
		t.Errorf("unexpected build response: %s", out)
	}

	time.Sleep(50 * time.Millisecond)

	// 2. Differential build (file unchanged, no force flag)
	outDiff, err := execWithStdout("fts", "build", ftsDir, "-i", idxFile, "--include-ext", "md")
	if err != nil {
		t.Fatalf("fts build diff failed: %v", err)
	}
	if !strings.Contains(outDiff, `"skipped_files": 1`) && !strings.Contains(outDiff, `"skipped_files":1`) {
		t.Errorf("expected skipped_files: 1 in response: %s", outDiff)
	}

	time.Sleep(50 * time.Millisecond)

	// 3. Timeout build (--file-timeout 1ns -f)
	outTimeout, err := execWithStdout("fts", "build", ftsDir, "-i", idxFile, "--file-timeout", "1ns", "-f", "--include-ext", "md")
	if err != nil {
		t.Fatalf("fts build timeout failed: %v", err)
	}
	if !strings.Contains(outTimeout, `"timeout_files": 1`) && !strings.Contains(outTimeout, `"timeout_files":1`) {
		t.Errorf("expected timeout_files: 1 in response: %s", outTimeout)
	}

	// 4. Invalid timeout string
	_, err = execWithStdout("fts", "build", ftsDir, "-i", idxFile, "-t", "invalid_duration")
	// Should fail parsing duration

	// 5. Query CLI with metadata check
	_, _ = execWithStdout("fts", "build", ftsDir, "-i", idxFile, "-f", "--include-ext", "md")
	outQuery, err := execWithStdout("fts", "query", idxFile, "structured", "-l", "5")
	if err != nil {
		t.Fatalf("fts query failed: %v", err)
	}
	if !strings.Contains(outQuery, `"source"`) || !strings.Contains(outQuery, `"target"`) || !strings.Contains(outQuery, `"snippet"`) {
		t.Errorf("query response missing structured metadata or snippet: %s", outQuery)
	}

	// 6. Test flag aliases (--ext, --exclude)
	idxAlias := filepath.Join(tmpDir, "cli_alias.bleve")
	outAlias, err := execWithStdout("fts", "build", ftsDir, "-i", idxAlias, "-f", "--ext", "md,txt", "--exclude", "txt")
	if err != nil {
		t.Fatalf("fts build with alias flags failed: %v", err)
	}
	if !strings.Contains(outAlias, `"indexed_files": 1`) && !strings.Contains(outAlias, `"indexed_files":1`) {
		t.Errorf("expected 1 indexed file with alias flags, got: %s", outAlias)
	}
}


