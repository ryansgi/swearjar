// Package module wires the hallmonitor service and exposes its ports
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"

	"swearjar/internal/services/hallmonitor/service"
)

// Module defines the hallmonitor module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs the hallmonitor module with its ports
func New(deps modkit.Deps, overrides Options) *Module {
	// Load defaults from config then apply overrides from CLI (if provided)
	opts := FromConfig(deps.Cfg)

	// Merge simple overrides (non-zero/explicit)
	if overrides.Concurrency != 0 {
		opts.Concurrency = overrides.Concurrency
	}
	if overrides.RatePerSec != 0 {
		opts.RatePerSec = overrides.RatePerSec
	}
	if overrides.Burst != 0 {
		opts.Burst = overrides.Burst
	}
	if overrides.TokensCSV != "" {
		opts.TokensCSV = overrides.TokensCSV
	}
	if overrides.DryRun {
		opts.DryRun = true
	}

	// @TODO: allow other overrides (limits, retry, etc)?
	// @TODO: TokensCSV should use platform config - this would speed up the gh client
	svc := service.New(deps, service.Config{
		Concurrency:         opts.Concurrency,
		RatePerSec:          opts.RatePerSec,
		Burst:               opts.Burst,
		TokensCSV:           opts.TokensCSV,
		DryRun:              opts.DryRun,
		DefaultSeedLimit:    opts.DefaultSeedLimit,
		DefaultRefreshLimit: opts.DefaultRefreshLimit,
		QueueTakeBatch:      opts.QueueTakeBatch,
		RetryBaseMs:         int(opts.RetryBase.Milliseconds()),
		MaxAttempts:         opts.MaxAttempts,
	})

	m := &Module{deps: deps}
	m.ports = Ports{
		Worker:    svc,
		Seeder:    svc,
		Refresher: svc,
	}
	return m
}

// Name returns the module name
func (m *Module) Name() string { return "hallmonitor" }

// Ports returns the module ports (Worker, Seeder, Refresher)
func (m *Module) Ports() any { return m.ports }

// Prefix returns the module config prefix (none for hallmonitor)
func (m *Module) Prefix() string { return "" }

// MountRoutes returns no HTTP routes for hallmonitor (it's a worker/CLI service)
func (m *Module) MountRoutes(_ httpkit.Router) {}
