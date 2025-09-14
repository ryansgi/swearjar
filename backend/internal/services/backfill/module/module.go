// Package module provides the backfill module implementation
package module

import (
	"context"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	modreg "swearjar/internal/modkit/module"
	"swearjar/internal/modkit/repokit"

	"swearjar/internal/core/normalize"
	"swearjar/internal/services/backfill/domain"
	"swearjar/internal/services/backfill/guardrails"
	"swearjar/internal/services/backfill/ingest"
	"swearjar/internal/services/backfill/repo"
	"swearjar/internal/services/backfill/service"

	detectdom "swearjar/internal/services/detect/domain"
	detectmod "swearjar/internal/services/detect/module"
	identRepoBinder "swearjar/internal/services/ident/repo"
	identservice "swearjar/internal/services/ident/service"
	nightshiftmod "swearjar/internal/services/nightshift/module"
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
// Writer port to svc.DetectorPort.
// If nightshift is enabled (CORE_BACKFILL_NIGHTSHIFT or config), it looks up the
// already-registered nightshift module from the global registry and assigns its
// Runner port to svc.NightshiftHour
func New(deps modkit.Deps) *Module {
	opts := FromConfig(deps.Cfg)

	storeBinder := repo.NewHybrid(deps.CH)

	// Non-DB adapters
	fetch := ingest.NewFetcher(deps)
	reader := ingest.NewReaderFactory()
	extract := ingest.NewExtractor()
	norm := ingest.NewNormalizer(normalize.New())
	leaseFn := guardrails.MakeAdvisoryLease(deps, "backfill", opts.LeaseTTL)

	var detWriter detectdom.WriterPort
	if opts.DetectEnabled {
		if dp, ok := modreg.PortsAs[detectmod.Ports]("detect"); ok && dp.Writer != nil {
			detWriter = dp.Writer
		}
	}

	var nightshiftFn func(context.Context, time.Time) error
	if np, ok := modreg.PortsAs[nightshiftmod.Ports]("nightshift"); ok && np.Runner != nil {
		if r, ok := any(np.Runner).(interface {
			ApplyHour(context.Context, time.Time) error
		}); ok {
			nightshiftFn = r.ApplyHour
		}
	}

	// Construct service and chain optional hooks
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
			InsertChunk:   0,
			DetectEnabled: opts.DetectEnabled,
		},
		leaseFn,
		detWriter,
	).WithIdentService(
		identservice.New(repokit.TxRunner(deps.PG), identRepoBinder.NewPG()),
	).WithNightshift(nightshiftFn)

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
