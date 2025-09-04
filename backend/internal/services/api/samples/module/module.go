// Package module wires samples into the API using modkit
package module

import (
	"net/http"

	modkit "swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	str "swearjar/internal/platform/strings"
	sampleshttp "swearjar/internal/services/api/samples/http"
	samplesrepo "swearjar/internal/services/api/samples/repo"
	samplessvc "swearjar/internal/services/api/samples/service"
)

// Module implements the modkit.Module interface
type Module struct {
	deps   modkit.Deps
	name   string
	prefix string

	mws       []func(http.Handler) http.Handler
	ports     any
	swaggerOn bool

	subrouter func(httpkit.Router) httpkit.Router
	register  func(httpkit.Router)

	svc samplessvc.Service
}

// New constructs a samples module with the provided dependencies and options
func New(deps modkit.Deps, opts ...modkit.Option) modkit.Module {
	b := modkit.Build(append([]modkit.Option{modkit.WithName("samples"), modkit.WithPrefix("/samples")}, opts...)...)

	repo := samplesrepo.NewPG()
	svc := samplessvc.New(deps.PG, repo)

	m := &Module{
		deps:      deps,
		name:      b.Name,
		prefix:    b.Prefix,
		mws:       b.Mw,
		swaggerOn: b.SwaggerOn,
		subrouter: b.Subrouter,
		svc:       svc,
	}
	m.ports = adaptSamplesPort{svc: svc}

	external := b.Register
	m.register = func(r httpkit.Router) {
		sampleshttp.Register(r, m.svc)
		if external != nil {
			external(r)
		}
	}
	return m
}

// MountRoutes implements the modkit.Module interface
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
