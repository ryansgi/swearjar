package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"swearjar/internal/platform/net"
	"swearjar/internal/platform/net/middleware"
)

type fakeAuthPort struct {
	user string
	ten  string
	err  error
}

func (f fakeAuthPort) Parse(r *http.Request) (string, string, error) {
	return f.user, f.ten, f.err
}

func writeStub(w http.ResponseWriter, status int, body any) {
	w.WriteHeader(status)
}

func TestAuth_NilPortPassesThrough(t *testing.T) {
	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(200)
	})

	mw := middleware.Auth(nil, writeStub)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatal("expected next to be called")
	}
	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
}

func TestAuth_ErrorFromPortWritesMappedError(t *testing.T) {
	p := fakeAuthPort{err: http.ErrNoCookie}
	mw := middleware.Auth(p, writeStub)

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if nextCalled {
		t.Fatal("did not expect next to be called on auth error")
	}
	// exact status is delegated to pnet.Error, which can vary
	// assert it is a 4xx or 5xx rather than a 2xx
	if rr.Code < 400 {
		t.Fatalf("expected error status got %d", rr.Code)
	}
}

func TestAuth_SetsTenantOnContext(t *testing.T) {
	p := fakeAuthPort{user: "u1", ten: "t1", err: nil}
	mw := middleware.Auth(p, writeStub)

	var seenTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTenant = net.TenantID(r.Context())
		w.WriteHeader(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	if seenTenant != "t1" {
		t.Fatalf("expected tenant t1 got %q", seenTenant)
	}
}
