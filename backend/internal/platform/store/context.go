package store

import "context"

type (
	tenantKey     struct{}
	reqIDKey      struct{}
	superadminKey struct{}
)

// WithTenant attaches a tenant id to the context
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// TenantID retrieves a tenant id from context if present
func TenantID(ctx context.Context) (string, bool) {
	v := ctx.Value(tenantKey{})
	if v == nil {
		return "", false
	}
	s, _ := v.(string)
	return s, s != ""
}

// WithSuperadmin marks the context to bypass RLS via app.superadmin set_config
func WithSuperadmin(ctx context.Context) context.Context {
	return context.WithValue(ctx, superadminKey{}, true)
}

// IsSuperadmin reports if the context has superadmin privileges
func IsSuperadmin(ctx context.Context) bool {
	v := ctx.Value(superadminKey{})
	b, _ := v.(bool)
	return b
}

// WithRequestID attaches a request id to the context
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDKey{}, id)
}

// RequestID retrieves a request id from context if present
func RequestID(ctx context.Context) (string, bool) {
	v := ctx.Value(reqIDKey{})
	if v == nil {
		return "", false
	}
	s, _ := v.(string)
	return s, s != ""
}
