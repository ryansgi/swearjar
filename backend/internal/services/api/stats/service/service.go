// Package service contains stats workflows
package service

import (
	"context"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/api/stats/domain"
	"swearjar/internal/services/api/stats/repo"
)

// Service defines the stats service contract
type Service interface {
	domain.ServicePort
}

// Svc implements the stats service
type Svc struct {
	Repo   repo.Repo
	binder repokit.Binder[repo.Repo]
	db     repokit.TxRunner
}

// New constructs a stats service
func New(db repokit.TxRunner, binder repokit.Binder[repo.Repo]) *Svc {
	if db == nil {
		panic("stats.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("stats.Service requires a non nil Repo binder")
	}
	return &Svc{Repo: binder.Bind(db), binder: binder, db: db}
}

// ByLang returns swearjar usage stats by programming language
func (s *Svc) ByLang(ctx context.Context, in domain.ByLangInput) ([]domain.ByLangRow, error) {
	rows, err := s.Repo.ByLang(ctx, in.Range.Start, in.Range.End, in.Repo, in.Lang, in.MinSeverity)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ByLangRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.ByLangRow{
			Day:        r.Day,
			Lang:       r.Lang,
			Hits:       r.Hits,
			Utterances: r.Utterances,
		})
	}
	return out, nil
}

// ByRepo returns top repos in a given time window
func (s *Svc) ByRepo(ctx context.Context, in domain.ByRepoInput) ([]domain.ByRepoRow, error) {
	rows, err := s.Repo.ByRepo(ctx, in.Range.Start, in.Range.End, in.Lang)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ByRepoRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.ByRepoRow{Repo: r.Repo, Hits: r.Hits})
	}
	return out, nil
}

// ByCategory returns swearjar usage stats by category and severity
func (s *Svc) ByCategory(ctx context.Context, in domain.ByCategoryInput) ([]domain.ByCategoryRow, error) {
	rows, err := s.Repo.ByCategory(ctx, in.Range.Start, in.Range.End, in.Repo)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ByCategoryRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.ByCategoryRow{Category: r.Category, Severity: r.Severity, Hits: r.Hits})
	}
	return out, nil
}
