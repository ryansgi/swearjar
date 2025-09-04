package http

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

// MountSwagger mounts /docs if enabled by caller
func MountSwagger(r Router, enabled bool) {
	if !enabled {
		return
	}
	r.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		httpSwagger.WrapHandler(w, r)
	})
}
