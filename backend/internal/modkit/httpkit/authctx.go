package httpkit

import (
	"net/http"
	"strings"

	perrs "swearjar/internal/platform/errors"
	pnet "swearjar/internal/platform/net"
)

// User returns the authenticated user id from the request context
func User(r *http.Request) (string, error) {
	uid := pnet.UserID(r.Context())
	if uid == "" {
		return "", perrs.Unauthorizedf("missing bearer token")
	}
	return uid, nil
}

// Tenant returns the authenticated tenant id from the request context
func Tenant(r *http.Request) (string, error) {
	tid := pnet.TenantID(r.Context())
	if tid == "" {
		return "", perrs.Unauthorizedf("missing tenant scope")
	}
	return tid, nil
}

// MustUser returns the authenticated user id or panics
func MustUser(r *http.Request) string {
	uid, err := User(r)
	if err != nil {
		panic(err)
	}
	return uid
}

// MustTenant returns the authenticated tenant id or panics
func MustTenant(r *http.Request) string {
	tid, err := Tenant(r)
	if err != nil {
		panic(err)
	}
	return tid
}

// JWT returns the raw bearer token from the Authorization header
func JWT(r *http.Request) (string, error) {
	authz := r.Header.Get("Authorization")
	if strings.TrimSpace(authz) == "" {
		return "", perrs.Unauthorizedf("missing bearer token")
	}
	// case-insensitive Bearer prefix (don't trim the whole header first)
	const prefix = "bearer "
	if len(authz) < len(prefix) || strings.ToLower(authz[:len(prefix)]) != prefix {
		return "", perrs.Unauthorizedf("missing bearer token")
	}
	raw := strings.TrimSpace(authz[len(prefix):])
	if raw == "" {
		return "", perrs.Unauthorizedf("missing bearer token")
	}
	return raw, nil
}

// MustJWT returns the raw bearer token or panics
// only use on routes protected by the auth middleware
func MustJWT(r *http.Request) string {
	raw, err := JWT(r)
	if err != nil {
		panic(err)
	}
	return raw
}
