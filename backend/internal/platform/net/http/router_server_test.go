package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"swearjar/internal/platform/config"
	phttp "swearjar/internal/platform/net/http"
)

func TestNewServer_DefaultsAndMux(t *testing.T) {
	srv := phttp.NewServer(config.New()) // no env, should default to :4000
	if srv.Addr() == "" {
		t.Fatalf("expected non-empty addr")
	}
	r := srv.Router()
	if r == nil || r.Mux() == nil {
		t.Fatalf("router or mux is nil")
	}

	// simple route
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "pong")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	r.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "pong" {
		t.Fatalf("bad response: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRespondData_AliasForOK(t *testing.T) {
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/respond-data", "rid-data-classic")

	phttp.RespondData(rec, req, map[string]any{"k": "v"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var env phttp.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.StatusCode != http.StatusOK || env.RequestID != "rid-data-classic" {
		t.Fatalf("bad envelope: %+v", env)
	}
	// shallow check that data round-tripped
	m, ok := env.Data.(map[string]any)
	if !ok || m["k"] != "v" {
		t.Fatalf("expected data map with k=v, got %#v", env.Data)
	}
}
