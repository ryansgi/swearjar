// Package api provides the HTTP API for the application
package api

import (
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	phttp "swearjar/internal/platform/net/http"
	"swearjar/internal/platform/store"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/modkit/swaggerkit"

	metamod "swearjar/internal/services/api/meta/module"
	samplesmod "swearjar/internal/services/api/samples/module"
	statsmod "swearjar/internal/services/api/stats/module"
)

// Options are the API options
type Options struct {
	Config         config.Conf
	Store          *store.Store
	Logger         *logger.Logger
	EnableSwagger  bool
	EnableProfiler bool
}

// Mount mounts the API service onto the given router
func Mount(r phttp.Router, opt Options) {
	// shared deps for modules
	deps := modkit.Deps{
		Cfg: opt.Config,
		PG:  opt.Store.PG,
	}

	// build modules
	meta := metamod.New(deps)
	stats := statsmod.New(deps)
	samples := samplesmod.New(deps)

	mods := []module.Module{
		stats,
		samples,
		meta,
	}

	// versioned API with a common middleware stack
	httpkit.MountAPIV1(r, httpkit.CommonStack(), func(api httpkit.Router) {
		// Swagger + profiler
		swaggerkit.Mount(r, opt.EnableSwagger)
		phttp.MountProfiler(r, "/debug", opt.EnableProfiler)

		for _, m := range mods {
			// register each module's ports under its own name (for cross-module lookups)
			module.Register(m.Name(), m.Ports())

			// mount module routes under its Prefix()
			m.MountRoutes(api)
		}
	})

	// TODO: Remove/create middleware or endpoint for this.
	// if mux, ok := r.Mux().(*chi.Mux); ok {
	// 	_ = chi.Walk(mux, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
	// 		fmt.Println(method, route)
	// 		return nil
	// 	})
	// }
}
