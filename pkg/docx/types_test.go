package docx_test

import (
	"encoding/json"
	"encoding/xml"
	"testing"

	"doctools-cli/pkg/docx"
)

func TestDocxSectionTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		actual   docx.DocxSectionType
		expected string
	}{
		{"SectionHeading", docx.SectionHeading, "heading"},
		{"SectionParagraph", docx.SectionParagraph, "paragraph"},
		{"SectionTable", docx.SectionTable, "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.actual) != tt.expected {
				t.Errorf("got %q, want %q", tt.actual, tt.expected)
			}
		})
	}
}

func TestDocxSectionJSONSerialization(t *testing.T) {
	t.Run("FullSection", func(t *testing.T) {
		sec := docx.DocxSection{
			Type:         docx.SectionHeading,
			HeadingLevel: 1,
			HeadingTitle: "Introduction",
			Content:      "Introduction Header",
			Index:        0,
		}

		data, err := json.Marshal(sec)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var restored docx.DocxSection
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		if restored != sec {
			t.Errorf("restored struct does not match: got %+v, want %+v", restored, sec)
		}
	})

	t.Run("OmitemptyFields", func(t *testing.T) {
		sec := docx.DocxSection{
			Type:    docx.SectionParagraph,
			Content: "This is a normal paragraph.",
			Index:   1,
		}

		data, err := json.Marshal(sec)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		jsonStr := string(data)
		if jsonStr == "" {
			t.Fatal("serialized json is empty")
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("json.Unmarshal into map failed: %v", err)
		}

		if _, exists := m["heading_level"]; exists {
			t.Errorf("heading_level should be omitted when zero, but present in: %s", jsonStr)
		}
		if _, exists := m["heading_title"]; exists {
			t.Errorf("heading_title should be omitted when empty, but present in: %s", jsonStr)
		}
		if m["type"] != "paragraph" {
			t.Errorf("type mismatch: got %v, want paragraph", m["type"])
		}
		if m["content"] != "This is a normal paragraph." {
			t.Errorf("content mismatch: got %v", m["content"])
		}
	})
}

func TestXMLDocumentUnmarshal(t *testing.T) {
	xmlSnippet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading1"/>
        <w:outlineLvl w:val="0"/>
      </w:pPr>
      <w:r>
        <w:t>Title Text</w:t>
      </w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:p>
            <w:r>
              <w:t xml:space="preserve">Cell 1 </w:t>
            </w:r>
          </w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`

	var doc docx.XMLDocument
	if err := xml.Unmarshal([]byte(xmlSnippet), &doc); err != nil {
		t.Fatalf("xml.Unmarshal failed: %v", err)
	}

	if len(doc.Body.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph in body, got %d", len(doc.Body.Paragraphs))
	}

	p := doc.Body.Paragraphs[0]
	if p.Pr == nil || p.Pr.PStyle == nil || p.Pr.PStyle.Val != "Heading1" {
		t.Errorf("expected Heading1 style, got %+v", p.Pr)
	}
	if p.Pr.OutlineLvl == nil || p.Pr.OutlineLvl.Val != 0 {
		t.Errorf("expected outlineLvl 0, got %+v", p.Pr)
	}
	if len(p.Runs) != 1 || p.Runs[0].Text.Value != "Title Text" {
		t.Errorf("unexpected run/text: %+v", p.Runs)
	}

	if len(doc.Body.Tables) != 1 {
		t.Fatalf("expected 1 table in body, got %d", len(doc.Body.Tables))
	}
	tbl := doc.Body.Tables[0]
	if len(tbl.Rows) != 1 || len(tbl.Rows[0].Cells) != 1 {
		t.Fatalf("unexpected table structure: %+v", tbl)
	}
	cellP := tbl.Rows[0].Cells[0].Paragraphs
	if len(cellP) != 1 || len(cellP[0].Runs) != 1 || cellP[0].Runs[0].Text.Value != "Cell 1 " {
		t.Errorf("unexpected cell paragraph: %+v", cellP)
	}
	if cellP[0].Runs[0].Text.Space != "preserve" {
		t.Errorf("expected space preserve, got %s", cellP[0].Runs[0].Text.Space)
	}
}
