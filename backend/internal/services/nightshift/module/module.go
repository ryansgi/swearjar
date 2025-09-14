// Package module wires up the Nightshift service as a modkit.Module
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	modreg "swearjar/internal/modkit/module"
	"swearjar/internal/modkit/repokit"

	nsdom "swearjar/internal/services/nightshift/domain"
	"swearjar/internal/services/nightshift/guardrails"
	nsrepo "swearjar/internal/services/nightshift/repo"
	nsservice "swearjar/internal/services/nightshift/service"
)

// Ports exported by the Nightshift module
type Ports struct {
	Runner nsdom.RunnerPort
}

// Module implements modkit.Module for Nightshift
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs and wires the Nightshift module using deps.Cfg
func New(deps modkit.Deps) *Module {
	opts := FromConfig(deps.Cfg)

	binder := nsrepo.NewHybrid(deps.CH)

	leaseFn := guardrails.MakeNSLease(deps, "nightshift", opts.LeaseTTL)

	svc := nsservice.New(
		repokit.TxRunner(deps.PG),
		binder,
		nsservice.Config{
			Workers:         opts.Workers,
			DetectorVersion: opts.DetectorVersion,
			RetentionMode:   opts.RetentionMode,
			EnableLeases:    opts.EnableLeases,
		},
		leaseFn,
	)

	m := &Module{deps: deps}
	m.ports = Ports{Runner: svc}
	return m
}

// Name returns the module name
func (m *Module) Name() string { return "nightshift" }

// Ports returns the module ports
func (m *Module) Ports() any { return m.ports }

// Prefix returns the module config prefix (none)
func (m *Module) Prefix() string { return "" }

// MountRoutes is a no-op: Nightshift has no HTTP routes
func (m *Module) MountRoutes(_ httpkit.Router) {}

// Register convenience: allow others to resolve our ports via registry
func Register(deps modkit.Deps) {
	modreg.Register("nightshift", New(deps))
}
