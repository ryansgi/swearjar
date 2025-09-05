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

	// Detect integration
	DetectEnabled bool
	DetectVersion int
	DetectDryRun  bool
}

// FromConfig reads the backfill options from config with CORE_BACKFILL_ prefix
func FromConfig(cfg config.Conf) Options {
	bf := cfg.Prefix("CORE_BACKFILL_")
	return Options{
		DelayPerHour:  bf.MayDuration("DELAY", 0),
		Workers:       bf.MayInt("WORKERS", 4),
		MaxRetries:    bf.MayInt("RETRIES", 3),
		RetryBase:     bf.MayDuration("RETRY_BASE", 500*time.Millisecond),
		FetchTimeout:  bf.MayDuration("FETCH_TIMEOUT", 10*time.Minute), // was 60s
		ReadTimeout:   bf.MayDuration("READ_TIMEOUT", 10*time.Minute),
		MaxRangeHours: bf.MayInt("MAX_RANGE_HOURS", 0),
		EnableLeases:  bf.MayBool("LEASES", true),
		DetectEnabled: bf.MayBool("DETECT", false),
		DetectVersion: bf.MayInt("DET_VERSION", 1),
		DetectDryRun:  bf.MayBool("DET_DRY_RUN", false),
	}
}
