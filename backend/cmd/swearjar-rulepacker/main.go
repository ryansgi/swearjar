// Command swearjar-rulepacker
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// todo: centralize with rulepack/pack.go?
type coreFile struct {
	Version      int                  `json:"version"`
	Meta         map[string]any       `json:"meta"`
	Categories   []string             `json:"categories"`
	VariantsSpec map[string]any       `json:"variants_spec"`
	Zones        map[string]any       `json:"zones"`
	Slots        map[string]slotBlock `json:"slots"`
	Allowlist    allowlistBlock       `json:"allowlist"`
	EngineHints  map[string]any       `json:"engine_hints"`
	SeverityMods []map[string]any     `json:"severity_mods"`
}

type slotBlock struct {
	Aliases []struct {
		ID    string   `json:"id"`
		Names []string `json:"names"`
	} `json:"aliases"`
}

type allowlistBlock struct {
	Global []string            `json:"global"`
	ByZone map[string][]string `json:"by_zone"`
}

type fragmentFile struct {
	Language    string          `json:"language"`
	Lemmas      []lemma         `json:"lemmas"`
	Templates   []template      `json:"templates"`
	Allowlist   *allowlistBlock `json:"allowlist,omitempty"`
	EngineHints map[string]any  `json:"engine_hints,omitempty"`
}

type lemma struct {
	Term           string         `json:"term"`
	Category       string         `json:"category"`
	Severity       int            `json:"severity"`
	Variants       []string       `json:"variants,omitempty"`
	ContextSignals map[string]any `json:"context_signals,omitempty"`
}

type template struct {
	ID             string         `json:"id"`
	Pattern        string         `json:"pattern"`
	Category       string         `json:"category"`
	Severity       int            `json:"severity"`
	Variants       []string       `json:"variants,omitempty"`
	ContextSignals map[string]any `json:"context_signals,omitempty"`
	Examples       []string       `json:"examples,omitempty"`
}

type outV2 struct {
	Version      int                  `json:"version"`
	Meta         map[string]any       `json:"meta,omitempty"`
	Categories   []string             `json:"categories,omitempty"`
	VariantsSpec map[string]any       `json:"variants_spec,omitempty"`
	Zones        map[string]any       `json:"zones,omitempty"`
	Slots        map[string]slotBlock `json:"slots"`
	Lemmas       []lemma              `json:"lemmas"`
	Templates    []template           `json:"templates"`
	Allowlist    allowlistBlock       `json:"allowlist,omitempty"`
	EngineHints  map[string]any       `json:"engine_hints,omitempty"`
	SeverityMods []map[string]any     `json:"severity_mods,omitempty"`
}

func readJSON[T any](path string, into *T) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, into); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func must(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func mergeAllowlist(dst *allowlistBlock, src *allowlistBlock) {
	if src == nil {
		return
	}
	if len(src.Global) > 0 {
		dst.Global = append(dst.Global, src.Global...)
	}
	if dst.ByZone == nil && len(src.ByZone) > 0 {
		dst.ByZone = make(map[string][]string, len(src.ByZone))
	}
	for k, v := range src.ByZone {
		dst.ByZone[k] = append(dst.ByZone[k], v...)
	}
}

func mergeEngineHints(dst *map[string]any, src map[string]any) {
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = make(map[string]any, len(src))
	}
	maps.Copy((*dst), src)
}

func findFragmentFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if strings.HasPrefix(rel, "schema") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == "core.json" && filepath.Dir(path) == root {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func hasCore(dir string) bool {
	return pathExists(filepath.Join(dir, "core.json"))
}

func latestNumericSubdir(dir string) (string, bool) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var nums []int
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		n, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if hasCore(filepath.Join(dir, e.Name())) {
			nums = append(nums, n)
		}
	}
	if len(nums) == 0 {
		return "", false
	}
	sort.Ints(nums)
	best := nums[len(nums)-1]
	return filepath.Join(dir, strconv.Itoa(best)), true
}

// resolveRoot tries, in order: flag, env, common locations.
// - If you pass /app/rules, it picks the latest numeric subdir containing core.json.
// - If you pass /app/rules/1, it uses that.
// Returns chosen root and an ordered list of attempts (for error messages)
func resolveRoot(flagRoot string) (string, []string, error) {
	var attempts []string
	try := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		attempts = append(attempts, p)
		// exact rules/<n>
		if hasCore(p) {
			return p, true
		}
		// parent rules/ (pick latest)
		if sub, ok := latestNumericSubdir(p); ok && hasCore(sub) {
			attempts = append(attempts, sub)
			return sub, true
		}
		return "", false
	}

	// explicit flag
	if root, ok := try(flagRoot); ok {
		return root, attempts, nil
	}
	// env
	if env := strings.TrimSpace(os.Getenv("SWEARJAR_RULES_ROOT")); env != "" {
		if root, ok := try(env); ok {
			return root, attempts, nil
		}
	}
	// common relative and absolute locations
	candidates := []string{
		"./rules/1",
		"./rules",
		"/app/rules/1",
		"/app/rules",
	}
	for _, c := range candidates {
		if root, ok := try(c); ok {
			return root, attempts, nil
		}
	}
	return "", attempts, errors.New("core.json not found in any known location")
}

