package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"swearjar/internal/platform/net/middleware"
)

func TestAccessLogZerolog_PassThroughStatusAndBody(t *testing.T) {
	mw := middleware.AccessLogZerolog(middleware.AccessLogOptions{}) // no slow marking

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()

	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("expected body ok got %q", rr.Body.String())
	}
}

func TestAccessLogZerolog_SlowMarkDoesNotAffectResponse(t *testing.T) {
	mw := middleware.AccessLogZerolog(middleware.AccessLogOptions{Slow: time.Nanosecond})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Microsecond)
		_, _ = io.WriteString(w, "slow")
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rr := httptest.NewRecorder()

	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rr.Code)
	}
	if rr.Body.String() != "slow" {
		t.Fatalf("expected body slow got %q", rr.Body.String())
	}
}

func TestAccessLogZerolog_WritesCountedBytes(t *testing.T) {
	mw := middleware.AccessLogZerolog(middleware.AccessLogOptions{})

	// write twice to ensure byte capture wraps Write
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
		_, _ = w.Write([]byte("there"))
	})

	req := httptest.NewRequest(http.MethodGet, "/bytes", nil)
	rr := httptest.NewRecorder()

	mw(next).ServeHTTP(rr, req)

	if rr.Body.String() != "hithere" {
		t.Fatalf("expected concatenated body got %q", rr.Body.String())
	}
}
