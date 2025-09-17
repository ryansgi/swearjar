// Package module wires the swearjar API into HTTP via modkit
package module

import (
	"net/http"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/strings"
	"swearjar/internal/services/api/swearjar/domain"

	swearjarhttp "swearjar/internal/services/api/swearjar/http"
	"swearjar/internal/services/api/swearjar/repo"
	"swearjar/internal/services/api/swearjar/service"
)

// Ports exposes the service port for cross-module lookups
type Ports struct {
	Service domain.ServicePort
}

// Module implements the swearjar module
type Module struct {
	deps   modkit.Deps
	name   string
	prefix string

	mws       []func(http.Handler) http.Handler
	ports     Ports
	swaggerOn bool

	subrouter func(httpkit.Router) httpkit.Router
	register  func(httpkit.Router)

	svc *service.Service
}

// New constructs the swearjar module
func New(deps modkit.Deps, opts ...modkit.Option) modkit.Module {
	b := modkit.Build(append([]modkit.Option{modkit.WithName("swearjar"), modkit.WithPrefix("/swearjar")}, opts...)...)

	binder := repo.NewHybrid(deps.CH)
	svc := service.New(repokit.TxRunner(deps.PG), binder)

	m := &Module{
		deps:      deps,
		name:      b.Name,
		prefix:    b.Prefix,
		mws:       b.Mw,
		swaggerOn: b.SwaggerOn,
		subrouter: b.Subrouter,
		svc:       svc,
	}
	m.ports = Ports{Service: svc}

	external := b.Register
	m.register = func(r httpkit.Router) {
		swearjarhttp.Register(r, m.svc)
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

// Name is the module name
func (m *Module) Name() string { return strings.MustString(m.name, "module name") }

// Prefix is the module route prefix
func (m *Module) Prefix() string { return strings.MustPrefix(m.prefix) }

// Middlewares is the module middlewares
func (m *Module) Middlewares() []func(http.Handler) http.Handler { return m.mws }

// Ports returns the module ports
func (m *Module) Ports() any { return m.ports }
