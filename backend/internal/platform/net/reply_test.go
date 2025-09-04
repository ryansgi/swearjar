package net_test

import (
	"net/http"
	"testing"

	perr "swearjar/internal/platform/errors"
	pnet "swearjar/internal/platform/net"
)

func TestOK(t *testing.T) {
	reqID := "req-1"
	data := map[string]any{"x": 1}

	status, w := pnet.OK(data, reqID)

	if status != http.StatusOK {
		t.Fatalf("status %d want %d", status, http.StatusOK)
	}
	if w.StatusCode != http.StatusOK || w.Status != http.StatusText(http.StatusOK) {
		t.Fatalf("wire status mismatch: %+v", w)
	}
	if w.RequestID != reqID {
		t.Fatalf("req id %q want %q", w.RequestID, reqID)
	}
	if got, ok := w.Data.(map[string]any)["x"]; !ok || got != 1 {
		t.Fatalf("data mismatch: %+v", w.Data)
	}
}

func TestCreated(t *testing.T) {
	reqID := "req-2"
	data := []int{1, 2, 3}

	status, w := pnet.Created(data, reqID)

	if status != http.StatusCreated {
		t.Fatalf("status %d want %d", status, http.StatusCreated)
	}
	if w.StatusCode != http.StatusCreated || w.Status != http.StatusText(http.StatusCreated) {
		t.Fatalf("wire status mismatch: %+v", w)
	}
	if w.RequestID != reqID {
		t.Fatalf("req id %q want %q", w.RequestID, reqID)
	}
	if got := w.Data.([]int); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("data mismatch: %+v", w.Data)
	}
}

func TestNoContent(t *testing.T) {
	reqID := "req-3"

	status, w := pnet.NoContent(reqID)

	if status != http.StatusNoContent {
		t.Fatalf("status %d want %d", status, http.StatusNoContent)
	}
	if w.StatusCode != http.StatusNoContent || w.Status != http.StatusText(http.StatusNoContent) {
		t.Fatalf("wire status mismatch: %+v", w)
	}
	if w.RequestID != reqID {
		t.Fatalf("req id %q want %q", w.RequestID, reqID)
	}
	if w.Data != nil || w.Error != "" {
		t.Fatalf("expected empty body fields, got data=%v error=%q", w.Data, w.Error)
	}
}

func TestError_NilFallsBackToOK(t *testing.T) {
	reqID := "req-4"

	status, w := pnet.Error(nil, reqID)

	if status != http.StatusOK {
		t.Fatalf("status %d want %d", status, http.StatusOK)
	}
	if w.StatusCode != http.StatusOK || w.Status != http.StatusText(http.StatusOK) {
		t.Fatalf("wire status mismatch: %+v", w)
	}
	if w.RequestID != reqID {
		t.Fatalf("req id %q want %q", w.RequestID, reqID)
	}
	if w.Error != "" || w.Code != 0 {
		t.Fatalf("expected no error/code, got error=%q code=%d", w.Error, w.Code)
	}
}

func TestError_ProjectErrorMapped(t *testing.T) {
	reqID := "req-5"
	// create a project error that perr maps to 401
	err := perr.New(perr.ErrorCodeUnauthorized, "not allowed")

	status, w := pnet.Error(err, reqID)

	if status != http.StatusUnauthorized {
		t.Fatalf("status %d want %d", status, http.StatusUnauthorized)
	}
	if w.StatusCode != http.StatusUnauthorized || w.Status != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("wire status mismatch: %+v", w)
	}
	if w.RequestID != reqID {
		t.Fatalf("req id %q want %q", w.RequestID, reqID)
	}
	if w.Code != perr.ErrorCodeUnauthorized {
		t.Fatalf("code %v want %v", w.Code, perr.ErrorCodeUnauthorized)
	}
	if w.Error == "" {
		t.Fatalf("expected error message to be set")
	}
	if w.Data != nil {
		t.Fatalf("expected data to be nil on error, got %v", w.Data)
	}
}
