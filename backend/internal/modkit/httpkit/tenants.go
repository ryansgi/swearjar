package httpkit

import (
	"net/http"

	pnet "swearjar/internal/platform/net"
)

// TenancyPort validates tenant context. Stub until we wire a real service.
type TenancyPort interface {
	Validate(r *http.Request, tenantID string) error
}

// Tenadncy is middleware that validates the tenant ID from context using the port
func Tenadncy(p TenancyPort, write func(w http.ResponseWriter, status int, body any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p == nil {
				next.ServeHTTP(w, r)
				return
			}
			tid := pnet.TenantID(r.Context())
			if err := p.Validate(r, tid); err != nil {
				status, body := pnet.Error(err, pnet.RequestID(r.Context()))
				write(w, status, body)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
