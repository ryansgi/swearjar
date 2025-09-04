// Package strings provides string slice helpers
package strings

import std "strings"

// IfEmpty returns def if in is empty, otherwise returns in
func IfEmpty[T any](in []T, def []T) []T {
	if len(in) == 0 {
		return def
	}
	return in
}

// Contains reports whether sub is within s
func Contains(s, sub string) bool { return std.Contains(s, sub) }

// HasSuffix reports whether s ends with suf
func HasSuffix(s, suf string) bool { return std.HasSuffix(s, suf) }

// MustString returns s if it has non whitespace content otherwise panics
// name is used in the panic message so you can tell what was missing
func MustString(s string, name string) string {
	if std.TrimSpace(s) == "" {
		panic(name + " is required")
	}
	return s
}

// MustPrefix normalizes and asserts a root path like /auth or /tenants
// ensures a single leading slash and no trailing slash except for the root itself
// panics if the input is empty after trimming
func MustPrefix(s string) string {
	s = std.TrimSpace(s)
	s = "/" + std.Trim(s, " /")
	if s == "/" {
		panic("root path is required")
	}
	return s
}
