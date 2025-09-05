// Package config handles application configuration via environment variables
package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"swearjar/internal/platform/logger"
)

// Conf is a namespaced view over environment variables (e.g., "API_", "PG_")
// Use New("") for global access, or Prefix("API_") for module scopes.
type Conf struct{ prefix string }

// New creates a root Conf (no prefix)
func New() Conf { return Conf{} }

// Prefix creates a child Conf with an additional prefix, e.g. cfg.Prefix("API_")
func (c Conf) Prefix(p string) Conf { return Conf{prefix: c.prefix + p} }

// key composes the fully-qualified env var name
func (c Conf) key(k string) string { return c.prefix + k }

// MustString panics if the given key is missing or empty
func (c Conf) MustString(key string) string {
	k := c.key(key)
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		logger.Get().Panic().Str("key", k).Msg("missing required env")
	}
	return v
}

// MustInt panics if the given key is missing, empty, or not an int
func (c Conf) MustInt(key string) int {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		logger.Get().Panic().Str("key", c.key(key)).Msg("missing required env")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		logger.Get().Panic().Str("key", c.key(key)).Str("value", s).Msg("invalid int value")
	}
	return v
}

// MustBool panics if the given key is missing or empty
func (c Conf) MustBool(key string) bool {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		logger.Get().Panic().Str("key", c.key(key)).Msg("missing required env")
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		logger.Get().Panic().Str("key", c.key(key)).Str("value", s).Msg("invalid bool value")
	}
	return v
}

// MustDuration panics if the given key is missing, empty, or not a valid duration
func (c Conf) MustDuration(key string) time.Duration {
	s := c.MustString(key)
	d, err := time.ParseDuration(s)
	if err != nil {
		logger.Get().Panic().Str("key", c.key(key)).Str("value", s).Msg("invalid duration (e.g., 250ms, 2s, 1h)")
	}
	return d
}

// MustURL panics if the given key is missing, empty, or not a valid absolute URL
func (c Conf) MustURL(key string) *url.URL {
	s := c.MustString(key)
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() {
		logger.Get().Panic().Str("key", c.key(key)).Str("value", s).Msg("invalid absolute URL")
	}
	return u
}

// MustPort returns a Go net/http addr like ":4000" after validation 1..65535
func (c Conf) MustPort(key string) string {
	s := c.MustString(key)
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		logger.Get().Panic().Str("key", c.key(key)).Str("value", s).Msg("invalid TCP port; expected 1..65535")
	}
	return ":" + s
}

// Require ensures that all given keys are present (non-empty). Panics otherwise.
func (c Conf) Require(keys ...string) {
	for _, k := range keys {
		if strings.TrimSpace(os.Getenv(c.key(k))) == "" {
			logger.Get().Panic().Str("key", c.key(k)).Msg("missing required env")
		}
	}
}

// MayString returns the value or def if missing/empty
func (c Conf) MayString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(c.key(key)))
	if v == "" {
		return def
	}
	return v
}

// MayInt returns the value or def if missing/empty; logs and returns def if invalid
func (c Conf) MayInt(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	logger.Get().Warn().Str("key", c.key(key)).Str("value", s).Int("default", def).Msg("invalid int; using default")
	return def
}

// MayFloat64 returns the value or def if missing/empty; logs and returns def if invalid
func (c Conf) MayFloat64(key string, def float64) float64 {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	logger.Get().Warn().Str("key", c.key(key)).Str("value", s).Float64("default", def).
		Msg("invalid float64; using default")
	return def
}

// MayBool returns the value or def if missing/empty; logs and returns def if invalid
func (c Conf) MayBool(key string, def bool) bool {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	if v, err := strconv.ParseBool(s); err == nil {
		return v
	}
	logger.Get().Warn().Str("key", c.key(key)).Str("value", s).Bool("default", def).Msg("invalid bool; using default")
	return def
}

// MayDuration returns the value or def if missing/empty; logs and returns def if invalid
func (c Conf) MayDuration(key string, def time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	logger.Get().Warn().Str("key", c.key(key)).Str("value", s).Dur("default", def).Msg("invalid duration; using default")
	return def
}

// MayCSV returns a slice of strings from a comma-separated env var; def if missing/empty
func (c Conf) MayCSV(key string, def []string) []string {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// MayEnum ensures value is one of allowed; returns def if empty; panics if invalid.
func (c Conf) MayEnum(key, def string, allowed ...string) string {
	v := c.MayString(key, def)
	if v == "" {
		return v
	}
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return v
		}
	}
	logger.Get().Panic().Str("key", c.key(key)).Str("value", v).Strs("allowed", allowed).Msg("invalid enum value")
	return "" // unreachable
}
