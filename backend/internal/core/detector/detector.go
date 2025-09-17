// Package detector implements swear word detection over normalized text
package detector

import (
	"strings"
	"unicode/utf8"

	"swearjar/internal/core/normalize"
	"swearjar/internal/core/rulepack"
)

// Source indicates how a Hit was generated
type Source string

const (
	// SourceTemplate indicates a hit from a template regex
	SourceTemplate Source = "template"
	// SourceLemma indicates a hit from a lemma/AC match
	SourceLemma Source = "lemma"
)

// Hit spans are [start,end) over the normalized input
type Hit struct {
	Term            string
	Category        string
	Severity        int
	Spans           [][2]int
	Source          Source
	DetectorVersion int

	Pre   string   // up to Options.ContextWindow bytes preceding the first span
	Post  string   // up to Options.ContextWindow bytes following the last span
	Zones []string // zone tags that overlap this hit (code_fence, code_inline, quote)

	// Context-targeting results
	TargetType     string // "bot" | "tool" | "lang" | "framework" | ""
	TargetID       string // alias id from rulepack (e.g., "dependabot")
	TargetName     string // exact surface mention matched (e.g., "@dependabot")
	TargetStart    int    // absolute byte offset (inclusive)
	TargetEnd      int    // absolute byte offset (exclusive)
	TargetDistance int    // abs(bytes) from hit center to target start
	CtxAction      string // "none" | "upgraded" | "downgraded"
}

// Options controls detector behavior
type Options struct {
	// MaxTotalHits is the hard cap on total emitted hits (0 = no cap)
	MaxTotalHits int
	// AllowOverlapping allows lemma hits to overlap (default false)
	AllowOverlapping bool
	// Context window size (bytes) for Pre/Post + target search; 0 disables context capture/targeting
	ContextWindow int
	// Severity dampening within zones (negative numbers reduce severity)
	// Clamp is applied after summing deltas (min 1)
	SeverityDeltaInCodeFence  int
	SeverityDeltaInCodeInline int
	SeverityDeltaInQuote      int
}

// slotType mirrors the logical slot kinds
type slotType string

const (
	slotBot       slotType = "bot"
	slotTool      slotType = "tool"
	slotLang      slotType = "lang"
	slotFramework slotType = "framework"
)

// aliasEntry is a flattened alias name -> (type,id) row
type aliasEntry struct {
	typ  slotType
	id   string
	name string // normalized alias form (also includes "@name")
}

// Detector runs detection over normalized text
type Detector struct {
	p       *rulepack.Pack
	version int
	opts    Options

	ac         *acAutomaton
	lemmaIndex []rulepack.Lemma
	lemmaLens  []int

	// contextual targeting
	aliases []aliasEntry // flat index to scan quickly
}

// New creates a Detector with default options
func New(p *rulepack.Pack, detectorVersion int) *Detector {
	return NewWithOptions(p, detectorVersion, Options{})
}

// NewWithOptions creates a Detector with custom options
func NewWithOptions(p *rulepack.Pack, detectorVersion int, opts Options) *Detector {
	d := &Detector{p: p, version: detectorVersion, opts: opts}

	// Build AC automaton over lemmas
	ac := newAutomaton()
	lemmaLens := make([]int, len(p.Lemmas))
	for i, lm := range p.Lemmas {
		term := lm.Term
		if term == "" {
			continue
		}
		ac.AddPattern([]byte(term), i)
		lemmaLens[i] = len(term)
	}
	ac.Build()
	d.ac = ac
	d.lemmaIndex = p.Lemmas
	d.lemmaLens = lemmaLens

	// Context targeting alias index (built from pack.SlotNameToRef)
	d.initAliasIndex()

	return d
}

func (d *Detector) initAliasIndex() {
	if d.p == nil || d.p.SlotNameToRef == nil {
		return
	}
	out := make([]aliasEntry, 0, len(d.p.SlotNameToRef))
	for nm, ref := range d.p.SlotNameToRef {
		var t slotType
		switch ref.Type {
		case "bot":
			t = slotBot
		case "tool":
			t = slotTool
		case "lang":
			t = slotLang
		case "framework":
			t = slotFramework
		default:
			continue
		}
		out = append(out, aliasEntry{
			typ:  t,
			id:   ref.ID,
			name: strings.ToLower(strings.TrimSpace(nm)),
		})
	}
	d.aliases = out
}

