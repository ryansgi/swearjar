package modkit

import (
	"net/http"

	phttp "swearjar/internal/platform/net/http"
)

// Option mutates build configuration for a module
type Option func(*buildCfg)

// buildCfg is internal wiring state for options
type buildCfg struct {
	name      string
	prefix    string
	mw        []func(http.Handler) http.Handler
	ports     any
	swaggerOn bool
	subrouter func(phttp.Router) phttp.Router
	register  func(phttp.Router)
}

// WithName sets a module name used in logs and registry
func WithName(name string) Option {
	return func(c *buildCfg) { c.name = name }
}

// WithPrefix mounts a module under a path prefix
func WithPrefix(prefix string) Option {
	return func(c *buildCfg) { c.prefix = prefix }
}

// WithMiddlewares attaches per module middleware in order
func WithMiddlewares(mw ...func(http.Handler) http.Handler) Option {
	return func(c *buildCfg) { c.mw = append(c.mw, mw...) }
}

// WithPorts injects cross module ports declared by another module
// the concrete type is owned by the importing module
func WithPorts[T any](p T) Option {
	return func(c *buildCfg) { c.ports = p }
}

// WithSwagger toggles swagger UI for this module when mounted
func WithSwagger(enabled bool) Option {
	return func(c *buildCfg) { c.swaggerOn = enabled }
}

// WithSubrouter lets a caller provide a subrouter factory using the platform seam
func WithSubrouter(fn func(phttp.Router) phttp.Router) Option {
	return func(c *buildCfg) { c.subrouter = fn }
}

// WithRegister sets the function that attaches endpoints to the module router
func WithRegister(fn func(phttp.Router)) Option {
	return func(c *buildCfg) { c.register = fn }
}
