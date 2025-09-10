// Package raw provides a minimal env reader used during bootstrap.
// It intentionally has NO dependency on the logger package to avoid import cycles
package raw

import (
	"os"
	"strings"
)

// Conf is a namespaced view over environment variables (e.g., "API_", "PG_")
type Conf struct{ prefix string }

// New returns a root Conf (no prefix)
func New() Conf { return Conf{} }

// Prefix returns a child Conf with an additional prefix (e.g. "LOG_")
func (c Conf) Prefix(p string) Conf { return Conf{prefix: c.prefix + p} }

// key composes the fully-qualified env var
func (c Conf) key(k string) string { return c.prefix + k }

// Get returns the trimmed env var or the provided default if empty
func (c Conf) Get(key, def string) string {
	v := strings.TrimSpace(os.Getenv(c.key(key)))
	if v == "" {
		return def
	}
	return v
}

// GetBool parses a bool-like env ("1|true|yes") with default fallback
func (c Conf) GetBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(c.key(key))))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

// GetInt parses a positive integer with default fallback; non-numeric -> def
func (c Conf) GetInt(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(c.key(key)))
	if s == "" {
		return def
	}
	n := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
