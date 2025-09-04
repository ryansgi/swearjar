package httpkit

import (
	"net/http"
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

// fakeAuthPort satisfies middleware.AuthPort without hitting real auth
type fakeAuthPort struct{ calls int }

func (f *fakeAuthPort) Parse(*http.Request) (string, string, error) {
	f.calls++
	return "user-x", "ten-y", nil
}

func TestProtected_WiresAuthAndSecuredRoutes(t *testing.T) {
	t.Parallel()

	// Reuse the shared fakeRouter defined in routes_test.go
	root := &fakeRouter{}
	ap := &fakeAuthPort{}

	var h phttp.Handler = nil

	Protected(root, ap, func(gr Router) {
		// gr is a *securedRouter; calls should be forwarded to the underlying router
		gr.Get("/a", h)
		gr.Post("b", h)
		gr.Put("/v1/c", h)
		gr.Patch("v1/d", h)

		gr.Route("/api", func(rr Router) {
			rr.Delete("/x", h)
			rr.Head("y", h)
			rr.Options("/z", h)
			rr.Handle("/raw", http.NewServeMux())
		})
	})

	// Route nesting recorded
	if got, want := len(root.prefixes), 1; got != want {
		t.Fatalf("expected %d nested Route call, got %d (prefixes=%v)", want, got, root.prefixes)
	}
	if root.prefixes[0] != "/api" {
		t.Fatalf("expected nested prefix /api, got %q", root.prefixes[0])
	}

	// Verb registrations recorded (securedRouter marks swagger internally; we assert the forwarding)
	want := []struct {
		verb string
		path string
	}{
		{"GET", "/a"},
		{"POST", "b"},
		{"PUT", "/v1/c"},
		{"PATCH", "v1/d"}, // shared fakeRouter does not auto-prepend slash here
		{"DELETE", "/x"},
		{"HEAD", "y"},
		{"OPTIONS", "/z"},
		{"HANDLE", "/raw"}, // <- Handle shows up in verbCalls too
	}

	if len(root.verbCalls) != len(want) {
		t.Fatalf("expected %d verb calls, got %d: %#v", len(want), len(root.verbCalls), root.verbCalls)
	}
	for i, w := range want {
		if root.verbCalls[i].verb != w.verb {
			t.Fatalf("call %d verb mismatch: want %q, got %q", i, w.verb, root.verbCalls[i].verb)
		}
		if root.verbCalls[i].path != w.path {
			t.Fatalf("call %d path mismatch: want %q, got %q", i, w.path, root.verbCalls[i].path)
		}
	}
	// Ensure auth port isn't invoked during wiring (it runs at request time)
	if ap.calls != 0 {
		t.Fatalf("auth port Parse should not be called during route wiring, got %d", ap.calls)
	}
}

func TestSecuredRouter_JoinPath_Cases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		exp  string
	}{
		// a empty
		{"", "/x", "/x"},
		{"", "x", "/x"},
		// a ends with slash
		{"/a/", "/b", "/a/b"},
		{"/a/", "b", "/a/b"},
		// a no trailing slash, b with leading slash
		{"/a", "/b", "/a/b"},
		// neither have boundary slashes
		{"/a", "b", "/a/b"},
	}
	for i, c := range cases {
		if got := joinPath(c.a, c.b); got != c.exp {
			t.Fatalf("case %d joinPath(%q, %q): want %q, got %q", i, c.a, c.b, c.exp, got)
		}
	}
}
