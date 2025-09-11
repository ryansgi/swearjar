// Package module provides the utterances module
package module

import (
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/utterances/domain"
	"swearjar/internal/services/utterances/repo"
	"swearjar/internal/services/utterances/service"
)

// Ports exposed by the utterances module
type Ports struct {
	Reader domain.ReaderPort
}

// Module implements the utterances module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs a new utterances module
func New(deps modkit.Deps) *Module {
	if deps.CH == nil {
		panic("utterances module requires ClickHouse (deps.CH nil)")
	}
	opts := FromConfig(deps.Cfg)

	storage := repo.NewCH(deps.CH)
	svc := service.New(storage, service.Config{
		HardLimit: opts.HardLimit,
	})

	m := &Module{deps: deps}
	m.ports = Ports{Reader: svc}
	return m
}

// Name satisfies modkit.Module
func (m *Module) Name() string { return "utterances" }

// Ports satisfies modkit.Module
func (m *Module) Ports() any { return m.ports }

// Prefix satisfies modkit.Module
func (m *Module) Prefix() string { return "" }

// MountRoutes satisfies modkit.Module
func (m *Module) MountRoutes(_ httpkit.Router) {}
