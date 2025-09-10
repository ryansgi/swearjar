package normalize

import (
	"strings"
	"unicode/utf8"
)

// Sanitize removes bytes/runes we don't want in DB or downstream:
// - NUL (0x00)
// - ASCII controls except '\n', '\r', '\t'
// - DEL (0x7F)
// - C1 controls U+0080..U+009F
// It also drops invalid UTF-8 bytes.
// Fast path returns s unchanged when no cleaning is needed.
// Honestly, there's probably a library that does this, but this is straightforward enough
// and we want to avoid dependencies in this package if possible
func Sanitize(s string) string {
	if s == "" {
		return s
	}

	n := len(s)
	i := 0

	// Fast path: scan until first "bad" byte/rune
	for i < n {
		b := s[i]
		if b < 0x20 { // ASCII control
			if b == '\n' || b == '\r' || b == '\t' {
				i++
				continue
			}
			break
		}
		if b == 0x7F { // DEL
			break
		}
		if b < 0x80 { // ASCII
			i++
			continue
		}
		// Multibyte: decode once
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			break // invalid byte
		}
		if r >= 0x80 && r <= 0x9F { // C1 controls
			break
		}
		i += size
	}
	if i == n {
		return s // nothing to clean
	}

	// Slow path: build cleaned string from here on
	var bldr strings.Builder
	bldr.Grow(n)
	bldr.WriteString(s[:i]) // keep clean prefix

	for i < n {
		c := s[i]

		// ASCII controls (except allowed)
		if c < 0x20 {
			if c == '\n' || c == '\r' || c == '\t' {
				bldr.WriteByte(c)
			}
			i++
			continue
		}
		// DEL
		if c == 0x7F {
			i++
			continue
		}
		// Plain ASCII
		if c < 0x80 {
			bldr.WriteByte(c)
			i++
			continue
		}

		// Multibyte
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// invalid byte -> drop
			i++
			continue
		}
		if r >= 0x80 && r <= 0x9F {
			// C1 control -> drop
			i += size
			continue
		}

		// Good rune: write exact bytes slice (no re-encode)
		bldr.WriteString(s[i : i+size])
		i += size
	}

	return bldr.String()
}
