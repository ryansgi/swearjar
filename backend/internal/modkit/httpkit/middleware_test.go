package httpkit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"swearjar/internal/platform/net/middleware"
)

func TestCommonStack_AppliesAllMiddleware(t *testing.T) {
	stack := CommonStack()
	if len(stack) == 0 {
		t.Fatalf("expected non-empty middleware stack")
	}

	// create a final handler that writes a marker header
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Final", "ok")
		w.WriteHeader(http.StatusTeapot)
	})

	var h http.Handler = final
	for _, mw := range stack {
		h = mw(h)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-Final") != "ok" {
		t.Errorf("expected final handler to run, headers=%v", rr.Header())
	}
}

func applyStack(h http.Handler, stack []func(http.Handler) http.Handler) http.Handler {
	for i := len(stack) - 1; i >= 0; i-- { // outermost first
		h = stack[i](h)
	}
	return h
}

func TestCommonStack_HealthEndpoint(t *testing.T) {
	stack := CommonStack()
	// no fallback handler needed; heartbeat should handle /health
	root := applyStack(http.NotFoundHandler(), stack)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected /health to be 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCommonStack_RequestReachesHandler(t *testing.T) {
	stack := CommonStack()

	hit := 0
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		w.WriteHeader(http.StatusNoContent)
	})
	root := applyStack(final, stack)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, req)

	if hit != 1 {
		t.Fatalf("expected final handler to be called once, got %d", hit)
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from final handler, got %d", rr.Code)
	}
}

func TestAuth_ReturnsMiddlewareAndWrapsHandler(t *testing.T) {
	var p middleware.AuthPort // nil is fine; we only check composition, not execution
	mw := Auth(p)
	if mw == nil {
		t.Fatalf("expected Auth to return a middleware function")
	}

	// wrapping should yield a non-nil http.Handler
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if h == nil {
		t.Fatalf("expected wrapped handler to be non-nil")
	}
}
