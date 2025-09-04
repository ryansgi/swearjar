package modkit

import (
	"net/http"

	"swearjar/internal/modkit/httpkit"
)

// Built is a plain struct with the fields modules care about
type Built struct {
	Name      string
	Prefix    string
	Mw        []func(http.Handler) http.Handler
	Ports     any
	SwaggerOn bool

	// router hooks set via options and exposed to modules
	Subrouter func(httpkit.Router) httpkit.Router
	Register  func(httpkit.Router)
}

// Build applies Option funcs to an internal buildCfg and returns a plain struct
func Build(opts ...Option) Built {
	var c buildCfg
	for _, o := range opts {
		o(&c)
	}
	// defaults for hooks
	if c.subrouter == nil {
		c.subrouter = func(r httpkit.Router) httpkit.Router { return r }
	}
	if c.register == nil {
		c.register = func(httpkit.Router) {}
	}
	return Built{
		Name:      c.name,
		Prefix:    c.prefix,
		Mw:        append([]func(http.Handler) http.Handler(nil), c.mw...),
		Ports:     c.ports,
		SwaggerOn: c.swaggerOn,
		Subrouter: c.subrouter,
		Register:  c.register,
	}
}
