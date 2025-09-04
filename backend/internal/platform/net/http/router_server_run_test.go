package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"swearjar/internal/platform/config"
	phttp "swearjar/internal/platform/net/http"

	"github.com/go-chi/chi/v5"
)

// TestServer_RunAndShutdown covers:
// - NewServer option hook (without adding routes there to avoid chi panic)
// - Router.Use (middleware) BEFORE routes
// - Router.Group
// - Router method adapters: Get/Post/Put/Patch/Delete
// - Run() + Shutdown() lifecycle and ErrServerClosed -> nil mapping
func TestServer_RunAndShutdown(t *testing.T) {
	// bind to an ephemeral local port to avoid collisions and permissions
	t.Setenv("API_PORT", "127.0.0.1:0")

	// option hook proves opts(...) are invoked; DO NOT add routes here
	optCalled := false
	srv := phttp.NewServer(config.New(), func(m *chi.Mux) {
		optCalled = true
	})
	if !optCalled {
		t.Fatalf("expected NewServer option to be called")
	}

	r := srv.Router()

	// middleware via Router.Use - must be defined BEFORE any routes
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-MW", "yes")
			next.ServeHTTP(w, req)
		})
	})

	// now add routes

	// group route using Router.Group
	r.Group(func(gr phttp.Router) {
		gr.Get("/group/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "pong") })
	})

	// method adapters on same path to cover Post/Put/Patch/Delete
	r.Post("/m", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	r.Put("/m", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	r.Patch("/m", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Delete("/m", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// a simple GET to assert middleware header
	r.Get("/mwcheck", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "x") })

	// start the server; it will listen on 127.0.0.1:0 (random port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Run(ctx)
	}()

	// give the listener a moment to come up
	time.Sleep(50 * time.Millisecond)

	// hit the mux directly via httptest to unit-test our router plumbing

	// group route
	recG := httptest.NewRecorder()
	reqG := httptest.NewRequest("GET", "/group/ping", nil)
	r.Mux().ServeHTTP(recG, reqG)
	if recG.Code != http.StatusOK || recG.Body.String() != "pong" {
		t.Fatalf("unexpected /group/ping: %d %q", recG.Code, recG.Body.String())
	}

	// middleware header check
	recMW := httptest.NewRecorder()
	reqMW := httptest.NewRequest("GET", "/mwcheck", nil)
	r.Mux().ServeHTTP(recMW, reqMW)
	if recMW.Header().Get("X-MW") != "yes" {
		t.Fatalf("middleware header missing")
	}

	// method adapters
	recPost := httptest.NewRecorder()
	r.Mux().ServeHTTP(recPost, httptest.NewRequest("POST", "/m", nil))
	if recPost.Code != http.StatusCreated {
		t.Fatalf("post adapter failed: %d", recPost.Code)
	}
	recPut := httptest.NewRecorder()
	r.Mux().ServeHTTP(recPut, httptest.NewRequest("PUT", "/m", nil))
	if recPut.Code != http.StatusAccepted {
		t.Fatalf("put adapter failed: %d", recPut.Code)
	}
	recPatch := httptest.NewRecorder()
	r.Mux().ServeHTTP(recPatch, httptest.NewRequest("PATCH", "/m", nil))
	if recPatch.Code != http.StatusNoContent {
		t.Fatalf("patch adapter failed: %d", recPatch.Code)
	}
	recDel := httptest.NewRecorder()
	r.Mux().ServeHTTP(recDel, httptest.NewRequest("DELETE", "/m", nil))
	if recDel.Code != http.StatusOK {
		t.Fatalf("delete adapter failed: %d", recDel.Code)
	}

	// exercise Addr() just for completeness
	if srv.Addr() == "" {
		t.Fatalf("Addr() should not be empty")
	}

	// graceful shutdown; Run() should return nil (ErrServerClosed mapped to nil)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return after Shutdown")
	}
}

// Guard against accidental reliance on outer env changing defaults
func TestNewServer_AddrFromEnv(t *testing.T) {
	// temporarily set and restore
	old := os.Getenv("API_PORT")
	defer func() {
		if err := os.Setenv("API_PORT", old); err != nil {
			t.Fatalf("failed to restore API_PORT: %v", err)
		}
	}()

	if err := os.Setenv("API_PORT", ":12345"); err != nil {
		t.Fatalf("failed to set API_PORT: %v", err)
	}
	srv := phttp.NewServer(config.New())
	if srv.Addr() != ":12345" {
		t.Fatalf("expected addr :12345, got %q", srv.Addr())
	}
}

func TestServer_Run_ReturnsListenError(t *testing.T) {
	t.Setenv("API_PORT", "127.0.0.1:abc") // invalid TCP port; net.Listen will fail
	srv := phttp.NewServer(config.New())

	err := srv.Run(context.Background())
	if err == nil {
		t.Fatalf("expected Run to return an error for invalid addr, got nil")
	}
	// no further assertion needed
}
