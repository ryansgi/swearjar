package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type inDTO struct {
	N int `json:"n"`
}

func TestJSONHandler_Success(t *testing.T) {
	t.Parallel()

	// doubles the input
	h := JSONHandler[inDTO](func(_ *http.Request, in inDTO) (any, error) {
		return map[string]int{"doubled": in.N * 2}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"n":7}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"doubled":14`) {
		t.Fatalf("body %q missing doubled result", body)
	}
}

func TestJSONHandler_BindError(t *testing.T) {
	t.Parallel()

	h := JSONHandler[inDTO](func(_ *http.Request, _ inDTO) (any, error) {
		t.Fatal("handler should not be called on bind error")
		return nil, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{`)) // invalid JSON
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 on bind error, got %d", rr.Code)
	}
	if !strings.Contains(strings.ToLower(rr.Body.String()), "error") {
		t.Fatalf("expected error text in body, got %q", rr.Body.String())
	}
}

func TestJSONHandler_HandlerError(t *testing.T) {
	t.Parallel()

	h := JSONHandler[inDTO](func(_ *http.Request, _ inDTO) (any, error) {
		return nil, errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"n":1}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 on handler error, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("expected error message in body, got %q", rr.Body.String())
	}
}
