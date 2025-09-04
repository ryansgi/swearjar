package net_test

import (
	"context"
	"testing"

	pnet "swearjar/internal/platform/net"
)

func TestWithRequest_And_Getters(t *testing.T) {
	base := context.Background()

	t.Run("sets both ids", func(t *testing.T) {
		ctx := pnet.WithRequest(base, "req-123", "ten-abc")

		if got := pnet.RequestID(ctx); got != "req-123" {
			t.Fatalf("RequestID got %q want %q", got, "req-123")
		}
		if got := pnet.TenantID(ctx); got != "ten-abc" {
			t.Fatalf("TenantID got %q want %q", got, "ten-abc")
		}
	})

	t.Run("sets only request id", func(t *testing.T) {
		ctx := pnet.WithRequest(base, "r-only", "")

		if got := pnet.RequestID(ctx); got != "r-only" {
			t.Fatalf("RequestID got %q want %q", got, "r-only")
		}
		if got := pnet.TenantID(ctx); got != "" {
			t.Fatalf("TenantID got %q want empty", got)
		}
	})

	t.Run("sets only tenant id", func(t *testing.T) {
		ctx := pnet.WithRequest(base, "", "t-only")

		if got := pnet.RequestID(ctx); got != "" {
			t.Fatalf("RequestID got %q want empty", got)
		}
		if got := pnet.TenantID(ctx); got != "t-only" {
			t.Fatalf("TenantID got %q want %q", got, "t-only")
		}
	})

	t.Run("no ids returns same ctx and empty getters", func(t *testing.T) {
		ctx := pnet.WithRequest(base, "", "")

		// should be the same reference since nothing was set
		if ctx != base {
			t.Fatalf("expected ctx to be unchanged when both ids empty")
		}
		if got := pnet.RequestID(ctx); got != "" {
			t.Fatalf("RequestID got %q want empty", got)
		}
		if got := pnet.TenantID(ctx); got != "" {
			t.Fatalf("TenantID got %q want empty", got)
		}
	})
}
