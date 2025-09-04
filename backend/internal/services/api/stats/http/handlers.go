// Package http provides http transport for stats
package http

import (
	stdhttp "net/http"

	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/api/stats/domain"
	svc "swearjar/internal/services/api/stats/service"
)

// Register mounts stats endpoints on the given router
func Register(r httpkit.Router, s svc.Service) {
	h := &handlers{svc: s}

	// buckets by language and day
	httpkit.PostJSON[domain.ByLangInput](r, "/lang", h.byLang)

	// top repos in window
	httpkit.PostJSON[domain.ByRepoInput](r, "/repo", h.byRepo)

	// buckets by category and severity
	httpkit.PostJSON[domain.ByCategoryInput](r, "/category", h.byCategory)
}

type handlers struct{ svc svc.Service }

// swagger:route POST /stats/lang Stats statsByLang
// @Summary Stats by language and day
// @Tags Stats
// @Accept json
// @Produce json
// @Param payload body domain.ByLangInput true "Query"
// @Success 200 {array} domain.ByLangRow "ok"
// @Router /stats/lang [post]
func (h *handlers) byLang(r *stdhttp.Request, in domain.ByLangInput) (any, error) {
	return h.svc.ByLang(r.Context(), in)
}

// swagger:route POST /stats/repo Stats statsByRepo
// @Summary Stats by repo
// @Tags Stats
// @Accept json
// @Produce json
// @Param payload body domain.ByRepoInput true "Query"
// @Success 200 {array} domain.ByRepoRow "ok"
// @Router /stats/repo [post]
func (h *handlers) byRepo(r *stdhttp.Request, in domain.ByRepoInput) (any, error) {
	return h.svc.ByRepo(r.Context(), in)
}

// swagger:route POST /stats/category Stats statsByCategory
// @Summary Stats by category and severity
// @Tags Stats
// @Accept json
// @Produce json
// @Param payload body domain.ByCategoryInput true "Query"
// @Success 200 {array} domain.ByCategoryRow "ok"
// @Router /stats/category [post]
func (h *handlers) byCategory(r *stdhttp.Request, in domain.ByCategoryInput) (any, error) {
	return h.svc.ByCategory(r.Context(), in)
}
