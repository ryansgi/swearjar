// Package service provides the utterances service implementation
package service

import (
	"context"

	"swearjar/internal/core/normalize"
	utdom "swearjar/internal/services/utterances/domain"
	"swearjar/internal/services/utterances/repo"
)

// Config for the utterances service
type Config struct {
	// HardLimit is the maximum allowed limit per List call; defaults to 5000 if <=0
	HardLimit int
}

// Service implements domain.ReaderPort directly against the CH repo
type Service struct {
	Storage *repo.CH
	Norm    *normalize.Normalizer
	Cfg     Config
}

// New constructs a new utterances service
func New(storage *repo.CH, cfg Config) *Service {
	if cfg.HardLimit <= 0 {
		cfg.HardLimit = 5000
	}
	return &Service{
		Storage: storage,
		Norm:    normalize.New(),
		Cfg:     cfg,
	}
}

// List implements domain.ReaderPort.
// Guarantees that TextNorm is populated (empty string is fine if no text present)
func (s *Service) List(ctx context.Context, in utdom.ListInput) ([]utdom.Row, utdom.AfterKey, error) {
	limit := in.Limit
	if limit <= 0 || limit > s.Cfg.HardLimit {
		limit = s.Cfg.HardLimit
	}

	rows, next, err := s.Storage.List(ctx, in, limit)
	if err != nil {
		return nil, utdom.AfterKey{}, err
	}

	// If you later want to enforce normalization here, you can, but detector already normalizes
	return rows, next, nil
}
