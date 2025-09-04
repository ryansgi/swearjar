// Package service contains samples workflows
package service

import (
	"context"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/api/samples/domain"
	"swearjar/internal/services/api/samples/repo"
)

// Service defines the service contract for samples
type Service interface{ domain.ServicePort }

// Svc implements the Service interface
type Svc struct {
	Repo   repo.Repo
	binder repokit.Binder[repo.Repo]
	db     repokit.TxRunner
}

// New creates a new samples service
func New(db repokit.TxRunner, binder repokit.Binder[repo.Repo]) *Svc {
	if db == nil {
		panic("samples.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("samples.Service requires a non nil Repo binder")
	}
	return &Svc{Repo: binder.Bind(db), binder: binder, db: db}
}

// Recent retrieves recent samples based on the provided input filters
func (s *Svc) Recent(ctx context.Context, in domain.SamplesInput) ([]domain.Sample, error) {
	rows, err := s.Repo.Recent(ctx, in.Repo, in.Lang, in.Category, in.Severity, in.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Sample, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Sample{
			UtteranceID:  r.UtteranceID,
			Repo:         r.Repo,
			Lang:         r.Lang,
			Source:       r.Source,
			SourceDetail: r.SourceDetail,
			Text:         r.Text,
			Term:         r.Term,
			Category:     r.Category,
			Severity:     r.Severity,
			DetectorVer:  r.DetectorVer,
			CreatedAt:    r.CreatedAt,
		})
	}
	return out, nil
}
