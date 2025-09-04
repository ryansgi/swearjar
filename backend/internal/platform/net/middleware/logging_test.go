package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"swearjar/internal/platform/net/middleware"
)

func TestAccessLog_Basic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = io.WriteString(w, "ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()

	middleware.AccessLog(next).ServeHTTP(rr, req)

	if rr.Code != 201 {
		t.Fatalf("expected 201 got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("expected body ok got %q", rr.Body.String())
	}
}

func TestAccessLog_SlowThresholdDoesNotChangeResponse(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		_, _ = io.WriteString(w, "slow")
	})
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rr := httptest.NewRecorder()

	middleware.AccessLog(next).ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	if rr.Body.String() != "slow" {
		t.Fatalf("expected slow got %q", rr.Body.String())
	}
}
