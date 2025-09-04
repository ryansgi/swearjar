package module

import (
	"context"

	"swearjar/internal/services/api/stats/domain"
	statssvc "swearjar/internal/services/api/stats/service"
)

// Ports returns the module ports
func (m *Module) Ports() any { return m.ports }

type adaptStatsPort struct{ svc statssvc.Service }

// ByLang returns swearjar usage stats by programming language
func (a adaptStatsPort) ByLang(ctx context.Context, in domain.ByLangInput) ([]domain.ByLangRow, error) {
	return a.svc.ByLang(ctx, in)
}

// ByRepo returns top repos in a given time window
func (a adaptStatsPort) ByRepo(ctx context.Context, in domain.ByRepoInput) ([]domain.ByRepoRow, error) {
	return a.svc.ByRepo(ctx, in)
}

// ByCategory returns swearjar usage stats by category and severity
func (a adaptStatsPort) ByCategory(ctx context.Context, in domain.ByCategoryInput) ([]domain.ByCategoryRow, error) {
	return a.svc.ByCategory(ctx, in)
}
