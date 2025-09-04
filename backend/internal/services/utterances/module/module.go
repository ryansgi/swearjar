// Package module provides the utterances module
package module

import (
	"net/http"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/utterances/domain"
	"swearjar/internal/services/utterances/repo"
	"swearjar/internal/services/utterances/service"
)

// Ports exposed by the utterances module
type Ports struct {
	Reader domain.ReaderPort
}

// Module implements modkit.Module
type Module struct {
	deps  modkit.Deps
	ports Ports
}

// New constructs a new utterances module
func New(deps modkit.Deps) *Module {
	opts := FromConfig(deps.Cfg)

	binder := repo.NewPG()
	svc := service.New(repokit.TxRunner(deps.PG), binder, service.Config{
		HardLimit: opts.HardLimit,
	})

	m := &Module{deps: deps}
	m.ports = Ports{Reader: svc}
	return m
}

// Name implements modkit.Module
func (m *Module) Name() string { return "utterances" }

// Ports implements modkit.Module
func (m *Module) Ports() any { return m.ports }

// Prefix implements modkit.Module
func (m *Module) Prefix() string { return "" }

// Middlewares implements modkit.Module
func (m *Module) Middlewares() []func(http.Handler) http.Handler { return nil }

// MountRoutes implements modkit.Module
func (m *Module) MountRoutes(r httpkit.Router) {}
