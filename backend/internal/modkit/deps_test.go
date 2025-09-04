package modkit

import (
	"testing"

	"swearjar/internal/platform/config"
	ch "swearjar/internal/platform/store/ch"
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
		CH:  &ch.CH{},
	}

	if !d.ZeroOK() {
		t.Fatal("non-zero Deps should also report ZeroOK == true")
	}
}
