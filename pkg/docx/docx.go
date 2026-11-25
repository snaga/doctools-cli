package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrFileNotFound は指定された入力ファイルが存在しない場合のエラーです。
	// 呼び出し側がファイル不在を明確に判別できるように定義されています。
	ErrFileNotFound = errors.New("docx file not found")

	// ErrLegacyDoc はレガシーなバイナリ .doc 形式が指定された場合のエラーです。
	// doctools-cli は純 Go による OOXML (.docx) 処理に特化しているため、レガシー形式は明示的に拒否します。
	ErrLegacyDoc = errors.New("legacy .doc format is not supported; please convert to .docx")

	// ErrInvalidZip はファイルが破損しているか、有効な ZIP アーカイブでない場合のエラーです。
	ErrInvalidZip = errors.New("invalid or corrupted docx zip archive")

	// ErrZipSlip は ZIP アーカイブ内にパストラバーサル（ディレクトリ遡及）が検出された場合のセキュリティエラーです。
	ErrZipSlip = errors.New("potential zip slip vulnerability detected in archive")

	// ErrEncryptedDocx は Word ドキュメントがパスワード暗号化（EncryptedPackage 等）されている場合のエラーです。
	// 暗号化解除には外部鍵や追加ランタイムが必要となるため、純 Go 実装のセーフティガードとして明示的に検知・拒否します。
	ErrEncryptedDocx = errors.New("encrypted docx file is not supported")

	// ErrDocumentXMLNotFound は docx アーカイブ内に必須の word/document.xml が見つからない場合のエラーです。
	ErrDocumentXMLNotFound = errors.New("word/document.xml not found in docx archive")

	// ErrDecompressedSizeLimitExceeded は展開後の XML サイズが安全上限を超えた場合の Zip Bomb ガードエラーです。
	ErrDecompressedSizeLimitExceeded = errors.New("decompressed XML size limit exceeded")
)

// MaxDecompressedXMLSize は Zip Bomb 攻撃防止のための XML 展開最大許容サイズ（50MB）です。
// 極端に巨大な XML によるインメモリ OOM（メモリ枯渇）クラッシュを未然に防止します。
const MaxDecompressedXMLSize = 50 * 1024 * 1024

// maxDecompressedSize は実際の判定に利用される上限サイズです（テストでの安全な検証を可能にするため変数化）。
var maxDecompressedSize int64 = MaxDecompressedXMLSize

// SetMaxDecompressedSizeForTest はテスト目的で一時的に XML 展開サイズ上限を変更するためのヘルパー関数です。
// 巨大ファイルの生成なしに Zip Bomb ガードレールを確実に検証するために提供されます。
func SetMaxDecompressedSizeForTest(size int64) func() {
	orig := maxDecompressedSize
	maxDecompressedSize = size
	return func() {
		maxDecompressedSize = orig
	}
}

// oleHeader は OLE Compound Document（レガシー .doc や暗号化 Office 文書）のマジックバイトです。
var oleHeader = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// validateDocxFile は入力ファイルの存在、拡張子、およびマジックバイトを事前検査し、
// 不正ファイルやレガシー形式、暗号化ファイルを早期に安全拒否します。
func validateDocxFile(inputPath string) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, inputPath)
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrInvalidZip, inputPath)
	}

	// 拡張子が .doc の場合は明示的にレガシー拒否
	if strings.EqualFold(filepath.Ext(inputPath), ".doc") {
		return ErrLegacyDoc
	}

	// 先頭バイトを読み込み、マジックナンバーを検証
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file for validation: %w", err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 8)
	n, _ := io.ReadFull(f, header)
	if n >= 8 && bytes.Equal(header, oleHeader) {
		if strings.EqualFold(filepath.Ext(inputPath), ".doc") {
			return ErrLegacyDoc
		}
		return ErrEncryptedDocx
	}

	// ZIP マジックナンバー（"PK"）の存在チェック
	if n < 4 || (header[0] != 'P' || header[1] != 'K') {
		return ErrInvalidZip
	}

	return nil
}

