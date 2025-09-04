// Package module implements the detect module
package module

import (
	"net/http"

	"swearjar/internal/core/rulepack"
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/detect/domain"
	"swearjar/internal/services/detect/service"
)

// Ports exposed by the detect module
type Ports struct {
	Runner domain.RunnerPort
}

// Module implements modkit.Module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs a new detect module
func New(deps modkit.Deps, overrides Options, opts ...modkit.Option) *Module {
	b := modkit.Build(append([]modkit.Option{
		modkit.WithName("detect"),
	}, opts...)...)

	// Basic guardrails against incorrect wiring
	ports, ok := b.Ports.(domain.Ports)
	if !ok {
		panic("detect module: expected WithPorts(detect/domain.Ports)")
	}

	if ports.Utterances == nil || ports.HitsWriter == nil {
		panic("detect module: Ports missing Utterances or HitsWriter")
	}

	// Merge config + overrides
	// @Todo: improve config management
	cfg := FromConfig(deps.Cfg)
	if overrides.Version != 0 {
		cfg.Version = overrides.Version
	}
	if overrides.Workers != 0 {
		cfg.Workers = overrides.Workers
	}
	if overrides.PageSize != 0 {
		cfg.PageSize = overrides.PageSize
	}
	if overrides.MaxRangeHours != 0 {
		cfg.MaxRangeHours = overrides.MaxRangeHours
	}

	// bool override wins (defaults false if caller didn't set)
	cfg.DryRun = overrides.DryRun

	rp, err := rulepack.Load()
	if err != nil {
		panic(err)
	}

	svc := service.New(ports.Utterances, ports.HitsWriter, rp, service.Config{
		Version:       cfg.Version,
		Workers:       cfg.Workers,
		PageSize:      cfg.PageSize,
		MaxRangeHours: cfg.MaxRangeHours,
		DryRun:        cfg.DryRun,
	})

	m := &Module{deps: deps}

	// Export the interface directly so registry lookups are trivial
	m.ports = Ports{Runner: svc}

	return m
}

// Name satisfies modkit.Module
func (m *Module) Name() string { return "detect" }

// Ports satisfies modkit.Module
func (m *Module) Ports() any { return m.ports }

// Prefix satisfies modkit.Module
func (m *Module) Prefix() string { return "" }

// Middlewares satisfies modkit.Module
func (m *Module) Middlewares() []func(http.Handler) http.Handler { return nil }

// MountRoutes satisfies modkit.Module
func (m *Module) MountRoutes(_ httpkit.Router) {}
