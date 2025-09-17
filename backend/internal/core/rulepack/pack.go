// Package rulepack loads and compiles detection rules from the embedded v2 rules.json.
// It prepares regex templates and lemma sets for the detector
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

// slot aliases block (kept small; we only need names for expansion)
type slotAlias struct {
	ID    string   `json:"id"`
	Names []string `json:"names"`
}
type slotBlock struct {
	Aliases []slotAlias `json:"aliases"`
}

type allowlistBlock struct {
	Global []string            `json:"global"`
	ByZone map[string][]string `json:"by_zone"`
}

type rawTemplateV2 struct {
	ID             string         `json:"id"`
	Pattern        string         `json:"pattern"`
	Category       string         `json:"category"`
	Severity       int            `json:"severity"`
	Variants       []string       `json:"variants,omitempty"`
	ContextSignals map[string]any `json:"context_signals,omitempty"`
	Examples       []string       `json:"examples,omitempty"`
}

type rawLemmaV2 struct {
	Term           string         `json:"term"`
	Category       string         `json:"category"`
	Severity       int            `json:"severity"`
	Variants       []string       `json:"variants,omitempty"`
	ContextSignals map[string]any `json:"context_signals,omitempty"`
}

type rawPackV2 struct {
	Version      int                  `json:"version"`
	Meta         map[string]any       `json:"meta"`
	Categories   []string             `json:"categories"`
	VariantsSpec map[string]any       `json:"variants_spec"`
	Zones        map[string]any       `json:"zones"`
	Slots        map[string]slotBlock `json:"slots"`
	Lemmas       []rawLemmaV2         `json:"lemmas"`
	Templates    []rawTemplateV2      `json:"templates"`
	Allowlist    allowlistBlock       `json:"allowlist"`
	EngineHints  map[string]any       `json:"engine_hints"`
	SeverityMods []map[string]any     `json:"severity_mods"`
}

// SlotRef is a normalized reference for a single alias name
type SlotRef struct {
	Type string // "bot" | "tool" | "lang" | "framework" (derived from slot key)
	ID   string // stable alias id from rules.json (e.g., "dependabot")
}

// Pack represents a compiled rule pack for the detector (minimally extended)
type Pack struct {
	Version int

	// Compiled templates
	Templates []Template // 1:1 with Compiled
	Compiled  []*regexp.Regexp

	// Lemma backstop
	Lemmas   []Lemma
	LemmaSet map[string]Lemma // lowercased term -> lemma meta

	// Stoplist: token set to suppress lemma hits within those tokens
	Stopset map[string]struct{}

	// Optional extras (not used by detector today but handy later)
	Meta         map[string]any
	Categories   []string
	VariantsSpec map[string]any
	Zones        map[string]any
	EngineHints  map[string]any
	SeverityMods []map[string]any

	// Flattened slot values (escaped later in expandSlots)
	flatSlots map[string][]string

	// Name -> SlotRef (both plain "name" and "@name" are present, all lowercased)
	SlotNameToRef map[string]SlotRef
}

// Template represents a compiled regex template rule
type Template struct {
	PatternExpanded string
	Category        string
	Severity        int
	// forwarded from json (used for context gating, e.g. "frustration": true)
	ContextSignals map[string]any
}

// Lemma represents a substring rule
type Lemma struct {
	Term           string
	Category       string
	Severity       int
	ContextSignals map[string]any
}

