// Package http provides http transport for samples
package http

import (
	stdhttp "net/http"

	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/api/samples/domain"
	svc "swearjar/internal/services/api/samples/service"
)

// Register mounts samples endpoints on the given router
func Register(r httpkit.Router, s svc.Service) {
	h := &handlers{svc: s}
	httpkit.PostJSON[domain.SamplesInput](r, "/commit-crimes", h.recent) // playful name
}

type handlers struct{ svc svc.Service }

// swagger:route POST /samples/commit-crimes Samples samplesRecent
// @Summary Recent spicy samples joined to hits
// @Tags Samples
// @Accept json
// @Produce json
// @Param payload body domain.SamplesInput true "Query"
// @Success 200 {array} domain.Sample "ok"
// @Router /samples/commit-crimes [post]
func (h *handlers) recent(r *stdhttp.Request, in domain.SamplesInput) (any, error) {
	return h.svc.Recent(r.Context(), in)
}
