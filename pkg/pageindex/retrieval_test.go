package pageindex_test

import (
	"archive/zip"
	"doctools-cli/pkg/pageindex"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestGetTree(t *testing.T) {
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "doc.pdf")
	os.WriteFile(docPath, []byte("dummy pdf content"), 0644)

	jsonPath := docPath + pageindex.PageIndexExt
	jsonContent := `{
		"node_id": "root",
		"title": "Root Document",
		"nodes": [
			{
				"node_id": "sec1",
				"title": "Section 1",
				"start_index": 1,
				"end_index": 2,
				"nodes": [
					{
						"node_id": "sub1",
						"title": "Subsection 1"
					}
				]
			}
		]
	}`
	os.WriteFile(jsonPath, []byte(jsonContent), 0644)

	// Get tree root
	tree, err := pageindex.GetTree(docPath, "", 0) // depth 0 defaults to 2
	if err != nil {
		t.Fatalf("GetTree failed: %v", err)
	}
	if tree["node_id"] != "root" {
		t.Errorf("unexpected root node_id: %v", tree["node_id"])
	}

	// Get tree sec1 with depth 1
	secTree, err := pageindex.GetTree(docPath, "sec1", 1)
	if err != nil {
		t.Fatalf("GetTree node sec1 failed: %v", err)
	}
	if secTree["node_id"] != "sec1" {
		t.Errorf("unexpected section node_id: %v", secTree["node_id"])
	}

	// Error case: file not found
	_, err = pageindex.GetTree(filepath.Join(tmpDir, "nonexistent"), "", 2)
	if err == nil {
		t.Errorf("expected error for non-existent pageindex file")
	}

	// Error case: invalid json
	invalidDoc := filepath.Join(tmpDir, "invalid.pdf")
	os.WriteFile(invalidDoc+pageindex.PageIndexExt, []byte("{invalid json"), 0644)
	_, err = pageindex.GetTree(invalidDoc, "", 2)
	if err == nil {
		t.Errorf("expected error for invalid json")
	}

	// Error case: node id not found
	_, err = pageindex.GetTree(docPath, "nonexistent_node", 2)
	if err == nil {
		t.Errorf("expected error for nonexistent node id")
	}
}

func TestGetContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Error case: file not found
	_, err := pageindex.GetContent(filepath.Join(tmpDir, "nonexistent.pdf"), "pdf", "", 1, 1, "")
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}

	// PDF node type (using dummy pdf)
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	os.WriteFile(pdfPath, []byte("%PDF-1.4 header"), 0644)
	// PDF ExtractText will attempt reading PDF header/structure
	_, _ = pageindex.GetContent(pdfPath, "pdf", "", 1, 1, "")

	// PPTX node type (using dummy pptx zip)
	pptxPath := filepath.Join(tmpDir, "test.pptx")
	pf, _ := os.Create(pptxPath)
	pw := zip.NewWriter(pf)
	ws, _ := pw.Create("ppt/slides/slide1.xml")
	ws.Write([]byte(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>PPTX Slide Text</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`))
	pw.Close()
	pf.Close()

	pptxJsonPath := pptxPath + pageindex.PageIndexExt
	os.WriteFile(pptxJsonPath, []byte(`{
		"node_id": "root",
		"nodes": [
			{"node_id": "p1", "start_index": 1, "end_index": 1}
		]
	}`), 0644)

	pptxRes, err := pageindex.GetContent(pptxPath, "pptx", "p1", 0, 0, "")
	if err != nil {
		t.Fatalf("GetContent pptx failed: %v", err)
	}
	if pptxRes == "" {
		t.Errorf("expected pptx extracted content")
	}

	// Excel node type (using real excel file)
	excelPath := filepath.Join(tmpDir, "test.xlsx")
	ef := excelize.NewFile()
	ef.SetCellValue("Sheet1", "A1", "Excel Cell Content")
	ef.SaveAs(excelPath)

	excelJsonPath := excelPath + pageindex.PageIndexExt
	os.WriteFile(excelJsonPath, []byte(`{
		"node_id": "root",
		"nodes": [
			{"node_id": "Sheet1", "sheet_name": "Sheet1"}
		]
	}`), 0644)

	excelRes, err := pageindex.GetContent(excelPath, "xlsx", "Sheet1", 0, 0, "")
	if err != nil {
		t.Fatalf("GetContent excel failed: %v", err)
	}
	if excelRes == "" {
		t.Errorf("expected excel extracted content")
	}

	// Excel with node matching fallback node_id as sheetName
	os.WriteFile(excelJsonPath, []byte(`{
		"node_id": "Sheet1"
	}`), 0644)
	excelRes2, err := pageindex.GetContent(excelPath, "excel", "Sheet1", 0, 0, "")
	if err != nil {
		t.Fatalf("GetContent excel fallback failed: %v", err)
	}
	if excelRes2 == "" {
		t.Errorf("expected excel content from fallback sheetName")
	}

	// Unsupported node type
	_, err = pageindex.GetContent(excelPath, "unsupported", "", 0, 0, "")
	if err == nil {
		t.Errorf("expected error for unsupported node type")
	}
}
