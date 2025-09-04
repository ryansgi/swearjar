package middleware

import (
	stdjson "encoding/json"
	stdhttp "net/http"
	"runtime/debug"
	"strings"

	perr "swearjar/internal/platform/errors"
	"swearjar/internal/platform/logger"
	pnet "swearjar/internal/platform/net"
)

type panicWire struct {
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// RecoverJSON converts panics into a JSON 500 and logs stack with request id
func RecoverJSON(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		defer func() {
			if v := recover(); v != nil {
				reqID := pnet.RequestID(r.Context())

				// format stack like chi recover
				raw := debug.Stack()
				lines := strings.Split(string(raw), "\n")
				stack := strings.Join(lines, "\n\t")

				log := logger.C(r.Context())
				if log == nil {
					log = logger.Named("http")
				}
				log.Error().
					Str("request_id", reqID).
					Interface("panic", v).
					Msgf("panic recovered\n%s", stack)

				// mirror id in response header
				if reqID != "" {
					w.Header().Set("X-Request-ID", reqID)
				}

				body := panicWire{
					StatusCode: stdhttp.StatusInternalServerError,
					Status:     stdhttp.StatusText(stdhttp.StatusInternalServerError),
					Error:      perr.Root(perr.PanicErrf("panic recovered")).Error(),
					RequestID:  reqID,
				}

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(stdhttp.StatusInternalServerError)
				_ = stdjson.NewEncoder(w).Encode(body)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
