// Package service implements the bouncer worker and enqueue service
package service

import (
	"context"
	"time"

	gh "swearjar/internal/adapters/ingest/github"
	"swearjar/internal/modkit"
	"swearjar/internal/modkit/repokit"

	dom "swearjar/internal/services/bouncer/domain"
	brepo "swearjar/internal/services/bouncer/repo"
)

// Service implements both worker+enqueue ports
type Service interface {
	dom.WorkerPort
	dom.EnqueuePort
}

// Config controls the worker
type Config struct {
	Concurrency    int
	RatePerSec     float64
	Burst          int
	TokensCSV      string
	QueueTakeBatch int
	RetryBaseMs    int
	MaxAttempts    int
}

// Svc implements the bouncer worker and enqueue service
type Svc struct {
	db     repokit.TxRunner
	binder repokit.Binder[brepo.Repo]
	repo   brepo.Repo

	gh   *gh.Client
	cfg  Config
	deps modkit.Deps
}

// New constructs the service
func New(deps modkit.Deps, cfg Config) *Svc {
	b := brepo.NewPG()
	client := gh.NewClient(gh.Options{
		TokensCSV:  cfg.TokensCSV,
		MaxRetries: cfg.MaxAttempts,
		RetryBase:  durationMs(cfg.RetryBaseMs),
	})
	return &Svc{
		db:     deps.PG,
		binder: b,
		repo:   b.Bind(deps.PG),
		gh:     client,
		cfg:    cfg,
		deps:   deps,
	}
}

// EnqueueVerification enqueues a verification job
func (s *Svc) EnqueueVerification(ctx context.Context, in dom.EnqueueArgs) error {
	_, err := s.repo.EnqueueVerification(ctx,
		in.Principal,
		in.Resource,
		in.PrincipalHID,
		in.ChallengeHash,
	)
	return err
}

func durationMs(ms int) time.Duration {
	if ms <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}
