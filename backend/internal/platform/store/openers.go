package store

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"swearjar/internal/platform/store/ch"
	"swearjar/internal/platform/store/pg"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// openPG opens pg and wraps it with our sql adapter
func openPG(ctx context.Context, cfg Config, s *Store) (TxRunner, error) {
	var tracer pg.QueryTracer
	if cfg.PG.LogSQL {
		tracer = pg.Tracer(s.Log)
	}

	p, err := pg.Open(ctx, pg.Config{
		URL:      cfg.PG.URL,
		MaxConns: cfg.PG.MaxConns,
		SlowMs:   cfg.PG.SlowQueryMs,
	}, tracer, nil)
	if err != nil {
		return nil, err
	}

	// Connection guardrails: ping with retry/backoff using the *pool* directly
	const (
		maxAttempts    = 20
		pingTimeout    = 3 * time.Second
		backoffStart   = 150 * time.Millisecond
		backoffCeiling = 2 * time.Second
	)

	var lastErr error
	backoff := backoffStart
	for range maxAttempts {
		toCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		lastErr = p.Pool.Ping(toCtx) // no adapter, no SQL trace line
		cancel()

		if lastErr == nil {
			a := newPGAdapter(p) // publish adapter only after the pool is healthy
			s.PG = a
			return a, nil
		}
		if ctx.Err() != nil {
			p.Close() // close the pool we opened
			return nil, ctx.Err()
		}
		time.Sleep(backoff)
		if backoff < backoffCeiling {
			backoff *= 2
			if backoff > backoffCeiling {
				backoff = backoffCeiling
			}
		}
	}

	p.Close()
	return nil, fmt.Errorf("postgres ping failed after %d attempts: %w", maxAttempts, lastErr)
}

// openCH parses the DSN into a ch.Config and opens the client.
// It wires a CH tracer when CH.LogSQL is enabled
// openCH parses the DSN in CHConfig and opens a ClickHouse client using ch.Config.
// It mirrors the PG opener pattern: tracing is enabled when c.LogSQL is true
func openCH(ctx context.Context, c CHConfig, s *Store) (*ch.CH, error) {
	if !c.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(c.URL) == "" {
		return nil, fmt.Errorf("ch: empty URL")
	}

	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, fmt.Errorf("ch: parse url: %w", err)
	}
	qs := u.Query()

	proto := clickhouse.Native
	if u.Scheme == "http" || u.Scheme == "https" {
		proto = clickhouse.HTTP
	}

	secure := u.Scheme == "https" || qs.Get("secure") == "true"
	skipVerify := qs.Get("skip_verify") == "1" || qs.Get("skip_verify") == "true"
	var tlsCfg *tls.Config
	if secure {
		tlsCfg = &tls.Config{InsecureSkipVerify: skipVerify}
	}

	user, pass := "", ""
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
		} else if v := qs.Get("key"); v != "" {
			pass = v
		}
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = qs.Get("database")
	}

	dialTO := 5 * time.Second
	if v := qs.Get("dial_timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			dialTO = d
		}
	}
	readTO := time.Duration(0)
	if v := qs.Get("read_timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			readTO = d
		}
	}

	settings := clickhouse.Settings{}
	maxQuerySize := uint64(16 << 20) // 16 MiB default
	if v := qs.Get("max_query_size"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			maxQuerySize = n
		}
	}
	settings["max_query_size"] = maxQuerySize

	maxDepth := uint64(10000)
	if v := qs.Get("max_parser_depth"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			maxDepth = n
		}
	}
	settings["max_parser_depth"] = maxDepth
	settings["max_insert_block_size"] = 10000
	settings["max_execution_time"] = 0
	settings["allow_experimental_nlp_functions"] = 1

	d := &net.Dialer{Timeout: dialTO}
	dialFn := func(ctx context.Context, addr string) (net.Conn, error) {
		return d.DialContext(ctx, "tcp", addr)
	}

	ccfg := ch.Config{
		Addrs:    []string{u.Host},
		Protocol: proto,
		TLS:      tlsCfg,
		Auth:     clickhouse.Auth{Database: db, Username: user, Password: pass},
		Dialer:   dialFn,
		Settings: settings,
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct{ Name, Version string }{
				{Name: fmt.Sprintf("swearjar-%s", c.ClientName), Version: c.ClientTag},
			},
		},

		DialTimeout: dialTO,
		ReadTimeout: readTO,
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionLZ4},

		InsertChunk: c.InsertChunk,
		MaxRetries:  c.MaxRetries,
		RetryBase:   time.Duration(c.RetryBaseMs) * time.Millisecond,
	}

	if c.LogSQL && s != nil {
		ccfg.Tracer = ch.Tracer(s.Log)
	}

	return ch.Open(ctx, ccfg)
}
