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
	nightshiftmod "swearjar/internal/services/nightshift/module"
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
			LogSQL:      pgCfg.MayBool("LOG_SQL", true),
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

		// Nightshift flags
		fNightshift  = flag.Bool("nightshift", false, "run Nightshift after backfill for the same range")
		fNSResume    = flag.Bool("ns-resume", false, "run Nightshift resume loop (ignores -start/-end)")
		fNSDetVer    = flag.Int("ns-detver", 1, "Nightshift detector version stamped into archives/rollups")
		fNSRetention = flag.String("ns-retention", "full", "Nightshift retention mode: full | aggressive | timebox:Nd")
		fNSWorkers   = flag.Int("ns-workers", 2, "Nightshift worker concurrency")
		fNSLeases    = flag.Bool("ns-leases", true, "use advisory leases for Nightshift")
	)
	flag.Parse()

	// Validate flag combos
	if *fPlanOnly && *fResume {
		l.Panic().Msg("--plan-only and --resume are mutually exclusive")
	}

	if !*fResume && (*fStart == "" || *fEnd == "") {
		l.Panic().Msg("must provide -start and -end (unless --resume or --ns-resume)")
	}
	var start, end time.Time
	if *fStart != "" {
		t, err := time.Parse("2006-01-02T15", *fStart)
		if err != nil {
			l.Panic().Err(err).Msg("bad -start")
		}
		start = t
	}
	if *fEnd != "" {
		t, err := time.Parse("2006-01-02T15", *fEnd)
		if err != nil {
			l.Panic().Err(err).Msg("bad -end")
		}
		end = t
		if end.Before(start) {
			l.Panic().Str("start", start.String()).Str("end", end.String()).Msg("-end before -start")
		}
	}

	// Shared deps for modules
	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		CH:  st.CH,
		Log: *l,
	}

	// Surface opts to modules that read FromConfig
	mustSetEnv("CORE_BACKFILL_DETECT", map[bool]string{true: "1", false: "0"}[*fDetect])
	mustSetEnv("CORE_DETECT_VERSION", strconv.Itoa(*fDetVer))

	// Nightshift envs: modules/nightshift/module/options.go reads CORE_NIGHTSHIFT_*
	mustSetEnv("CORE_NIGHTSHIFT_WORKERS", strconv.Itoa(*fNSWorkers))
	mustSetEnv("CORE_NIGHTSHIFT_DET_VERSION", strconv.Itoa(*fNSDetVer))
	mustSetEnv("CORE_NIGHTSHIFT_RETENTION_MODE", *fNSRetention)
	mustSetEnv("CORE_NIGHTSHIFT_LEASES", map[bool]string{true: "1", false: "0"}[*fNSLeases])

	// Optional: Detect stack (when --detect)
	if *fDetect {
		ut := utmod.New(deps)
		hm := hitsmod.New(deps)
		dm := detectmod.New(
			deps,
			detectmod.Options{Version: *fDetVer},
			modkit.WithPorts(detectdom.Ports{
				Utterances: module.MustPortsOf[utmod.Ports](ut).Reader,
				HitsWriter: module.MustPortsOf[hitsmod.Ports](hm).Writer,
			}),
		)
		module.Register(ut.Name(), ut.Ports())
		module.Register(hm.Name(), hm.Ports())
		module.Register(dm.Name(), dm.Ports())
	}

	// Nightshift module (always register; running is controlled by flags)
	ns := nightshiftmod.New(deps)
	module.Register(ns.Name(), ns.Ports())

	// Backfill module
	bf := backfillmod.New(deps)
	module.Register(bf.Name(), bf.Ports())

	ctx := context.Background()

	// Optional: run Nightshift resume independently, then return
	if *fNSResume {
		nsPorts := ns.Ports().(nightshiftmod.Ports)
		if err := nsPorts.Runner.RunResume(ctx); err != nil {
			l.Fatal().Err(err).Msg("nightshift resume failed")
		}
		return
	}

	// Plan-only / resume / run-range for Backfill
	bfPorts := bf.Ports().(backfillmod.Ports)
	switch {
	case *fPlanOnly:
		if err := bfPorts.Runner.PlanRange(ctx, start.UTC(), end.UTC()); err != nil {
			l.Fatal().Err(err).Msg("backfill plan-only failed")
		}
		return

	case *fResume:
		if err := bfPorts.Runner.RunResume(ctx); err != nil {
			l.Fatal().Err(err).Msg("backfill resume failed")
		}
		// Optionally follow with Nightshift resume if requested via --nightshift
		if *fNightshift {
			nsPorts := ns.Ports().(nightshiftmod.Ports)
			if err := nsPorts.Runner.RunResume(ctx); err != nil {
				l.Fatal().Err(err).Msg("nightshift resume after backfill-resume failed")
			}
		}
		return

	default:
		if err := bfPorts.Runner.RunRange(ctx, start.UTC(), end.UTC()); err != nil {
			l.Fatal().Err(err).Msg("backfill failed")
		}
		// If asked, run Nightshift for the same range right after backfill
		if *fNightshift {
			nsPorts := ns.Ports().(nightshiftmod.Ports)
			if err := nsPorts.Runner.RunRange(ctx, start.UTC(), end.UTC()); err != nil {
				l.Fatal().Err(err).Msg("nightshift (post-backfill) failed")
			}
		}
	}
}
