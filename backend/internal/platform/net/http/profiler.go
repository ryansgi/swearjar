// Package http hosts server adapters. Profiler mounts pprof endpoints when enabled
package http

import (
	stdhttp "net/http"

	mw "github.com/go-chi/chi/v5/middleware"
)

// MountProfiler mounts pprof under prefix. Example: "/debug"
func MountProfiler(r Router, prefix string, enabled bool) {
	if !enabled {
		return
	}
	// emulate r.Mount by stripping the prefix before handing off to the profiler mux
	h := stdhttp.StripPrefix(prefix, mw.Profiler())

	// handle the prefix itself and any subpaths under it
	r.Get(prefix, func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		h.ServeHTTP(w, req)
	})
	r.Get(prefix+"/*", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		h.ServeHTTP(w, req)
	})
}
