package httpkit

import (
	"net/http"
	"strings"

	"swearjar/internal/platform/net/middleware"

	phttp "swearjar/internal/platform/net/http"
)

// Protected groups routes under bearer auth and records secured endpoints for swagger
func Protected(r Router, p middleware.AuthPort, fn func(Router)) {
	r.Group(func(gr Router) {
		gr.Use(Auth(p))
		fn(&securedRouter{Router: gr})
	})
}

type securedRouter struct {
	Router
	base string
}

func joinPath(a, b string) string {
	if a == "" {
		if strings.HasPrefix(b, "/") {
			return b
		}
		return "/" + b
	}
	if strings.HasSuffix(a, "/") {
		if strings.HasPrefix(b, "/") {
			return a + b[1:]
		}
		return a + b
	}
	if strings.HasPrefix(b, "/") {
		return a + b
	}
	return a + "/" + b
}

func (s *securedRouter) Route(prefix string, fn func(Router)) {
	child := &securedRouter{Router: s.Router, base: joinPath(s.base, prefix)}
	s.Router.Route(prefix, func(_ Router) { fn(child) })
}

func (s *securedRouter) Handle(path string, h http.Handler) { s.Router.Handle(path, h) }

func (s *securedRouter) Options(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "options")
	s.Router.Options(path, h)
}

func (s *securedRouter) Head(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "head")
	s.Router.Head(path, h)
}

func (s *securedRouter) Delete(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "delete")
	s.Router.Delete(path, h)
}

func (s *securedRouter) Get(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "get")
	s.Router.Get(path, h)
}

func (s *securedRouter) Post(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "post")
	s.Router.Post(path, h)
}

func (s *securedRouter) Put(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "put")
	s.Router.Put(path, h)
}

func (s *securedRouter) Patch(path string, h phttp.Handler) {
	// swaggerkit.MarkSecurePath(joinPath(s.base, path), "patch")
	s.Router.Patch(path, h)
}
