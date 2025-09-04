package module

import (
	"swearjar/internal/platform/config"
)

// Options configures the utterances module
type Options struct {
	HardLimit int
}

// FromConfig reads options from config.Conf
func FromConfig(cfg config.Conf) Options {
	uf := cfg.Prefix("CORE_UTTERANCES_")
	return Options{
		HardLimit: uf.MayInt("HARD_LIMIT", 5000),
	}
}
