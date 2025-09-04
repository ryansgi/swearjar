package module

import (
	"testing"

	pstrings "swearjar/internal/platform/strings"

	"swearjar/internal/modkit/httpkit"
)

// FooPort is a tiny test interface that our Ports() payloads can implement
type FooPort interface {
	Foo() int
}

type fooImpl struct{ v int }

func (f fooImpl) Foo() int { return f.v }

// fakeModule is a small module double for tests
type fakeModule struct {
	name  string
	ports any
}

func (m fakeModule) Name() string               { return m.name }
func (m fakeModule) Ports() PortSet             { return m.ports }
func (m fakeModule) MountRoutes(httpkit.Router) {} // no-op, satisfies Module

func TestPortsOf_NilPorts(t *testing.T) {
	t.Parallel()

	m := fakeModule{name: "nilPorts", ports: nil}
	if _, ok := PortsOf[FooPort](m); ok {
		t.Fatalf("expected ok=false when Ports() is nil")
	}
}

func TestPortsOf_DirectInterfaceMatch(t *testing.T) {
	t.Parallel()

	want := fooImpl{v: 42}
	m := fakeModule{name: "direct", ports: FooPort(want)}

	got, ok := PortsOf[FooPort](m)
	if !ok {
		t.Fatalf("expected ok=true for direct interface match")
	}
	if got.Foo() != 42 {
		t.Fatalf("unexpected Foo value, got %d want 42", got.Foo())
	}
}

func TestPortsOf_StructBundle_ExportedField(t *testing.T) {
	t.Parallel()

	// Exported field should be discoverable
	type Ports struct {
		Foo FooPort
		Bar int
	}
	want := fooImpl{v: 7}
	m := fakeModule{
		name:  "bundle",
		ports: Ports{Foo: want, Bar: 1},
	}

	got, ok := PortsOf[FooPort](m)
	if !ok {
		t.Fatalf("expected ok=true when bundle has exported Foo field")
	}
	if got.Foo() != 7 {
		t.Fatalf("unexpected Foo value, got %d want 7", got.Foo())
	}
}

func TestPortsOf_StructBundle_UnexportedField_Ignored(t *testing.T) {
	t.Parallel()

	// Unexported field should be ignored by PortsOf
	type ports struct {
		foo FooPort // unexported
		bar int
	}
	m := fakeModule{
		name:  "unexported",
		ports: ports{foo: fooImpl{v: 1}, bar: 2},
	}

	if _, ok := PortsOf[FooPort](m); ok {
		t.Fatalf("expected ok=false when only unexported field implements T")
	}
}

func TestMustPortsOf_PanicsWithModuleName(t *testing.T) {
	t.Parallel()

	m := fakeModule{name: "auth", ports: nil}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic from MustPortsOf when port missing")
		}
		msg, _ := r.(string)
		if msg == "" || !pstrings.Contains(msg, "auth") || !pstrings.Contains(msg, "requested port not found") {
			t.Fatalf("panic message should include module name and hint, got %q", msg)
		}
	}()

	_ = MustPortsOf[FooPort](m) // should panic
}

func TestMustPortsOf_ReturnsValue(t *testing.T) {
	t.Parallel()

	// fakeModule and FooPort/fooImpl are already defined above in this file
	m := fakeModule{
		name:  "ok",
		ports: FooPort(fooImpl{v: 99}), // direct match so PortsOf succeeds
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("did not expect panic, got %v", r)
		}
	}()

	got := MustPortsOf[FooPort](m) // should not panic; should return the value
	if got.Foo() != 99 {
		t.Fatalf("unexpected Foo value from MustPortsOf, got %d want 99", got.Foo())
	}
}
