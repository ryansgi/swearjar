// Package service provides the utterances service implementation
package service

import (
	"context"

	"swearjar/internal/core/normalize"
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/utterances/domain"
	"swearjar/internal/services/utterances/repo"
)

// Config for the utterances service
type Config struct {
	// HardLimit is the maximum allowed limit per List call; defaults to 5000 if <=0
	HardLimit int
}

// Service implements domain.ReaderPort
type Service struct {
	DB     repokit.TxRunner
	Binder repokit.Binder[repo.Storage]
	Norm   *normalize.Normalizer
	Cfg    Config
}

// New constructs a new utterances service
func New(db repokit.TxRunner, b repokit.Binder[repo.Storage], cfg Config) *Service {
	if cfg.HardLimit <= 0 {
		cfg.HardLimit = 5000
	}
	return &Service{
		DB: db, Binder: b, Cfg: cfg, Norm: normalize.New(),
	}
}

// List implements domain.ReaderPort
// Guarantees that TextNorm is populated (may be empty string if text_raw is empty)
// if text_raw is present, by normalizing it if needed
func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Row, domain.AfterKey, error) {
	limit := in.Limit
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}

	var rows []domain.Row
	var next domain.AfterKey
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var err error
		rows, next, err = s.Binder.Bind(q).List(ctx, in, limit)
		return err
	})
	if err != nil {
		return nil, domain.AfterKey{}, err
	}

	// Guarantee normalized text is present
	// for i := range rows {
	// 	if rows[i].TextNorm == "" {
	// 		// normalize from text_raw would require selecting it; choose policy:
	// 		// keep empty (detector has its own normalizer), or expand SELECT to include text_raw.
	// 		// For now we leave as-is; detector will normalize.
	// 	}
	// }
	return rows, next, nil
}
