// Package detector scans normalized text using a rulepack. It prioritizes
// template regex matches, then falls back to lemma multi-pattern search.
// Inputs must be normalized (see internal/normalize).
package detector

import (
	"unicode"
	"unicode/utf8"

	"swearjar/internal/core/rulepack"
)

// Source indicates which rule type triggered the hit.
type Source string

const (
	// SourceTemplate means a regex template matched
	SourceTemplate Source = "template"

	// SourceLemma means a lemma substring matched
	SourceLemma Source = "lemma"
)

// Hit describes a detection occurrence. Spans are [start,end) byte offsets
// in the *normalized* input string.
type Hit struct {
	Term            string
	Category        string
	Severity        int
	Spans           [][2]int
	Source          Source
	DetectorVersion int
}

// Detector holds compiled rules and version.
type Detector struct {
	p       *rulepack.Pack
	version int
}

// New constructs a detector for the given rule pack and version stamp.
func New(p *rulepack.Pack, detectorVersion int) *Detector {
	return &Detector{p: p, version: detectorVersion}
}

// Scan runs template matches first, then lemma backstop. It returns merged hits
// (same term/category/severity/source are combined; spans are appended).
func (d *Detector) Scan(norm string) []Hit {
	var hits []Hit
	if norm == "" {
		return hits
	}

	// Templates (regex). Emit one hit per match with the matched text as term.
	for i, re := range d.p.Compiled {
		idxs := re.FindAllStringIndex(norm, -1)
		if len(idxs) == 0 {
			continue
		}
		meta := d.p.Templates[i]
		for _, pr := range idxs {
			hits = append(hits, Hit{
				Term:            norm[pr[0]:pr[1]], // store matched substring, not the regex
				Category:        meta.Category,
				Severity:        meta.Severity,
				Source:          SourceTemplate,
				DetectorVersion: d.version,
				Spans:           [][2]int{{pr[0], pr[1]}},
			})
		}
	}

	// Lemmas (multi-substring search with conservative boundaries).
	// We avoid heavy deps initially; can swap to AC later under this loop.
	for _, lm := range d.p.Lemmas {
		needle := lm.Term
		for off := 0; off < len(norm); {
			i := indexUnsafe(norm[off:], needle)
			if i < 0 {
				break
			}
			start := off + i
			end := start + len(needle)

			// Word-boundary check: letters/digits must not continue on either side.
			if d.boundaryOK(norm, start, end) && !d.inStoplist(norm, start, end) {
				hits = append(hits, Hit{
					Term:            needle,
					Category:        lm.Category,
					Severity:        lm.Severity,
					Spans:           [][2]int{{start, end}},
					Source:          SourceLemma,
					DetectorVersion: d.version,
				})
			}
			off = end // continue after this hit
		}
	}

	// Merge identical hits (term+category+severity+source)
	return mergeHits(hits)
}

// indexUnsafe is a simple byte-level substring search; inputs are normalized lowercase.
func indexUnsafe(haystack, needle string) int {
	// naive search; stdlib strings.Index is fine too, but we inline to keep
	// control on offsets
	return indexKMP(haystack, needle)
}

// boundaryOK ensures rune-wise word boundaries around [start,end).
func (d *Detector) boundaryOK(s string, start, end int) bool {
	var prev, next rune
	if start > 0 {
		prev, _ = utf8.DecodeLastRuneInString(s[:start])
	}
	if end < len(s) {
		next, _ = utf8.DecodeRuneInString(s[end:])
	}
	// Inside must be letters/digits somewhere; but we just ensure we're not
	// glued to other word chars on the edges
	return !isWord(prev) && !isWord(next)
}

func isWord(r rune) bool {
	if r == utf8.RuneError || r == 0 {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// inStoplist checks if the matched window sits wholly within any stoplist word.
// We do a minimal check: expand to the containing token and see if token is stoplisted.
func (d *Detector) inStoplist(s string, start, end int) bool {
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
	token := s[ls:rs]
	_, banned := d.p.Stopset[token]
	return banned
}

// mergeHits combines hits with identical identity fields
func mergeHits(in []Hit) []Hit {
	type key struct {
		t, c string
		sv   int
		src  Source
		ver  int
	}
	m := make(map[key]int, len(in))
	var out []Hit
	for _, h := range in {
		k := key{h.Term, h.Category, h.Severity, h.Source, h.DetectorVersion}
		if idx, ok := m[k]; ok {
			out[idx].Spans = append(out[idx].Spans, h.Spans...)
			continue
		}
		m[k] = len(out)
		out = append(out, h)
	}
	return out
}

// indexKMP is a Knuth-Morris-Pratt substring search implementation
func indexKMP(text, pat string) int {
	if len(pat) == 0 {
		return 0
	}
	lps := make([]int, len(pat))
	// build lps
	for i, l := 1, 0; i < len(pat); {
		if pat[i] == pat[l] {
			l++
			lps[i] = l
			i++
		} else if l != 0 {
			l = lps[l-1]
		} else {
			lps[i] = 0
			i++
		}
	}
	for i, j := 0, 0; i < len(text); {
		if text[i] == pat[j] {
			i++
			j++
			if j == len(pat) {
				return i - j
			}
		} else if j != 0 {
			j = lps[j-1]
		} else {
			i++
		}
	}
	return -1
}
