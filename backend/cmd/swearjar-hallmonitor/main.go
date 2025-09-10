package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"

	halldom "swearjar/internal/services/hallmonitor/domain"
	hallmod "swearjar/internal/services/hallmonitor/module"
)

func mustSetEnv(key, val string) {
	if val != "" {
		_ = os.Setenv(key, val)
	}
}

func parseWhen(label, v string) time.Time {
	// Accept either date or date+hour, like the backfill tool
	// - "YYYY-MM-DD" (midnight UTC)
	// - "YYYY-MM-DDTHH"
	if v == "" {
		return time.Time{}
	}
	layouts := []string{"2006-01-02T15", "2006-01-02"}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, v)
		if err == nil {
			// Normalize to UTC
			if layout == "2006-01-02" {
				// midnight at start of the day
				return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			return t.UTC()
		}
		lastErr = err
	}
	panic(fmt.Errorf("bad -%s: %w", label, lastErr))
}

func main() {
	root := config.New()
	dbCfg := root.Prefix("SERVICE_PGSQL_")

	l := logger.Get()

	st, err := store.Open(context.Background(), store.Config{
		PG: store.PGConfig{
			Enabled:     true,
			URL:         dbCfg.MustString("DBURL_HM"),
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

	// Flags
	var (
		fMode   = flag.String("mode", "worker", "hallmonitor mode: worker | backfill | refresh")
		fSince  = flag.String("since", "", "seed/refresh lower bound (UTC) YYYY-MM-DD or YYYY-MM-DDTHH")
		fUntil  = flag.String("until", "", "seed/refresh upper bound (UTC) YYYY-MM-DD or YYYY-MM-DDTHH (exclusive)")
		fLimit  = flag.Int("limit", 0, "max items to process (0 = unlimited)")
		fConc   = flag.Int("concurrency", 4, "worker concurrency")
		fRPS    = flag.Float64("rps", 2.0, "global GitHub API target requests/sec")
		fBurst  = flag.Int("burst", 4, "token-bucket burst for GitHub API")
		fTokens = flag.String("tokens", "", "comma-separated GitHub tokens (optional; can also come from env)")
		fDryRun = flag.Bool("dryrun", false, "in backfill/refresh modes, plan but do not write (for smoke tests)")
	)
	flag.Parse()

	// Shared deps
	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		Log: *l,
	}

	// Export a few knobs as env so the module can read via FromConfig if desired
	mustSetEnv("HALLMONITOR_WORKER_CONCURRENCY", fmt.Sprintf("%d", *fConc))
	mustSetEnv("HALLMONITOR_GH_RPS", fmt.Sprintf("%.3f", *fRPS))
	mustSetEnv("HALLMONITOR_GH_BURST", fmt.Sprintf("%d", *fBurst))
	mustSetEnv("HALLMONITOR_GH_TOKENS", *fTokens)
	mustSetEnv("HALLMONITOR_DRYRUN", map[bool]string{true: "1", false: "0"}[*fDryRun])

	hm := hallmod.New(
		deps,
		hallmod.Options{
			Concurrency: *fConc,
			RatePerSec:  *fRPS,
			Burst:       *fBurst,
			TokensCSV:   *fTokens,
			DryRun:      *fDryRun,
		},
	)

	module.Register(hm.Name(), hm.Ports())

	ports := module.MustPortsOf[hallmod.Ports](hm)

	ctx := context.Background()

	switch *fMode {
	case "worker":
		// Run forever (until ctx cancel) consuming repo/actor queues
		if err := ports.Worker.Run(ctx); err != nil {
			l.Fatal().Err(err).Msg("hallmonitor worker failed")
		}

	case "backfill":
		// Seed queues from historical utterances within a window, then exit
		since := parseWhen("since", *fSince)
		until := parseWhen("until", *fUntil)
		if since.IsZero() {
			l.Panic().Msg("hallmonitor backfill mode: -since is required (YYYY-MM-DD or YYYY-MM-DDTHH)")
		}

		// until optional; if zero, module may interpret as "open-ended"
		if err := ports.Seeder.SeedFromUtterances(ctx, halldom.SeedRange{
			Since: since,
			Until: until, // may be zero
			Limit: *fLimit,
		}); err != nil {
			l.Fatal().Err(err).Msg("hallmonitor backfill seeding failed")
		}

	case "refresh":
		// Enqueue/refresh items that are due (based on next_refresh_at), then exit
		since := parseWhen("since", *fSince) // optional filter on pushed/fetched recency if module uses it
		until := parseWhen("until", *fUntil)
		if err := ports.Refresher.RefreshDue(ctx, halldom.RefreshParams{
			Since: since,
			Until: until,
			Limit: *fLimit,
		}); err != nil {
			l.Fatal().Err(err).Msg("hallmonitor refresh sweep failed")
		}

	default:
		l.Panic().Str("mode", *fMode).Msg("hallmonitor unknown -mode (expected: worker | backfill | refresh)")
	}
}
