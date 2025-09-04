// Package version provides information about the build version of the service.
package version

// BuildInfo holds version information about the service build.
type BuildInfo struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Info returns the build information. The version, commit, and date variables
// are intended to be set at build time using -ldflags.
func Info() BuildInfo {
	// Set via -ldflags "-X 'swearjar/internal/version.version=v0.0.1'
	// -X 'swearjar/internal/version.commit=abcd' -X 'swearjar/internal/version.date=2025-09-02'"
	return BuildInfo{
		Service: "swearjar-api",
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