// Load returns the compiled pack from the embedded v2 rules.json
func Load() (*Pack, error) {
	var rp rawPackV2
	if err := json.Unmarshal(embedded, &rp); err != nil {
		return nil, fmt.Errorf("rulepack: parse rules.json: %w", err)
	}
	if rp.Version != 2 {
		return nil, fmt.Errorf("rulepack: unsupported rules.json version %d (want 2)", rp.Version)
	}

	p := &Pack{
		Version:       rp.Version,
		Stopset:       make(map[string]struct{}, 256),
		LemmaSet:      make(map[string]Lemma, 1024),
		Meta:          rp.Meta,
		Categories:    rp.Categories,
		VariantsSpec:  rp.VariantsSpec,
		Zones:         rp.Zones,
		EngineHints:   rp.EngineHints,
		SeverityMods:  rp.SeverityMods,
		SlotNameToRef: make(map[string]SlotRef, 256),
	}

	// Flatten slots for expansion: map slot -> []names (lowercased, deduped)
	p.flatSlots = flattenSlots(rp.Slots)

	// Build stoplist from allowlist (global + all by_zone values), lowercased+deduped
	for _, s := range rp.Allowlist.Global {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			p.Stopset[s] = struct{}{}
		}
	}
	for _, lst := range rp.Allowlist.ByZone {
		for _, s := range lst {
			s = strings.ToLower(strings.TrimSpace(s))
			if s != "" {
				p.Stopset[s] = struct{}{}
			}
		}
	}

	// Compile templates: expand {SLOT} with flattened slot tokens (regex-quoted)
	for _, t := range rp.Templates {
		exp, err := expandSlots(t.Pattern, p.flatSlots)
		if err != nil {
			return nil, fmt.Errorf("rulepack: expand %q: %w", t.Pattern, err)
		}
		re, err := regexp.Compile(exp)
		if err != nil {
			return nil, fmt.Errorf("rulepack: compile %q: %w", exp, err)
		}
		p.Templates = append(p.Templates, Template{
			PatternExpanded: exp,
			Category:        t.Category,
			Severity:        t.Severity,
			ContextSignals:  t.ContextSignals,
		})
		p.Compiled = append(p.Compiled, re)
	}

	// Lemmas (lowercased; ignore empty)
	for _, l := range rp.Lemmas {
		term := strings.ToLower(strings.TrimSpace(l.Term))
		if term == "" {
			continue
		}
		lemma := Lemma{
			Term:           term,
			Category:       l.Category,
			Severity:       l.Severity,
			ContextSignals: l.ContextSignals,
		}
		p.Lemmas = append(p.Lemmas, lemma)
		p.LemmaSet[term] = lemma
	}

	// Slots: build name -> (type,id) map (also add '@name' forms)
	for rawKey, blk := range rp.Slots {
		st, ok := slotKeyToType(rawKey)
		if !ok {
			continue
		}
		for _, a := range blk.Aliases {
			id := strings.TrimSpace(a.ID)
			if id == "" {
				continue
			}
			for _, nm := range a.Names {
				nm = strings.ToLower(strings.TrimSpace(nm))
				if nm == "" {
					continue
				}
				p.SlotNameToRef[nm] = SlotRef{Type: st, ID: id}
				if nm[0] != '@' {
					p.SlotNameToRef["@"+nm] = SlotRef{Type: st, ID: id}
				}
			}
		}
	}

	// Deterministic iteration for tests/debug
	sort.Slice(p.Templates, func(i, j int) bool {
		return p.Templates[i].PatternExpanded < p.Templates[j].PatternExpanded
	})
	sort.Slice(p.Lemmas, func(i, j int) bool {
		return p.Lemmas[i].Term < p.Lemmas[j].Term
	})

	return p, nil
}

// flattenSlots converts the v2 alias blocks into simple lowercased name lists per slot
func flattenSlots(in map[string]slotBlock) map[string][]string {
	out := make(map[string][]string, len(in))
	for slot, blk := range in {
		var acc []string
		seen := make(map[string]struct{}, 32)
		for _, a := range blk.Aliases {
			for _, nm := range a.Names {
				nm = strings.ToLower(strings.TrimSpace(nm))
				if nm == "" {
					continue
				}
				if _, ok := seen[nm]; ok {
					continue
				}
				seen[nm] = struct{}{}
				acc = append(acc, nm)
			}
		}
		out[slot] = acc
	}
	return out
}

// expandSlots replaces {NAME} with a non-capturing group of OR'ed, regex-quoted values.
// Unknown {NAME} leaves the token literally (debug-friendly)
func expandSlots(pattern string, flatSlots map[string][]string) (string, error) {
	out := pattern
	for {
		i := strings.Index(out, "{")
		if i < 0 {
			break
		}
		j := strings.Index(out[i:], "}")
		if j < 0 {
			break // unbalanced; leave as-is
		}
		j = i + j
		name := out[i+1 : j]

		values, ok := flatSlots[name]
		if !ok || len(values) == 0 {
			// Leave unknown slot literal (helps surface wiring mistakes)
			out = out[:i] + "{" + name + "}" + out[j+1:]
			// continue after this brace to avoid infinite loop
			k := strings.Index(out[j+1:], "{")
			if k < 0 {
				break
			}
			continue
		}

		parts := make([]string, 0, len(values))
		for _, v := range values {
			// Values are plain strings (e.g., "c++"); QuoteMeta will escape regex chars
			parts = append(parts, regexp.QuoteMeta(v))
		}
		group := "(?:" + strings.Join(parts, "|") + ")"
		out = out[:i] + group + out[j+1:]
	}
	// Inputs are assumed normalized+folded; author controls anchors
	return strings.ToLower(out), nil
}

func slotKeyToType(k string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(k)) {
	case "TARGET_BOT":
		return "bot", true
	case "TARGET_TOOL":
		return "tool", true
	case "TARGET_LANG":
		return "lang", true
	case "TARGET_FRAMEWORK":
		return "framework", true
	default:
		return "", false
	}
}
