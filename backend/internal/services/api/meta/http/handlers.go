// Package http provides meta endpoints
package http

import (
	stdctx "context"
	"net/http"
	"time"

	"swearjar/internal/core/version"
	"swearjar/internal/modkit/httpkit"
)

// Pinger is satisfied by adapters that expose Ping
type Pinger interface {
	Ping(stdctx.Context) error
}

// Deps are the handler dependencies
type Deps struct {
	ServiceName string
	StartedAt   time.Time
	PG          any
	CH          any
}

type handlers struct {
	deps Deps
}

// Register mounts the meta routes
func Register(r httpkit.Router, d Deps) {
	h := &handlers{deps: d}

	// mount routes
	httpkit.Get(r, "/health", h.health)
	httpkit.Get(r, "/ready", h.ready)
	httpkit.Get(r, "/version", h.version)
	httpkit.Get(r, "/service", h.service)
	httpkit.Get(r, "/detector", h.detector)
}

//
// Swagger DTOs and route docs
//

// HealthResponse is the health payload
// swagger:model
type HealthResponse struct {
	OK      bool   `json:"ok"       example:"true"`
	Service string `json:"service"  example:"swearjar-api"`
	Started string `json:"started"  example:"2025-09-03T13:00:00Z"`
	Now     string `json:"now"      example:"2025-09-03T13:05:00Z"`
}

// ReadyCheck describes a single dependency check
type ReadyCheck struct {
	Name   string `json:"name"   example:"pg"`
	Status string `json:"status" example:"ok"` // ok fail skipped unknown
	Error  string `json:"error,omitempty" example:"dial tcp 127.0.0.1:5432 connect: connection refused"`
}

// ReadyResponse summarizes readiness
type ReadyResponse struct {
	Status string       `json:"status" example:"ok"` // ok degraded fail
	Checks []ReadyCheck `json:"checks"`
	Now    string       `json:"now"    example:"2025-09-03T13:05:00Z"`
}

// ServiceResponse describes service info
type ServiceResponse struct {
	Name    string `json:"name"    example:"swearjar-api"`
	Started string `json:"started" example:"2025-09-03T13:00:00Z"`
	Uptime  int64  `json:"uptime"  example:"300"`
}

// DetectorResponse reports detector version and build info
type DetectorResponse struct {
	DetectorVersion int               `json:"detector_version" example:"1"`
	Build           version.BuildInfo `json:"build"`
}

// swagger:route GET /meta/health Meta metaHealth
// @Summary Health check
// @Tags Meta
// @Produce json
// @Success 200 type HealthResponse "login success"
// @Router /meta/health [get]
func (h *handlers) health(_ *http.Request) (any, error) {
	return HealthResponse{
		OK:      true,
		Service: h.deps.ServiceName,
		Started: h.deps.StartedAt.UTC().Format(time.RFC3339),
		Now:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// swagger:route GET /meta/ready Meta metaReady
// @Summary Readiness probe with dependency checks
// @Tags Meta
// @Produce json
// @Success 200 type ReadyResponse ok
// @Router /meta/ready [get]
func (h *handlers) ready(_ *http.Request) (any, error) {
	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 2*time.Second)
	defer cancel()

	check := func(name string, c any) ReadyCheck {
		if c == nil {
			return ReadyCheck{Name: name, Status: "skipped"}
		}
		if p, ok := c.(Pinger); ok {
			if err := p.Ping(ctx); err != nil {
				return ReadyCheck{Name: name, Status: "fail", Error: err.Error()}
			}
			return ReadyCheck{Name: name, Status: "ok"}
		}
		return ReadyCheck{Name: name, Status: "unknown"}
	}

	pg := check("pg", h.deps.PG)
	ch := check("ch", h.deps.CH)

	overall := "ok"
	if pg.Status != "ok" || ch.Status != "ok" {
		overall = "degraded"
		if pg.Status == "fail" || ch.Status == "fail" {
			overall = "fail"
		}
	}

	return ReadyResponse{
		Status: overall,
		Checks: []ReadyCheck{pg, ch},
		Now:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// swagger:route GET /meta/version Meta metaVersion
// @Summary Build and version info
// @Tags Meta
// @Produce json
// @Success 200 type version.BuildInfo ok
// @Router /meta/version [get]
func (h *handlers) version(_ *http.Request) (any, error) {
	return version.Info(), nil
}

// swagger:route GET /meta/service Meta metaService
// @Summary Service info and uptime
// @Tags Meta
// @Produce json
// @Success 200 type ServiceResponse ok
// @Router /meta/service [get]
func (h *handlers) service(_ *http.Request) (any, error) {
	uptime := time.Since(h.deps.StartedAt)
	return ServiceResponse{
		Name:    h.deps.ServiceName,
		Started: h.deps.StartedAt.UTC().Format(time.RFC3339),
		Uptime:  int64(uptime / time.Second),
	}, nil
}

// swagger:route GET /meta/detector Meta metaDetector
// @Summary Detector version and build
// @Tags Meta
// @Produce json
// @Success 200 type DetectorResponse ok
// @Router /meta/detector [get]
func (h *handlers) detector(_ *http.Request) (any, error) {
	return DetectorResponse{
		DetectorVersion: 1,
		Build:           version.Info(),
	}, nil
}
