package module

import (
	modkit "swearjar/internal/modkit"
	mmodule "swearjar/internal/modkit/module"
)

// DepsModules is a convenience carrier of dependency *modules*.
// The detect module will extract the required ports internally
type DepsModules struct {
	Utterances mmodule.Module
	Hits       mmodule.Module
}

// WithDepsModules lets callers pass dependency modules without exposing MustPortsOf in main
func WithDepsModules(ut mmodule.Module, hits mmodule.Module) modkit.Option {
	return modkit.WithPorts(DepsModules{Utterances: ut, Hits: hits})
}
