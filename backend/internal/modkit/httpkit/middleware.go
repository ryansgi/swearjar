package httpkit

import (
	"compress/flate"
	"net/http"

	phttp "swearjar/internal/platform/net/http"
	"swearjar/internal/platform/net/middleware"
)

// CommonStack returns a baseline per module middleware slice
// compose with your auth or tenancy middleware as needed in main
func CommonStack() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		// tracing / correlation
		middleware.RequestID(),
		middleware.RealIP(),

		// safety
		middleware.RecoverJSON,

		// cache / freshness
		middleware.NoCache(),

		// observability
		middleware.Logger(),

		// cross-origin (tweak config in main if needed)
		middleware.CORS(middleware.CORSOptions{}),
		middleware.Compress(flate.BestSpeed),
		middleware.Heartbeat("/health"),
		middleware.RedirectSlashes(),
		middleware.StripSlashes(),
		middleware.Timeout(30 * 1e9), // 30s
	}
}

// Auth wires the auth middleware to the platform JSON writer
func Auth(p middleware.AuthPort) func(http.Handler) http.Handler {
	// middleware expects write func(w http.ResponseWriter, status int, body any)
	// use phttp.JSON which matches that signature
	return middleware.Auth(p, phttp.JSON)
}
