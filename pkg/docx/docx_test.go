package docx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/docx"
)

// createSampleDocx はテスト用の .docx ファイル（ZIP アーカイブ）を動的に生成するテストヘルパーです。
func createSampleDocx(t *testing.T, path string, xmlContent string) {
	t.Helper()
	createSampleDocxWithFiles(t, path, xmlContent, nil)
}

// createSampleDocxWithFiles は追加ファイルを含むテスト用 .docx ファイルを生成するテストヘルパーです。
func createSampleDocxWithFiles(t *testing.T, path string, xmlContent string, extraFiles map[string][]byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for sample docx: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create sample docx file: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)

	if xmlContent != "" {
		w, err := zw.Create("word/document.xml")
		if err != nil {
			t.Fatalf("failed to create word/document.xml in zip: %v", err)
		}
		if _, err := w.Write([]byte(xmlContent)); err != nil {
			t.Fatalf("failed to write document.xml: %v", err)
		}
	}

	for name, content := range extraFiles {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create extra file %s in zip: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("failed to write extra file %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

// TestExtractHeadingsAndParagraphs は見出し（Heading 1, Heading 2）、通常段落、および表を含む
// Word ドキュメントから期待通りの構造化セクションおよび Markdown が抽出・保存されるかを包括的に検証します。
func TestExtractHeadingsAndParagraphs(t *testing.T) {
	sampleXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading1"/>
        <w:outlineLvl w:val="0"/>
      </w:pPr>
      <w:r><w:t>見出し1</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>本文段落</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading2"/>
        <w:outlineLvl w:val="1"/>
      </w:pPr>
      <w:r><w:t>見出し2</w:t></w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>ヘッダーA</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>ヘッダーB</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>データ1</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>データ2</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`

	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "test.docx")
	outPath := filepath.Join(tmpDir, "output.md")

	createSampleDocx(t, docxPath, sampleXML)

	// 1. ExtractSections の検証
	sections, err := docx.ExtractSections(docxPath)
	if err != nil {
		t.Fatalf("ExtractSections failed: %v", err)
	}

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	// Section 0: 見出し1
	if sections[0].Type != docx.SectionHeading || sections[0].HeadingLevel != 1 || sections[0].Content != "見出し1" {
		t.Errorf("unexpected section 0: %+v", sections[0])
	}
	if sections[0].Index != 0 {
		t.Errorf("expected index 0, got %d", sections[0].Index)
	}

	// Section 1: 本文段落
	if sections[1].Type != docx.SectionParagraph || sections[1].HeadingLevel != 1 || sections[1].HeadingTitle != "見出し1" || sections[1].Content != "本文段落" {
		t.Errorf("unexpected section 1: %+v", sections[1])
	}
	if sections[1].Index != 1 {
		t.Errorf("expected index 1, got %d", sections[1].Index)
	}

	// Section 2: 見出し2
	if sections[2].Type != docx.SectionHeading || sections[2].HeadingLevel != 2 || sections[2].Content != "見出し2" {
		t.Errorf("unexpected section 2: %+v", sections[2])
	}
	if sections[2].Index != 2 {
		t.Errorf("expected index 2, got %d", sections[2].Index)
	}

	// Section 3: 表
	if sections[3].Type != docx.SectionTable || sections[3].HeadingTitle != "見出し2" {
		t.Errorf("unexpected section 3: %+v", sections[3])
	}
	expectedTable := "| ヘッダーA | ヘッダーB |\n| --- | --- |\n| データ1 | データ2 |"
	if sections[3].Content != expectedTable {
		t.Errorf("expected table:\n%s\ngot:\n%s", expectedTable, sections[3].Content)
	}

	// 2. ExtractText の検証
	extracted, err := docx.ExtractText(docxPath, outPath)
	if err != nil {
		t.Fatalf("ExtractText failed: %v", err)
	}

	expectedPrefix := "# 見出し1\n\n本文段落\n\n## 見出し2"
	if !strings.HasPrefix(extracted, expectedPrefix) {
		t.Errorf("expected prefix:\n%s\ngot:\n%s", expectedPrefix, extracted)
	}
	if !strings.Contains(extracted, expectedTable) {
		t.Errorf("expected output to contain table:\n%s\ngot:\n%s", expectedTable, extracted)
	}

	// 3. 出力ファイルの検証
	savedBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(savedBytes) != extracted {
		t.Errorf("saved file content does not match returned text")
	}
}

// TestExtractHeadings_StyleVariants は Title や見出し階層（heading 3, 見出し 4 等）など
// 多様な段落スタイル指定に対して正確な見出しレベルが判定されることを検証します。
func TestExtractHeadings_StyleVariants(t *testing.T) {
	xmlSnippet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Title"/></w:pPr>
      <w:r><w:t>Doc Title</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:pStyle w:val="heading 3"/></w:pPr>
      <w:r><w:t>Sub Sub Heading</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:pStyle w:val="見出し 4"/></w:pPr>
      <w:r><w:t>Japanese Heading</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "variants.docx")
	createSampleDocx(t, docxPath, xmlSnippet)

	sections, err := docx.ExtractSections(docxPath)
	if err != nil {
		t.Fatalf("ExtractSections failed: %v", err)
	}
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].HeadingLevel != 1 || sections[0].Content != "Doc Title" {
		t.Errorf("unexpected section 0: %+v", sections[0])
	}
	if sections[1].HeadingLevel != 3 || sections[1].Content != "Sub Sub Heading" {
		t.Errorf("unexpected section 1: %+v", sections[1])
	}
	if sections[2].HeadingLevel != 4 || sections[2].Content != "Japanese Heading" {
		t.Errorf("unexpected section 2: %+v", sections[2])
	}
}

// TestExtractErrors はファイル不在、レガシー .doc 形式、暗号化ファイル、非 ZIP 形式、
// Zip Slip、Zip Bomb（展開サイズ上限超過）、破損 XML に対する防御的エラーハンドリングを検証します。
func TestExtractErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 不在ファイル
	t.Run("FileNotFound", func(t *testing.T) {
		_, err := docx.ExtractSections(filepath.Join(tmpDir, "non_existent.docx"))
		if !errors.Is(err, docx.ErrFileNotFound) {
			t.Errorf("expected ErrFileNotFound, got %v", err)
		}
	})

	// 2. レガシー .doc 拡張子
	t.Run("LegacyDocExtension", func(t *testing.T) {
		legacyPath := filepath.Join(tmpDir, "old.doc")
		if err := os.WriteFile(legacyPath, []byte("fake doc"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		_, err := docx.ExtractSections(legacyPath)
		if !errors.Is(err, docx.ErrLegacyDoc) {
			t.Errorf("expected ErrLegacyDoc, got %v", err)
		}
	})

	// 3. レガシー / 暗号化 OLE マジックバイト
	t.Run("OLECompoundHeader", func(t *testing.T) {
		oleHeader := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00, 0x00}
		oleDocx := filepath.Join(tmpDir, "encrypted.docx")
		if err := os.WriteFile(oleDocx, oleHeader, 0644); err != nil {
			t.Fatalf("failed to write ole header: %v", err)
		}
		_, err := docx.ExtractSections(oleDocx)
		if !errors.Is(err, docx.ErrEncryptedDocx) {
			t.Errorf("expected ErrEncryptedDocx, got %v", err)
		}
	})

	// 4. 不正な非 ZIP ファイル
	t.Run("InvalidZip", func(t *testing.T) {
		corruptPath := filepath.Join(tmpDir, "corrupted.docx")
		if err := os.WriteFile(corruptPath, []byte("NOT_A_ZIP_FILE_AT_ALL"), 0644); err != nil {
			t.Fatalf("failed to write corrupted file: %v", err)
		}
		_, err := docx.ExtractSections(corruptPath)
		if !errors.Is(err, docx.ErrInvalidZip) {
			t.Errorf("expected ErrInvalidZip, got %v", err)
		}
	})

	// 5. EncryptedPackage エントリを含む ZIP
	t.Run("EncryptedPackageEntry", func(t *testing.T) {
		encPath := filepath.Join(tmpDir, "enc_entry.docx")
		createSampleDocxWithFiles(t, encPath, "<dummy/>", map[string][]byte{
			"EncryptedPackage": []byte("encrypted stream"),
		})
		_, err := docx.ExtractSections(encPath)
		if !errors.Is(err, docx.ErrEncryptedDocx) {
			t.Errorf("expected ErrEncryptedDocx, got %v", err)
		}
	})

	// 6. Zip Slip ガード
	t.Run("ZipSlip", func(t *testing.T) {
		slipPath := filepath.Join(tmpDir, "zipslip.docx")
		createSampleDocxWithFiles(t, slipPath, "<dummy/>", map[string][]byte{
			"../evil.xml": []byte("evil"),
		})
		_, err := docx.ExtractSections(slipPath)
		if !errors.Is(err, docx.ErrZipSlip) {
			t.Errorf("expected ErrZipSlip, got %v", err)
		}
	})

	// 7. word/document.xml が存在しない ZIP
	t.Run("MissingDocumentXML", func(t *testing.T) {
		missingPath := filepath.Join(tmpDir, "missing_doc.docx")
		createSampleDocxWithFiles(t, missingPath, "", map[string][]byte{
			"other.xml": []byte("<other/>"),
		})
		_, err := docx.ExtractSections(missingPath)
		if !errors.Is(err, docx.ErrDocumentXMLNotFound) {
			t.Errorf("expected ErrDocumentXMLNotFound, got %v", err)
		}
	})

	// 8. 破損した XML
	t.Run("MalformedXML", func(t *testing.T) {
		malformedPath := filepath.Join(tmpDir, "malformed.docx")
		createSampleDocx(t, malformedPath, "<w:document><w:body><w:p><unclosed_tag>")
		_, err := docx.ExtractSections(malformedPath)
		if err == nil {
			t.Errorf("expected error for malformed XML, got nil")
		}
	})

	// 9. Zip Bomb ガード（展開サイズ上限超過）
	t.Run("ZipBombLimitExceeded", func(t *testing.T) {
		// 一時的に上限サイズを 20 バイトに縮小して検証
		restore := docx.SetMaxDecompressedSizeForTest(20)
		defer restore()

		largeDocx := filepath.Join(tmpDir, "bomb.docx")
		createSampleDocx(t, largeDocx, "<w:document><w:body><w:p><w:r><w:t>This is definitely longer than 20 bytes</w:t></w:r></w:p></w:body></w:document>")

		_, err := docx.ExtractSections(largeDocx)
		if !errors.Is(err, docx.ErrDecompressedSizeLimitExceeded) {
			t.Errorf("expected ErrDecompressedSizeLimitExceeded, got %v", err)
		}
	})

	// 10. 入力パスがディレクトリの場合
	t.Run("DirectoryAsInput", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "some_dir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
		_, err := docx.ExtractSections(subDir)
		if !errors.Is(err, docx.ErrInvalidZip) {
			t.Errorf("expected ErrInvalidZip for directory input, got %v", err)
		}
	})

	// 11. OLE ヘッダ付きの .doc ファイル
	t.Run("OLEWithDocExtension", func(t *testing.T) {
		oleDocPath := filepath.Join(tmpDir, "legacy_ole.doc")
		oleHeader := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00, 0x00}
		if err := os.WriteFile(oleDocPath, oleHeader, 0644); err != nil {
			t.Fatalf("failed to write test OLE file: %v", err)
		}
		_, err := docx.ExtractSections(oleDocPath)
		if !errors.Is(err, docx.ErrLegacyDoc) {
			t.Errorf("expected ErrLegacyDoc for .doc with OLE header, got %v", err)
		}
	})

	// 12. ExtractText のエラーハンドリング伝播
	t.Run("ExtractText_FileNotFound", func(t *testing.T) {
		_, err := docx.ExtractText(filepath.Join(tmpDir, "non_existent.docx"), "")
		if !errors.Is(err, docx.ErrFileNotFound) {
			t.Errorf("expected ErrFileNotFound from ExtractText, got %v", err)
		}
	})
}

// TestExtractText_OutputFile は ExtractText に outputPath が渡された場合に
// 指定ディレクトリ（未作成ディレクトリ含む）へ Markdown が正常保存されることを検証します。
func TestExtractText_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "test.docx")
	outPath := filepath.Join(tmpDir, "nested", "output", "result.md")

	xml := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:body>
			<w:p>
				<w:pPr><w:pStyle w:val="Heading 1"/></w:pPr>
				<w:r><w:t>Heading from ExtractText</w:t></w:r>
			</w:p>
			<w:p>
				<w:r><w:t>Paragraph line 1</w:t></w:r>
			</w:p>
		</w:body>
	</w:document>`

	createSampleDocx(t, docxPath, xml)

	md, err := docx.ExtractText(docxPath, outPath)
	if err != nil {
		t.Fatalf("unexpected ExtractText error: %v", err)
	}

	expectedPrefix := "# Heading from ExtractText"
	if !strings.HasPrefix(md, expectedPrefix) {
		t.Errorf("expected md to start with %q, got: %s", expectedPrefix, md)
	}

	saved, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read written output file: %v", err)
	}

	if string(saved) != md {
		t.Errorf("file content mismatch: expected %q, got %q", md, string(saved))
	}
}

// TestExtractSections_OutlineLvlAndSubtitle は outlineLvl 属性（0-based）や
// subtitle スタイル、空段落のスキップ処理を検証します。
func TestExtractSections_OutlineLvlAndSubtitle(t *testing.T) {
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "outline_test.docx")

	xml := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:body>
			<w:p>
				<w:pPr><w:outlineLvl w:val="0"/></w:pPr>
				<w:r><w:t>Outline Level 0 -> Heading 1</w:t></w:r>
			</w:p>
			<w:p>
				<w:pPr><w:outlineLvl w:val="2"/></w:pPr>
				<w:r><w:t>Outline Level 2 -> Heading 3</w:t></w:r>
			</w:p>
			<w:p>
				<w:pPr><w:pStyle w:val="subtitle"/></w:pPr>
				<w:r><w:t>Subtitle -> Heading 2</w:t></w:r>
			</w:p>
			<w:p>
				<w:pPr><w:pStyle w:val="見出し_1"/></w:pPr>
				<w:r><w:t>見出し_1 -> Heading 1</w:t></w:r>
			</w:p>
			<w:p>
				<w:r><w:t>   </w:t></w:r>
			</w:p>
			<w:p/>
		</w:body>
	</w:document>`

	createSampleDocx(t, docxPath, xml)

	sections, err := docx.ExtractSections(docxPath)
	if err != nil {
		t.Fatalf("ExtractSections failed: %v", err)
	}

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections (whitespace/empty paragraphs skipped), got %d", len(sections))
	}

	if sections[0].HeadingLevel != 1 || sections[0].Content != "Outline Level 0 -> Heading 1" {
		t.Errorf("section 0 unexpected: %+v", sections[0])
	}
	if sections[1].HeadingLevel != 3 || sections[1].Content != "Outline Level 2 -> Heading 3" {
		t.Errorf("section 1 unexpected: %+v", sections[1])
	}
	if sections[2].HeadingLevel != 2 || sections[2].Content != "Subtitle -> Heading 2" {
		t.Errorf("section 2 unexpected: %+v", sections[2])
	}
	if sections[3].HeadingLevel != 1 || sections[3].Content != "見出し_1 -> Heading 1" {
		t.Errorf("section 3 unexpected: %+v", sections[3])
	}
}

// TestExtractSections_TableEdgeCases は空テーブル、空行テーブル、
// 不揃い列数（Ragged）のテーブルおよびパイプ文字のエスケープを検証します。
func TestExtractSections_TableEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "table_edge.docx")

	xml := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:body>
			<w:tbl></w:tbl>
			<w:tbl><w:tr></w:tr></w:tbl>
			<w:tbl>
				<w:tr>
					<w:tc><w:p><w:r><w:t>Header A | pipe</w:t></w:r></w:p></w:tc>
					<w:tc><w:p><w:r><w:t>Header B</w:t></w:r></w:p></w:tc>
				</w:tr>
				<w:tr>
					<w:tc><w:p><w:r><w:t>Only one cell</w:t></w:r></w:p></w:tc>
				</w:tr>
			</w:tbl>
		</w:body>
	</w:document>`

	createSampleDocx(t, docxPath, xml)

	sections, err := docx.ExtractSections(docxPath)
	if err != nil {
		t.Fatalf("ExtractSections failed: %v", err)
	}

	// 空テーブルおよび行のみの空テーブルはスキップされるため、有効なテーブルセクションは1つのみ
	if len(sections) != 1 {
		t.Fatalf("expected 1 valid table section, got %d", len(sections))
	}

	sec := sections[0]
	if sec.Type != docx.SectionTable {
		t.Errorf("expected SectionTable, got %s", sec.Type)
	}

	// パイプがエスケープされていること
	if !strings.Contains(sec.Content, `Header A \| pipe`) {
		t.Errorf("expected pipe to be escaped, got content:\n%s", sec.Content)
	}
	// 不揃い行が空列でパディングされていること (| Only one cell |  |)
	if !strings.Contains(sec.Content, "| Only one cell |  |") {
		t.Errorf("expected padded row, got content:\n%s", sec.Content)
	}
}
