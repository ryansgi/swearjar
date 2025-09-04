package module

import "swearjar/internal/platform/config"

// Options holds configuration settings for the hits module
type Options struct {
	HardLimit int
}

// FromConfig reads configuration settings from the config.Conf
func FromConfig(cfg config.Conf) Options {
	hf := cfg.Prefix("CORE_HITS_")
	return Options{
		HardLimit: hf.MayInt("HARD_LIMIT", 100),
	}
}
