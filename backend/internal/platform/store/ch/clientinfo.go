package ch

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// BuildClientInfo returns a ClientInfo describing this process and role
// role examples: "api", "backfill", "detect", "hallmonitor"
func BuildClientInfo(role, tag string) clickhouse.ClientInfo {
	host, _ := os.Hostname()
	sha := vcsShortSHA()
	gover := runtime.Version()

	type kv = struct{ Name, Version string }

	products := []kv{
		{Name: "swearjar", Version: safe(tag)},
		{Name: "role", Version: safe(role)},
		{Name: "go", Version: safe(gover)},
		{Name: "commit", Version: safe(sha)},
		{Name: "host", Version: safe(host)},
	}

	return clickhouse.ClientInfo{Products: products}
}

func vcsShortSHA() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi != nil {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return s.Value[:7]
			}
		}
	}
	return "unknown"
}

func safe(s string) string {
	// Defensive: ClickHouse client info is lightweight; keep it tidy
	return strings.TrimSpace(s)
}
