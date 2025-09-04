//go:build !swag

package swaggerkit

import "net/http"

var docReader = func() string {
	return `{"openapi":"3.0.3","info":{"title":"API","version":"0.0.0"},"paths":{}}`
}

// serveDocJSON (no-swag build) serves the skeleton so the UI can still load
func serveDocJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(docReader()))
	}
}
