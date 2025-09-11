// Package ch provides a minimal ClickHouse client using clickhouse-go
package ch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config configures the client.
// URL examples:
//   - clickhouse://user:pass@host:9000/db            (native)
//   - http://user:pass@host:8123/db                  (http)
//   - https://user:pass@host:8443/db?skip_verify=1   (https, skip TLS verify)
//
// Optional query params: dial_timeout, read_timeout (Go durations)
type Config struct {
	URL string
}

// Rows is the minimal result-set surface exposed by this package
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
	Columns() []string
}

// CH is a concrete client
type CH struct {
	conn clickhouse.Conn
}

// Open establishes the connection and pings the server
func Open(ctx context.Context, cfg Config) (*CH, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("ch: empty URL")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("ch: parse url: %w", err)
	}
	opts, err := optsFromURL(u)
	if err != nil {
		return nil, err
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("ch: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ch: ping: %w", err)
	}
	return &CH{conn: conn}, nil
}

// Insert inserts rows into table
func (c *CH) Insert(ctx context.Context, table string, rows [][]any) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("ch: nil client")
	}
	if strings.TrimSpace(table) == "" {
		return fmt.Errorf("ch: empty table")
	}
	if len(rows) == 0 {
		return nil
	}

	// Allow DSN override: ?insert_chunk=5000 (default 5000)
	chunk := 5000
	if v := c.insertChunkSize(); v > 0 {
		chunk = v
	}

	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		if err := c.insertChunkWithRetry(ctx, table, rows[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (c *CH) insertChunkWithRetry(ctx context.Context, table string, rows [][]any) error {
	const (
		maxAttempts = 3
		baseDelay   = 200 * time.Millisecond
	)
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.insertChunk(ctx, table, rows); err != nil {
			last = err
			if !isEOFish(err) || attempt == maxAttempts {
				return err
			}
			time.Sleep(baseDelay * time.Duration(attempt))
			continue
		}
		return nil
	}
	return last
}

func (c *CH) insertChunk(ctx context.Context, table string, rows [][]any) error {
	stmt := "INSERT INTO " + table + " VALUES"
	batch, err := c.conn.PrepareBatch(ctx, stmt)
	if err != nil {
		return fmt.Errorf("ch: prepare: %w", err)
	}
	for i := range rows {
		if err := batch.Append(rows[i]...); err != nil {
			return fmt.Errorf("ch: append row %d: %w", i, err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("ch: send: %w", err)
	}
	return nil
}

func isEOFish(err error) bool {
	if err == nil {
		return false
	}
	// Driver wraps, so unwrap and string-match is pragmatic
	if errors.Is(err, io.EOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "server closed idle connection") ||
		strings.Contains(s, "use of closed network connection")
}

// parse ?insert_chunk= from the DSN used to open this CH
func (c *CH) insertChunkSize() int {
	// Best-effort: get url from conn info not exposed by driver;
	// fall back to env/known defaults. Since Options isn't held,
	// we can also parse from CLICKHOUSE_URL in env if you prefer.
	// For now: read once from env var used by your config, if present
	if u := os.Getenv("SERVICE_CLICKHOUSE_DBURL"); u != "" {
		if chunkStr := getQueryParam(u, "insert_chunk"); chunkStr != "" {
			if n, err := strconv.Atoi(chunkStr); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

func getQueryParam(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// Query executes sql with args and returns rows
func (c *CH) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("ch: nil client")
	}
	r, err := c.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &rows{r}, nil
}

// Close closes the connection
func (c *CH) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// rows adapts driver.Rows to our local Rows via anonymous embedding.
// This promotes Columns/Next/Scan/Err/Close from driver.Rows
type rows struct{ driver.Rows }

func (r *rows) Close() error { return r.Rows.Close() }

func optsFromURL(u *url.URL) (*clickhouse.Options, error) {
	addrs := []string{u.Host}
	if u.Host == "" {
		return nil, fmt.Errorf("ch: missing host in URL")
	}

	qs := u.Query()

	// --- Auth & DB ------------------------------------------------------------
	// Prefer userinfo, but allow query fallbacks (?username=, ?password=, ?database=)
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	if user == "" {
		if v := qs.Get("username"); v != "" {
			user = v
		} else if v := qs.Get("user"); v != "" {
			user = v
		}
	}
	if pass == "" {
		if v := qs.Get("password"); v != "" {
			pass = v
		} else if v := qs.Get("key"); v != "" { // ClickHouse Cloud often calls this "key"
			pass = v
		}
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = qs.Get("database")
	}

	// --- Timeouts -------------------------------------------------------------
	dialTO := 5 * time.Second
	readTO := 0 * time.Second
	if v := qs.Get("dial_timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			dialTO = d
		}
	}
	if v := qs.Get("read_timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			readTO = d
		}
	}
	// (WriteTimeout not supported in v2 options)

	// --- TLS / Protocol -------------------------------------------------------
	secure := u.Scheme == "https" || qs.Get("secure") == "true"
	skipVerify := qs.Get("skip_verify") == "1" || qs.Get("skip_verify") == "true"

	var tlsCfg *tls.Config
	if secure {
		tlsCfg = &tls.Config{InsecureSkipVerify: skipVerify}
	}

	proto := clickhouse.Native
	if u.Scheme == "http" || u.Scheme == "https" {
		proto = clickhouse.HTTP
	}

	// --- Dialer adapter (addr-only) ------------------------------------------
	d := &net.Dialer{Timeout: dialTO}
	dialFn := func(ctx context.Context, addr string) (net.Conn, error) {
		return d.DialContext(ctx, "tcp", addr)
	}

	return &clickhouse.Options{
		Addr:        addrs,
		Protocol:    proto,
		TLS:         tlsCfg,
		Auth:        clickhouse.Auth{Database: db, Username: user, Password: pass},
		DialTimeout: dialTO,
		ReadTimeout: readTO,
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct{ Name, Version string }{{Name: "swearjar", Version: "v0"}},
		},
		Debugf: func(format string, args ...any) {
			// temporary; wire to your zerolog if you prefer
			fmt.Printf("ch: "+format+"\n", args...)
		},
		DialContext: dialFn,
		Settings: clickhouse.Settings{
			"max_execution_time":               0,
			"allow_experimental_nlp_functions": 1,
			"max_insert_block_size":            10000,
		},
	}, nil
}
