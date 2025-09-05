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
		log:    *logger.Named("github"),
		now:    time.Now,
		sleep:  time.Sleep,
	}
}

// getToken returns the next token in a round robin rotation
func (c *Client) getToken() string {
	n := int(c.cur.Add(1))
	if len(c.tokens) == 0 {
		return ""
	}
	return c.tokens[n%len(c.tokens)]
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
		if tok := c.getToken(); tok != "" {
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
			// Respect Retry-After and X-RateLimit-Reset when present
			wait := computeWait(rem, reset, retryAfter, c.now())
			if wait <= 0 {
				wait = c.backoff(attempts)
			}
			if !c.shouldRetry(attempts) {
				_ = drainAndClose(resp.Body)
				return nil, perr.Newf(perr.ErrorCodeTooManyRequests, "github rate limited")
			}
			c.log.Warn().Dur("sleep", wait).Msg("github rate limited backing off")
			_ = drainAndClose(resp.Body)
			c.sleep(wait)
			attempts++
			continue
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// transient server side
			if !c.shouldRetry(attempts) {
				_ = drainAndClose(resp.Body)
				return nil, perr.Newf(perr.ErrorCodeUnavailable, "github transient server error")
			}
			back := c.backoff(attempts)
			c.log.Warn().Dur("retry_in", back).Int("attempt", attempts).Msg("github transient error retrying")
			_ = drainAndClose(resp.Body)
			c.sleep(back)
			attempts++
			continue
		default:
			// read a small tail for diagnostics then return
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			_ = resp.Body.Close()
			return nil, perr.Newf(perr.ErrorCodeUnknown, "github unexpected status %d body %s", resp.StatusCode, string(body))
		}
	}
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
