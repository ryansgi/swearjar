// Package module defines the minimal contract for a modkit module
package module

import (
	phttp "swearjar/internal/platform/net/http"
)

// Module defines the minimal contract used by modkit
// keep this sibling to avoid import knots when a module also exports its own ports type
type Module interface {
	MountRoutes(r phttp.Router)
	Ports() any
	Name() string
}
