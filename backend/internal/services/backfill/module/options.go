package module

import (
	"time"

	"swearjar/internal/platform/config"
)

// Options holds configuration options for the backfill service
type Options struct {
	DelayPerHour  time.Duration
	Workers       int
	MaxRetries    int
	RetryBase     time.Duration
	FetchTimeout  time.Duration
	ReadTimeout   time.Duration
	MaxRangeHours int
	EnableLeases  bool
}

// FromConfig reads the backfill options from config with CORE_BACKFILL_ prefix
func FromConfig(cfg config.Conf) Options {
	bf := cfg.Prefix("CORE_BACKFILL_")
	return Options{
		DelayPerHour:  bf.MayDuration("DELAY", 0),
		Workers:       bf.MayInt("WORKERS", 2),
		MaxRetries:    bf.MayInt("RETRIES", 3),
		RetryBase:     bf.MayDuration("RETRY_BASE", 500*time.Millisecond),
		FetchTimeout:  bf.MayDuration("FETCH_TIMEOUT", 60*time.Second),
		ReadTimeout:   bf.MayDuration("READ_TIMEOUT", 10*time.Minute),
		MaxRangeHours: bf.MayInt("MAX_RANGE_HOURS", 0),
		EnableLeases:  bf.MayBool("LEASES", true),
	}
}
