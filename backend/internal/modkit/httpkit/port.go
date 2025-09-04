// Package httpkit provides tiny HTTP helpers and adapters
package httpkit

import (
	"net/http"
	"strings"

	perrs "swearjar/internal/platform/errors"
)

// TokenFunc parses a bearer token and returns userID and tenantID
// httpkit does not care about tenancy, callers may return an empty tenant id
type TokenFunc func(token string) (userID string, tenantID string, err error)

// Port implements middleware.AuthPort by reading Authorization and delegating to a TokenFunc
type Port struct {
	parse TokenFunc
}

// NewPortFunc builds a Port from a simple parser function
func NewPortFunc(fn TokenFunc) *Port {
	return &Port{parse: fn}
}

// Parse extracts user and tenant ids from Authorization Bearer token
// returns unauthorized when the header is missing, malformed, or the parser returns an error
// Parse extracts user and tenant ids from Authorization Bearer token
// returns unauthorized when the header is missing, malformed, or the parser returns an error
func (p *Port) Parse(r *http.Request) (string, string, error) {
	authz := r.Header.Get("Authorization")
	// normalize whitespace around the whole header
	s := strings.TrimSpace(authz)
	if s == "" {
		return "", "", perrs.Unauthorizedf("missing bearer token")
	}
	ls := strings.ToLower(s)
	const prefix = "bearer"
	if !strings.HasPrefix(ls, prefix) {
		return "", "", perrs.Unauthorizedf("missing bearer token")
	}
	// slice after "Bearer" (no trailing space required), then trim any spaces before token
	raw := strings.TrimSpace(s[len(prefix):])
	if raw == "" {
		return "", "", perrs.Unauthorizedf("missing bearer token")
	}

	if p.parse == nil {
		return "", "", perrs.Unauthorizedf("invalid bearer token")
	}

	uid, tid, err := p.parse(raw)
	if err != nil {
		return "", "", perrs.Unauthorizedf("invalid bearer token")
	}
	return uid, tid, nil
}