// parseHeadingLevel は段落プロパティ（<w:pPr>）から見出しレベル（1〜6）を判定します。
// outlineLvl 属性（0-based）または pStyle 属性（"Heading 1", "見出し 1", "Title" 等）から
// 柔軟かつ決定論的に Markdown 見出しレベルを算出します。見出しでない場合は 0 を返します。
func parseHeadingLevel(p *XMLParagraph) int {
	if p.Pr == nil {
		return 0
	}

	// 1. OutlineLvl チェック (0-based: 0=Level 1, 1=Level 2, ..., 5=Level 6)
	if p.Pr.OutlineLvl != nil {
		lvl := p.Pr.OutlineLvl.Val + 1
		if lvl >= 1 && lvl <= 6 {
			return lvl
		}
	}

	// 2. PStyle チェック
	if p.Pr.PStyle != nil {
		val := strings.TrimSpace(p.Pr.PStyle.Val)
		valLower := strings.ToLower(val)

		for i := 1; i <= 6; i++ {
			patterns := []string{
				fmt.Sprintf("heading%d", i),
				fmt.Sprintf("heading %d", i),
				fmt.Sprintf("heading_%d", i),
				fmt.Sprintf("見出し%d", i),
				fmt.Sprintf("見出し %d", i),
				fmt.Sprintf("見出し_%d", i),
			}
			for _, pat := range patterns {
				if valLower == pat {
					return i
				}
			}
		}
		if valLower == "title" {
			return 1
		}
		if valLower == "subtitle" {
			return 2
		}
	}

	return 0
}

// extractParagraphText は XMLParagraph 内の全ラン（XMLRun）からテキストを連結して返します。
func extractParagraphText(p *XMLParagraph) string {
	var sb strings.Builder
	for _, r := range p.Runs {
		sb.WriteString(r.Text.Value)
	}
	return sb.String()
}

// convertTableToMarkdown は XMLTable を GitHub Flavored Markdown 形式の表文字列に変換します。
// 行および列を整列させ、セル内の改行やパイプ文字を安全にエスケープ処理します。
func convertTableToMarkdown(tbl *XMLTable) string {
	if len(tbl.Rows) == 0 {
		return ""
	}

	var rowData [][]string
	maxCols := 0

	for _, row := range tbl.Rows {
		var cols []string
		for _, cell := range row.Cells {
			var cellTexts []string
			for _, p := range cell.Paragraphs {
				txt := strings.TrimSpace(extractParagraphText(&p))
				if txt != "" {
					cellTexts = append(cellTexts, txt)
				}
			}
			cellStr := strings.Join(cellTexts, " ")
			cellStr = strings.ReplaceAll(cellStr, "|", `\|`)
			cols = append(cols, cellStr)
		}
		if len(cols) > maxCols {
			maxCols = len(cols)
		}
		rowData = append(rowData, cols)
	}

	if maxCols == 0 {
		return ""
	}

	var lines []string
	for i, cols := range rowData {
		for len(cols) < maxCols {
			cols = append(cols, "")
		}
		line := "| " + strings.Join(cols, " | ") + " |"
		lines = append(lines, line)

		if i == 0 {
			var sep []string
			for c := 0; c < maxCols; c++ {
				sep = append(sep, "---")
			}
			lines = append(lines, "| "+strings.Join(sep, " | ")+" |")
		}
	}

	return strings.Join(lines, "\n")
}

