package gharchive

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"swearjar/internal/platform/logger"
)

const (
	baseURL          = "https://data.gharchive.org"
	defaultHTTPTO    = 0
	maxScanTokenSize = 32 * 1024 * 1024
	sampleRawMax     = 2048 // max bytes of raw JSON to log for the sample
)

// Fetcher fetches a reader for a given hour
type Fetcher interface {
	Fetch(ctx context.Context, hour HourRef) (io.ReadCloser, error)
}

// HTTPFetcher fetches directly from gharchive.org
type HTTPFetcher struct {
	Client *http.Client
}

// NewHTTPFetcherWithTimeout creates a new HTTPFetcher with default settings
func NewHTTPFetcherWithTimeout(d time.Duration) *HTTPFetcher {
	return &HTTPFetcher{Client: &http.Client{Timeout: d}}
}

// Fetch returns a reader for the gzip file for the given hour
func (f *HTTPFetcher) Fetch(ctx context.Context, hour HourRef) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/%s.json.gz", baseURL, hour.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			return nil, fmt.Errorf(
				"gharchive: unexpected status %d for %s; error closing body: %v",
				resp.StatusCode, url, closeErr,
			)
		}
		return nil, fmt.Errorf("gharchive: unexpected status %d for %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

// Reader streams EventEnvelope items from a gzip file
type Reader struct {
	r       io.ReadCloser
	gz      *gzip.Reader
	sc      *bufio.Scanner
	err     error
	events  int
	bytes   int64
	sampled bool // logs exactly one sample raw line per gzip
}

// NewReader creates a new Reader from the given ReadCloser
func NewReader(r io.ReadCloser) (*Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		if cerr := r.Close(); cerr != nil {
			return nil, cerr
		}
		return nil, err
	}
	sc := bufio.NewScanner(gz)
	buf := make([]byte, 512*1024)
	sc.Buffer(buf, maxScanTokenSize)
	return &Reader{r: r, gz: gz, sc: sc}, nil
}

// Next reads the next event; returns io.EOF when done
func (rd *Reader) Next() (EventEnvelope, error) {
	if rd.err != nil {
		return EventEnvelope{}, rd.err
	}
	for {
		if !rd.sc.Scan() {
			if err := rd.sc.Err(); err != nil {
				rd.err = err
				return EventEnvelope{}, err
			}
			rd.err = io.EOF
			return EventEnvelope{}, io.EOF
		}
		line := rd.sc.Bytes()
		cp := make([]byte, len(line))
		copy(cp, line)

		var env EventEnvelope
		if err := json.Unmarshal(cp, &env); err != nil {
			// skip malformed lines
			continue
		}
		rd.events++
		rd.bytes += int64(len(cp) + 1) // include newline

		// Log a single raw-line sample (first valid JSON line in this gzip)
		if !rd.sampled {
			rd.sampled = true
			l := logger.Named("gharchive")
			l.Debug().
				Int("line_bytes", len(cp)).
				Str("sample_raw", truncateUTF8(cp, sampleRawMax)).
				Msg("gharchive: sample raw line")
		}

		return env, nil
	}
}

// Close closes the underlying reader
func (rd *Reader) Close() error {
	var first error
	if rd.gz != nil {
		if err := rd.gz.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			first = err
		}
	}
	if rd.r != nil {
		if err := rd.r.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Stats returns the number of events parsed and total uncompressed bytes read so far
func (rd *Reader) Stats() (events int, bytes int64) {
	return rd.events, rd.bytes
}

// truncateUTF8 returns a string made from b, truncated to at most max bytes,
// backing up to a UTF-8 boundary if needed, and appending an ellipsis if truncated
func truncateUTF8(b []byte, max int) string {
	if max <= 0 || len(b) <= max {
		return string(b)
	}
	i := max
	// back up to the start of a rune (0b10xxxxxx indicates continuation byte)
	for i > 0 && (b[i]&0xC0) == 0x80 {
		i--
	}
	if i <= 0 {
		i = max
	}
	return string(b[:i]) + "..."
}
