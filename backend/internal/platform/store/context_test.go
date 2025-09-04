package store

import (
	"context"
	"testing"
)

// TestTenantID_SetAndGet sets a tenant id and retrieves it
func TestTenantID_SetAndGet(t *testing.T) {
	t.Parallel()

	base := context.Background()
	ctx := WithTenant(base, "acme")

	id, ok := TenantID(ctx)
	if !ok {
		t.Fatalf("TenantID not found")
	}
	if id != "acme" {
		t.Fatalf("TenantID mismatch got=%q want=%q", id, "acme")
	}
}

// TestTenantID_EmptyString reports false when empty string is stored
func TestTenantID_EmptyString(t *testing.T) {
	t.Parallel()

	ctx := WithTenant(context.Background(), "")

	id, ok := TenantID(ctx)
	if ok {
		t.Fatalf("TenantID ok should be false for empty value")
	}
	if id != "" {
		t.Fatalf("TenantID should be empty got=%q", id)
	}
}

// TestTenantID_NotPresent returns false on base context
func TestTenantID_NotPresent(t *testing.T) {
	t.Parallel()

	id, ok := TenantID(context.Background())
	if ok || id != "" {
		t.Fatalf("TenantID should be absent on base context")
	}
}

// TestTenantID_NoLeak ensures adding value returns a new ctx and base has no value
func TestTenantID_NoLeak(t *testing.T) {
	t.Parallel()

	base := context.Background()
	_ = WithTenant(base, "acme")

	id, ok := TenantID(base)
	if ok || id != "" {
		t.Fatalf("base context should not have tenant value")
	}
}

// TestRequestID_SetAndGet sets a request id and retrieves it
func TestRequestID_SetAndGet(t *testing.T) {
	t.Parallel()

	base := context.Background()
	ctx := WithRequestID(base, "req-123")

	id, ok := RequestID(ctx)
	if !ok {
		t.Fatalf("RequestID not found")
	}
	if id != "req-123" {
		t.Fatalf("RequestID mismatch got=%q want=%q", id, "req-123")
	}
}

// TestRequestID_EmptyString reports false when empty string is stored
func TestRequestID_EmptyString(t *testing.T) {
	t.Parallel()

	ctx := WithRequestID(context.Background(), "")

	id, ok := RequestID(ctx)
	if ok {
		t.Fatalf("RequestID ok should be false for empty value")
	}
	if id != "" {
		t.Fatalf("RequestID should be empty got=%q", id)
	}
}

// TestRequestID_NotPresent returns false on base context
func TestRequestID_NotPresent(t *testing.T) {
	t.Parallel()

	id, ok := RequestID(context.Background())
	if ok || id != "" {
		t.Fatalf("RequestID should be absent on base context")
	}
}

// TestKeys_Isolation ensures tenant and request keys do not collide
func TestKeys_Isolation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = WithTenant(ctx, "acme")
	ctx = WithRequestID(ctx, "req-123")

	ten, tok := TenantID(ctx)
	req, rok := RequestID(ctx)

	if !tok || ten != "acme" {
		t.Fatalf("TenantID mismatch tok=%v ten=%q", tok, ten)
	}
	if !rok || req != "req-123" {
		t.Fatalf("RequestID mismatch rok=%v req=%q", rok, req)
	}
}
