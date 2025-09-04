// Package service provides the hits service implementation
package service

import (
	"context"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/hits/domain"
	"swearjar/internal/services/hits/repo"
)

// Config for the hits service
type Config struct {
	HardLimit int
}

// Service implements domain.WriterPort and domain.QueryPort
type Service struct {
	DB     repokit.TxRunner
	Binder repokit.Binder[repo.Storage]
	Cfg    Config
}

// New constructs a new hits service
func New(db repokit.TxRunner, b repokit.Binder[repo.Storage], cfg Config) *Service {
	if cfg.HardLimit <= 0 {
		cfg.HardLimit = 100
	}
	return &Service{DB: db, Binder: b, Cfg: cfg}
}

// WriteBatch implements domain.WriterPort
func (s *Service) WriteBatch(ctx context.Context, xs []domain.HitWrite) error {
	return s.DB.Tx(ctx, func(q repokit.Queryer) error {
		return s.Binder.Bind(q).WriteBatch(ctx, xs)
	})
}

// ListSamples implements domain.QueryPort
func (s *Service) ListSamples(
	ctx context.Context,
	w domain.Window,
	f domain.Filters,
	after domain.AfterKey,
	limit int,
) ([]domain.Sample, domain.AfterKey, error) {
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}
	var rows []domain.Sample
	var next domain.AfterKey
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var err error
		rows, next, err = s.Binder.Bind(q).ListSamples(ctx, w, f, after, limit)
		return err
	})
	return rows, next, err
}

// AggByLang implements domain.QueryPort
func (s *Service) AggByLang(ctx context.Context, w domain.Window, f domain.Filters) ([]domain.AggByLangRow, error) {
	var out []domain.AggByLangRow
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var err error
		out, err = s.Binder.Bind(q).AggByLang(ctx, w, f)
		return err
	})
	return out, err
}

// AggByRepo implements domain.QueryPort
func (s *Service) AggByRepo(
	ctx context.Context,
	w domain.Window,
	f domain.Filters,
	limit int,
) ([]domain.AggByRepoRow, error) {
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}
	var out []domain.AggByRepoRow
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var err error
		out, err = s.Binder.Bind(q).AggByRepo(ctx, w, f, limit)
		return err
	})
	return out, err
}

// AggByCategory implements domain.QueryPort
func (s *Service) AggByCategory(
	ctx context.Context,
	w domain.Window,
	f domain.Filters,
) ([]domain.AggByCategoryRow, error) {
	var out []domain.AggByCategoryRow
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var err error
		out, err = s.Binder.Bind(q).AggByCategory(ctx, w, f)
		return err
	})
	return out, err
}
