// Package module wires meta endpoints into the API using a tiny module
package module

import (
	"net/http"
	"time"

	modkit "swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	str "swearjar/internal/platform/strings"

	metahttp "swearjar/internal/services/api/meta/http"
)

// Module implements the modkit.Module interface
type Module struct {
	deps      modkit.Deps
	name      string
	prefix    string
	mws       []func(http.Handler) http.Handler
	swaggerOn bool

	subrouter func(httpkit.Router) httpkit.Router
	register  func(httpkit.Router)

	startedAt time.Time
}

// New constructs a meta module with the provided dependencies and options
func New(deps modkit.Deps, opts ...modkit.Option) modkit.Module {
	b := modkit.Build(append([]modkit.Option{
		modkit.WithName("meta"),
		modkit.WithPrefix("/meta"),
	}, opts...)...)

	m := &Module{
		deps:      deps,
		name:      b.Name,
		prefix:    b.Prefix,
		mws:       b.Mw,
		swaggerOn: b.SwaggerOn,
		subrouter: b.Subrouter,
		startedAt: time.Now(),
	}

	external := b.Register
	m.register = func(r httpkit.Router) {
		metahttp.Register(r, metahttp.Deps{
			ServiceName: "swearjar-api",
			StartedAt:   m.startedAt,
			PG:          deps.PG,
		})
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

// Name implements the modkit.Module interface
func (m *Module) Name() string { return str.MustString(m.name, "meta") }

// Prefix implements the modkit.Module interface
func (m *Module) Prefix() string { return str.MustPrefix(m.prefix) }

// Middlewares implements the modkit.Module interface
func (m *Module) Middlewares() []func(http.Handler) http.Handler { return m.mws }

// Ports implements the modkit.Module interface
func (m *Module) Ports() any { return nil }
