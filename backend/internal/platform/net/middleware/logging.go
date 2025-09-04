package middleware

import (
	"net/http"
	"time"

	"swearjar/internal/platform/logger"
)

// AccessLog logs request duration and status
func AccessLog(next http.Handler) http.Handler {
	type capture struct {
		http.ResponseWriter
		status int
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &capture{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(sw, r)

		elapsed := time.Since(start)
		log := logger.C(r.Context())
		evt := log.Info()
		if elapsed >= 500*time.Millisecond {
			evt = log.Warn()
		}
		evt.Int("status", sw.status).
			Dur("elapsed", elapsed).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("request done")
	})
}

type capture struct {
	http.ResponseWriter
	status int
}

func (c *capture) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}