// ExtractSections は Word ドキュメント (.docx) から見出し、段落、表の各論理セクションを構造化抽出します。
// Zip Slip や Zip Bomb、暗号化パッケージに対するセキュリティガードを備え、
// 各セクションの階層レベル（HeadingLevel）や上位見出し名（HeadingTitle）を保持したリストを返します。
func ExtractSections(inputPath string) ([]DocxSection, error) {
	if err := validateDocxFile(inputPath); err != nil {
		return nil, err
	}

	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidZip, err)
	}
	defer func() { _ = r.Close() }()

	var docXMLFile *zip.File

	for _, f := range r.File {
		cleaned := filepath.Clean(f.Name)
		if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) || strings.Contains(f.Name, "../") || strings.Contains(f.Name, "..\\") {
			return nil, ErrZipSlip
		}
		if f.Name == "EncryptedPackage" || strings.HasSuffix(strings.ToLower(f.Name), "encryptedpackage") {
			return nil, ErrEncryptedDocx
		}
		if f.Name == "word/document.xml" {
			docXMLFile = f
		}
	}

	if docXMLFile == nil {
		return nil, ErrDocumentXMLNotFound
	}

	if docXMLFile.UncompressedSize64 > uint64(maxDecompressedSize) {
		return nil, ErrDecompressedSizeLimitExceeded
	}

	rc, err := docXMLFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open word/document.xml: %w", err)
	}
	defer func() { _ = rc.Close() }()

	limitedReader := io.LimitReader(rc, maxDecompressedSize+1)
	xmlData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read word/document.xml: %w", err)
	}
	if int64(len(xmlData)) > maxDecompressedSize {
		return nil, ErrDecompressedSizeLimitExceeded
	}

	return parseDocumentXML(xmlData)
}

// parseDocumentXML は word/document.xml のバイト列をトークンストリーム走査し、
// 出現順を維持したまま段落（<w:p>）と表（<w:tbl>）を DocxSection 一覧へ変換します。
func parseDocumentXML(xmlData []byte) ([]DocxSection, error) {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))

	var sections []DocxSection
	var currentHeadingLevel int
	var currentHeadingTitle string
	sectionIndex := 0

	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("xml decoding error: %w", err)
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			if elem.Name.Local == "p" {
				var p XMLParagraph
				if err := decoder.DecodeElement(&p, &elem); err != nil {
					return nil, fmt.Errorf("failed to decode paragraph: %w", err)
				}
				text := extractParagraphText(&p)
				trimmed := strings.TrimSpace(text)
				if trimmed == "" {
					continue
				}

				level := parseHeadingLevel(&p)
				if level > 0 {
					currentHeadingLevel = level
					currentHeadingTitle = trimmed
					sections = append(sections, DocxSection{
						Type:         SectionHeading,
						HeadingLevel: level,
						HeadingTitle: currentHeadingTitle,
						Content:      trimmed,
						Index:        sectionIndex,
					})
					sectionIndex++
				} else {
					sections = append(sections, DocxSection{
						Type:         SectionParagraph,
						HeadingLevel: currentHeadingLevel,
						HeadingTitle: currentHeadingTitle,
						Content:      trimmed,
						Index:        sectionIndex,
					})
					sectionIndex++
				}
			} else if elem.Name.Local == "tbl" {
				var tbl XMLTable
				if err := decoder.DecodeElement(&tbl, &elem); err != nil {
					return nil, fmt.Errorf("failed to decode table: %w", err)
				}
				tableMd := convertTableToMarkdown(&tbl)
				if strings.TrimSpace(tableMd) == "" {
					continue
				}

				sections = append(sections, DocxSection{
					Type:         SectionTable,
					HeadingLevel: currentHeadingLevel,
					HeadingTitle: currentHeadingTitle,
					Content:      tableMd,
					Index:        sectionIndex,
				})
				sectionIndex++
			}
		}
	}

	return sections, nil
}

// ExtractText は Word ドキュメント (.docx) からテキストを抽出し、見出し階層を反映した Markdown 形式の文字列を生成します。
// outputPath が空でない場合、生成された Markdown を指定パスに UTF-8 で保存します。
func ExtractText(inputPath string, outputPath string) (string, error) {
	sections, err := ExtractSections(inputPath)
	if err != nil {
		return "", err
	}

	var mdBlocks []string
	for _, sec := range sections {
		switch sec.Type {
		case SectionHeading:
			level := sec.HeadingLevel
			if level < 1 {
				level = 1
			}
			prefix := strings.Repeat("#", level)
			mdBlocks = append(mdBlocks, fmt.Sprintf("%s %s", prefix, sec.Content))
		case SectionParagraph, SectionTable:
			mdBlocks = append(mdBlocks, sec.Content)
		}
	}

	content := strings.Join(mdBlocks, "\n\n")

	if outputPath != "" {
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write output file: %w", err)
		}
	}

	return content, nil
}
