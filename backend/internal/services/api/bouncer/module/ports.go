package module

import (
	"context"

	"swearjar/internal/services/api/bouncer/domain"
	bsvc "swearjar/internal/services/api/bouncer/service"
)

// Ports returns the module ports (parity with stats)
func (m *Module) Ports() any { return m.ports }

// adaptBouncerPort exposes service methods as module ports for cross-module usage
type adaptBouncerPort struct{ svc bsvc.Service }

func (a adaptBouncerPort) Issue(ctx context.Context, in domain.IssueInput) (domain.IssueOutput, error) {
	return a.svc.Issue(ctx, in)
}

func (a adaptBouncerPort) Reverify(ctx context.Context, in domain.ReverifyInput) (domain.StatusRow, error) {
	return a.svc.Reverify(ctx, in)
}

func (a adaptBouncerPort) Status(ctx context.Context, in domain.StatusQuery) (domain.StatusRow, error) {
	return a.svc.Status(ctx, in)
}
