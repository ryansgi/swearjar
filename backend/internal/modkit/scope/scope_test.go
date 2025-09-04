package scope

import (
	"context"
	"reflect"
	"testing"
)

func TestFrom_NoValueReturnsEmptyOrZeroScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := From(ctx)
	// With Option A, From returns a Scope with a non-nil map
	// With Option B, From returns zero scope and Get must still behave
	// We verify behavior via With and Get below
	if s.Values == nil {
		// acceptable under Option B
	} else if len(s.Values) != 0 {
		t.Fatalf("expected empty map when no values present, got %v", s.Values)
	}
}

func TestWith_MergesAndOverrides(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = With(ctx, map[string]string{"a": "1"})
	ctx = With(ctx, map[string]string{"b": "2", "a": "override"})

	s := From(ctx)
	want := map[string]string{"a": "override", "b": "2"}
	if s.Values == nil {
		t.Fatalf("expected non-nil map after With")
	}
	if !reflect.DeepEqual(s.Values, want) {
		t.Fatalf("expected %v got %v", want, s.Values)
	}
}

func TestWith_InitializesNilValues(t *testing.T) {
	t.Parallel()

	// Force a context that has a Scope with nil Values
	ctx := context.WithValue(context.Background(), key{}, Scope{Values: nil})
	ctx = With(ctx, map[string]string{"x": "1"})

	s := From(ctx)
	if s.Values == nil {
		t.Fatalf("expected map to be initialized after With")
	}
	if got, ok := s.Values["x"]; !ok || got != "1" {
		t.Fatalf("expected x=1 set via With, got %q ok=%v", got, ok)
	}
}

func TestGet_ReturnsValueAndBool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = With(ctx, map[string]string{"foo": "bar"})

	v, ok := Get(ctx, "foo")
	if !ok || v != "bar" {
		t.Fatalf("expected foo=bar ok=true, got %q ok=%v", v, ok)
	}

	v, ok = Get(ctx, "missing")
	if ok {
		t.Fatalf("expected ok=false for missing key, got value=%q", v)
	}
}
