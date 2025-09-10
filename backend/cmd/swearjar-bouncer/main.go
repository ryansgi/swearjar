package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"

	bouncermod "swearjar/internal/services/bouncer/module"
)

func mustSetEnv(key, val string) {
	if val != "" {
		_ = os.Setenv(key, val)
	}
}

func main() {
	root := config.New()
	dbCfg := root.Prefix("SERVICE_PGSQL_")

	l := logger.Get()

	st, err := store.Open(context.Background(), store.Config{
		PG: store.PGConfig{
			Enabled:     true,
			URL:         dbCfg.MustString("DBURL_BOUNCER"),
			MaxConns:    int32(dbCfg.MayInt("MAX_CONNS", 4)),
			SlowQueryMs: dbCfg.MayInt("SLOW_MS", 500),
			LogSQL:      dbCfg.MayBool("LOG_SQL", false),
		},
	}, store.WithLogger(*l))
	if err != nil {
		l.Panic().Err(err).Msg("store.Open failed")
	}
	defer func() {
		if err := st.Close(context.Background()); err != nil {
			l.Error().Err(err).Msg("failed to close store")
		}
	}()

	// Flags (same spirit as hallmonitor)
	var (
		fConc   = flag.Int("concurrency", 4, "worker concurrency")
		fRPS    = flag.Float64("rps", 2.0, "global GitHub API target requests/sec")
		fBurst  = flag.Int("burst", 4, "token-bucket burst for GitHub API")
		fTokens = flag.String("tokens", "", "comma-separated GitHub tokens (optional; can also come from env)")
		fBatch  = flag.Int("batch", 64, "DB lease batch size per poll")
		fRetry  = flag.Int("retry_base_ms", 500, "base backoff (ms) for transient/RL")
		fMaxAtt = flag.Int("max_attempts", 10, "max attempts before giving up")
	)
	flag.Parse()

	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		Log: *l,
	}

	// Export as env so module can also read via FromConfig (parity with HM).
	mustSetEnv("BOUNCER_WORKER_CONCURRENCY", fmt.Sprintf("%d", *fConc))
	mustSetEnv("BOUNCER_GH_RPS", fmt.Sprintf("%.3f", *fRPS))
	mustSetEnv("BOUNCER_GH_BURST", fmt.Sprintf("%d", *fBurst))
	mustSetEnv("BOUNCER_GH_TOKENS", *fTokens)
	mustSetEnv("BOUNCER_QUEUE_TAKE_BATCH", fmt.Sprintf("%d", *fBatch))
	mustSetEnv("BOUNCER_RETRY_BASE", fmt.Sprintf("%dms", *fRetry))
	mustSetEnv("BOUNCER_MAX_ATTEMPTS", fmt.Sprintf("%d", *fMaxAtt))

	mod := bouncermod.New(deps, bouncermod.Options{
		Concurrency:    *fConc,
		RatePerSec:     *fRPS,
		Burst:          *fBurst,
		TokensCSV:      *fTokens,
		QueueTakeBatch: *fBatch,
		RetryBaseMs:    *fRetry,
		MaxAttempts:    *fMaxAtt,
	})
	module.Register(mod.Name(), mod.Ports())

	ports := module.MustPortsOf[bouncermod.Ports](mod)

	if err := ports.Worker.Run(context.Background()); err != nil {
		l.Fatal().Err(err).Msg("bouncer worker failed")
	}
}
