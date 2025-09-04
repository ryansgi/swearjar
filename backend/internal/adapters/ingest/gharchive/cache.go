package gharchive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
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
	return c
}

// Fetch returns a reader for the gzip file for the given hour
// Serves from disk when present and may revalidate recent hours
func (c *CachedFetcher) Fetch(ctx context.Context, hour HourRef) (io.ReadCloser, error) {
	filename := hour.String() + ".json.gz"
	path := filepath.Join(c.dir, filename)
	metaPath := path + ".meta"

	// file exists path
	if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
		// revalidate only for recent hours
		if c.shouldRevalidate(hour) {
			rc, _, err := c.tryConditionalFetch(ctx, hour, path, metaPath)
			if err == nil && rc != nil {
				c.maybeCleanup()
				return rc, nil
			}
			// best effort fallback to local file
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		c.maybeCleanup()
		return f, nil
	}

	// cache miss path
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
		f, err := os.Open(path)
		return f, true, err

	case http.StatusOK:
		// overwrite cache with new bytes

		rc, err := c.writeResponseToCache(resp, path, metaPath, true)
		return rc, false, err

	default:
		// unexpected status, fall back to local file when present
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
	url := fmt.Sprintf("%s/%s.json.gz", baseURL, hour.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if cerr := resp.Body.Close(); cerr != nil {
			return nil, fmt.Errorf("gharchive: unexpected status %d for %s (body close err: %v)", resp.StatusCode, url, cerr)
		}
		return nil, fmt.Errorf("gharchive: unexpected status %d for %s", resp.StatusCode, url)
	}
	// on first download we return a reader that does not expose Name to keep cache hit metrics correct
	rc, err := c.writeResponseToCache(resp, path, metaPath, !markAsHit)
	if err != nil {
		return nil, err
	}
	c.maybeCleanup()
	return rc, nil
}

// writeResponseToCache saves body atomically and writes meta then returns a reader
// When wrapNoHit is true the returned reader will not expose Name to callers
func (c *CachedFetcher) writeResponseToCache(
	resp *http.Response,
	path string,
	metaPath string,
	wrapNoHit bool,
) (io.ReadCloser, error) {
	tmp := path + ".part"
	defer func() {
		if err := os.Remove(tmp); err != nil {
			fmt.Fprintf(os.Stderr, "error closing file: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "error closing file: %v\n", cerr)
		}
	}()
	var m cacheMeta
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// saveMeta writes the sidecar json atomically
func saveMeta(path string, m *cacheMeta) error {
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(m); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
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
	_ = c.cleanupOnce()
}

// cleanupOnce applies age and size retention
func (c *CachedFetcher) cleanupOnce() error {
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
			continue
		}
		items = append(items, item{Path: full, Size: fi.Size(), HourTS: hr})
		total += fi.Size()
	}

	if c.retainMaxBytes > 0 && total > c.retainMaxBytes {
		sort.Slice(items, func(i, j int) bool { return items[i].HourTS.Before(items[j].HourTS) })
		for _, it := range items {
			if total <= c.retainMaxBytes {
				break
			}
			_ = os.Remove(it.Path)
			_ = os.Remove(it.Path + ".meta")
			total -= it.Size
		}
	}
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
