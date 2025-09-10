package module

import (
	"time"

	"swearjar/internal/platform/config"
)

// Options controls bouncer behavior and GH client settings
type Options struct {
	Secret string        // HMAC seed for Issue()
	Grace  time.Duration // staleness window for Status()

	// GitHub client
	TokensCSV  string
	BaseURL    string
	UserAgent  string
	Timeout    time.Duration
	MaxRetries int
	RetryBase  time.Duration
}

// FromConfig reads BOUNCER_* values from process config/env
func FromConfig(cfg config.Conf) Options {
	bc := cfg.Prefix("BOUNCER_")
	return Options{
		Secret:     bc.MayString("SECRET", ""),
		Grace:      bc.MayDuration("GRACE", 7*24*time.Hour),
		TokensCSV:  bc.MayString("GH_TOKENS", ""),
		BaseURL:    bc.MayString("GH_BASE_URL", ""),
		UserAgent:  bc.MayString("GH_UA", "swearjar-bouncer"),
		Timeout:    bc.MayDuration("GH_TIMEOUT", 10*time.Second),
		MaxRetries: bc.MayInt("GH_MAX_RETRIES", 5),
		RetryBase:  bc.MayDuration("GH_RETRY_BASE", 500*time.Millisecond),
	}
}
