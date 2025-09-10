package module

import (
	"time"

	"swearjar/internal/platform/config"
)

// Options controls the bouncer verifier worker
type Options struct {
	Concurrency    int
	RatePerSec     float64
	Burst          int
	TokensCSV      string
	QueueTakeBatch int
	RetryBaseMs    int
	MaxAttempts    int
}

// FromConfig reads with BOUNCER_ prefix (parity with HM)
func FromConfig(cfg config.Conf) Options {
	c := cfg.Prefix("BOUNCER_")
	return Options{
		Concurrency:    c.MayInt("WORKER_CONCURRENCY", 4),
		RatePerSec:     c.MayFloat64("GH_RPS", 2.0),
		Burst:          c.MayInt("GH_BURST", 4),
		TokensCSV:      c.MayString("GH_TOKENS", ""),
		QueueTakeBatch: c.MayInt("QUEUE_TAKE_BATCH", 64),
		RetryBaseMs:    int(c.MayDuration("RETRY_BASE", 500*time.Millisecond).Milliseconds()),
		MaxAttempts:    c.MayInt("MAX_ATTEMPTS", 10),
	}
}
