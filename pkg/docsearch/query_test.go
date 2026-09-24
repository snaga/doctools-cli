package docsearch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

func TestIsJapaneseRune(t *testing.T) {
	tests := []struct {
		name     string
		r        rune
		expected bool
	}{
		{"Hiragana", 'あ', true},
		{"Hiragana end", 'ん', true},
		{"Katakana", 'ア', true},
		{"Katakana prolonged", 'ー', true},
		{"Kanji", '漢', true},
		{"Kanji extension", '𠮷', true},
		{"CJK Japanese punctuation full stop", '。', true},
		{"CJK Japanese punctuation comma", '、', true},
		{"CJK bracket opening", '「', true},
		{"CJK bracket closing", '」', true},
		{"Halfwidth Katakana", 'ｱ', true},
		{"ASCII letter", 'a', false},
		{"ASCII digit", '1', false},
		{"ASCII space", ' ', false},
		{"Fullwidth space", '\u3000', false},
		{"ASCII symbol", '+', false},
		{"Cyrillic letter", 'Д', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJapaneseRune(tt.r)
			if got != tt.expected {
				t.Errorf("IsJapaneseRune(%q / %U) = %v; want %v", tt.r, tt.r, got, tt.expected)
			}
		})
	}
}

func TestContainsJapanese(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Pure hiragana", "ひらがな", true},
		{"Pure katakana", "カタカナ", true},
		{"Pure kanji", "漢字", true},
		{"Mixed Japanese and English", "Go言語", true},
		{"Only English", "Hello World", false},
		{"Numbers and symbols", "123 + 456 - 789", false},
		{"Empty string", "", false},
		{"Only spaces", "   \t \n", false},
		{"Fullwidth space only", "　", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsJapanese(tt.input)
			if got != tt.expected {
				t.Errorf("ContainsJapanese(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTransformJapaneseQuery_Unit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single Japanese term",
			input:    "テスト戦略",
			expected: `"テスト戦略"`,
		},
		{
			name:     "Mixed English-Japanese multiple terms",
			input:    "Go言語 テスト",
			expected: `"Go言語" "テスト"`,
		},
		{
			name:     "Already quoted term",
			input:    `"テスト戦略"`,
			expected: `"テスト戦略"`,
		},
		{
			name:     "English only terms",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "Lucene prefix + and -",
			input:    "+テスト -戦略",
			expected: `+"テスト" -"戦略"`,
		},
		{
			name:     "Fullwidth space separated",
			input:    "テスト　戦略",
			expected: `"テスト" "戦略"`,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Halfwidth whitespace only",
			input:    "   \t  ",
			expected: "",
		},
		{
			name:     "Fullwidth whitespace only",
			input:    "　　",
			expected: "",
		},
		{
			name:     "Already quoted phrase with whitespace inside",
			input:    `"テスト 戦略"`,
			expected: `"テスト 戦略"`,
		},
		{
			name:     "Mixed quoted and unquoted tokens",
			input:    `"テスト" 戦略 go`,
			expected: `"テスト" "戦略" go`,
		},
		{
			name:     "Prefix with English token",
			input:    "+go -docker",
			expected: "+go -docker",
		},
		{
			name:     "Prefix with already quoted Japanese",
			input:    `+"テスト" -"戦略"`,
			expected: `+"テスト" -"戦略"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TransformJapaneseQuery(tt.input)
			if got != tt.expected {
				t.Errorf("TransformJapaneseQuery(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTransformJapaneseQuery_Integration_Bleve(t *testing.T) {
	// 統合テスト:
	// Bleve インデックスに「テスト」「戦略」「テスト戦略」の 3 つのドキュメントを投入し、
	// TransformJapaneseQuery("テスト戦略") による検索で「テスト戦略」を含むドキュメントのみが
	// 厳密にヒットし、「テスト」単体や「戦略」単体のドキュメントが誤ヒットしないことを検証。
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test_phrase.bleve")

	mapping := bleve.NewIndexMapping()
	idx, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("failed to create bleve index: %v", err)
	}
	var closed bool
	defer func() {
		if !closed {
			_ = idx.Close()
		}
	}()

	// 3つのドキュメントを投入
	docs := []struct {
		id      string
		content string
	}{
		{id: "doc-1", content: "テスト"},
		{id: "doc-2", content: "戦略"},
		{id: "doc-3", content: "テスト戦略"},
	}

	for _, d := range docs {
		err := idx.Index(d.id, map[string]interface{}{
			"file_name":     d.id + ".txt",
			"file_path":     filepath.Join(tmpDir, d.id+".txt"),
			"file_type":     "txt",
			"page_or_index": 1,
			"content":       d.content,
			"updated_at":    time.Now().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("failed to index doc %s: %v", d.id, err)
		}
	}

	// 1. TransformJapaneseQuery で変換したクエリ文字列による直接 Bleve 検索の検証
	transformedQuery := TransformJapaneseQuery("テスト戦略")
	if transformedQuery != `"テスト戦略"` {
		t.Fatalf("unexpected transformed query: got %s, want %s", transformedQuery, `"テスト戦略"`)
	}

	searchReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(transformedQuery), 10, 0, false)
	searchRes, err := idx.Search(searchReq)
	if err != nil {
		t.Fatalf("bleve search failed: %v", err)
	}

	if searchRes.Total != 1 {
		t.Fatalf("expected exactly 1 hit for phrase query %s, got %d hits", transformedQuery, searchRes.Total)
	}
	if searchRes.Hits[0].ID != "doc-3" {
		t.Fatalf("expected doc-3 to match, got %s", searchRes.Hits[0].ID)
	}

	// Close idx so Server can open it without bolt file lock conflicts
	closed = true
	if err := idx.Close(); err != nil {
		t.Fatalf("failed to close test bleve index: %v", err)
	}

	// 2. Server.MultiIndexSearch を介した検証
	cfg := &Config{
		Server: ServerConfig{Port: 18081, Host: "127.0.0.1"},
		Indexes: []IndexConfig{
			{ID: "test_idx", Name: "テストインデックス", Path: indexPath, DefaultSelected: true},
		},
	}
	srv, err := NewServer(cfg, "")
	if err != nil {
		t.Fatalf("failed to create Server: %v", err)
	}
	defer srv.Close()

	resp, err := srv.MultiIndexSearch("テスト戦略", nil, 10)
	if err != nil {
		t.Fatalf("MultiIndexSearch failed: %v", err)
	}

	// レスポンスの Query は生クエリが保持されていること
	if resp.Query != "テスト戦略" {
		t.Errorf("expected Query in response to be 'テスト戦略', got %s", resp.Query)
	}

	// ヒット件数は doc-3 の1件のみであること
	if resp.TotalHits != 1 {
		t.Fatalf("expected TotalHits = 1, got %d", resp.TotalHits)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(resp.Results))
	}
	if resp.Results[0].FileName != "doc-3.txt" {
		t.Errorf("expected matched file to be doc-3.txt, got %s", resp.Results[0].FileName)
	}
}
