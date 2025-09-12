package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"

	backfillmod "swearjar/internal/services/backfill/module"
	detectdom "swearjar/internal/services/detect/domain"
	detectmod "swearjar/internal/services/detect/module"
	hitsmod "swearjar/internal/services/hits/module"
	utmod "swearjar/internal/services/utterances/module"
)

func mustSetEnv(key, val string) {
	if val != "" {
		_ = os.Setenv(key, val)
	}
}

func main() {
	root := config.New()
	pgCfg := root.Prefix("SERVICE_PGSQL_")
	chCfg := root.Prefix("SERVICE_CLICKHOUSE_")

	l := logger.Get()
	st, err := store.Open(context.Background(), store.Config{
		PG: store.PGConfig{
			Enabled:     true,
			URL:         pgCfg.MustString("DBURL"),
			MaxConns:    int32(pgCfg.MayInt("MAX_CONNS", 4)),
			SlowQueryMs: pgCfg.MayInt("SLOW_MS", 500),
			LogSQL:      pgCfg.MayBool("LOG_SQL", false),
		},
		CH: store.CHConfig{
			Enabled:    true,
			URL:        chCfg.MustString("DBURL"),
			LogSQL:     chCfg.MayBool("LOG_SQL", true),
			ClientName: "swearjar",
			ClientTag:  "backfill",
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

	var (
		fStart    = flag.String("start", "", "UTC start hour YYYY-MM-DDTHH")
		fEnd      = flag.String("end", "", "UTC end hour YYYY-MM-DDTHH inclusive")
		fDetect   = flag.Bool("detect", false, "also run detection and write hits during backfill")
		fDetVer   = flag.Int("detver", 1, "detector version to stamp into hits (when --detect)")
		fPlanOnly = flag.Bool("plan-only", false, "seed ingest_hours for the range and exit without processing")
		fResume   = flag.Bool("resume", false, "ignore -start/-end and drain any pending/error hours")
	)
	flag.Parse()

	// Validate flag combos
	if *fPlanOnly && *fResume {
		l.Panic().Msg("--plan-only and --resume are mutually exclusive")
	}

	if *fStart == "" || *fEnd == "" {
		l.Panic().Msg("must provide -start and -end")
	}
	start, err := time.Parse("2006-01-02T15", *fStart)
	if err != nil {
		l.Panic().Err(err).Msg("bad -start")
	}
	end, err := time.Parse("2006-01-02T15", *fEnd)
	if err != nil {
		l.Panic().Err(err).Msg("bad -end")
	}
	if end.Before(start) {
		l.Panic().Str("start", start.String()).Str("end", end.String()).Msg("-end before -start")
	}

	// Shared deps for modules
	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		CH:  st.CH,
		Log: *l,
	}

	// We set env flags so modules that read FromConfig pick these up
	mustSetEnv("CORE_BACKFILL_DETECT", map[bool]string{true: "1", false: "0"}[*fDetect])
	mustSetEnv("CORE_DETECT_VERSION", strconv.Itoa(*fDetVer))

	// If detect is enabled, we need Hits + Utterances + Detect wired and registered,
	// so that backfill can call the detect writer (directly or via injected ports)
	var dm modkit.Module
	if *fDetect {
		// Dependencies for detect
		ut := utmod.New(deps)
		hm := hitsmod.New(deps)

		// Detect needs both Utterances (for runner) and HitsWriter (for writer)
		dm = detectmod.New(
			deps,
			detectmod.Options{Version: *fDetVer}, // runner/writer will use this stamp
			modkit.WithPorts(detectdom.Ports{
				Utterances: module.MustPortsOf[utmod.Ports](ut).Reader,
				HitsWriter: module.MustPortsOf[hitsmod.Ports](hm).Writer,
			}),
		)

		// Register deps first so other modules can resolve ports if they need to
		module.Register(ut.Name(), ut.Ports())
		module.Register(hm.Name(), hm.Ports())
		module.Register(dm.Name(), dm.Ports())
	}

	bf := backfillmod.New(deps)
	module.Register(bf.Name(), bf.Ports())

	// Run backfill
	ports := bf.Ports().(backfillmod.Ports)
	ctx := context.Background()
	switch {
	case *fPlanOnly:
		if err := ports.Runner.PlanRange(ctx, start.UTC(), end.UTC()); err != nil {
			l.Fatal().Err(err).Msg("backfill plan-only failed")
		}
		return

	case *fResume:
		if err := ports.Runner.RunResume(ctx); err != nil {
			l.Fatal().Err(err).Msg("backfill resume failed")
		}
		return

	default:
		if err := ports.Runner.RunRange(ctx, start.UTC(), end.UTC()); err != nil {
			l.Fatal().Err(err).Msg("backfill failed")
		}
	}
}
