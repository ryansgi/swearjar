package modkit

import (
	"net/http"
	"reflect"
	"testing"

	"swearjar/internal/modkit/httpkit"
)

func TestBuild_Defaults(t *testing.T) {
	t.Parallel()

	b := Build()

	if b.Name != "" {
		t.Fatalf("default Name = %q, want empty", b.Name)
	}
	if b.Prefix != "" {
		t.Fatalf("default Prefix = %q, want empty", b.Prefix)
	}
	if b.Ports != nil {
		t.Fatalf("default Ports non-nil")
	}
	if b.SwaggerOn {
		t.Fatalf("default SwaggerOn = true, want false")
	}
	if len(b.Mw) != 0 {
		t.Fatalf("default Mw length = %d, want 0", len(b.Mw))
	}

	// Subrouter default is identity; should return what it was given
	var r httpkit.Router
	if r2 := b.Subrouter(r); r2 != r {
		t.Fatalf("default Subrouter should be identity")
	}

	// Register default is no-op; ensure it doesn't panic
	defer func() {
		if v := recover(); v != nil {
			t.Fatalf("default Register panicked: %v", v)
		}
	}()
	b.Register(r)
}

func TestBuild_WithOptionsAndCopySemantics(t *testing.T) {
	t.Parallel()

	// helpers to compare funcs by pointer (program counter)
	fnPtr := func(f func(http.Handler) http.Handler) uintptr {
		return reflect.ValueOf(f).Pointer()
	}

	// identifiable middlewares
	mwA := func(next http.Handler) http.Handler { return next }
	mwB := func(next http.Handler) http.Handler { return next }
	mid := []func(http.Handler) http.Handler{mwA, mwB}

	// track that our hooks were invoked
	subCalled := 0
	regCalled := 0

	sub := func(in httpkit.Router) httpkit.Router {
		subCalled++
		return in
	}
	reg := func(in httpkit.Router) {
		regCalled++
	}

	type ports struct {
		X int
		Y string
	}
	p := ports{X: 7, Y: "ok"}

	// internal-only hook wiring via custom Option (same package)
	hooks := Option(func(c *buildCfg) {
		c.subrouter = sub
		c.register = reg
		c.swaggerOn = true
	})

	b := Build(
		WithName("auth"),
		WithPrefix("/api/v1/auth"),
		WithMiddlewares(mid...),
		WithPorts[ports](p),
		hooks,
	)

	// name/prefix/ports
	if b.Name != "auth" {
		t.Fatalf("Name = %q, want %q", b.Name, "auth")
	}
	if b.Prefix != "/api/v1/auth" {
		t.Fatalf("Prefix = %q, want %q", b.Prefix, "/api/v1/auth")
	}
	if got, ok := b.Ports.(ports); !ok || got != p {
		t.Fatalf("Ports mismatch after Build")
	}
	if !b.SwaggerOn {
		t.Fatalf("SwaggerOn = false, want true")
	}

	// middleware slice should be copied and ordered
	if len(b.Mw) != 2 {
		t.Fatalf("Mw length = %d, want 2", len(b.Mw))
	}
	if fnPtr(b.Mw[0]) != fnPtr(mwA) || fnPtr(b.Mw[1]) != fnPtr(mwB) {
		t.Fatalf("Mw contents not preserved")
	}

	// mutate the original slice after Build; Built.Mw must not change
	mwC := func(next http.Handler) http.Handler { return next }
	mid[0] = mwC

	if fnPtr(b.Mw[0]) != fnPtr(mwA) {
		t.Fatalf("Built.Mw changed after source slice mutation (index 0)")
	}
	if fnPtr(b.Mw[1]) != fnPtr(mwB) {
		t.Fatalf("Built.Mw changed after source slice mutation (index 1)")
	}

	// hooks are plumbed through
	var r httpkit.Router
	if out := b.Subrouter(r); out != r {
		t.Fatalf("Subrouter did not return input Router as expected")
	}
	if subCalled != 1 {
		t.Fatalf("Subrouter not invoked the expected number of times: %d", subCalled)
	}

	b.Register(r)
	if regCalled != 1 {
		t.Fatalf("Register not invoked the expected number of times: %d", regCalled)
	}
}
