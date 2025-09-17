// Package normalize provides a deterministic text normalizer used by the detector
// Pipeline order
// 1 UTF-8 repair drop invalid bytes
// 2 Unicode NFKC normalization
// 3 Case folding
// 4 Remove zero-width and combining marks
// 5 Width fold fullwidth to ASCII
// 6 Simple leet folding eg 4/@->a 0->o 1/!->i 3->e 5/$->s 7->t
// 7 Collapse whitespace to single spaces and trim
package normalize

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// Normalizer is concurrency safe when used with the pool below
type Normalizer struct{}

// pool of fresh transformer chains
var chainPool = sync.Pool{
	New: func() any {
		// order matters and mirrors the documented pipeline
		return transform.Chain(
			norm.NFKC,
			cases.Fold(),                       // unicode case folding
			runes.Remove(runes.In(unicode.Mn)), // strip combining marks
			runes.Remove(runes.In(unicode.Cf)), // strip format chars ZWJ ZWNJ FEFF etc
			width.Fold,                         // map fullwidth forms to ASCII
		)
	},
}

// New constructs a Normalizer
func New() *Normalizer { return &Normalizer{} }

// Normalize returns the normalized form of s following the pipeline described above
func (n *Normalizer) Normalize(s string) string {
	if s == "" {
		return ""
	}

	s = Sanitize(s)

	// 1 repair UTF-8 drop invalid bytes
	s = strings.ToValidUTF8(s, "")

	// 2-5 transform via pooled chain then reset and return it
	tr := chainPool.Get().(transform.Transformer)
	ns, _, _ := transform.String(tr, s)
	tr.Reset()
	chainPool.Put(tr)

	// 6 simple leet folding
	ns = leetFold(ns)

	// 7 collapse whitespace and trim
	ns = collapseSpaces(ns)

	return ns
}

// leetFold maps a tiny curated set of ASCII lookalikes to their letters
func leetFold(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '4', '@':
			b.WriteRune('a')
		case '0':
			b.WriteRune('o')
		case '1', '!':
			b.WriteRune('i')
		case '3':
			b.WriteRune('e')
		case '5', '$':
			b.WriteRune('s')
		case '7':
			b.WriteRune('t')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// collapseSpaces converts whitespace runs to a single ASCII space, but preserves line breaks.
// Runs that contain any newline are collapsed to a single newline. Leading/trailing spaces/newlines are trimmed
func collapseSpaces(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	inWS := false
	sawNL := false
	flush := func() {
		if !inWS {
			return
		}
		if sawNL {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		inWS = false
		sawNL = false
	}
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWS = true
			if r == '\n' || r == '\r' {
				sawNL = true
			}
			continue
		}
		flush()
		b.WriteRune(r)
	}
	flush()
	out := b.String()
	// Trim both spaces and newlines on edges
	out = strings.Trim(out, " \n\t\r")
	return out
}
