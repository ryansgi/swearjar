package modkit

import (
	"testing"

	"swearjar/internal/platform/config"
)

func TestDeps_ZeroValue_IsOK(t *testing.T) {
	t.Parallel()
	var d Deps // zero value across all fields
	if !d.ZeroOK() {
		t.Fatal("zero-value Deps should be safe in tests (ZeroOK == true)")
	}
}

func TestDeps_NonZero_IsAlsoOK(t *testing.T) {
	t.Parallel()

	d := Deps{
		// Log left zero (allowed)
		Cfg: config.New(), // safe zero-friendly Conf
		// CH:  &ch.CH{}, // todo: uncomment when we have a real CH impl
	}

	if !d.ZeroOK() {
		t.Fatal("non-zero Deps should also report ZeroOK == true")
	}
}