// Scan runs detection over a normalized string, returning hits
func (d *Detector) Scan(norm string) []Hit {
	var hits []Hit
	if norm == "" {
		return hits
	}

	maxHits := d.opts.MaxTotalHits
	if maxHits > 0 {
		hits = make([]Hit, 0, maxHits)
	}

	appendHit := func(h Hit) {
		hits = append(hits, h)
	}

	// Detect zones once per document
	zones := normalize.DetectZones(norm)
	cwEnabled := d.opts.ContextWindow > 0

	// Stage A: templates (use Pack.Compiled and metadata from Pack.Templates)
TEMPLATES:
	for i := range d.p.Compiled {
		re := d.p.Compiled[i]
		if re == nil {
			continue
		}
		tmeta := d.p.Templates[i] // PatternExpanded/Category/Severity/ContextSignals
		prs := re.FindAllStringIndex(norm, -1)
		if len(prs) == 0 {
			continue
		}

		isFrustration := hasFrustration(tmeta.ContextSignals)

		for _, pr := range prs {
			start, end := pr[0], pr[1]
			if !d.boundaryOK(norm, start, end) || d.inStoplist(norm, start, end) {
				continue
			}

			match := norm[start:end]

			h := Hit{
				Term:            match,
				Category:        tmeta.Category,
				Severity:        tmeta.Severity,
				Source:          SourceTemplate,
				DetectorVersion: d.version,
				Spans:           [][2]int{{start, end}},
			}

			h.Zones = zoneTagsForSpan(zones, start, end)
			h.Severity = d.applyZoneDampening(h.Severity, h.Zones)

			if cwEnabled {
				h.Pre, h.Post = contextAround(norm, start, end, d.opts.ContextWindow)
				d.applyTargetingAndGating(norm, &h, isFrustration)
			}

			appendHit(h)
			if maxHits > 0 && len(hits) >= maxHits {
				break TEMPLATES
			}
		}
	}

	// Stage B: lemmas (unchanged)
	if d.ac != nil && len(d.lemmaIndex) > 0 {
		lastEnd := -1
		d.ac.FindAll([]byte(norm), func(end int, lemmaID int) bool {
			llen := d.lemmaLens[lemmaID]
			start := end - llen
			if !d.opts.AllowOverlapping && start < lastEnd {
				return true
			}
			if d.boundaryOK(norm, start, end) && !d.inStoplist(norm, start, end) {
				lm := d.lemmaIndex[lemmaID]
				h := Hit{
					Term:            lm.Term,
					Category:        lm.Category,
					Severity:        lm.Severity,
					Source:          SourceLemma,
					DetectorVersion: d.version,
					Spans:           [][2]int{{start, end}},
				}
				h.Zones = zoneTagsForSpan(zones, start, end)
				h.Severity = d.applyZoneDampening(h.Severity, h.Zones)
				if cwEnabled {
					h.Pre, h.Post = contextAround(norm, start, end, d.opts.ContextWindow)
					d.applyTargetingAndGating(norm, &h, false)
				}
				hits = append(hits, h)
				if !d.opts.AllowOverlapping {
					lastEnd = end
				}
				if maxHits > 0 && len(hits) >= maxHits {
					return false
				}
			}
			return true
		})
	}

	return hits
}

func hasFrustration(cs map[string]any) bool {
	if len(cs) == 0 {
		return false
	}
	if v, ok := cs["frustration"]; ok {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return strings.EqualFold(t, "true") || strings.EqualFold(t, "yes")
		}
	}
	return false
}

// applyTargetingAndGating scans for a nearby target and upgrades/downgrades category.
// Also fills Target* fields on the hit. If no context window configured, this is a no-op
func (d *Detector) applyTargetingAndGating(s string, h *Hit, isFrustration bool) {
	if d.opts.ContextWindow <= 0 || len(d.aliases) == 0 {
		return
	}

	a := h.Spans[0][0]
	b := h.Spans[len(h.Spans)-1][1]

	// pick preferred slot types based on the current category
	var prefer []slotType
	switch h.Category {
	case "bot_rage":
		prefer = []slotType{slotBot}
	case "tooling_rage":
		prefer = []slotType{slotTool}
	case "lang_rage":
		prefer = []slotType{slotLang, slotFramework}
	}

	ok, typ, id, name, ts, te, dist := d.scanNearbyTarget(s, a, b, prefer...)
	if ok {
		h.TargetType = string(typ)
		h.TargetID = id
		h.TargetName = name
		h.TargetStart = ts
		h.TargetEnd = te
		h.TargetDistance = dist
		// Upgrade generic frustration to specific rage when a concrete target is present
		if h.Category == "generic" && isFrustration {
			switch typ {
			case slotBot:
				h.Category = "bot_rage"
			case slotTool:
				h.Category = "tooling_rage"
			case slotLang, slotFramework:
				h.Category = "lang_rage"
			}
			h.CtxAction = "upgraded"
		} else {
			h.CtxAction = "none"
		}
	} else {
		// If a specific rage category lacks its expected target, downgrade to generic
		switch h.Category {
		case "bot_rage", "tooling_rage", "lang_rage":
			h.Category = "generic"
			h.CtxAction = "downgraded"
		default:
			h.CtxAction = "none"
		}
	}
}

func (d *Detector) boundaryOK(s string, start, end int) bool {
	var prev, next rune
	if start > 0 {
		prev, _ = utf8.DecodeLastRuneInString(s[:start])
	}
	if end < len(s) {
		next, _ = utf8.DecodeRuneInString(s[end:])
	}
	return !isWord(prev) && !isWord(next)
}

