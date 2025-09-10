// Package module provides the backfill module implementation
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	modreg "swearjar/internal/modkit/module" // registry (Register/PortsAs/Reset)
	"swearjar/internal/modkit/repokit"

	"swearjar/internal/core/normalize"
	"swearjar/internal/services/backfill/domain"
	"swearjar/internal/services/backfill/guardrails"
	"swearjar/internal/services/backfill/ingest"
	"swearjar/internal/services/backfill/repo"
	"swearjar/internal/services/backfill/service"

	detectmod "swearjar/internal/services/detect/module"
	identRepoBinder "swearjar/internal/services/ident/repo"
	identservice "swearjar/internal/services/ident/service"
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
// It wires adapters and the service using config from deps.Cfg.
// If detection is enabled (CORE_BACKFILL_DETECT or config), it looks up the
// already-registered detect module from the global registry and assigns its
// Writer port to svc.Detect
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

	// Construct the backfill service
	svc := service.New(
		repokit.TxRunner(deps.PG),
		storeBinder,
		fetch,
		reader,
		extract,
		norm,
		service.Config{
			DelayPerHour:  opts.DelayPerHour,
			Workers:       opts.Workers,
			MaxRetries:    opts.MaxRetries,
			RetryBase:     opts.RetryBase,
			FetchTimeout:  opts.FetchTimeout,
			ReadTimeout:   opts.ReadTimeout,
			MaxRangeHours: opts.MaxRangeHours,
			EnableLeases:  opts.EnableLeases,
			InsertChunk:   0, // keep default
			DetectEnabled: opts.DetectEnabled,
		},
		leaseFn,
		nil, // detect writer (optional); set below when enabled and available
	).WithIdentService(
		identservice.New(repokit.TxRunner(deps.PG), identRepoBinder.NewPG()),
	)

	// If detect is enabled, resolve detect module's Writer from the registry.
	// main.go registers detect before backfill when --detect is used
	if opts.DetectEnabled {
		if dp, ok := modreg.PortsAs[detectmod.Ports]("detect"); ok && dp.Writer != nil {
			svc.Detect = dp.Writer
		}
	}

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
func (m *Module) MountRoutes(_ httpkit.Router) {}
