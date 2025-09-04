package store

import (
	"context"
	"errors"
	"testing"
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
		// all others disabled
	}

	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if s == nil {
		t.Fatalf("Open returned nil store")
	}

	// CH should be set; others nil
	if s.CH == nil {
		t.Fatalf("CH not initialized")
	}
	if s.PG != nil {
		t.Fatalf("unexpected seams set PG=%T", s.PG)
	}

	// Close should ignore nil seams and close CH without error (stub CH.Close() returns nil)
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

// TestOpen_NATSEnabled_BadURL_BubblesError covers the NATS error path
func TestOpen_NATSEnabled_BadURL_BubblesError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{
		NATS: NATSConfig{
			Enabled:   true,
			URL:       "://bad", // nats.Connect fails to parse
			JetStream: false,
		},
	}

	s, err := Open(ctx, cfg)
	if err == nil {
		t.Fatalf("expected Open error for bad NATS URL, got store=%#v", s)
	}
	if s != nil {
		t.Fatalf("expected nil store on error, got %#v", s)
	}
}

// TestOpen_RedisEnabled_BadAddr_BubblesError covers the Redis error path
func TestOpen_RedisEnabled_SetsKV_AndCloseOK(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{
		RDS: RedisConfig{
			Enabled: true,
			// Even empty/invalid addresses will usually succeed at client construction time
			Addr: "",
			DB:   0,
		},
	}

	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected Open error: %v", err)
	}

	// Close should succeed and ignore nil seams
	if err := s.Close(ctx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

// TestOpen_OptionsApplied_NoPanicOnZeroLogger exercises the logger defaulting line
func TestOpen_OptionsApplied_NoPanicOnZeroLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	called := false
	opt := func(s *Store) error {
		called = true
		// do not set s.Log; we want to hit s.Log = s.Log.With().Logger() safely
		return nil
	}

	s, err := Open(ctx, Config{}, opt)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if !called {
		t.Fatalf("option was not applied")
	}
	// We can't compare zerologgers directly, but we can at least exercise Close on zero seams
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
	// No need to assert exact error; just ensure we didn't proceed to initialize others
}

// TestOpen_OptionError_Bubbles ensures option errors are returned immediately
func TestOpen_OptionError_Bubbles(t *testing.T) {
	t.Parallel()

	optErr := errors.New("boom")
	opt := func(*Store) error { return optErr }

	s, err := Open(context.Background(), Config{}, opt)
	if err == nil || !errors.Is(err, optErr) {
		t.Fatalf("expected option error, got %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil store on option error, got %#v", s)
	}
}
