package store

// import (
// 	"context"
// 	"os"
// 	"testing"
// 	"time"
// )

// func fastFailPGURL() string {
// 	// user/pass/db don't matter; 127.0.0.1:1 is a closed port on all systems
// 	return "postgres://u:p@127.0.0.1:1/db?sslmode=disable"
// }

// func testConfig() Config {
// 	return Config{
// 		PG: PGConfig{
// 			URL:         "postgres://local", // lazy pool; ping attempts will fail fast
// 			MaxConns:    2,
// 			SlowQueryMs: 500,
// 			LogSQL:      false,
// 		},
// 		CH: CHConfig{
// 			URL: "clickhouse://local",
// 		},
// 	}
// }

// // integrationURL returns an override URL from env if present
// func integrationURL(envKey string) (string, bool) {
// 	v := os.Getenv(envKey)
// 	return v, v != ""
// }

// func TestOpenPG_ParentAlreadyCanceled(t *testing.T) {
// 	t.Parallel()

// 	ctx, cancel := context.WithCancel(context.Background())
// 	cancel()

// 	cfg := testConfig()
// 	cfg.PG.URL = fastFailPGURL()

// 	s := &Store{}

// 	start := time.Now()
// 	txr, err := openPG(ctx, cfg, s)
// 	elapsed := time.Since(start)

// 	if err == nil {
// 		t.Fatalf("expected error due to canceled context, got nil (txr=%T)", txr)
// 	}
// 	if txr != nil {
// 		t.Fatalf("expected nil TxRunner on canceled context, got %T", txr)
// 	}
// 	// should return quickly (no DNS, immediate ECONNREFUSED)
// 	if elapsed > time.Second {
// 		t.Fatalf("expected quick failure, got %v", elapsed)
// 	}
// }

// func TestOpenPG_BackoffThenCancel(t *testing.T) {
// 	t.Parallel()

// 	// parent will be canceled shortly after first backoff sleep
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	cfg := testConfig()
// 	cfg.PG.URL = fastFailPGURL()

// 	// cancel in a goroutine *after* the first backoff is likely in progress
// 	// openPG backoffStart is 150ms; cancel a bit after that to ensure we
// 	// exercise time.Sleep(backoff) and the next iteration
// 	go func() {
// 		time.Sleep(160 * time.Millisecond)
// 		cancel()
// 	}()

// 	s := &Store{}

// 	start := time.Now()
// 	txr, err := openPG(ctx, cfg, s)
// 	elapsed := time.Since(start)

// 	if err == nil {
// 		t.Fatalf("expected error due to parent cancellation, got nil (txr=%T)", txr)
// 	}
// 	if txr != nil {
// 		t.Fatalf("expected nil TxRunner when parent deadline hits, got %T", txr)
// 	}

// 	// We should have slept at least once (~150ms), so give a safe lower bound
// 	if elapsed < 140*time.Millisecond {
// 		t.Fatalf("expected at least one backoff sleep (~150ms), got %v", elapsed)
// 	}
// 	// And we shouldn't take multiple seconds; cancellation should short-circuit
// 	if elapsed > 1*time.Second {
// 		t.Fatalf("test took too long (%v); expected early cancel", elapsed)
// 	}
// }

// func TestOpenPG(t *testing.T) {
// 	t.Parallel()

// 	url, ok := integrationURL("TEST_PG_URL")
// 	if !ok {
// 		t.Skip("skipping PG integration test: set TEST_PG_URL to enable")
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
// 	defer cancel()

// 	cfg := testConfig()
// 	cfg.PG.URL = url

// 	s := &Store{} // zero logger is fine for tracer wiring

// 	// LogSQL = false
// 	cfg.PG.LogSQL = false
// 	txr, err := openPG(ctx, cfg, s)
// 	if err != nil {
// 		t.Fatalf("openPG (LogSQL=false) error: %v", err)
// 	}
// 	if txr == nil {
// 		t.Fatalf("openPG (LogSQL=false) returned nil TxRunner")
// 	}

// 	// LogSQL = true
// 	cfg.PG.LogSQL = true
// 	txr2, err := openPG(ctx, cfg, s)
// 	if err != nil {
// 		t.Fatalf("openPG (LogSQL=true) error: %v", err)
// 	}
// 	if txr2 == nil {
// 		t.Fatalf("openPG (LogSQL=true) returned nil TxRunner")
// 	}
// }

// func TestOpenCH(t *testing.T) {
// 	t.Parallel()

// 	url, ok := integrationURL("TEST_CH_URL")
// 	if !ok {
// 		t.Skip("skipping ClickHouse integration test: set TEST_CH_URL to enable")
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	cfg := testConfig()
// 	cfg.CH.URL = url

// 	ch, err := openCH(ctx, cfg, nil)
// 	if err != nil {
// 		t.Fatalf("openCH error: %v", err)
// 	}
// 	if ch == nil {
// 		t.Fatalf("openCH returned nil Clickhouse")
// 	}
// }