func (d *Detector) inStoplist(s string, start, end int) bool {
	ls, rs := expandToToken(s, start, end)
	token := s[ls:rs]
	_, banned := d.p.Stopset[token]
	return banned
}

// applyZoneDampening adjusts severity by configured deltas for any overlapping zones.
// Clamps to a minimum of 1
func (d *Detector) applyZoneDampening(sev int, zones []string) int {
	if len(zones) == 0 || (d.opts.SeverityDeltaInCodeFence|
		d.opts.SeverityDeltaInCodeInline|
		d.opts.SeverityDeltaInQuote) == 0 {
		if sev < 1 {
			return 1
		}
		return sev
	}

	delta := 0
	for _, z := range zones {
		switch z {
		case string(normalize.ZoneCodeFence):
			delta += d.opts.SeverityDeltaInCodeFence
		case string(normalize.ZoneCodeInline):
			delta += d.opts.SeverityDeltaInCodeInline
		case string(normalize.ZoneQuote):
			delta += d.opts.SeverityDeltaInQuote
		}
	}
	sev += delta
	if sev < 1 {
		sev = 1
	}
	return sev
}

// contextAround returns [pre, post] around [start,end)
func contextAround(s string, start, end, win int) (string, string) {
	if win <= 0 {
		return "", ""
	}
	ls := max(start-win, 0)
	rs := min(end+win, len(s))
	return s[ls:start], s[end:rs]
}

// scanNearbyTarget searches within the configured ContextWindow around [a,b) for the
// nearest alias; if 'prefer' is non-empty, it prefers returning a target of those types
func (d *Detector) scanNearbyTarget(
	s string,
	a, b int,
	prefer ...slotType,
) (bool, slotType, string, string, int, int, int) {
	if len(d.aliases) == 0 || a < 0 || b > len(s) || a >= b {
		return false, "", "", "", 0, 0, 0
	}
	win := d.opts.ContextWindow
	ls := a - win
	if ls < 0 {
		ls = 0
	}
	rs := min(b+win, len(s))
	region := s[ls:rs]

	// Preference filter (optional)
	var hasPrefer bool
	var preferSet map[slotType]struct{}
	if len(prefer) > 0 {
		hasPrefer = true
		preferSet = make(map[slotType]struct{}, len(prefer))
		for _, t := range prefer {
			preferSet[t] = struct{}{}
		}
	}

	nearest := struct {
		ok         bool
		typ        slotType
		id, name   string
		start, end int
		dist       int
	}{}

	center := (a + b) / 2

	consider := func(al aliasEntry) {
		pos := strings.Index(region, al.name)
		for pos >= 0 {
			absStart := ls + pos
			absEnd := absStart + len(al.name)
			if d.boundaryOK(s, absStart, absEnd) {
				dist := abs(center - absStart)
				if !nearest.ok || dist < nearest.dist ||
					(dist == nearest.dist && absStart < nearest.start) {
					nearest.ok = true
					nearest.typ = al.typ
					nearest.id = al.id
					nearest.name = al.name
					nearest.start = absStart
					nearest.end = absEnd
					nearest.dist = dist
				}
			}
			// find next occurrence
			next := strings.Index(region[pos+len(al.name):], al.name)
			if next < 0 {
				break
			}
			pos += len(al.name) + next
		}
	}

	// First pass: preferred types only (if any)
	if hasPrefer {
		for _, al := range d.aliases {
			if _, ok := preferSet[al.typ]; ok {
				consider(al)
			}
		}
		// If we found a preferred target, return early
		if nearest.ok {
			return true, nearest.typ, nearest.id, nearest.name, nearest.start, nearest.end, nearest.dist
		}
	}

	// Second pass: any type
	for _, al := range d.aliases {
		if hasPrefer {
			if _, ok := preferSet[al.typ]; ok {
				continue // already scanned these in the first pass
			}
		}
		consider(al)
	}

	if !nearest.ok {
		return false, "", "", "", 0, 0, 0
	}
	return true, nearest.typ, nearest.id, nearest.name, nearest.start, nearest.end, nearest.dist
}

func zoneTagsForSpan(zs []normalize.ZoneSpan, start, end int) []string {
	if len(zs) == 0 {
		return nil
	}
	var inFence, inInline, inQuote bool
	for _, z := range zs {
		if end <= z.Start || start >= z.End {
			continue
		}
		switch z.Type {
		case normalize.ZoneCodeFence:
			inFence = true
		case normalize.ZoneCodeInline:
			inInline = true
		case normalize.ZoneQuote:
			inQuote = true
		}
	}
	// Pack without allocating a map
	if !(inFence || inInline || inQuote) {
		return nil
	}
	out := make([]string, 0, 3)
	if inFence {
		out = append(out, string(normalize.ZoneCodeFence))
	}
	if inInline {
		out = append(out, string(normalize.ZoneCodeInline))
	}
	if inQuote {
		out = append(out, string(normalize.ZoneQuote))
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
