package middleware_test

import (
	"compress/flate"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"swearjar/internal/platform/net/middleware"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestWrappers_ReturnHandlers(t *testing.T) {
	if middleware.RequestID() == nil ||
		middleware.RealIP() == nil ||
		middleware.Recover() == nil ||
		middleware.Logger() == nil ||
		middleware.Timeout(time.Second) == nil ||
		middleware.NoCache() == nil ||
		middleware.RedirectSlashes() == nil ||
		middleware.StripSlashes() == nil ||
		middleware.AllowContentType("application/json") == nil ||
		middleware.SetHeader("X", "Y") == nil ||
		middleware.ContentCharset("utf-8") == nil ||
		middleware.Throttle(10) == nil ||
		middleware.ThrottleBacklog(10, 10, time.Second) == nil ||
		middleware.Heartbeat("/healthz") == nil {
		t.Fatal("expected non nil handlers from wrappers")
	}
}

func TestCompress_DeflateWhenAccepted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// write a body big enough to trigger compression
		_, _ = io.WriteString(w, strings.Repeat("a", 4<<10)) // 4 KB
	})

	mw := middleware.Compress(flate.DefaultCompression)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip") // chi prefers gzip
	rr := httptest.NewRecorder()

	mw(h).ServeHTTP(rr, req)

	enc := rr.Result().Header.Get("Content-Encoding")
	if enc == "" {
		t.Fatalf("expected Content-Encoding to be set (e.g., gzip)")
	}
}

func TestCORS_DefaultsFillMissing(t *testing.T) {
	cors := middleware.CORS(middleware.CORSOptions{
		AllowedOrigins: []string{"https://example.com"},
		// leave other fields empty to exercise defaults
	})

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	// ask for a header so the lib returns Access-Control-Allow-Headers
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	rr := httptest.NewRecorder()
	cors(h).ServeHTTP(rr, req)

	if rr.Code != 200 && rr.Code != 204 {
		t.Fatalf("expected 200 or 204 got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected Access-Control-Allow-Methods to be set")
	}
	if rr.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("expected Access-Control-Allow-Headers to be set")
	}
}

func TestDefaults_BundleRuns(t *testing.T) {
	chain := middleware.Defaults()
	if len(chain) == 0 {
		t.Fatal("expected some defaults")
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RequestID should be present in context
		if rid := chimw.GetReqID(r.Context()); rid == "" {
			t.Fatalf("expected request id in context from RequestID middleware")
		}

		// RealIP may rewrite RemoteAddr to bare IP; accept either ip or host:port
		if r.RemoteAddr == "" {
			t.Fatalf("expected RemoteAddr to be set, got empty")
		}
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err != nil || host == "" {
			if ip := net.ParseIP(r.RemoteAddr); ip == nil {
				t.Fatalf("expected RemoteAddr ip or host:port, got %q", r.RemoteAddr)
			}
		}

		w.WriteHeader(200)
	})

	// apply chain in order: first element is outermost
	var wrapped http.Handler = h
	for i := len(chain) - 1; i >= 0; i-- {
		wrapped = chain[i](wrapped)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	// NoCache should add cache control headers
	if rr.Header().Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control to be set by NoCache")
	}
}
