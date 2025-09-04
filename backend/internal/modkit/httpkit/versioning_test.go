package httpkit

import (
	"net/http"
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

type fakeRouterVersioning struct {
	prefixes  []string
	useCalls  int
	lastMWLen int
	mountHits int

	verbCalls []struct {
		verb string
		path string
		ph   phttp.Handler
		h    http.Handler
	}
}

func (f *fakeRouterVersioning) Route(prefix string, fn func(Router)) {
	f.prefixes = append(f.prefixes, prefix)
	fn(f) // pass itself as subrouter
}

func (f *fakeRouterVersioning) Group(fn func(Router)) { fn(f) }
func (f *fakeRouterVersioning) Use(mw ...func(http.Handler) http.Handler) {
	f.useCalls++
	f.lastMWLen = len(mw)
}

// stdlib handler variant
func (f *fakeRouterVersioning) Handle(path string, h http.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb string
		path string
		ph   phttp.Handler
		h    http.Handler
	}{"HANDLE", path, nil, h})
}

// required to satisfy the interface, not exercised here
func (f *fakeRouterVersioning) Get(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"GET", path, h, nil})
}

func (f *fakeRouterVersioning) Post(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"POST", path, h, nil})
}

func (f *fakeRouterVersioning) Put(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"PUT", path, h, nil})
}

func (f *fakeRouterVersioning) Patch(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"PATCH", path, h, nil})
}

func (f *fakeRouterVersioning) Delete(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"DELETE", path, h, nil})
}

func (f *fakeRouterVersioning) Options(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"OPTIONS", path, h, nil})
}

func (f *fakeRouterVersioning) Head(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"HEAD", path, h, nil})
}

func (f *fakeRouterVersioning) Mux() http.Handler { return http.NewServeMux() }

func TestMountAPI_MountsPrefixAndAppliesMiddleware(t *testing.T) {
	r := &fakeRouterVersioning{}

	mwA := func(next http.Handler) http.Handler { return next }
	mwB := func(next http.Handler) http.Handler { return next }

	MountAPI(r, "v2", []func(http.Handler) http.Handler{mwA, mwB}, func(api Router) {
		r.mountHits++
	})

	if got, want := len(r.prefixes), 1; got != want {
		t.Fatalf("expected 1 Route call, got %d", got)
	}
	if got, want := r.prefixes[0], "/api/v2"; got != want {
		t.Fatalf("expected prefix %q, got %q", want, got)
	}
	if r.useCalls != 1 || r.lastMWLen != 2 {
		t.Fatalf("expected Use once with 2 middleware, got calls=%d len=%d", r.useCalls, r.lastMWLen)
	}
	if r.mountHits != 1 {
		t.Fatalf("expected mount closure to be invoked once, got %d", r.mountHits)
	}
}

func TestMountAPI_TrimsLeadingSlashOnVersion(t *testing.T) {
	r := &fakeRouterVersioning{}
	MountAPI(r, "/v3", nil, func(api Router) { r.mountHits++ })

	if got, want := r.prefixes[0], "/api/v3"; got != want {
		t.Fatalf("expected prefix %q, got %q", want, got)
	}
	// no middleware provided
	if r.useCalls != 0 {
		t.Fatalf("expected Use not called for empty middleware, got %d", r.useCalls)
	}
	if r.mountHits != 1 {
		t.Fatalf("expected mount closure to be invoked once, got %d", r.mountHits)
	}
}

func TestMountAPIV1_Convenience(t *testing.T) {
	r := &fakeRouterVersioning{}
	mw := func(next http.Handler) http.Handler { return next }

	MountAPIV1(r, []func(http.Handler) http.Handler{mw}, func(api Router) { r.mountHits++ })

	if got, want := r.prefixes[0], "/api/v1"; got != want {
		t.Fatalf("expected prefix %q, got %q", want, got)
	}
	if r.useCalls != 1 || r.lastMWLen != 1 {
		t.Fatalf("expected Use once with 1 middleware, got calls=%d len=%d", r.useCalls, r.lastMWLen)
	}
	if r.mountHits != 1 {
		t.Fatalf("expected mount closure to be invoked once, got %d", r.mountHits)
	}
}
