// Package module wires stats into the API using modkit
package module

import (
	"net/http"

	modkit "swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	str "swearjar/internal/platform/strings"
	statshhtp "swearjar/internal/services/api/stats/http"
	statsrepo "swearjar/internal/services/api/stats/repo"
	statssvc "swearjar/internal/services/api/stats/service"
)

// Module implements the stats module
type Module struct {
	deps   modkit.Deps
	name   string
	prefix string

	mws       []func(http.Handler) http.Handler
	ports     any
	swaggerOn bool

	subrouter func(httpkit.Router) httpkit.Router
	register  func(httpkit.Router)

	svc statssvc.Service
}

// New constructs the stats module
func New(deps modkit.Deps, opts ...modkit.Option) modkit.Module {
	b := modkit.Build(append([]modkit.Option{modkit.WithName("stats"), modkit.WithPrefix("/stats")}, opts...)...)

	repo := statsrepo.NewPG()
	svc := statssvc.New(deps.PG, repo)

	m := &Module{
		deps:      deps,
		name:      b.Name,
		prefix:    b.Prefix,
		mws:       b.Mw,
		swaggerOn: b.SwaggerOn,
		subrouter: b.Subrouter,
		svc:       svc,
	}
	m.ports = adaptStatsPort{svc: svc}

	external := b.Register
	m.register = func(r httpkit.Router) {
		statshhtp.Register(r, m.svc)
		if external != nil {
			external(r)
		}
	}
	return m
}

// MountRoutes mounts the module routes on the given router
func (m *Module) MountRoutes(r httpkit.Router) {
	r.Route(m.prefix, func(rr httpkit.Router) {
		for _, mw := range m.mws {
			rr.Use(mw)
		}
		if m.subrouter != nil {
			rr = m.subrouter(rr)
		}
		if m.register != nil {
			m.register(rr)
		}
	})
}

// Name returns the module name
func (m *Module) Name() string { return str.MustString(m.name, "module name") }

// Prefix returns the module route prefix
func (m *Module) Prefix() string { return str.MustPrefix(m.prefix) }

// Middlewares returns the module middlewares
func (m *Module) Middlewares() []func(http.Handler) http.Handler { return m.mws }
