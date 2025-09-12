package gharchive

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"swearjar/internal/platform/logger"
)

// CachedFetcher fetches GH Archive hours with on disk caching
// Local dir holds one .json.gz per hour plus a .meta sidecar
// Supports conditional GET for recent hours using ETag and Last Modified
// Optional retention by max age and total bytes
type CachedFetcher struct {
	dir             string
	client          *http.Client
	refreshRecent   time.Duration
	retainMaxAge    time.Duration
	retainMaxBytes  int64
	lastCleanupUnix atomic.Int64
}

// cacheMeta is a tiny sidecar json with fields we actually use
type cacheMeta struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	Size         int64     `json:"size,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	LastChecked  time.Time `json:"last_checked"`
}

// CachedOption configures the fetcher
type CachedOption func(*CachedFetcher)

// WithRefreshRecent enables conditional GET for hours within d of now
func WithRefreshRecent(d time.Duration) CachedOption {
	return func(c *CachedFetcher) { c.refreshRecent = d }
}

// WithRetention sets optional age and size retention
// Pass zero to disable either dimension
func WithRetention(maxAge time.Duration, maxBytes int64) CachedOption {
	return func(c *CachedFetcher) {
		c.retainMaxAge = maxAge
		c.retainMaxBytes = maxBytes
	}
}

// NewCachedFetcher builds a fetcher with local caching dir is required
// base may be nil and its client is reused when present
func NewCachedFetcher(dir string, base *HTTPFetcher, opts ...CachedOption) *CachedFetcher {
	_ = os.MkdirAll(dir, 0o755)
	c := &CachedFetcher{
		dir:    dir,
		client: &http.Client{Timeout: defaultHTTPTO},
	}
	if base != nil && base.Client != nil {
		c.client = base.Client
	}
	for _, o := range opts {
		o(c)
	}

	logger.Named("gharchive").Debug().
		Str("cache_dir", dir).
		Dur("refresh_recent", c.refreshRecent).
		Dur("retain_max_age", c.retainMaxAge).
		Int64("retain_max_bytes", c.retainMaxBytes).
		Msg("gharchive: cached fetcher initialized")

	return c
}

// Fetch returns a reader for the gzip file for the given hour
// Serves from disk when present and may revalidate recent hours
func (c *CachedFetcher) Fetch(ctx context.Context, hour HourRef) (io.ReadCloser, error) {
	l := logger.C(ctx)

	filename := hour.String() + ".json.gz"
	path := filepath.Join(c.dir, filename)
	metaPath := path + ".meta"

	// file exists path
	if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
		l.Debug().
			Str("hour", hour.String()).
			Str("path", path).
			Int64("size_bytes", fi.Size()).
			Msg("gharchive: cache hit")

		// revalidate only for recent hours
		if c.shouldRevalidate(hour) {
			l.Debug().
				Str("hour", hour.String()).
				Msg("gharchive: attempting conditional revalidation")
			rc, fromCache, err := c.tryConditionalFetch(ctx, hour, path, metaPath)
			if err == nil && rc != nil {
				if fromCache {
					l.Debug().Str("hour", hour.String()).Msg("gharchive: revalidation 304, reading from cache")
				} else {
					l.Debug().Str("hour", hour.String()).Msg("gharchive: revalidation 200, refreshed cache and streaming")
				}
				c.maybeCleanup()
				return rc, nil
			}
			if err != nil {
				l.Debug().
					Str("hour", hour.String()).
					Err(err).
					Msg("gharchive: conditional fetch failed, falling back to local file")
			}
		}

		f, err := os.Open(path)
		if err != nil {
			l.Error().Str("path", path).Err(err).Msg("gharchive: failed to open cached file")
			return nil, err
		}
		l.Debug().Str("hour", hour.String()).Msg("gharchive: reading from cache")
		c.maybeCleanup()
		return f, nil
	}

	// cache miss path
	l.Debug().
		Str("hour", hour.String()).
		Str("path", path).
		Msg("gharchive: cache miss, downloading")
	return c.downloadAndStore(ctx, hour, path, metaPath, false)
}

func (c *CachedFetcher) shouldRevalidate(hour HourRef) bool {
	if c.refreshRecent <= 0 {
		return false
	}
	hrTime := time.Date(hour.Year, time.Month(hour.Month), hour.Day, hour.Hour, 0, 0, 0, time.UTC)
	return time.Since(hrTime) <= c.refreshRecent
}

// tryConditionalFetch issues a GET with If None Match and If Modified Since when available.
// Returns a reader from cache on 304 or a fresh reader after writing cache on 200
func (c *CachedFetcher) tryConditionalFetch(
	ctx context.Context,
	hour HourRef,
	path string,
	metaPath string,
) (io.ReadCloser, bool, error) {
	l := logger.C(ctx)

	url := fmt.Sprintf("%s/%s.json.gz", baseURL, hour.String())

	meta, _ := loadMeta(metaPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	if meta != nil {
		if meta.ETag != "" {
			req.Header.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			req.Header.Set("If-Modified-Since", meta.LastModified)
		}
	}

	l.Debug().
		Str("hour", hour.String()).
		Str("url", url).
		Str("if_none_match", req.Header.Get("If-None-Match")).
		Str("if_modified_since", req.Header.Get("If-Modified-Since")).
		Msg("gharchive: conditional GET")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if resp != nil && resp.Body != nil && resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
		}
	}()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if meta == nil {
			meta = &cacheMeta{}
		}
		meta.LastChecked = time.Now().UTC()
		_ = saveMeta(metaPath, meta)
		l.Debug().
			Str("hour", hour.String()).
			Str("path", path).
			Msg("gharchive: 304 Not Modified, serving cached file")
		f, err := os.Open(path)
		return f, true, err

	case http.StatusOK:
		// overwrite cache with new bytes
		l.Debug().
			Str("hour", hour.String()).
			Str("url", url).
			Msg("gharchive: 200 OK, writing new bytes to cache")
		rc, err := c.writeResponseToCache(ctx, resp, path, metaPath, true)
		return rc, false, err

	default:
		// unexpected status, fall back to local file when present
		l.Debug().
			Str("hour", hour.String()).
			Str("url", url).
			Int("status", resp.StatusCode).
			Msg("gharchive: unexpected status on conditional GET")
		if _, err := os.Stat(path); err == nil {
			f, oerr := os.Open(path)
			return f, true, oerr
		}
		return nil, false, fmt.Errorf("gharchive: unexpected status %d for %s", resp.StatusCode, url)
	}
}

func (c *CachedFetcher) downloadAndStore(
	ctx context.Context,
	hour HourRef,
	path string,
	metaPath string,
	markAsHit bool,
) (io.ReadCloser, error) {
	l := logger.C(ctx)

	url := fmt.Sprintf("%s/%s.json.gz", baseURL, hour.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	l.Debug().Str("hour", hour.String()).Str("url", url).Msg("gharchive: starting download")
	resp, err := c.client.Do(req)
	if err != nil {
		l.Error().Str("url", url).Err(err).Msg("gharchive: download request failed")
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if cerr := resp.Body.Close(); cerr != nil {
			return nil, fmt.Errorf("gharchive: unexpected status %d for %s (body close err: %v)", resp.StatusCode, url, cerr)
		}
		return nil, fmt.Errorf("gharchive: unexpected status %d for %s", resp.StatusCode, url)
	}
	// on first download we return a reader that does not expose Name to keep cache hit metrics correct
	rc, err := c.writeResponseToCache(ctx, resp, path, metaPath, !markAsHit)
	if err != nil {
		l.Error().Str("hour", hour.String()).Err(err).Msg("gharchive: failed writing response to cache")
		return nil, err
	}
	l.Debug().Str("hour", hour.String()).Msg("gharchive: download complete")
	c.maybeCleanup()
	return rc, nil
}

// writeResponseToCache saves body atomically and writes meta then returns a reader
// When wrapNoHit is true the returned reader will not expose Name to callers
func (c *CachedFetcher) writeResponseToCache(
	ctx context.Context,
	resp *http.Response,
	path string,
	metaPath string,
	wrapNoHit bool,
) (io.ReadCloser, error) {
	l := logger.C(ctx)

	tmp := path + ".part"
	defer func() {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			// best-effort cleanup; log as a warning if anything *other* than not-exist
			logger.Named("gharchive").Debug().
				Str("tmp", tmp).
				Err(err).
				Msg("gharchive: cache cleanup warning removing temp file")
		}
	}()

	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	out, err := os.Create(tmp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	n, werr := io.Copy(out, resp.Body)
	cerr := out.Close()
	_ = resp.Body.Close()
	if werr != nil {
		return nil, werr
	}
	if cerr != nil {
		return nil, cerr
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}

	meta := &cacheMeta{
		ETag:         strings.TrimSpace(resp.Header.Get("ETag")),
		LastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
		Size:         n,
		FetchedAt:    time.Now().UTC(),
		LastChecked:  time.Now().UTC(),
	}
	_ = saveMeta(metaPath, meta)

	l.Debug().
		Str("path", path).
		Int64("size_bytes", n).
		Str("etag", meta.ETag).
		Str("last_modified", meta.LastModified).
		Msg("gharchive: cached file written")

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if wrapNoHit {
		return &fileBody{f: f}, nil
	}
	return f, nil
}

// fileBody wraps *os.File but hides the Name method
type fileBody struct{ f *os.File }

func (b *fileBody) Read(p []byte) (int, error) { return b.f.Read(p) }
func (b *fileBody) Close() error               { return b.f.Close() }

// loadMeta reads a sidecar json file
func loadMeta(path string) (*cacheMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Named("gharchive").Debug().
				Str("path", path).
				Err(cerr).
				Msg("gharchive: error closing meta file")
		}
	}()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var m cacheMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// saveMeta writes the sidecar json atomically
func saveMeta(path string, m *cacheMeta) error {
	tmp := path + ".part"

	b, err := json.Marshal(m) // or MarshalIndent(m, "", "  ") if you want pretty
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// maybeCleanup throttles retention cleanup to once per ten minutes
func (c *CachedFetcher) maybeCleanup() {
	now := time.Now().Unix()
	last := c.lastCleanupUnix.Load()
	if last != 0 && now-last < 600 {
		return
	}
	if c.retainMaxAge <= 0 && c.retainMaxBytes <= 0 {
		return
	}
	if !c.lastCleanupUnix.CompareAndSwap(last, now) {
		return
	}
	if err := c.cleanupOnce(); err != nil {
		logger.Named("gharchive").Debug().Err(err).Msg("gharchive: retention cleanup failed (ignored)")
	}
}

// cleanupOnce applies age and size retention
func (c *CachedFetcher) cleanupOnce() error {
	l := logger.Named("gharchive").With().Logger()
	start := time.Now()

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	type item struct {
		Path   string
		Size   int64
		HourTS time.Time
	}
	var items []item
	var total int64
	cutoff := time.Now().Add(-c.retainMaxAge)

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json.gz") {
			continue
		}
		full := filepath.Join(c.dir, name)
		fi, err := os.Stat(full)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		hr, ok := parseHourFromName(name)
		if !ok {
			continue
		}
		if c.retainMaxAge > 0 && hr.Before(cutoff) {
			_ = os.Remove(full)
			_ = os.Remove(full + ".meta")
			l.Debug().Str("path", full).Msg("gharchive: retention (age) removed file")
			continue
		}
		items = append(items, item{Path: full, Size: fi.Size(), HourTS: hr})
		total += fi.Size()
	}

	removed := int64(0)
	if c.retainMaxBytes > 0 && total > c.retainMaxBytes {
		sort.Slice(items, func(i, j int) bool { return items[i].HourTS.Before(items[j].HourTS) })
		for _, it := range items {
			if total <= c.retainMaxBytes {
				break
			}
			_ = os.Remove(it.Path)
			_ = os.Remove(it.Path + ".meta")
			total -= it.Size
			removed += it.Size
			l.Debug().Str("path", it.Path).Msg("gharchive: retention (size) removed file")
		}
	}

	l.Debug().
		Dur("took", time.Since(start)).
		Int("files_scanned", len(entries)).
		Int64("bytes_removed", removed).
		Int64("bytes_remaining", total).
		Msg("gharchive: retention cleanup complete")

	return nil
}

// parseHourFromName parses YYYY MM DD H from the filename
func parseHourFromName(name string) (time.Time, bool) {
	base := strings.TrimSuffix(name, ".json.gz")
	t, err := time.Parse("2006-01-02-15", base)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
