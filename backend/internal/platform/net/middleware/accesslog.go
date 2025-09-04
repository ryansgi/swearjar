// Package middleware holds adapters and in house middlewares
package middleware

import (
	"net/http"
	"time"

	"swearjar/internal/platform/logger"
)

// AccessLogOptions configures the zerolog access log
type AccessLogOptions struct {
	// Slow marks requests taking >= Slow as warn level, 0 disables slow marking
	Slow time.Duration
}

// captureWriter wraps the original ResponseWriter and records status & bytes
type captureWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (cw *captureWriter) WriteHeader(code int) {
	cw.status = code
	cw.ResponseWriter.WriteHeader(code)
}

func (cw *captureWriter) Write(b []byte) (int, error) {
	n, err := cw.ResponseWriter.Write(b)
	if n > 0 {
		cw.bytes += n
	}
	return n, err
}

// AccessLogZerolog logs method, path, status, elapsed, and bytes written
// uses the request scoped logger from our logger package
func AccessLogZerolog(opt AccessLogOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(cw, r)

			elapsed := time.Since(start)
			log := logger.C(r.Context())
			evt := log.Info()
			if opt.Slow > 0 && elapsed >= opt.Slow {
				evt = log.Warn()
			}
			evt.Int("status", cw.status).
				Dur("elapsed", elapsed).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("bytes", cw.bytes).
				Msg("request done")
		})
	}
}
