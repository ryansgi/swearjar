package module

import (
	"time"

	"swearjar/internal/platform/config"
)

// Options for Nightshift module
type Options struct {
	Workers         int
	DetectorVersion int
	RetentionMode   string
	EnableLeases    bool
	LeaseTTL        time.Duration
}

// FromConfig fills options from environment
// CORE_NIGHTSHIFT_WORKERS (default 2) is the number of concurrent workers
// CORE_NIGHTSHIFT_DET_VERSION (default 1) is the detector version to stamp archives/rollups with
// CORE_NIGHTSHIFT_RETENTION_MODE (default "full") is the retention mode to apply: "full", "aggressive", "timebox:Nd"
// CORE_NIGHTSHIFT_LEASES (default true) enables the advisory lock around hour processing
func FromConfig(cfg config.Conf) Options {
	n := cfg.Prefix("CORE_NIGHTSHIFT_")
	return Options{
		Workers:         n.MayInt("WORKERS", 2),
		DetectorVersion: n.MayInt("DET_VERSION", 1),
		RetentionMode:   n.MayString("RETENTION_MODE", "full"),
		EnableLeases:    n.MayBool("LEASES", true),
		LeaseTTL:        n.MayDuration("LEASE_TTL", 3*time.Minute),
	}
}
