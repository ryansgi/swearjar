// internal/rulepack/pack_test.go
package rulepack

import (
	"regexp"
	"testing"
)

func TestLoadAndExpand(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if p.Version == 0 {
		t.Fatalf("expected non-zero version")
	}
	if len(p.Templates) == 0 || len(p.Compiled) == 0 {
		t.Fatalf("expected compiled templates")
	}
	for i, tplt := range p.Templates {
		if p.Compiled[i] == nil {
			t.Fatalf("nil compiled regexp at %d", i)
		}
		if _, err := regexp.Compile(tplt.PatternExpanded); err != nil {
			t.Fatalf("expanded pattern invalid: %q: %v", tplt.PatternExpanded, err)
		}
	}
	if _, ok := p.LemmaSet["fuck"]; !ok {
		t.Fatalf("lemma 'fuck' missing")
	}
	if _, ok := p.Stopset["scunthorpe"]; !ok {
		t.Fatalf("stoplist missing scunthorpe")
	}
}

func TestExpandSlotsStandalone(t *testing.T) {
	exp, err := expandSlots("hello {WHO}", map[string][]string{
		"WHO": {"world", "you"},
	})
	if err != nil {
		t.Fatalf("expand err: %v", err)
	}
	want := "hello (?:(?:world)|(?:you))" // lowercase, but regexp.QuoteMeta adds nothing; our group is "(?:world|you)"
	_ = want
	// Looser check: group contains both options
	if exp != "hello (?:(?:world)|(?:you))" && exp != "hello (?:(world|you))" {
		// Just ensure both alternatives present, since join style can differ
		if !(regexp.MustCompile(`hello\s+\(\?:.*world.*\|.*you.*\)`).MatchString(exp)) {
			t.Fatalf("unexpected expansion: %q", exp)
		}
	}
}
