package httpkit

import (
	"context"
	"net/http"
	"testing"
)

// req helper
func newReq() *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "http://x.test/y", nil)
	return req
}

// anyValCtx returns a context that always yields a given value for any key
type anyValCtx struct {
	context.Context
	val any
}

func (c anyValCtx) Value(key any) any {
	return c.val
}

func TestUser_SuccessAndError(t *testing.T) {
	// success: force any ctx.Value(...) to return a non-empty user id
	{
		ctx := anyValCtx{Context: context.Background(), val: "u-123"}
		got, err := User(newReq().WithContext(ctx))
		if err != nil {
			t.Fatalf("User unexpected error: %v", err)
		}
		if got != "u-123" {
			t.Fatalf("User got %q want %q", got, "u-123")
		}
	}

	// error: empty/default context
	{
		_, err := User(newReq())
		if err == nil {
			t.Fatal("User expected error, got nil")
		}
		if got := err.Error(); got != "missing bearer token" {
			t.Fatalf("User error = %q want %q", got, "missing bearer token")
		}
	}
}

func TestTenant_SuccessAndError(t *testing.T) {
	// success: force any ctx.Value(...) to return a non-empty tenant id
	{
		ctx := anyValCtx{Context: context.Background(), val: "t-999"}
		got, err := Tenant(newReq().WithContext(ctx))
		if err != nil {
			t.Fatalf("Tenant unexpected error: %v", err)
		}
		if got != "t-999" {
			t.Fatalf("Tenant got %q want %q", got, "t-999")
		}
	}

	// error: empty/default context
	{
		_, err := Tenant(newReq())
		if err == nil {
			t.Fatal("Tenant expected error, got nil")
		}
		if got := err.Error(); got != "missing tenant scope" {
			t.Fatalf("Tenant error = %q want %q", got, "missing tenant scope")
		}
	}
}

func TestMustUser_SuccessAndPanic(t *testing.T) {
	// success
	{
		ctx := anyValCtx{Context: context.Background(), val: "ok-user"}
		if got := MustUser(newReq().WithContext(ctx)); got != "ok-user" {
			t.Fatalf("MustUser got %q want %q", got, "ok-user")
		}
	}
	// panic
	{
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("MustUser expected panic, got none")
			}
		}()
		_ = MustUser(newReq())
	}
}

func TestMustTenant_SuccessAndPanic(t *testing.T) {
	// success
	{
		ctx := anyValCtx{Context: context.Background(), val: "ok-tenant"}
		if got := MustTenant(newReq().WithContext(ctx)); got != "ok-tenant" {
			t.Fatalf("MustTenant got %q want %q", got, "ok-tenant")
		}
	}
	// panic
	{
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("MustTenant expected panic, got none")
			}
		}()
		_ = MustTenant(newReq())
	}
}

func TestJWT_SuccessVariants(t *testing.T) {
	cases := []struct {
		name string
		h    string
		want string
	}{
		{"canonical", "Bearer abc123", "abc123"},
		{"lowercase", "bearer xyz", "xyz"},
		{"weird-case", "BeArEr token", "token"},
		{"extra-spaces", "bearer     stuff", "stuff"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newReq()
			req.Header.Set("Authorization", tc.h)
			got, err := JWT(req)
			if err != nil {
				t.Fatalf("JWT unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("JWT got %q want %q", got, tc.want)
			}
		})
	}
}

func TestJWT_ErrorPaths(t *testing.T) {
	assertUnauthorized := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "missing bearer token" {
			t.Fatalf("error = %q want %q", err.Error(), "missing bearer token")
		}
	}

	// missing header
	{
		req := newReq()
		_, err := JWT(req)
		assertUnauthorized(t, err)
	}

	// wrong prefix
	{
		req := newReq()
		req.Header.Set("Authorization", "Token abc")
		_, err := JWT(req)
		assertUnauthorized(t, err)
	}

	// prefix only, no token (no space after word)
	{
		req := newReq()
		req.Header.Set("Authorization", "Bearer")
		_, err := JWT(req)
		assertUnauthorized(t, err)
	}

	// prefix + single space only (explicit raw == "")
	{
		req := newReq()
		req.Header.Set("Authorization", "Bearer ")
		_, err := JWT(req)
		assertUnauthorized(t, err)
	}

	// prefix + spaces only (still raw == "")
	{
		req := newReq()
		req.Header.Set("Authorization", "Bearer     ")
		_, err := JWT(req)
		assertUnauthorized(t, err)
	}
}

func TestMustJWT_SuccessAndPanic(t *testing.T) {
	// success
	{
		req := newReq()
		req.Header.Set("Authorization", "Bearer ok")
		if got := MustJWT(req); got != "ok" {
			t.Fatalf("MustJWT got %q want %q", got, "ok")
		}
	}
	// panic
	{
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic, got none")
			}
		}()
		_ = MustJWT(newReq())
	}
}
