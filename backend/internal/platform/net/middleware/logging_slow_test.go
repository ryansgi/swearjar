package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"swearjar/internal/platform/net/middleware"
)

func TestAccessLog_WarnsOnSlowRequests(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(550 * time.Millisecond)
		w.WriteHeader(204)
	})

	req := httptest.NewRequest(http.MethodGet, "/slow-warn", nil)
	rr := httptest.NewRecorder()

	middleware.AccessLog(next).ServeHTTP(rr, req)

	if rr.Code != 204 {
		t.Fatalf("expected 204 got %d", rr.Code)
	}
}
