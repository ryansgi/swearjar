// Package ingest holds adapter shims for backfill ingest ports.
package ingest

import (
	"context"
	"io"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/services/backfill/domain"

	"swearjar/internal/adapters/ingest/gharchive"
)

// fetcher implements domain.Fetcher using the cached GH Archive fetcher
type fetcher struct {
	f gharchive.Fetcher
}

// NewFetcher constructs a domain.Fetcher from config under CORE_INGEST_*.
// This keeps config-reading outside service and avoids passing platform deps into repos
func NewFetcher(deps modkit.Deps) domain.Fetcher {
	ing := deps.Cfg.Prefix("CORE_INGEST_")

	cacheDir := ing.MustString("CACHE_DIR")
	refreshH := time.Duration(ing.MayInt("REFRESH_RECENT_HOURS", 0)) * time.Hour
	retainDays := ing.MayInt("RETAIN_MAX_DAYS", 0)
	retainBytes := int64(ing.MayInt("RETAIN_MAX_BYTES", 0))

	httpTO := time.Duration(ing.MayInt("HTTP_TIMEOUT_SECONDS", 0)) * time.Second // 0 == no client timeout

	return &fetcher{
		f: gharchive.NewCachedFetcher(
			cacheDir,
			gharchive.NewHTTPFetcherWithTimeout(httpTO),
			gharchive.WithRefreshRecent(refreshH),
			gharchive.WithRetention(time.Duration(retainDays)*24*time.Hour, retainBytes),
		),
	}
}

func (f *fetcher) Fetch(ctx context.Context, hr domain.HourRef) (io.ReadCloser, error) {
	// Translate to the gharchive.HourRef and delegate
	return f.f.Fetch(
		ctx,
		gharchive.NewHourRef(
			time.Date(
				hr.Year,
				time.Month(hr.Month),
				hr.Day,
				hr.Hour,
				0, 0, 0, time.UTC,
			),
		),
	)
}
