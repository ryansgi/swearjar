package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type dto struct {
	N int `json:"n"`
}

func TestSugar_JSONVerbs(t *testing.T) {
	t.Parallel()

	m := chi.NewRouter()
	r := AdaptChi(m)

	// GET: accept body {}, ignore parsed input
	GetJSON(r, "/g", func(_ *http.Request) (any, error) {
		return map[string]string{"ok": "get"}, nil
	})

	// POST: double n
	PostJSON[dto](r, "/p", func(_ *http.Request, in dto) (any, error) {
		return map[string]int{"d": in.N * 2}, nil
	})

	// PUT: triple n
	PutJSON[dto](r, "/u", func(_ *http.Request, in dto) (any, error) {
		return map[string]int{"t": in.N * 3}, nil
	})

	// PATCH: echo n
	PatchJSON[dto](r, "/x", func(_ *http.Request, in dto) (any, error) {
		return map[string]int{"n": in.N}, nil
	})

	srv := httptest.NewServer(r.Mux())
	defer srv.Close()

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rr := httptest.NewRecorder()
		r.Mux().ServeHTTP(rr, req)
		return rr
	}

	// GET
	rr := do(http.MethodGet, "/g", `{}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok":"get"`) {
		t.Fatalf("GET /g => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// POST
	rr = do(http.MethodPost, "/p", `{"n":7}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"d":14`) {
		t.Fatalf("POST /p => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// PUT
	rr = do(http.MethodPut, "/u", `{"n":5}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"t":15`) {
		t.Fatalf("PUT /u => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// PATCH
	rr = do(http.MethodPatch, "/x", `{"n":9}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"n":9`) {
		t.Fatalf("PATCH /x => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// also verify bind error propagates via sugar+JSONHandler (bad JSON on POST)
	rr = do(http.MethodPost, "/p", `{`)
	if rr.Code == http.StatusOK {
		t.Fatalf("POST /p with bad json should not be 200; got %d", rr.Code)
	}
}
