// internal/detector/detector_test.go
package detector

import (
	"testing"

	"swearjar/internal/core/normalize"
	"swearjar/internal/core/rulepack"
)

func mustPack(t *testing.T) *rulepack.Pack {
	t.Helper()
	p, err := rulepack.Load()
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	return p
}

func TestDetector_TemplateFirst(t *testing.T) {
	n := normalize.New()
	p := mustPack(t)
	d := New(p, 101)

	in := n.Normalize("fuck you dependabot - thanks for nothing")
	hits := d.Scan(in)

	var sawTemplate, sawLemma bool
	for _, h := range hits {
		if h.Source == SourceTemplate && h.Category == "tool" {
			sawTemplate = true
		}
		if h.Source == SourceLemma && h.Term == "fuck" {
			sawLemma = true
		}
	}
	if !sawTemplate {
		t.Fatalf("expected template hit")
	}
	if !sawLemma {
		t.Fatalf("expected lemma hit as well")
	}
}

func TestDetector_BoundariesAndStoplist(t *testing.T) {
	n := normalize.New()
	p := mustPack(t)
	d := New(p, 7)

	// "Scunthorpe" contains a substring, but is stoplisted
	in := n.Normalize("The Scunthorpe problem is notorious.")
	hits := d.Scan(in)
	for _, h := range hits {
		if h.Source == SourceLemma && h.Term == "cunt" {
			t.Fatalf("should not match inside stoplisted token: %+v", h)
		}
	}

	// Word boundary: "assess" should not generate "ass"
	in2 := n.Normalize("We will assess the situation.")
	hits2 := d.Scan(in2)
	for _, h := range hits2 {
		if h.Term == "ass" {
			t.Fatalf("unexpected boundary match in 'assess'")
		}
	}
}

func TestDetector_SpansAndMerge(t *testing.T) {
	n := normalize.New()
	p := mustPack(t)
	d := New(p, 5)

	in := n.Normalize("shit shit and more shit")
	hits := d.Scan(in)

	var lemma Hit
	found := false
	for _, h := range hits {
		if h.Source == SourceLemma && h.Term == "shit" {
			lemma = h
			found = true
		}
	}
	if !found {
		t.Fatalf("expected lemma hit for 'shit'")
	}
	if len(lemma.Spans) < 3 {
		t.Fatalf("expected multiple spans merged, got %d", len(lemma.Spans))
	}
	if lemma.DetectorVersion != 5 {
		t.Fatalf("version stamp missing")
	}
}