func assemble(root string) (outV2, error) {
	corePath := filepath.Join(root, "core.json")
	var core coreFile
	if err := readJSON(corePath, &core); err != nil {
		return outV2{}, fmt.Errorf("read core.json: %w", err)
	}
	if core.Version != 2 {
		_, _ = fmt.Fprintf(os.Stderr, "warning: core.json version=%d (expected 2)\n", core.Version)
	}

	fragPaths, err := findFragmentFiles(root)
	if err != nil {
		return outV2{}, err
	}
	if len(fragPaths) == 0 {
		return outV2{}, errors.New("no fragment files found under " + root)
	}

	// merged accumulators
	type lrec struct {
		Lang string
		Val  lemma
	}
	var lemRecs []lrec
	var allTemplates []template
	mergedAllow := core.Allowlist
	var mergedHints map[string]any
	mergeEngineHints(&mergedHints, core.EngineHints)

	for _, p := range fragPaths {
		var fr fragmentFile
		if err := readJSON(p, &fr); err != nil {
			return outV2{}, err
		}
		if fr.Language == "" {
			return outV2{}, fmt.Errorf("fragment missing language: %s", p)
		}
		for _, l := range fr.Lemmas {
			lemRecs = append(lemRecs, lrec{Lang: fr.Language, Val: l})
		}
		allTemplates = append(allTemplates, fr.Templates...)
		mergeAllowlist(&mergedAllow, fr.Allowlist)
		mergeEngineHints(&mergedHints, fr.EngineHints)
	}

	// de-dupe lemmas by (language, term)
	type lkey struct {
		lang string
		term string
	}
	seenL := map[lkey]bool{}
	allLemmas := make([]lemma, 0, len(lemRecs))
	for _, r := range lemRecs {
		k := lkey{
			lang: strings.ToLower(strings.TrimSpace(r.Lang)),
			term: strings.ToLower(strings.TrimSpace(r.Val.Term)),
		}
		if k.lang == "" || k.term == "" || seenL[k] {
			continue
		}
		seenL[k] = true
		allLemmas = append(allLemmas, r.Val)
	}
	sort.Slice(allLemmas, func(i, j int) bool {
		if allLemmas[i].Category != allLemmas[j].Category {
			return allLemmas[i].Category < allLemmas[j].Category
		}
		return strings.ToLower(allLemmas[i].Term) < strings.ToLower(allLemmas[j].Term)
	})

	// de-dupe templates by ID (if present) then by (pattern,category,severity)
	type tkey struct {
		pat, cat string
		sev      int
	}
	seenID := map[string]bool{}
	seenPK := map[tkey]bool{}
	tout := make([]template, 0, len(allTemplates))
	for _, t := range allTemplates {
		if id := strings.TrimSpace(t.ID); id != "" {
			if seenID[id] {
				_, _ = fmt.Fprintf(os.Stderr, "warning: duplicate template id %q skipped\n", id)
				continue
			}
			seenID[id] = true
			tout = append(tout, t)
			continue
		}
		k := tkey{pat: t.Pattern, cat: t.Category, sev: t.Severity}
		if seenPK[k] {
			continue
		}
		seenPK[k] = true
		tout = append(tout, t)
	}
	sort.Slice(tout, func(i, j int) bool {
		li, lj := strings.TrimSpace(tout[i].ID), strings.TrimSpace(tout[j].ID)
		if li != "" || lj != "" {
			return li < lj
		}
		if tout[i].Category != tout[j].Category {
			return tout[i].Category < tout[j].Category
		}
		if tout[i].Severity != tout[j].Severity {
			return tout[i].Severity < tout[j].Severity
		}
		return tout[i].Pattern < tout[j].Pattern
	})

	return outV2{
		Version:      2,
		Meta:         core.Meta,
		Categories:   core.Categories,
		VariantsSpec: core.VariantsSpec,
		Zones:        core.Zones,
		Slots:        core.Slots,
		Lemmas:       allLemmas,
		Templates:    tout,
		Allowlist:    mergedAllow,
		EngineHints:  mergedHints,
		SeverityMods: core.SeverityMods,
	}, nil
}

func main() {
	var (
		flagRoot = flag.String("root", "", "path to rules version directory (e.g., ./rules/1 or ./rules). If empty, auto-discover") //nolint:lll
		out      = flag.String("out", "./internal/core/rulepack/rules.json", "output path or '-' for stdout")
		pretty   = flag.Bool("pretty", true, "pretty-print JSON")
		verbose  = flag.Bool("v", false, "verbose logging")
	)
	flag.Parse()

	root, attempts, err := resolveRoot(strings.TrimSpace(*flagRoot))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to locate rules root (looked in):\n")
		for _, a := range attempts {
			_, _ = fmt.Fprintf(os.Stderr, "  - %s\n", a)
		}
		_, _ = fmt.Fprintf(os.Stderr, "hint: mount ./rules into the container (e.g., - ./rules:/app/rules:ro) or set SWEARJAR_RULES_ROOT\n") //nolint:lll
		must(err)
	}
	if *verbose {
		_, _ = fmt.Fprintf(os.Stderr, "using rules root: %s\n", root)
	}

	obj, err := assemble(root)
	must(err)

	var enc []byte
	if *pretty {
		enc, err = json.MarshalIndent(obj, "", "  ")
	} else {
		enc, err = json.Marshal(obj)
	}
	must(err)

	if *out == "-" {
		if _, err := os.Stdout.Write(enc); err != nil {
			must(err)
		}
		if _, err := os.Stdout.WriteString("\n"); err != nil {
			must(err)
		}
		return
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		must(err)
	}
	must(os.WriteFile(*out, enc, 0o644))
	if *verbose {
		_, _ = fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *out, len(enc))
	}
}
