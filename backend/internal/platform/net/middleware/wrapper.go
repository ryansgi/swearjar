// Package middleware provides thin adapters over chi middleware without leaking chi types
package middleware

import (
	"compress/flate"
	"net/http"
	"time"

	pstrings "swearjar/internal/platform/strings"

	chimw "github.com/go-chi/chi/v5/middleware"
	chicors "github.com/go-chi/cors"
)

// RequestID attaches or propagates X-Request-ID and stores it on context
func RequestID() func(http.Handler) http.Handler { return chimw.RequestID }

// RealIP sets RemoteAddr to the upstream IP based on X-Forwarded-For headers
func RealIP() func(http.Handler) http.Handler { return chimw.RealIP }

// Recover catches panics and returns 500. Pair with your own error mapping in a wrapper if needed
func Recover() func(http.Handler) http.Handler { return chimw.Recoverer }

// Logger is chi's text logger
func Logger() func(http.Handler) http.Handler { return chimw.Logger }

// Timeout cancels the request context after d
func Timeout(d time.Duration) func(http.Handler) http.Handler { return chimw.Timeout(d) }

// NoCache sets headers to disable client and proxy caching
func NoCache() func(http.Handler) http.Handler { return chimw.NoCache }

// Compress wraps chi's compressor. level usually flate.DefaultCompression or flate.BestSpeed
func Compress(level int) func(http.Handler) http.Handler {
	c := chimw.NewCompressor(level)
	return func(next http.Handler) http.Handler { return c.Handler(next) }
}

// RedirectSlashes redirects /foo/ to /foo
func RedirectSlashes() func(http.Handler) http.Handler { return chimw.RedirectSlashes }

// StripSlashes strips a trailing slash from the request path
func StripSlashes() func(http.Handler) http.Handler { return chimw.StripSlashes }

// AllowContentType whitelists allowed content types
func AllowContentType(ct ...string) func(http.Handler) http.Handler {
	return chimw.AllowContentType(ct...)
}

// SetHeader sets a header on all responses
func SetHeader(name, value string) func(http.Handler) http.Handler {
	return chimw.SetHeader(name, value)
}

// ContentCharset ensures a request Content-Type charset
func ContentCharset(charsets ...string) func(http.Handler) http.Handler {
	return chimw.ContentCharset(charsets...)
}

// Throttle limits concurrent requests globally
func Throttle(limit int) func(http.Handler) http.Handler { return chimw.Throttle(limit) }

// ThrottleBacklog limits concurrency with a backlog and wait timeout
func ThrottleBacklog(limit, backlog int, ttl time.Duration) func(http.Handler) http.Handler {
	return chimw.ThrottleBacklog(limit, backlog, ttl)
}

// Heartbeat replies with 200 OK to GET path, useful for LB health checks
func Heartbeat(path string) func(http.Handler) http.Handler { return chimw.Heartbeat(path) }

// CORSOptions is a narrow surface over go-chi/cors
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// CORS wraps go-chi/cors with sane defaults applied
func CORS(o CORSOptions) func(http.Handler) http.Handler {
	return chicors.Handler(chicors.Options{
		AllowedOrigins: o.AllowedOrigins,
		AllowedMethods: pstrings.IfEmpty(o.AllowedMethods, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		AllowedHeaders: pstrings.IfEmpty(
			o.AllowedHeaders,
			[]string{
				"Accept",
				"Authorization",
				"Content-Type",
				"X-Request-ID",
			},
		),
		ExposedHeaders:   o.ExposedHeaders,
		AllowCredentials: o.AllowCredentials,
		MaxAge:           o.MaxAge,
	})
}

// Defaults is a convenience bundle for common web api needs
func Defaults() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		RealIP(),
		RequestID(),
		Recover(),
		Timeout(60 * time.Second),
		Compress(flate.DefaultCompression),
		NoCache(),
	}
}
