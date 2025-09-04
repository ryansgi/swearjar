package modkit

import (
	phttp "swearjar/internal/platform/net/http"
)

// Module is the common surface for API modules that can mount routes and expose ports
// keep this tiny so modules stay decoupled
type Module interface {
	// MountRoutes mounts HTTP routes under the provided router seam
	MountRoutes(r phttp.Router)
	// Ports returns a module specific port set interface for cross wiring
	Ports() any

	// Name returns the module name
	Name() string
}

// Builder constructs a Module from shared deps and options
// modules typically expose New(deps Deps, opts ...Option) Module and may delegate to this pattern
type Builder func(Deps, ...Option) Module
