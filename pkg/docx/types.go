package docx

import "encoding/xml"

// DocxSectionType は Word ドキュメントから抽出された各セクションの論理種別を表す型です。
// 見出し階層の判定や全文検索（FTS）でのインデックス区分に利用されます。
type DocxSectionType string

const (
	// SectionHeading は見出し（Heading 1〜6 など）を表すセクション種別です。
	SectionHeading DocxSectionType = "heading"

	// SectionParagraph は通常の段落テキストを表すセクション種別です。
	SectionParagraph DocxSectionType = "paragraph"

	// SectionTable は表（Markdown テーブル化されたデータ）を表すセクション種別です。
	SectionTable DocxSectionType = "table"
)

// DocxSection は Word ドキュメントから抽出された論理セクション（見出し階層、段落、表など）を保持する構造体です。
// FTS インデックス登録やピンポイント検索における Location 特定、および構造化 Markdown 出力に用いられます。
type DocxSection struct {
	// Type はセクション種別（heading, paragraph, table）です。
	Type DocxSectionType `json:"type"`

	// HeadingLevel は見出しの階層レベル（1〜6）です。見出し以外のセクションでは属する親見出しのレベルまたは 0 が設定されます。
	HeadingLevel int `json:"heading_level,omitempty"`

	// HeadingTitle は当セクションが属している上位見出しのテキストです。
	HeadingTitle string `json:"heading_title,omitempty"`

	// Content はセクションのテキスト本文です。表の場合は Markdown テーブル形式の文字列が格納されます。
	Content string `json:"content"`

	// Index はドキュメント全体でのセクションの出現順序番号（0-based）です。
	Index int `json:"index"`
}

// XMLDocument は OOXML word/document.xml のルート要素 <w:document> をパースするための構造体です。
type XMLDocument struct {
	XMLName xml.Name `xml:"document"`
	// Body は Word ドキュメントのメインコンテンツを格納する要素です。
	Body XMLBody `xml:"body"`
}

// XMLBody は <w:body> 要素を表し、ドキュメント内の段落や表などの要素を内包する構造体です。
type XMLBody struct {
	XMLName xml.Name `xml:"body"`
	// Paragraphs はドキュメント直下の段落要素群です。
	Paragraphs []XMLParagraph `xml:"p"`
	// Tables はドキュメント直下の表要素群です。
	Tables []XMLTable `xml:"tbl"`
}

// XMLParagraph は <w:p> 要素を表し、1つの段落を表現する構造体です。
type XMLParagraph struct {
	XMLName xml.Name `xml:"p"`
	// Pr は段落プロパティ（スタイルやアウトラインレベルなど）です。
	Pr *XMLParagraphPr `xml:"pPr"`
	// Runs は段落内に含まれるテキストラン（書式単位の文字列要素）の配列です。
	Runs []XMLRun `xml:"r"`
}

// XMLParagraphPr は <w:pPr> 要素を表し、段落の書式・スタイル指定情報を保持する構造体です。
type XMLParagraphPr struct {
	XMLName xml.Name `xml:"pPr"`
	// PStyle は段落に適用されているスタイル指定要素（Heading1 等）です。
	PStyle *XMLPStyle `xml:"pStyle"`
	// OutlineLvl は段落のアウトラインレベル指定要素です。
	OutlineLvl *XMLOutlineLvl `xml:"outlineLvl"`
}

// XMLPStyle は <w:pStyle> 要素を表し、段落に割り当てられたスタイル名（w:val 属性）を保持する構造体です。
type XMLPStyle struct {
	XMLName xml.Name `xml:"pStyle"`
	// Val はスタイル識別文字列（例: "Heading1", "Heading2", "Title" 等）です。
	Val string `xml:"val,attr"`
}

// XMLOutlineLvl は <w:outlineLvl> 要素を表し、見出し階層のアウトラインレベル（0=レベル1, 1=レベル2...）を保持する構造体です。
type XMLOutlineLvl struct {
	XMLName xml.Name `xml:"outlineLvl"`
	// Val は 0-based のアウトラインレベル数値です。
	Val int `xml:"val,attr"`
}

// XMLRun は <w:r> 要素を表し、共通の文字書式を持つテキストの塊（Run）を表現する構造体です。
type XMLRun struct {
	XMLName xml.Name `xml:"r"`
	// Text はランに含まれるテキスト要素です。
	Text XMLText `xml:"t"`
}

// XMLText は <w:t> 要素を表し、実際の文字列データを保持する構造体です。
type XMLText struct {
	XMLName xml.Name `xml:"t"`
	// Space は空白文字の保持設定（xml:space="preserve"）です。
	Space string `xml:"space,attr"`
	// Value は格納されている文字列本文です。
	Value string `xml:",chardata"`
}

// XMLTable は <w:tbl> 要素を表し、Word 内の表を表現する構造体です。
type XMLTable struct {
	XMLName xml.Name `xml:"tbl"`
	// Rows は表を構成する行要素の配列です。
	Rows []XMLTableRow `xml:"tr"`
}

// XMLTableRow は <w:tr> 要素を表し、表内の各行を表現する構造体です。
type XMLTableRow struct {
	XMLName xml.Name `xml:"tr"`
	// Cells は行を構成する各セル要素の配列です。
	Cells []XMLTableCell `xml:"tc"`
}

// XMLTableCell は <w:tc> 要素を表し、表内の個々のセルを表現する構造体です。
type XMLTableCell struct {
	XMLName xml.Name `xml:"tc"`
	// Paragraphs はセル内に含まれる段落要素群です。
	Paragraphs []XMLParagraph `xml:"p"`
}
