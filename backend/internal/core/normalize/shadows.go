package normalize

import "unicode"

// Shadows bundles alternate projections of a normalized string
// to support "gapped"/"repeat-collapsed" matching without touching the detector
type Shadows struct {
	Base         string // output of Normalizer.Normalize (what detector uses)
	NoPunct      string // letters/digits/underscore only (quick-and-dirty "gapped" projection)
	RepeatSquash string // Base with long character runs squashed (e.g., "fuuuuuck" -> "fuuck")
}

// BuildShadows constructs Shadows from a normalized string.
// It's cheap (single pass each) and safe to call per document
func BuildShadows(norm string) Shadows {
	return Shadows{
		Base:         norm,
		NoPunct:      stripNonWord(norm),
		RepeatSquash: squashRuns(norm, 2), // keep at most 2 repeats
	}
}

func stripNonWord(s string) string {
	if s == "" {
		return s
	}
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b = append(b, r)
		}
	}
	return string(b)
}

func squashRuns(s string, max int) string {
	if s == "" || max < 1 {
		return s
	}
	out := make([]rune, 0, len(s))
	var prev rune
	count := 0
	for _, r := range s {
		if r == prev {
			count++
			if count <= max {
				out = append(out, r)
			}
			continue
		}
		prev = r
		count = 1
		out = append(out, r)
	}
	return string(out)
}
