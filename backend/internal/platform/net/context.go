// Package net provides utilities for working with request contexts
package net

import (
	"context"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ctxKey is an unexported key type for context values
type ctxKey string

const (
	keyTenantID ctxKey = "tenant_id"
	keyUserID   ctxKey = "user_id"
)

// WithRequest annotates context with common request scoped ids
func WithRequest(ctx context.Context, reqID, tenantID string) context.Context {
	if reqID != "" {
		// set chi RequestID so chimw.GetReqID can retrieve it
		ctx = context.WithValue(ctx, chimw.RequestIDKey, reqID)
	}
	if tenantID != "" {
		ctx = context.WithValue(ctx, keyTenantID, tenantID)
	}
	return ctx
}

// WithUser annotates context with the authenticated user id
func WithUser(ctx context.Context, userID string) context.Context {
	if userID != "" {
		ctx = context.WithValue(ctx, keyUserID, userID)
	}
	return ctx
}

// RequestID returns the request id on the context if present
func RequestID(ctx context.Context) string {
	if v := chimw.GetReqID(ctx); v != "" {
		return v
	}
	return ""
}

// TenantID returns the tenant id on the context if present
func TenantID(ctx context.Context) string {
	if v, ok := ctx.Value(keyTenantID).(string); ok {
		return v
	}
	return ""
}

// UserID returns the user id on the context if present
func UserID(ctx context.Context) string {
	if v, ok := ctx.Value(keyUserID).(string); ok {
		return v
	}
	return ""
}
