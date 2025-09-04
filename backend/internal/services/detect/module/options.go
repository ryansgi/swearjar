package module

import "swearjar/internal/platform/config"

// Options holds configuration settings for the detect module
type Options struct {
	Version       int
	Workers       int
	PageSize      int
	MaxRangeHours int
	DryRun        bool
}

// FromConfig extracts Options from the given config.Conf
func FromConfig(cfg config.Conf) Options {
	df := cfg.Prefix("CORE_DETECT_")
	return Options{
		Version:       df.MayInt("VERSION", 1),
		Workers:       df.MayInt("WORKERS", 2),
		PageSize:      df.MayInt("PAGE_SIZE", 5000),
		MaxRangeHours: df.MayInt("MAX_RANGE_HOURS", 0),
		DryRun:        df.MayBool("DRY_RUN", false),
	}
}
