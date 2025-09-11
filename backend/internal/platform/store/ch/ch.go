// Package ch provides a minimal ClickHouse client using clickhouse-go
package ch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"swearjar/internal/platform/logger"
)

// QueryEvent describes a single CH operation (query/insert)
type QueryEvent struct {
	SQL       string
	Args      any
	ElapsedUS int64
	Err       error
	Slow      bool
	Op        string // "query" or "insert"
}

// QueryTracer receives CH events (mirrors pg.QueryTracer)
type QueryTracer interface {
	OnQuery(ctx context.Context, ev QueryEvent)
}

// Tracer returns a logger-backed tracer (same idea as pg.Tracer)
func Tracer(root logger.Logger) QueryTracer {
	ll := root.With().Str("component", "ch").Logger()
	return &zlTracer{log: ll}
}

type zlTracer struct{ log logger.Logger }

func (z *zlTracer) OnQuery(_ context.Context, ev QueryEvent) {
	const (
		maxSQL  = 1024 // cap SQL shown
		maxArgs = 512  // cap args preview
	)

	sql := ev.SQL
	if len(sql) > maxSQL {
		sql = sql[:maxSQL] + "..."
	}

	args := ev.Args
	switch v := ev.Args.(type) {
	case string:
		if len(v) > maxArgs {
			args = v[:maxArgs] + "..."
		}
	case fmt.Stringer:
		s := v.String()
		if len(s) > maxArgs {
			args = s[:maxArgs] + "..."
		}
	}

	evt := z.log.Info()
	if ev.Err != nil {
		evt = z.log.Error()
	} else if ev.Slow {
		evt = z.log.Warn()
	}

	builder := evt.
		Str("op", ev.Op).
		Float64("elapsed_ms", float64(ev.ElapsedUS)/1000.0).
		Bool("slow", ev.Slow).
		Str("sql", sql).
		Interface("args", args).
		Err(ev.Err)

	if ev.Err != nil {
		builder = builder.Bytes("stack", debug.Stack())
	}
	builder.Msg("ch op")
}

// Config fully describes a CH connection.
// Fields mirror clickhouse.Options, plus client behavior knobs
type Config struct {
	// Connection options
	Addrs      []string
	Protocol   clickhouse.Protocol // clickhouse.Native or clickhouse.HTTP
	TLS        *tls.Config
	Auth       clickhouse.Auth
	Dialer     func(ctx context.Context, addr string) (net.Conn, error)
	Settings   clickhouse.Settings
	ClientInfo clickhouse.ClientInfo

	// Timeouts & compression
	DialTimeout time.Duration
	ReadTimeout time.Duration
	Compression *clickhouse.Compression

	// Tracing & behavior
	Tracer      QueryTracer
	SlowMs      int
	InsertChunk int
	MaxRetries  int
	RetryBase   time.Duration

	// Driver debug hook (optional)
	Debugf func(format string, args ...any)
}

// Rows adapts driver.Rows to our local Rows
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
	Columns() []string
}

// CH is a minimal ClickHouse client with retry and tracing
type CH struct {
	conn        clickhouse.Conn
	tracer      QueryTracer
	slowUS      int64
	insertChunk int
	maxRetries  int
	retryBase   time.Duration
}

// Open establishes the connection and pings the server with small retry
func Open(ctx context.Context, cfg Config) (*CH, error) {
	if len(cfg.Addrs) == 0 {
		return nil, fmt.Errorf("ch: no addresses provided")
	}

	insertChunk := cfg.InsertChunk
	if insertChunk <= 0 {
		insertChunk = 5000
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryBase := cfg.RetryBase
	if retryBase <= 0 {
		retryBase = 200 * time.Millisecond
	}

	opts := &clickhouse.Options{
		Addr:        cfg.Addrs,
		Protocol:    cfg.Protocol,
		TLS:         cfg.TLS,
		Auth:        cfg.Auth,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
		Compression: cfg.Compression,
		ClientInfo:  cfg.ClientInfo,
		Debugf:      cfg.Debugf,
		DialContext: cfg.Dialer, // nil ok
		Settings:    cfg.Settings,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("ch: open: %w", err)
	}

	// Resilient ping handles initial EOF-ish
	var last error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := conn.Ping(ctx); err != nil {
			last = err
			if !isEOFish(err) || attempt == maxRetries {
				return nil, fmt.Errorf("ch: ping: %w", err)
			}
			time.Sleep(retryBase * time.Duration(attempt))
			continue
		}
		last = nil
		break
	}
	if last != nil {
		return nil, fmt.Errorf("ch: ping: %w", last)
	}

	return &CH{
		conn:        conn,
		tracer:      cfg.Tracer,
		slowUS:      int64(cfg.SlowMs) * 1000,
		insertChunk: insertChunk,
		maxRetries:  maxRetries,
		retryBase:   retryBase,
	}, nil
}

