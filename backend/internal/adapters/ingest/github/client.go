// Package github provides a resilient GitHub REST v3 client for hallmonitor
package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	perr "swearjar/internal/platform/errors"
	"swearjar/internal/platform/logger"
)

const (
	baseURLDefault   = "https://api.github.com"
	defaultTimeout   = 10 * time.Second
	defaultUA        = "swearjar-hallmonitor"
	defaultMaxRetry  = 5
	defaultRetryBase = 500 * time.Millisecond
)

// Options configures the Client
type Options struct {
	BaseURL   string
	UserAgent string
	Timeout   time.Duration

	// Comma separated tokens passed in from CLI or config
	// Empty means tokenless which is very low quota so not recommended
	TokensCSV string

	// Retry config for transient and rate limited responses
	MaxRetries int
	RetryBase  time.Duration
}

// Client is a minimal GitHub REST client with token rotation and ETag support
type Client struct {
	http   *http.Client
	opts   Options
	tokens []string
	cur    atomic.Int32
	log    logger.Logger
	now    func() time.Time
	sleep  func(time.Duration)
	state  []tokenState
}

// NewClient creates a new Client with sane defaults
func NewClient(o Options) *Client {
	if o.BaseURL == "" {
		o.BaseURL = baseURLDefault
	}
	if o.UserAgent == "" {
		o.UserAgent = defaultUA
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.MaxRetries <= 0 {
		o.MaxRetries = defaultMaxRetry
	}
	if o.RetryBase <= 0 {
		o.RetryBase = defaultRetryBase
	}
	var toks []string
	if s := strings.TrimSpace(o.TokensCSV); s != "" {
		for t := range strings.SplitSeq(s, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				toks = append(toks, t)
			}
		}
	}
	return &Client{
		http:   &http.Client{Timeout: o.Timeout},
		opts:   o,
		tokens: toks,
		state:  make([]tokenState, len(toks)),
		log:    *logger.Named("github"),
		now:    time.Now,
		sleep:  time.Sleep,
	}
}

// nextIndex returns the next round-robin index starting from current cursor.
func (c *Client) nextIndex() int {
	if len(c.tokens) == 0 {
		return -1
	}
	n := int(c.cur.Add(1))       // Add returns the NEW value
	i := (n - 1) % len(c.tokens) // so subtract 1 to start at 0
	if i < 0 {                   // paranoia
		i += len(c.tokens)
	}
	return i
}

// getToken chooses the next non-exhausted token if possible.
// Falls back to plain round-robin if all are exhausted
func (c *Client) getToken(now time.Time) (tok string, idx int) {
	n := len(c.tokens)
	if n == 0 {
		return "", -1
	}

	// Try up to N tokens to find one with quota or already reset.
	start := c.nextIndex()
	i := start
	for range n {
		st := c.state[i]
		if st.remaining > 0 || st.reset.IsZero() || !st.reset.After(now) {
			return c.tokens[i], i
		}
		// advance (simple wrap)
		i++
		if i == n {
			i = 0
		}
	}

	// All appear exhausted; use round-robin slot anyway (server will 403 and we'll sleep)
	return c.tokens[start], start
}

