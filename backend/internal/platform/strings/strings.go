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

// EmptyToNil returns empty string if s is all whitespace, otherwise returns s
func EmptyToNil(s string) string {
	if std.TrimSpace(s) == "" {
		return ""
	}
	return s
}

// Ptr returns a pointer to s, or nil if s is empty
func Ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SQLNull returns nil if s is blank/whitespace, else the original string.
// Useful for query args where NULL is desired for blanks
func SQLNull(s string) any {
	if std.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// SQLNullPtr returns nil if ps is nil or points to a blank string, else the dereferenced string
func SQLNullPtr(ps *string) any {
	if ps == nil || std.TrimSpace(*ps) == "" {
		return nil
	}
	return *ps
}

// Deref returns "" if ps is nil, else *ps.
func Deref(ps *string) string {
	if ps == nil {
		return ""
	}
	return *ps
}