// Insert inserts rows into table in chunks with retry on EOF-ish failures.
// Expects rows as [][]any (shape matching PrepareBatch.Append)
func (c *CH) Insert(ctx context.Context, table string, rows [][]any) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("ch: nil client")
	}
	if table = strings.TrimSpace(table); table == "" {
		return fmt.Errorf("ch: empty table")
	}
	if len(rows) == 0 {
		return nil
	}

	chunk := c.insertChunk
	startAll := time.Now()
	var last error

	totalRows := len(rows)
	totalChunks := (totalRows + chunk - 1) / chunk
	var sumChunkUS int64
	var maxChunkUS int64

	for start := 0; start < len(rows); start += chunk {
		end := min(start+chunk, len(rows))

		// retry per chunk
		last = nil
		for attempt := 1; attempt <= c.maxRetries; attempt++ {
			startChunk := time.Now()
			err := c.insertChunkDo(ctx, table, rows[start:end])
			elapsedUS := time.Since(startChunk).Microseconds()

			// Track stats for final summary
			sumChunkUS += elapsedUS
			if elapsedUS > maxChunkUS {
				maxChunkUS = elapsedUS
			}

			// Only emit per-chunk logs if slow or error
			isSlow := c.slowUS > 0 && elapsedUS >= c.slowUS
			if c.tracer != nil && (err != nil || isSlow) {
				c.tracer.OnQuery(ctx, QueryEvent{
					SQL:       "INSERT",
					Args:      fmt.Sprintf("%d rows (chunk %d/%d, table=%s)", end-start, (start/chunk)+1, totalChunks, table),
					ElapsedUS: elapsedUS,
					Err:       err,
					Slow:      isSlow,
					Op:        "insert",
				})
			}

			if err == nil {
				break
			}
			last = err
			if !isEOFish(err) || attempt == c.maxRetries {
				return err
			}
			time.Sleep(c.retryBase * time.Duration(attempt))
		}
		if last != nil {
			return last
		}
	}

	if c.tracer != nil {
		elapsedUS := time.Since(startAll).Microseconds()
		avgChunkMS := float64(0)
		if totalChunks > 0 {
			avgChunkMS = float64(sumChunkUS) / 1000.0 / float64(totalChunks)
		}
		c.tracer.OnQuery(ctx, QueryEvent{
			SQL: "INSERT BULK",
			Args: fmt.Sprintf("table=%s rows=%d chunks=%d total_ms=%.3f avg_chunk_ms=%.3f max_chunk_ms=%.3f",
				table, totalRows, totalChunks, float64(elapsedUS)/1000.0, avgChunkMS, float64(maxChunkUS)/1000.0),
			ElapsedUS: elapsedUS,
			Err:       nil,
			Slow:      c.slowUS > 0 && elapsedUS >= c.slowUS,
			Op:        "insert",
		})
	}
	return nil
}

func (c *CH) insertChunkDo(ctx context.Context, table string, rows [][]any) error {
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

// Query executes SQL with args and returns rows (retry on EOF-ish)
func (c *CH) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("ch: nil client")
	}

	var last error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		start := time.Now()
		r, err := c.conn.Query(ctx, sql, args...)
		elapsedUS := time.Since(start).Microseconds()
		if c.tracer != nil {
			c.tracer.OnQuery(ctx, QueryEvent{
				SQL:       sql,
				Args:      args,
				ElapsedUS: elapsedUS,
				Err:       err,
				Slow:      c.slowUS > 0 && elapsedUS >= c.slowUS,
				Op:        "query",
			})
		}
		if err == nil {
			return &rows{r}, nil
		}
		last = err
		if !isEOFish(err) || attempt == c.maxRetries {
			return nil, err
		}
		time.Sleep(c.retryBase * time.Duration(attempt))
	}
	return nil, last
}

// Close closes the connection
func (c *CH) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// rows adapts driver.Rows to our local Rows
type rows struct{ driver.Rows }

func (r *rows) Close() error { return r.Rows.Close() }

// isEOFish - classify driver/network churns we should retry
func isEOFish(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "server closed idle connection") ||
		strings.Contains(s, "use of closed network connection")
}