// Do issues a request with auth headers, etag, retries, and rate limit handling
// etagIn is optional and adds If-None-Match for conditional requests
func (c *Client) Do(ctx context.Context, method, path string, etagIn string) (*http.Response, error) {
	url := c.opts.BaseURL + path
	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, perr.Wrapf(err, perr.ErrorCodeUnknown, "github new request failed")
		}
		req.Header.Set("User-Agent", c.opts.UserAgent)
		req.Header.Set("Accept", "application/vnd.github+json")
		if etagIn != "" {
			req.Header.Set("If-None-Match", etagIn)
		}
		tok, tokIdx := c.getToken(c.now())
		if tok != "" {
			req.Header.Set("Authorization", "token "+tok)
		}

		start := c.now()
		resp, err := c.http.Do(req)
		lat := c.now().Sub(start)

		if err != nil {
			if !c.shouldRetry(attempts) {
				return nil, perr.Wrapf(err, perr.ErrorCodeUnavailable, "github do failed")
			}
			back := c.backoff(attempts)
			c.log.Warn().Dur("retry_in", back).Int("attempt", attempts).Msg("github transport error retrying")
			c.sleep(back)
			attempts++
			continue
		}

		// Always log lightweight response metadata
		rem, reset, retryAfter := parseRateHeaders(resp.Header)
		if tokIdx >= 0 && tokIdx < len(c.state) {
			// Only update if header was present; leave zero-values alone otherwise
			if rem >= 0 {
				c.state[tokIdx] = tokenState{remaining: rem, reset: reset}
			}
		}
		c.log.Debug().
			Str("method", method).
			Str("path", path).
			Int("status", resp.StatusCode).
			Int("attempt", attempts).
			Dur("latency", lat).
			Int("rate_remaining", rem).
			Time("rate_reset", reset).
			Int("retry_after_s", retryAfter).
			Msg("github http response")

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusAccepted:
			return resp, nil
		case http.StatusNotModified:
			return resp, nil

		case http.StatusTooManyRequests, http.StatusForbidden:
			// Respect Retry-After / X-RateLimit-Reset; when exhausted, return typed status error.
			wait := computeWait(rem, reset, retryAfter, c.now())
			if wait <= 0 {
				wait = c.backoff(attempts)
			}
			if !c.shouldRetry(attempts) {
				body := readSmall(resp.Body)
				_ = resp.Body.Close()
				return nil, &GHStatusError{
					Status: resp.StatusCode,
					Body:   body,
					Err:    perr.Newf(perr.ErrorCodeTooManyRequests, "github rate limited"),
				}
			}
			c.log.Warn().Dur("sleep", wait).Msg("github rate limited backing off")
			_ = drainAndClose(resp.Body)
			c.sleep(wait)
			attempts++
			continue

		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// transient server side
			if !c.shouldRetry(attempts) {
				body := readSmall(resp.Body)
				_ = resp.Body.Close()
				return nil, &GHStatusError{
					Status: resp.StatusCode,
					Body:   body,
					Err:    perr.Newf(perr.ErrorCodeUnavailable, "github transient server error"),
				}
			}
			back := c.backoff(attempts)
			c.log.Warn().Dur("retry_in", back).Int("attempt", attempts).Msg("github transient error retrying")
			_ = drainAndClose(resp.Body)
			c.sleep(back)
			attempts++
			continue

		default:
			// Non-2xx/3xx (404/410/451/etc.); return GHStatusError so worker can tombstone.
			body := readSmall(resp.Body)
			_ = resp.Body.Close()
			return nil, &GHStatusError{
				Status: resp.StatusCode,
				Body:   body,
				Err:    perr.Newf(mapPerrCode(resp.StatusCode), "github unexpected status %d", resp.StatusCode),
			}
		}
	}
}

// mapPerrCode maps HTTP status to platform error codes for easier policy handling.
func mapPerrCode(status int) perr.ErrorCode {
	switch status {
	case 404:
		return perr.ErrorCodeNotFound
	case 410:
		return perr.ErrorCodeGone // new code in platform/errors
	case 451:
		return perr.ErrorCodeLegal // new code in platform/errors
	case 401:
		return perr.ErrorCodeUnauthorized
	case 403:
		return perr.ErrorCodeForbidden
	case 429:
		return perr.ErrorCodeTooManyRequests
	case 500, 502, 503, 504:
		return perr.ErrorCodeUnavailable
	default:
		return perr.ErrorCodeUnknown
	}
}

// readSmall reads a small tail for diagnostics (kept separate from drainAndClose for reuse).
func readSmall(rc io.ReadCloser) string {
	b, _ := io.ReadAll(io.LimitReader(rc, 2048))
	// avoid logging newlines/control chars in single-line logs
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func (c *Client) backoff(attempt int) time.Duration {
	d := c.opts.RetryBase
	// simple exponential with cap
	ms := int64(d / time.Millisecond)
	ms = ms << uint(attempt)
	max := int64(30 * time.Second / time.Millisecond)
	if ms > max {
		ms = max
	}
	return time.Duration(ms) * time.Millisecond
}

func (c *Client) shouldRetry(attempt int) bool {
	return attempt < c.opts.MaxRetries
}
