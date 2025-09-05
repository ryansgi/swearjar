package module

import (
	"time"

	"swearjar/internal/platform/config"
)

// Options controls hallmonitor behavior. Values may also be read from env
type Options struct {
	Concurrency int
	RatePerSec  float64
	Burst       int
	TokensCSV   string
	DryRun      bool

	// Seeding/refresh defaults (can be overridden by flags)
	DefaultSeedLimit    int
	DefaultRefreshLimit int

	// DB knobs
	QueueTakeBatch int
	RetryBase      time.Duration
	MaxAttempts    int
}

// FromConfig reads options using HALLMONITOR_ prefix
func FromConfig(cfg config.Conf) Options {
	hm := cfg.Prefix("HALLMONITOR_")
	return Options{
		Concurrency:         hm.MayInt("WORKER_CONCURRENCY", 4),
		RatePerSec:          hm.MayFloat64("GH_RPS", 2.0),
		Burst:               hm.MayInt("GH_BURST", 4),
		TokensCSV:           hm.MayString("GH_TOKENS", ""),
		DryRun:              hm.MayBool("DRYRUN", false),
		DefaultSeedLimit:    hm.MayInt("SEED_LIMIT", 0),
		DefaultRefreshLimit: hm.MayInt("REFRESH_LIMIT", 0),
		QueueTakeBatch:      hm.MayInt("QUEUE_TAKE_BATCH", 64),
		RetryBase:           hm.MayDuration("RETRY_BASE", 500*time.Millisecond),
		MaxAttempts:         hm.MayInt("MAX_ATTEMPTS", 10),
	}
}
