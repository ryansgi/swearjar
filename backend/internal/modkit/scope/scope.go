// Package scope provides a way to manage context-scoped values
package scope

import "context"

// Scope holds cross boundary attributes
type Scope struct {
	Values map[string]string
}

type key struct{}

// With merges values into scope on ctx
func With(ctx context.Context, kv map[string]string) context.Context {
	s := From(ctx) // From already guarantees a non-nil map
	for k, v := range kv {
		s.Values[k] = v
	}
	return context.WithValue(ctx, key{}, s)
}

// Get returns a value and a boolean
func Get(ctx context.Context, k string) (string, bool) {
	s := From(ctx)
	v, ok := s.Values[k]
	return v, ok
}

// From returns scope on ctx or an empty one
func From(ctx context.Context) Scope {
	v := ctx.Value(key{})
	if v == nil {
		return Scope{Values: make(map[string]string)}
	}
	s, _ := v.(Scope)
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
	return s
}
