package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"swearjar/internal/platform/config"
	phttp "swearjar/internal/platform/net/http"
)

func TestMountProfiler_Enabled(t *testing.T) {
	srv := phttp.NewServer(config.New())
	r := srv.Router()
	phttp.MountProfiler(r, "/debug", true)

	// Profiler is served under /pprof/ when mounted at a prefix
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	r.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 at /debug/pprof/, got %d", rec.Code)
	}

	// sanity: one sub-endpoint
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/debug/pprof/cmdline", nil)
	r.Mux().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 at /debug/pprof/cmdline, got %d", rec2.Code)
	}

	// also hit the exact prefix to cover r.Get(prefix, ...)
	rec0 := httptest.NewRecorder()
	req0 := httptest.NewRequest("GET", "/debug", nil)
	r.Mux().ServeHTTP(rec0, req0)

	// Profiler mux typically redirects the prefix root to /pprof/ (301/308),
	// depending on stdlib/chi behavior. Either redirect or 404 is fine here.
	if rec0.Code != http.StatusMovedPermanently &&
		rec0.Code != http.StatusPermanentRedirect &&
		rec0.Code != http.StatusNotFound {
		t.Fatalf("expected 301/308/404 at /debug (prefix root), got %d", rec0.Code)
	}
}

func TestMountProfiler_Disabled(t *testing.T) {
	srv := phttp.NewServer(config.New())
	r := srv.Router()
	phttp.MountProfiler(r, "/debug", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	r.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when disabled, got %d", rec.Code)
	}
}
