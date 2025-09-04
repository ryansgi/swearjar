// Package module provides the backfill module implementation
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/repokit"

	"swearjar/internal/core/normalize"
	"swearjar/internal/services/backfill/domain"
	"swearjar/internal/services/backfill/guardrails"
	"swearjar/internal/services/backfill/ingest"
	"swearjar/internal/services/backfill/repo"
	"swearjar/internal/services/backfill/service"
)

// Ports defines the backfill module ports
type Ports struct {
	Runner domain.RunnerPort
}

// Module implements the backfill module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs the backfill module
// It wires up all the adapters and the service using config from deps.Cfg
// It does not mount any routes.
func New(deps modkit.Deps) *Module {
	opts := FromConfig(deps.Cfg)

	// DB binder (no deps passed into repo)
	storeBinder := repo.NewPG()

	// Non-DB adapters
	fetch := ingest.NewFetcher(deps)    // uses CORE_INGEST_* from deps.Cfg
	reader := ingest.NewReaderFactory() // wraps GHArchive reader
	extract := ingest.NewExtractor()    // wraps FromEvent
	norm := ingest.NewNormalizer(normalize.New())

	leaseFn := guardrails.MakeAdvisoryLease(deps)

	svc := service.New(
		repokit.TxRunner(deps.PG), storeBinder,
		fetch, reader, extract, norm,
		service.Config{
			DelayPerHour:  opts.DelayPerHour,
			Workers:       opts.Workers,
			MaxRetries:    opts.MaxRetries,
			RetryBase:     opts.RetryBase,
			FetchTimeout:  opts.FetchTimeout,
			ReadTimeout:   opts.ReadTimeout,
			MaxRangeHours: opts.MaxRangeHours,
			EnableLeases:  opts.EnableLeases,
		},
		leaseFn,
	)

	m := &Module{deps: deps}
	m.ports = Ports{Runner: svc}
	return m
}

// Name returns the module name
func (m *Module) Name() string { return "backfill" }

// Ports returns the module ports
func (m *Module) Ports() any { return m.ports }

// Prefix returns the module prefix (none)
func (m *Module) Prefix() string { return "" }

// MountRoutes is a no-op as backfill has no routes
func (m *Module) MountRoutes(_ interface{}) {}
