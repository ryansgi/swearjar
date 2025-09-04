// Package swaggerkit provides helpers to mount Swagger UI and JSON spec
package swaggerkit

import (
	"net/http"

	phttp "swearjar/internal/platform/net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

// Mount the Swagger UI and JSON spec if enabled
func Mount(r phttp.Router, enabled bool) {
	if !enabled {
		return
	}
	r.Get("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs/", http.StatusPermanentRedirect)
	})
	r.Get("/api/docs/doc.json", serveDocJSON())
	r.Handle("/api/docs/*", httpSwagger.Handler(
		httpSwagger.InstanceName("api"),
		httpSwagger.URL("/api/docs/doc.json"),
	))
}
