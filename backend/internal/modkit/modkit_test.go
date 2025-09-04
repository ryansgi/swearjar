// Package modkit provides building blocks for modular Go applications
package modkit

import (
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

// stub module that satisfies Module and records calls
type stub struct {
	mounted bool
	ports   any
}

func (s *stub) MountRoutes(_ phttp.Router) { s.mounted = true }
func (s *stub) Ports() any                 { return s.ports }
func (s *stub) Name() string               { return "" }

// compile-time assertion: stub implements Module
var _ Module = (*stub)(nil)

func TestModule_InterfaceSurface(t *testing.T) {
	t.Parallel()

	m := &stub{ports: 42}

	// typed nil router is fine; just validate call flow
	var r phttp.Router = nil
	m.MountRoutes(r)

	if !m.mounted {
		t.Fatal("expected MountRoutes to be called")
	}

	if got := m.Ports(); got != 42 {
		t.Fatalf("unexpected Ports value: got=%v want=42", got)
	}
}

func TestBuilder_TypeSignatureAndUse(t *testing.T) {
	t.Parallel()

	// A minimal Builder that ignores deps/options and returns a stub
	var b Builder = func(_ Deps, _ ...Option) Module {
		return &stub{ports: "ok"}
	}

	m := b(Deps{})
	if m == nil {
		t.Fatal("builder returned nil module")
	}

	if p := m.Ports(); p != "ok" {
		t.Fatalf("unexpected Ports value from built module: got=%v want=ok", p)
	}
}
