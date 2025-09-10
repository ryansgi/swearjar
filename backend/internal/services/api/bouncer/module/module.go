// Package module wires bouncer into the API using modkit
package module

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gh "swearjar/internal/adapters/ingest/github"
	modkit "swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/modkit/repokit"

	bhttp "swearjar/internal/services/api/bouncer/http"
	brepo "swearjar/internal/services/api/bouncer/repo"
	bsvc "swearjar/internal/services/api/bouncer/service"
	bdom "swearjar/internal/services/bouncer/domain"

	identRepoBinder "swearjar/internal/services/ident/repo"
	identsvc "swearjar/internal/services/ident/service"
)

// Module implements the bouncer API module
type Module struct {
	deps   modkit.Deps
	name   string
	prefix string

	mws       []func(http.Handler) http.Handler
	ports     any
	swaggerOn bool

	subrouter func(httpkit.Router) httpkit.Router
	register  func(httpkit.Router)

	svc bsvc.Service
}

// Ports declares the required injected worker port(s) for this API module
type Ports struct {
	Enqueuer bdom.EnqueuePort
}

// Identity adapter over the GH client
type ghIdentity struct{ c *gh.Client }

func (g ghIdentity) RepoID(ctx context.Context, full string) (int64, error) {
	full = strings.TrimSpace(full)
	parts := strings.SplitN(full, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, fmt.Errorf("invalid repo resource %q (want owner/repo)", full)
	}

	repo, _, _, err := g.c.RepoByFullName(ctx, parts[0], parts[1], "")
	if err != nil {
		return 0, err
	}
	if repo.ID == 0 {
		return 0, fmt.Errorf("github returned empty id for %q", full)
	}
	return repo.ID, nil
}

func (g ghIdentity) ActorID(ctx context.Context, login string) (int64, error) {
	l := strings.TrimSpace(login)
	if l == "" {
		return 0, fmt.Errorf("invalid login")
	}

	u, _, _, err := g.c.UserByLogin(ctx, l, "")
	if err != nil {
		return 0, err
	}
	if u.ID == 0 {
		return 0, fmt.Errorf("github returned empty id for login %q", l)
	}
	return u.ID, nil
}

// New constructs the bouncer module (config-driven, parity with other API modules)
func New(deps modkit.Deps, opts ...modkit.Option) modkit.Module {
	b := modkit.Build(append([]modkit.Option{
		modkit.WithName("bouncer"),
		modkit.WithPrefix("/bouncer"),
	}, opts...)...)

	cfg := FromConfig(deps.Cfg)

	var injected Ports
	if p, ok := b.Ports.(Ports); ok {
		injected = p
	}
	if injected.Enqueuer == nil {
		panic("bouncer API module requires Enqueuer port (from services/bouncer)")
	}

	repoBinder := brepo.NewPG()

	ghc := gh.NewClient(gh.Options{
		BaseURL:    cfg.BaseURL,
		UserAgent:  cfg.UserAgent,
		Timeout:    cfg.Timeout,
		TokensCSV:  cfg.TokensCSV,
		MaxRetries: cfg.MaxRetries,
		RetryBase:  cfg.RetryBase,
	})
	evidence := gh.NewProbe(ghc)

	ident := identsvc.New(repokit.TxRunner(deps.PG), identRepoBinder.NewPG())

	svc := bsvc.New(deps.PG, repoBinder, bsvc.Options{
		Secret:   cfg.Secret,
		Grace:    cfg.Grace,
		Resolver: newResolver(ghIdentity{c: ghc}, ident),
		Evidence: evidence,
		Enqueuer: injected.Enqueuer,
	})

	m := &Module{
		deps:      deps,
		name:      b.Name,
		prefix:    b.Prefix,
		mws:       b.Mw,
		swaggerOn: b.SwaggerOn,
		subrouter: b.Subrouter,
		svc:       svc,
	}
	m.ports = adaptBouncerPort{svc: svc}

	external := b.Register
	m.register = func(r httpkit.Router) {
		bhttp.Register(r, m.svc)
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
func (m *Module) Name() string { return m.name }

// Prefix returns the module route prefix
func (m *Module) Prefix() string { return m.prefix }
