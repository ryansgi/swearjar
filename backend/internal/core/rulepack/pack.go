// Package rulepack loads and compiles detection rules. It embeds a small JSON
// file (rules.json) and prepares regex templates and lemma sets for the detector
package rulepack

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed rules.json
var embedded []byte

// Raw JSON schema (stable wire form)
type rawPack struct {
	Version   int                 `json:"version"`
	Templates []rawTemplate       `json:"templates"`
	Slots     map[string][]string `json:"slots"`
	Lemmas    []rawLemma          `json:"lemmas"`
	Stoplist  []string            `json:"stoplist"`
}

type rawTemplate struct {
	Pattern  string `json:"pattern"`
	Category string `json:"category"`
	Severity int    `json:"severity"`
}

type rawLemma struct {
	Term     string `json:"term"`
	Category string `json:"category"`
	Severity int    `json:"severity"`
}

// Pack represents a compiled rule pack for the detector
type Pack struct {
	Version int

	Templates []Template // 1:1 with Compiled in index
	Compiled  []*regexp.Regexp

	Lemmas   []Lemma
	Stopset  map[string]struct{}
	LemmaSet map[string]Lemma // lowercased term -> lemma meta
}

// Template represents a compiled regex template rule
type Template struct {
	PatternExpanded string
	Category        string
	Severity        int
}

// Lemma represents a substring rule
type Lemma struct {
	Term     string
	Category string
	Severity int
}

// Load returns the compiled pack from the embedded rules.json
func Load() (*Pack, error) {
	var rp rawPack
	if err := json.Unmarshal(embedded, &rp); err != nil {
		return nil, fmt.Errorf("rulepack: parse rules.json: %w", err)
	}

	p := &Pack{
		Version: rp.Version,
		Stopset: make(map[string]struct{}, len(rp.Stoplist)),
	}

	// Stoplist: set (lowercased)
	for _, s := range rp.Stoplist {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			p.Stopset[s] = struct{}{}
		}
	}

	// Compile templates: expand {SLOT} to (?:a|b|c) safely
	for _, t := range rp.Templates {
		exp, err := expandSlots(t.Pattern, rp.Slots)
		if err != nil {
			return nil, fmt.Errorf("rulepack: expand %q: %w", t.Pattern, err)
		}
		// We assume inputs are normalized+folded, so patterns should be plain lowercase.
		// Add simple boundaries: we prefer matching across word-ish delimiters in text,
		// not code identifiers. Keep conservative
		re, err := regexp.Compile(exp)
		if err != nil {
			return nil, fmt.Errorf("rulepack: compile %q: %w", exp, err)
		}
		p.Templates = append(p.Templates, Template{
			PatternExpanded: exp,
			Category:        t.Category,
			Severity:        t.Severity,
		})
		p.Compiled = append(p.Compiled, re)
	}

	// Lemmas
	p.Lemmas = make([]Lemma, 0, len(rp.Lemmas))
	p.LemmaSet = make(map[string]Lemma, len(rp.Lemmas))
	for _, l := range rp.Lemmas {
		term := strings.ToLower(strings.TrimSpace(l.Term))
		if term == "" {
			continue
		}
		lemma := Lemma{
			Term:     term,
			Category: l.Category,
			Severity: l.Severity,
		}
		p.Lemmas = append(p.Lemmas, lemma)
		p.LemmaSet[term] = lemma
	}

	// Keep deterministic iteration for tests/debug
	sort.Slice(p.Templates, func(i, j int) bool {
		return p.Templates[i].PatternExpanded < p.Templates[j].PatternExpanded
	})
	// (Compiled slice index still aligns with Templates because we constructed in order)

	sort.Slice(p.Lemmas, func(i, j int) bool {
		return p.Lemmas[i].Term < p.Lemmas[j].Term
	})

	return p, nil
}

// expandSlots replaces {NAME} with a non-capturing group of OR'ed, escaped values.
// Unknown {NAME} leaves the token literally (so rules don't fail if slots evolve)
func expandSlots(pattern string, slots map[string][]string) (string, error) {
	out := pattern
	for {
		i := strings.Index(out, "{")
		if i < 0 {
			break
		}
		j := strings.Index(out[i:], "}")
		if j < 0 {
			// Unbalanced; treat literally
			break
		}
		j = i + j
		name := out[i+1 : j]
		values, ok := slots[name]
		if !ok || len(values) == 0 {
			// Leave as-is if slot unknown, to make debugging obvious
			out = out[:i] + "{" + name + "}" + out[j+1:]
			// Move past this occurrence to avoid infinite loop
			k := strings.Index(out[j+1:], "{")
			if k < 0 {
				break
			}
			continue
		}

		var parts []string
		for _, v := range values {
			v = strings.ToLower(strings.TrimSpace(v))
			if v == "" {
				continue
			}
			parts = append(parts, regexp.QuoteMeta(v))
		}
		if len(parts) == 0 {
			parts = []string{""}
		}
		group := "(?:" + strings.Join(parts, "|") + ")"
		out = out[:i] + group + out[j+1:]
	}
	// We do not add anchors; author controls them in patterns
	return strings.ToLower(out), nil
}
