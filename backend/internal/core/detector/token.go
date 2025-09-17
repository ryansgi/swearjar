package detector

import (
	"unicode"
	"unicode/utf8"
)

// isWord reports whether r is considered a word character for boundary checks.
// Keep conservative, but be a bit more Unicode-friendly: letters, numbers,
// combining marks (Mn), and connector punctuation (Pc, e.g. underscore).
// Hyphen and most punctuation remain non-word
func isWord(r rune) bool {
	if r == utf8.RuneError || r == 0 {
		return false
	}
	return unicode.IsLetter(r) ||
		unicode.IsNumber(r) ||
		unicode.In(r, unicode.Mn, unicode.Pc)
}

// expandToToken widens [start,end) to the containing token delimited by non-word chars
func expandToToken(s string, start, end int) (int, int) {
	ls, rs := start, end
	for ls > 0 {
		r, sz := utf8.DecodeLastRuneInString(s[:ls])
		if !isWord(r) {
			break
		}
		ls -= sz
	}
	for rs < len(s) {
		r, sz := utf8.DecodeRuneInString(s[rs:])
		if !isWord(r) {
			break
		}
		rs += sz
	}
	return ls, rs
}
