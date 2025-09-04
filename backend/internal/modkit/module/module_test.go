package module

import (
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

// stubModule is a minimal test double that satisfies Module
// it records when MountRoutes is called and returns a configurable ports value
type stubModule struct {
	mounted *bool
	ports   any
}

// MountRoutes marks that it was invoked
func (s *stubModule) MountRoutes(_ phttp.Router) {
	if s.mounted != nil {
		*s.mounted = true
	}
}

// Ports returns the configured ports value
func (s *stubModule) Ports() any   { return s.ports }
func (s *stubModule) Name() string { return "" }

// compile time assertion that stubModule implements Module
var _ Module = (*stubModule)(nil)

func HasPorts(m Module) bool {
	if m == nil {
		return false
	}
	return m.Ports() != nil
}

// TestModule_MountRoutes verifies that MountRoutes can be called and is observable
func TestModule_MountRoutes(t *testing.T) {
	called := false
	m := &stubModule{mounted: &called}

	// allow a nil typed router since the contract does not require usage
	var r phttp.Router = nil
	m.MountRoutes(r)

	if !called {
		t.Fatalf("expected MountRoutes to set called but it did not")
	}
}

// TestModule_Ports verifies that Ports can return arbitrary values including nil
func TestModule_Ports(t *testing.T) {
	type portSet struct {
		Name string
		ID   int
	}

	cases := []struct {
		name     string
		portsIn  any
		assertFn func(any) error
	}{
		{
			name:    "nil ports",
			portsIn: nil,
			assertFn: func(v any) error {
				if v != nil {
					return errf("expected nil ports got %T", v)
				}
				return nil
			},
		},
		{
			name:    "primitive ports",
			portsIn: 123,
			assertFn: func(v any) error {
				n, ok := v.(int)
				if !ok || n != 123 {
					return errf("expected int 123 got %v", v)
				}
				return nil
			},
		},
		{
			name:    "struct ports",
			portsIn: portSet{Name: "tenants", ID: 7},
			assertFn: func(v any) error {
				ps, ok := v.(portSet)
				if !ok {
					return errf("expected portSet got %T", v)
				}
				if ps.Name != "tenants" || ps.ID != 7 {
					return errf("unexpected portSet contents %+v", ps)
				}
				return nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &stubModule{ports: tc.portsIn}
			got := m.Ports()
			if err := tc.assertFn(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// errf is a tiny helper to avoid pulling in fmt in tests that only need Errorf style messages
type testErr struct{ s string }

func (e testErr) Error() string { return e.s }

func errf(format string, a ...any) error {
	// minimal formatting to avoid importing fmt just for tests
	// for simple cases we can stitch values with %v into the string
	// keep it simple since assertions are small
	s := format
	for _, v := range a {
		s = s + " " + sprint(v)
	}
	return testErr{s: s}
}

func sprint(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return itoa(x)
	default:
		// fallback type name only
		return "%v"
	}
}

func itoa(n int) string {
	// simple positive only since tests do not use negatives
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestHasPorts(t *testing.T) {
	m1 := &stubModule{ports: nil}
	m2 := &stubModule{ports: 123}

	if HasPorts(nil) {
		t.Fatal("nil module should report false")
	}
	if HasPorts(m1) {
		t.Fatal("nil ports should report false")
	}
	if !HasPorts(m2) {
		t.Fatal("non-nil ports should report true")
	}
}
