// Package service provides the hits service implementation
package service

import (
	"context"

	dom "swearjar/internal/services/hits/domain"
	"swearjar/internal/services/hits/repo"
)

// Config for the hits service
type Config struct {
	HardLimit int
}

// Service implements domain.WriterPort and domain.QueryPort directly against CH repo
type Service struct {
	Storage *repo.CH
	Cfg     Config
}

// New constructs a new hits service with a required CH repo
func New(storage *repo.CH, cfg Config) *Service {
	if cfg.HardLimit <= 0 {
		cfg.HardLimit = 100
	}
	return &Service{Storage: storage, Cfg: cfg}
}

// WriteBatch implements domain.WriterPort
func (s *Service) WriteBatch(ctx context.Context, xs []dom.HitWrite) error {
	return s.Storage.WriteBatch(ctx, xs)
}

// ListSamples implements domain.QueryPort
func (s *Service) ListSamples(
	ctx context.Context,
	w dom.Window,
	f dom.Filters,
	after dom.AfterKey,
	limit int,
) ([]dom.Sample, dom.AfterKey, error) {
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}
	return s.Storage.ListSamples(ctx, w, f, after, limit)
}

// AggByLang implements domain.QueryPort
func (s *Service) AggByLang(ctx context.Context, w dom.Window, f dom.Filters) ([]dom.AggByLangRow, error) {
	return s.Storage.AggByLang(ctx, w, f)
}

// AggByRepo implements domain.QueryPort
func (s *Service) AggByRepo(ctx context.Context, w dom.Window, f dom.Filters, limit int) ([]dom.AggByRepoRow, error) {
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}
	return s.Storage.AggByRepo(ctx, w, f, limit)
}

// AggByCategory implements domain.QueryPort
func (s *Service) AggByCategory(ctx context.Context, w dom.Window, f dom.Filters) ([]dom.AggByCategoryRow, error) {
	return s.Storage.AggByCategory(ctx, w, f)
}
