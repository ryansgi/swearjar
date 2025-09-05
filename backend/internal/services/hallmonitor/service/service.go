// Package service contains hallmonitor workflows
package service

import (
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/hallmonitor/domain"
	"swearjar/internal/services/hallmonitor/repo"

	gh "swearjar/internal/adapters/ingest/github"
)

// Service defines the hallmonitor service contract
type Service interface {
	// ports aggregated for convenience
	domainPorts
}

// domainPorts keeps the interface grouping local
type domainPorts interface {
	// these are satisfied by Svc
	domain.WorkerPort
	domain.SeederPort
	domain.RefresherPort
	domain.SignalsPort
	domain.ReaderPort
}

// CadenceConfig controls refresh schedules
type CadenceConfig struct {
	// repo cadence
	RepoHighStars int
	RepoMidStars  int
	RepoHighEvery time.Duration
	RepoMidEvery  time.Duration
	RepoLowEvery  time.Duration
	RepoPushMin   time.Duration

	// actor cadence
	ActorHighFollowers int
	ActorHighEvery     time.Duration
	ActorLowEvery      time.Duration
}

// Config carries runtime knobs for the worker and batch ops
type Config struct {
	Concurrency         int
	RatePerSec          float64
	Burst               int
	TokensCSV           string
	DryRun              bool
	DefaultSeedLimit    int
	DefaultRefreshLimit int
	QueueTakeBatch      int
	RetryBaseMs         int
	MaxAttempts         int
	Cadence             CadenceConfig
}

// Svc implements the hallmonitor service
type Svc struct {
	Repo   repo.Repo
	binder repokit.Binder[repo.Repo]
	db     repokit.TxRunner
	deps   modkit.Deps
	config Config
	gh     *gh.Client
}

// New constructs a hallmonitor service
func New(deps modkit.Deps, cfg Config) *Svc {
	if deps.PG == nil {
		panic("hallmonitor.Service requires a non nil TxRunner")
	}
	cfg = withCadenceDefaults(cfg)

	b := repo.NewPG()
	client := gh.NewClient(gh.Options{
		TokensCSV:  cfg.TokensCSV,
		MaxRetries: cfg.MaxAttempts,
		RetryBase:  durationMs(cfg.RetryBaseMs),
	})

	return &Svc{
		Repo:   b.Bind(deps.PG),
		binder: b,
		db:     deps.PG,
		deps:   deps,
		config: cfg,
		gh:     client,
	}
}

func durationMs(ms int) time.Duration {
	if ms <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// withCadenceDefaults fills zero values with sane defaults that match the mandate today
func withCadenceDefaults(cfg Config) Config {
	c := cfg.Cadence

	if c.RepoHighStars == 0 {
		c.RepoHighStars = 1000
	}
	if c.RepoMidStars == 0 {
		c.RepoMidStars = 100
	}
	if c.RepoHighEvery == 0 {
		c.RepoHighEvery = 7 * 24 * time.Hour
	}
	if c.RepoMidEvery == 0 {
		c.RepoMidEvery = 14 * 24 * time.Hour
	}
	if c.RepoLowEvery == 0 {
		c.RepoLowEvery = 30 * 24 * time.Hour
	}
	if c.RepoPushMin == 0 {
		c.RepoPushMin = 24 * time.Hour
	}

	if c.ActorHighFollowers == 0 {
		c.ActorHighFollowers = 1000
	}
	if c.ActorHighEvery == 0 {
		c.ActorHighEvery = 30 * 24 * time.Hour
	}
	if c.ActorLowEvery == 0 {
		c.ActorLowEvery = 60 * 24 * time.Hour
	}

	cfg.Cadence = c
	return cfg
}
