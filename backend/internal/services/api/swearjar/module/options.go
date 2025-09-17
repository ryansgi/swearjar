package module

import (
	"net/http"

	modkit "swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
)

// Option is a configuration option for the samples module
type Option = modkit.Option

// WithPrefix sets the route prefix for the module
func WithPrefix(prefix string) Option { return modkit.WithPrefix(prefix) }

// WithMiddlewares sets the middlewares for the module
func WithMiddlewares(mw ...func(http.Handler) http.Handler) Option {
	return modkit.WithMiddlewares(mw...)
}

// WithPorts sets the ports for the module
func WithPorts(p any) Option { return modkit.WithPorts(p) }

// WithRegister sets the register function for the module
func WithRegister(fn func(httpkit.Router)) Option { return modkit.WithRegister(fn) }

// WithSubrouter sets the subrouter function for the module
func WithSubrouter(fn func(httpkit.Router) httpkit.Router) Option { return modkit.WithSubrouter(fn) }
