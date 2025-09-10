// Package http provides http transport for bouncer
package http

import (
	stdhttp "net/http"

	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/api/bouncer/domain"
	svc "swearjar/internal/services/api/bouncer/service"
)

// Register mounts the router
func Register(r httpkit.Router, s svc.Service) {
	h := &handlers{svc: s}
	httpkit.PostJSON[domain.IssueInput](r, "/issue", h.issue)
	httpkit.PostJSON[domain.ReverifyInput](r, "/reverify", h.reverify)
	httpkit.PostJSON[domain.StatusQuery](r, "/status", h.status)
}

type handlers struct{ svc svc.Service }

// swagger:route POST /bouncer/issue Bouncer issue
// @Summary Issue bouncer challenge
// @Tags bouncer
// @Accept json
// @Produce json
// @Param payload body domain.IssueInput true "Issue"
// @Success 200 {object} domain.IssueOutput "ok"
// @Router /bouncer/issue [post]
func (h *handlers) issue(r *stdhttp.Request, in domain.IssueInput) (any, error) {
	return h.svc.Issue(r.Context(), in)
}

// swagger:route POST /bouncer/reverify Bouncer Reverify
// @Summary Reverify bouncer artifact
// @Tags bouncer
// @Accept json
// @Produce json
// @Param payload body domain.ReverifyInput true "Reverify"
// @Success 200 {object} domain.StatusRow "ok"
// @Failure 404 {object} httpkit.Envelope "not found"
// @Router /bouncer/reverify [post]
func (h *handlers) reverify(r *stdhttp.Request, in domain.ReverifyInput) (any, error) {
	return h.svc.Reverify(r.Context(), in)
}

// swagger:route POST /bouncer/status Bouncer Status
// @Summary Current bouncer status
// @Tags bouncer
// @Produce json
// @Param payload body domain.StatusQuery true "Status"
// @Success 200 {object} domain.StatusRow "ok"
// @Router /bouncer/status [get]
func (h *handlers) status(r *stdhttp.Request, in domain.StatusQuery) (any, error) {
	return h.svc.Status(r.Context(), in)
}
