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
	Writer domain.WriterPort
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

	// Shared rulepack for the range runner
	rp, err := rulepack.Load()
	if err != nil {
		panic(err)
	}

	// Range runner (scan window over utterances and write hits)
	runner := service.New(
		ports.Utterances,
		ports.HitsWriter,
		rp,
		service.Config{
			Version:       cfg.Version,
			Workers:       cfg.Workers,
			PageSize:      cfg.PageSize,
			MaxRangeHours: cfg.MaxRangeHours,
			DryRun:        cfg.DryRun,
		},
	)

	// Direct writer (per-utterance detection; used by backfill --detect and future live ingest)
	writer := service.NewWriter(
		ports.HitsWriter,
		service.WriterConfig{Version: cfg.Version},
	)

	m := &Module{deps: deps}
	m.ports = Ports{
		Runner: runner,
		Writer: writer,
	}
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
