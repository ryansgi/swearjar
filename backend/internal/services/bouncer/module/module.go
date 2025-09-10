// Package module wires the bouncer worker service and exposes its ports
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/bouncer/service"
)

// Module defines the bouncer worker module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs the bouncer worker module with its ports
func New(deps modkit.Deps, overrides Options) *Module {
	// Load defaults, then apply non-zero overrides
	opts := FromConfig(deps.Cfg)

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
	if overrides.QueueTakeBatch != 0 {
		opts.QueueTakeBatch = overrides.QueueTakeBatch
	}
	if overrides.RetryBaseMs != 0 {
		opts.RetryBaseMs = overrides.RetryBaseMs
	}
	if overrides.MaxAttempts != 0 {
		opts.MaxAttempts = overrides.MaxAttempts
	}

	svc := service.New(deps, service.Config{
		Concurrency:    opts.Concurrency,
		RatePerSec:     opts.RatePerSec,
		Burst:          opts.Burst,
		TokensCSV:      opts.TokensCSV,
		QueueTakeBatch: opts.QueueTakeBatch,
		RetryBaseMs:    opts.RetryBaseMs,
		MaxAttempts:    opts.MaxAttempts,
	})

	m := &Module{deps: deps}
	m.ports = Ports{
		Worker:   svc, // svc implements WorkerPort
		Enqueuer: svc, // svc also implements EnqueuePort
	}
	return m
}

// Ports returns the module ports (Worker, Enqueuer)
func (m *Module) Ports() any { return m.ports }

// Name returns the module name
func (m *Module) Name() string { return "bouncer" }

// Prefix returns the module config prefix (none for worker-only service)
func (m *Module) Prefix() string { return "" }

// MountRoutes returns no HTTP routes
func (m *Module) MountRoutes(_ httpkit.Router) {}
