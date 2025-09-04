// Package module implements the hits service module
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/hits/domain"
	"swearjar/internal/services/hits/repo"
	"swearjar/internal/services/hits/service"
)

// Ports exposed by the hits module
type Ports struct {
	Writer domain.WriterPort
	Query  domain.QueryPort
}

// Module implements the hits service module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs a new hits module
func New(deps modkit.Deps) *Module {
	opts := FromConfig(deps.Cfg)

	binder := repo.NewPG()
	svc := service.New(repokit.TxRunner(deps.PG), binder, service.Config{
		HardLimit: opts.HardLimit,
	})

	m := &Module{deps: deps}
	m.ports = Ports{
		Writer: svc,
		Query:  svc,
	}
	return m
}

// Name satisfies modkit.Module
func (m *Module) Name() string { return "hits" }

// Ports satisfies modkit.Module
func (m *Module) Ports() any { return m.ports }

// Prefix satisfies modkit.Module
func (m *Module) Prefix() string { return "" }

// MountRoutes satisfies modkit.Module
func (m *Module) MountRoutes(r httpkit.Router) {}
