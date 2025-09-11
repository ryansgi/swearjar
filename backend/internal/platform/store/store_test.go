package store

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

// TestOpen_CHOnly_SetsCHAndLeavesOthersNil exercises the CH success path from Open
func TestOpen_CHOnly_SetsCHAndLeavesOthersNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{
		CH: CHConfig{
			Enabled: true,
			URL:     "clickhouse://local", // ch.Open stub returns a client
		},
		// PG disabled; NATS/Redis intentionally not used by Open right now
	}

	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if s == nil {
		t.Fatalf("Open returned nil store")
	}

	// CH should be set; PG should still be nil
	if s.CH == nil {
		t.Fatalf("CH not initialized")
	}
	if s.PG != nil {
		t.Fatalf("unexpected seams set PG=%T", s.PG)
	}

	// Close should ignore nil seams and close CH without error
	if err := s.Close(ctx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

// TestOpen_PGEnabled_BadURL_BubblesError covers the PG error path
func TestOpen_PGEnabled_BadURL_BubblesError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{
		PG: PGConfig{
			Enabled:     true,
			URL:         "://bad", // parse error inside pg.Open
			MaxConns:    1,
			SlowQueryMs: 0,
			LogSQL:      false,
		},
	}

	s, err := Open(ctx, cfg)
	if err == nil {
		t.Fatalf("expected Open error for bad PG URL, got store=%#v", s)
	}
	if s != nil {
		t.Fatalf("expected nil store on error, got %#v", s)
	}
}

// TestOpen_OptionsApplied_NoPanicOnWithLogger exercises the WithLogger option path
func TestOpen_OptionsApplied_NoPanicOnWithLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Build a zero-value zerolog.Logger (valid, no-op)
	var zl zerolog.Logger

	s, err := Open(ctx, Config{}, WithLogger(zl))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if s == nil {
		t.Fatalf("Open returned nil store")
	}
	// Close on empty store should be fine
	if e := s.Close(ctx); e != nil {
		t.Fatalf("Close on empty store returned error: %v", e)
	}
}

// TestOpen_MultipleBackends_ErrShortCircuits verifies we stop on the first failing backend path
func TestOpen_MultipleBackends_ErrShortCircuits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{
		PG: PGConfig{
			Enabled: true,
			URL:     "://bad", // will fail first
		},
		CH: CHConfig{
			Enabled: true,
			URL:     "clickhouse://local",
		},
	}

	s, err := Open(ctx, cfg)
	if err == nil {
		t.Fatalf("expected Open error on first failing backend")
	}
	if s != nil {
		t.Fatalf("expected nil store when Open fails early, got %#v", s)
	}
}
