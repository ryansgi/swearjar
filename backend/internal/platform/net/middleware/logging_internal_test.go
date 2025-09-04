package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// exercises capture.WriteHeader directly
func TestCapture_WriteHeader_SetsStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	c := &capture{ResponseWriter: rr, status: http.StatusOK}

	c.WriteHeader(201)

	if c.status != 201 {
		t.Fatalf("expected status 201 got %d", c.status)
	}
	if rr.Code != 201 {
		t.Fatalf("expected recorder code 201 got %d", rr.Code)
	}
}
