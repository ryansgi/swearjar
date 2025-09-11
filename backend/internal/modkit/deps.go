// Package modkit provides module wiring and core deps
package modkit

import (
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"
)

// Deps holds core dependencies passed to modules
// this is wiring only and does not introduce new abstractions
type Deps struct {
	Log logger.Logger
	Cfg config.Conf
	PG  repokit.TxRunner
	CH  store.Clickhouse
}

// ZeroOK returns true when deps are safe to use with zero values in tests
// consumers should still nil check for optional stores
func (d Deps) ZeroOK() bool { return true }
