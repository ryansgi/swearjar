package module

import (
	"sync"
	"testing"
)

// simple type used in tests
type portSet struct {
	Name string
	ID   int
}

// must is a tiny helper for ok checks
func must(t *testing.T, ok bool, msg string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s", msg)
	}
}

func TestRegistry_RegisterAndPortsAs_Success(t *testing.T) {
	t.Parallel()
	Reset()

	want := portSet{Name: "tenants", ID: 1}
	Register("tenants", want)

	got, ok := PortsAs[portSet]("tenants")
	must(t, ok, "expected ok for existing name")
	if got != want {
		t.Fatalf("unexpected value got=%v want=%v", got, want)
	}
}

func TestRegistry_PortsAs_MissingReturnsZeroAndFalse(t *testing.T) {
	t.Parallel()
	Reset()

	got, ok := PortsAs[portSet]("missing")
	if ok {
		t.Fatal("expected ok=false for missing name")
	}
	if got != (portSet{}) {
		t.Fatalf("expected zero value got=%v", got)
	}
}

func TestRegistry_PortsAs_TypeMismatchReturnsFalse(t *testing.T) {
	t.Parallel()
	Reset()

	Register("tenants", portSet{Name: "tenants", ID: 2})

	// ask for wrong type
	_, ok := PortsAs[int]("tenants")
	if ok {
		t.Fatal("expected ok=false for type mismatch")
	}
}

func TestRegistry_Register_OverwritesExisting(t *testing.T) {
	t.Parallel()
	Reset()

	Register("svc", portSet{Name: "a", ID: 1})
	Register("svc", portSet{Name: "b", ID: 2})

	got, ok := PortsAs[portSet]("svc")
	must(t, ok, "expected ok for svc after overwrite")
	if got.Name != "b" || got.ID != 2 {
		t.Fatalf("expected overwritten value got=%v", got)
	}
}

func TestRegistry_Reset_ClearsAll(t *testing.T) {
	t.Parallel()
	Reset()

	Register("x", portSet{Name: "x", ID: 9})
	Reset()

	_, ok := PortsAs[portSet]("x")
	if ok {
		t.Fatal("expected ok=false after reset")
	}
}

func TestRegistry_ConcurrentRegisterAndRead_NoRace(t *testing.T) {
	t.Parallel()
	Reset()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(2)

	// writer
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			Register("concurrent", portSet{Name: "k", ID: i})
		}
	}()

	// reader
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_, _ = PortsAs[portSet]("concurrent")
		}
	}()

	wg.Wait()

	got, ok := PortsAs[portSet]("concurrent")
	must(t, ok, "expected ok after concurrent writes")
	if got.Name != "k" {
		t.Fatalf("unexpected final value got=%v", got)
	}
}
