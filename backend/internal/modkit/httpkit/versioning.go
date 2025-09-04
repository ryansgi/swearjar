package httpkit

import (
	"net/http"
	"strings"
)

// MountAPI mounts a subrouter under /api/{version}, applies any per-scope middleware,
// then invokes mount to register routes on that scoped router
//
// example:
//
//	httpkit.MountAPI(r, "v1", httpkit.CommonStack(), func(api httpkit.Router) {
//	  auth.MountRoutes(api)
//	})
func MountAPI(r Router, version string, mw []func(http.Handler) http.Handler, mount func(Router)) {
	ver := strings.TrimPrefix(version, "/")
	prefix := "/api/" + ver
	r.Route(prefix, func(api Router) {
		if len(mw) > 0 {
			api.Use(mw...)
		}
		mount(api)
	})
}

// MountAPIV1 is a convenience for MountAPI with version v1
func MountAPIV1(r Router, mw []func(http.Handler) http.Handler, mount func(Router)) {
	MountAPI(r, "v1", mw, mount)
}
