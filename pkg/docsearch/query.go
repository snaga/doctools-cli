package docsearch

import (
	"strings"
	"unicode"
)

// IsJapaneseRune reports whether r is a Japanese character:
// Hiragana (\u3040-\u309F), Katakana (\u30A0-\u30FF), Han (Kanji, \u4E00-\u9FFF, \u3400-\u4DBF, etc.),
// CJK Symbols and Punctuation (\u3001-\u303F), or Halfwidth Katakana (\uFF66-\uFF9F).
func IsJapaneseRune(r rune) bool {
	// Hiragana (\u3040-\u309F)
	if (r >= 0x3040 && r <= 0x309f) || unicode.Is(unicode.Hiragana, r) {
		return true
	}
	// Katakana (\u30A0-\u30FF) including prolonged sound mark 'ー' (\u30FC)
	if (r >= 0x30a0 && r <= 0x30ff) || unicode.Is(unicode.Katakana, r) {
		return true
	}
	// Han (Kanji, \u4E00-\u9FFF, \u3400-\u4DBF, etc.)
	if unicode.Is(unicode.Han, r) {
		return true
	}
	// CJK Symbols and Punctuation (excluding U+3000 ideographic space)
	if r >= 0x3001 && r <= 0x303f {
		return true
	}
	// Halfwidth Katakana (U+FF66-U+FF9F)
	if r >= 0xff66 && r <= 0xff9f {
		return true
	}
	return false
}

// ContainsJapanese reports whether s contains at least one Japanese character.
func ContainsJapanese(s string) bool {
	for _, r := range s {
		if IsJapaneseRune(r) {
			return true
		}
	}
	return false
}

// TransformJapaneseQuery automatically quotes Japanese terms as phrase queries.
// It parses tokens separated by spaces (including full-width spaces) while preserving
// quoted phrases, and wraps Japanese tokens in double quotes if not already quoted.
// Lucene prefixes '+' and '-' are preserved (e.g. +テスト -> +"テスト").
func TransformJapaneseQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}

	tokens := splitQueryTokens(trimmed)
	if len(tokens) == 0 {
		return ""
	}

	// If the entire trimmed query is already a single quoted phrase, return as-is
	if len(tokens) == 1 && isQuoted(tokens[0]) {
		return tokens[0]
	}

	transformed := make([]string, len(tokens))
	for i, token := range tokens {
		transformed[i] = transformToken(token)
	}

	return strings.Join(transformed, " ")
}

// splitQueryTokens splits query by half-width or full-width whitespace,
// keeping phrases enclosed in double quotes together.
func splitQueryTokens(query string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for _, r := range query {
		switch {
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case (r == ' ' || r == '\u3000' || unicode.IsSpace(r)) && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// isQuoted reports whether s starts and ends with double quotes.
func isQuoted(s string) bool {
	return len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")
}

// transformToken transforms a single search token according to Lucene prefix and Japanese content.
func transformToken(token string) string {
	if isQuoted(token) {
		return token
	}

	prefix := ""
	body := token
	if strings.HasPrefix(token, "+") || strings.HasPrefix(token, "-") {
		prefix = token[:1]
		body = token[1:]
	}

	if isQuoted(body) {
		return token
	}

	if ContainsJapanese(body) {
		return prefix + `"` + body + `"`
	}

	return token
}
