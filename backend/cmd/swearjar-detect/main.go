package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"

	detectdom "swearjar/internal/services/detect/domain"
	detectmod "swearjar/internal/services/detect/module"

	hitsmod "swearjar/internal/services/hits/module"
	utmod "swearjar/internal/services/utterances/module"
)

func mustSetEnv(k, v string) {
	if v != "" {
		_ = os.Setenv(k, v)
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
			Enabled: true,
			URL:     chCfg.MustString("DBURL"),
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
		startStr = flag.String("start", "", "inclusive hour, e.g. 2025-08-01T00")
		endStr   = flag.String("end", "", "exclusive hour, e.g. 2025-08-01T03")
		ver      = flag.Int("ver", 1, "detector version to stamp")
		workers  = flag.Int("workers", 2, "concurrency (>=1)")
		page     = flag.Int("page", 5000, "page size (rows)")
		dryRun   = flag.Bool("dry-run", false, "compute but do not write hits")
	)
	flag.Parse()

	if *startStr == "" || *endStr == "" {
		log.Fatal("start/end are required (hour resolution)")
	}
	start, err := time.Parse("2006-01-02T15", *startStr)
	if err != nil {
		log.Fatalf("bad -start: %v", err)
	}
	end, err := time.Parse("2006-01-02T15", *endStr)
	if err != nil {
		log.Fatalf("bad -end: %v", err)
	}
	if !start.Before(end) {
		log.Fatal("start must be < end")
	}

	// Pass CLI flags into CORE_DETECT_* so the module can read its own config
	mustSetEnv("CORE_DETECT_VERSION", strconv.Itoa(*ver))
	mustSetEnv("CORE_DETECT_WORKERS", strconv.Itoa(*workers))
	mustSetEnv("CORE_DETECT_PAGE_SIZE", strconv.Itoa(*page))
	mustSetEnv("CORE_DETECT_DRY_RUN", map[bool]string{true: "1", false: "0"}[*dryRun])

	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		Log: *l,
	}

	// Build dependency modules first
	ut := utmod.New(deps)
	hm := hitsmod.New(deps)

	// Build detect module with ports injected from deps modules
	dm := detectmod.New(
		deps,
		detectmod.Options{
			Version:  *ver,
			Workers:  *workers,
			PageSize: *page,
			DryRun:   *dryRun,
		},
		modkit.WithPorts(detectdom.Ports{
			Utterances: module.MustPortsOf[utmod.Ports](ut).Reader,
			HitsWriter: module.MustPortsOf[hitsmod.Ports](hm).Writer,
		}),
	)

	// Register ports
	module.Register(ut.Name(), ut.Ports())
	module.Register(hm.Name(), hm.Ports())
	module.Register(dm.Name(), dm.Ports())

	// Kick the runner
	ports := dm.Ports().(detectmod.Ports)
	if err := ports.Runner.RunRange(context.Background(), start.UTC(), end.UTC()); err != nil {
		l.Fatal().Err(err).Msg("detect failed")
	}
}
