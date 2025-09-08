package github

import (
	"io"
	"net/http"
	"strconv"
	"time"
)

// GHStatusError wraps non-2xx HTTP responses from GitHub
type GHStatusError struct {
	Status int
	Body   string
	Err    error
}

// Error interface
func (e *GHStatusError) Error() string { return e.Err.Error() }

// Unwrap interface
func (e *GHStatusError) Unwrap() error { return e.Err }

// HTTPStatus interface
func (e *GHStatusError) HTTPStatus() int { return e.Status }

// tokenState
type tokenState struct {
	remaining int
	reset     time.Time
}

func parseRateHeaders(h http.Header) (remaining int, reset time.Time, retryAfter int) {
	remaining = atoi(h.Get("X-RateLimit-Remaining"))
	rs := h.Get("X-RateLimit-Reset")
	if rs != "" {
		sec := atoi(rs)
		if sec > 0 {
			reset = time.Unix(int64(sec), 0).UTC()
		}
	}
	retryAfter = atoi(h.Get("Retry-After"))
	return
}

// computeWait decides how long to wait based on headers
func computeWait(remaining int, reset time.Time, retryAfter int, now time.Time) time.Duration {
	if retryAfter > 0 {
		return time.Duration(retryAfter) * time.Second
	}
	if remaining <= 0 && !reset.IsZero() {
		if reset.After(now) {
			return reset.Sub(now)
		}
		return 0
	}
	return 0
}

// @TODO move to platform
func atoi(s string) int {
	if s == "" {
		return 0
	}
	i, _ := strconv.Atoi(s)
	return i
}

func drainAndClose(rc io.ReadCloser) error {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 512))
	return rc.Close()
}
