package middleware

import (
	"net/http"

	pnet "swearjar/internal/platform/net"
)

// AuthPort is a tiny seam the future auth service will implement
type AuthPort interface {
	// Parse returns a user id and tenant id from the request or an error
	Parse(r *http.Request) (userID string, tenantID string, err error)
}

// Auth is a no-op until wired. It uses the port when provided
func Auth(p AuthPort, write func(w http.ResponseWriter, status int, body any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p == nil {
				next.ServeHTTP(w, r)
				return
			}
			uid, tid, err := p.Parse(r)
			if err != nil {
				status, body := pnet.Error(err, pnet.RequestID(r.Context()))
				write(w, status, body)
				return
			}
			ctx := pnet.WithUser(r.Context(), uid)
			ctx = pnet.WithRequest(ctx, pnet.RequestID(ctx), tid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
