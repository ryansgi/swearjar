package httpkit

import "net/http"

// MountUnder mounts a subrouter at prefix and applies per-module middlewares
func MountUnder(r Router, prefix string, mw []func(http.Handler) http.Handler, mount func(Router)) {
	r.Route(prefix, func(sub Router) {
		if len(mw) > 0 {
			sub.Use(mw...)
		}
		mount(sub)
	})
}
