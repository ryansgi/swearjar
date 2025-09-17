package normalize

// ZoneType identifies simple layout/markup zones over normalized text
type ZoneType string

const (
	// ZoneCodeFence is fenced code block
	ZoneCodeFence ZoneType = "code_fence"
	// ZoneCodeInline is inline code
	ZoneCodeInline ZoneType = "code_inline"
	// ZoneQuote is a quoted line
	ZoneQuote ZoneType = "quote"
)

// ZoneSpan is a byte-range [Start,End) over the normalized string
type ZoneSpan struct {
	Type       ZoneType
	Start, End int
}

// DetectZones scans a normalized string and returns spans for:
// - fenced code between ``` ... ``` (excluding the backticks)
// - inline code between ` ... ` (excluding backticks; not inside fences)
// - quoted lines that start with '>' (after any leading spaces) up to newline
//
// Notes: we operate on the *normalized* text; newlines are preserved by collapseSpaces
func DetectZones(norm string) []ZoneSpan {
	if norm == "" {
		return nil
	}
	var out []ZoneSpan

	// 1) Fenced code: ``` ... ```
	for i := 0; i+2 < len(norm); {
		if norm[i] == '`' && norm[i+1] == '`' && norm[i+2] == '`' {
			// opening fence
			j := i + 3
			// Optional: skip language id until newline/space; we simply ignore it
			// Find closing fence
			close := indexTripleBacktick(norm, j)
			if close < 0 {
				// no closing fence; stop
				break
			}
			// content excludes the backticks themselves
			start := j
			end := close
			if start < end {
				out = append(out, ZoneSpan{Type: ZoneCodeFence, Start: start, End: end})
			}
			i = close + 3
			continue
		}
		i++
	}

	// Build a mask for fence regions to avoid double-tagging inline ticks inside fences
	inFence := func(pos int) bool {
		for _, z := range out {
			if z.Type == ZoneCodeFence && pos >= z.Start && pos < z.End {
				return true
			}
		}
		return false
	}

	// Inline code: ` ... `
	for i := 0; i < len(norm); i++ {
		if norm[i] != '`' || inFence(i) {
			continue
		}
		// opening tick
		j := i + 1
		for j < len(norm) && norm[j] != '`' {
			j++
		}
		if j < len(norm) && !inFence(j) {
			if i+1 < j {
				out = append(out, ZoneSpan{Type: ZoneCodeInline, Start: i + 1, End: j})
			}
			i = j // advance past closing
		}
	}

	// Quoted lines: start-of-line '>' up to newline
	// Walk line by line
	lineStart := 0
	for lineStart <= len(norm) {
		lineEnd := lineStart
		for lineEnd < len(norm) && norm[lineEnd] != '\n' {
			lineEnd++
		}
		// Trim leading spaces and test for '>'
		i := lineStart
		for i < lineEnd && (norm[i] == ' ' || norm[i] == '\t') {
			i++
		}
		if i < lineEnd && norm[i] == '>' {
			// Exclude the '>' itself; include the rest of line
			qs := i + 1
			for qs < lineEnd && norm[qs] == ' ' {
				qs++
			}
			if qs < lineEnd {
				out = append(out, ZoneSpan{Type: ZoneQuote, Start: qs, End: lineEnd})
			}
		}
		if lineEnd == len(norm) {
			break
		}
		lineStart = lineEnd + 1
	}

	return out
}

func indexTripleBacktick(s string, from int) int {
	for i := from; i+2 < len(s); i++ {
		if s[i] == '`' && s[i+1] == '`' && s[i+2] == '`' {
			return i
		}
	}
	return -1
}
