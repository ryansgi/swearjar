package httpkit

import (
	"net/http"
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

type fakeRouter struct {
	prefixes  []string
	useCalls  int
	lastMWLen int

	verbCalls []struct {
		verb string
		path string
		ph   phttp.Handler
		h    http.Handler
	}
}

func (f *fakeRouter) Mux() http.Handler {
	return http.NewServeMux()
}

func (f *fakeRouter) Route(prefix string, fn func(Router)) {
	f.prefixes = append(f.prefixes, prefix)
	fn(f)
}

func (f *fakeRouter) Group(fn func(Router)) {
	fn(f)
}

func (f *fakeRouter) Use(mw ...func(http.Handler) http.Handler) {
	f.useCalls++
	f.lastMWLen = len(mw)
}

// stdlib handler version
func (f *fakeRouter) Handle(path string, h http.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb string
		path string
		ph   phttp.Handler
		h    http.Handler
	}{"HANDLE", path, nil, h})
}

// phttp handler verbs
func (f *fakeRouter) Get(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"GET", path, h, nil})
}

func (f *fakeRouter) Post(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"POST", path, h, nil})
}

func (f *fakeRouter) Put(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"PUT", path, h, nil})
}

func (f *fakeRouter) Patch(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"PATCH", path, h, nil})
}

func (f *fakeRouter) Delete(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"DELETE", path, h, nil})
}

func (f *fakeRouter) Options(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"OPTIONS", path, h, nil})
}

func (f *fakeRouter) Head(path string, h phttp.Handler) {
	f.verbCalls = append(f.verbCalls, struct {
		verb, path string
		ph         phttp.Handler
		h          http.Handler
	}{"HEAD", path, h, nil})
}

func TestMountUnder_AppliesMiddleware_And_CallsMount(t *testing.T) {
	root := &fakeRouter{}

	// two simple no-op middlewares (stdlib signature)
	mwA := func(next http.Handler) http.Handler { return next }
	mwB := func(next http.Handler) http.Handler { return next }

	MountUnder(root, "/api/v1", []func(http.Handler) http.Handler{mwA, mwB}, func(sub Router) {
		// register a platform handler on the subrouter
		sub.Get("/ping", phttp.Handle(func(r *http.Request) phttp.Response {
			return phttp.NoContent()
		}))
	})

	// prefix routed once
	if len(root.prefixes) != 1 || root.prefixes[0] != "/api/v1" {
		t.Fatalf("expected Route to be called with /api/v1, got %v", root.prefixes)
	}

	// middleware applied once to the subrouter
	if root.useCalls != 1 || root.lastMWLen != 2 {
		t.Fatalf("expected Use once with 2 middleware, got calls=%d len=%d", root.useCalls, root.lastMWLen)
	}

	// route registered under the subrouter
	if len(root.verbCalls) == 0 {
		t.Fatalf("expected at least one route to be registered in mount closure")
	}
	first := root.verbCalls[0]
	if first.verb != "GET" || first.path != "/ping" || first.ph == nil {
		t.Fatalf("expected GET /ping with non-nil phttp handler, got verb=%s path=%s ph=%v",
			first.verb, first.path, first.ph,
		)
	}
}

func TestMountUnder_NoMiddleware_SkipsUse(t *testing.T) {
	root := &fakeRouter{}

	MountUnder(root, "/x", nil, func(sub Router) {
		sub.Delete("/gone", phttp.Handle(func(r *http.Request) phttp.Response {
			return phttp.NoContent()
		}))
	})

	if root.useCalls != 0 {
		t.Fatalf("expected Use to not be called when mw is empty, got %d", root.useCalls)
	}

	if len(root.prefixes) != 1 || root.prefixes[0] != "/x" {
		t.Fatalf("expected Route to be called with /x, got %v", root.prefixes)
	}

	if len(root.verbCalls) != 1 ||
		root.verbCalls[0].verb != "DELETE" || root.verbCalls[0].path != "/gone" || root.verbCalls[0].ph == nil {
		t.Fatalf("expected DELETE /gone registration with phttp handler, got %+v", root.verbCalls)
	}
}
