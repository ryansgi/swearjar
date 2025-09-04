package main

import (
	"context"
	"flag"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/modkit/module"
	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	"swearjar/internal/platform/store"

	backfillmod "swearjar/internal/services/backfill/module"
)

func main() {
	root := config.New()
	dbCfg := root.Prefix("SERVICE_PGSQL_")

	l := logger.Get()

	st, err := store.Open(context.Background(), store.Config{
		PG: store.PGConfig{
			Enabled:     true,
			URL:         dbCfg.MustString("DBURL"),
			MaxConns:    int32(dbCfg.MayInt("MAX_CONNS", 4)),
			SlowQueryMs: dbCfg.MayInt("SLOW_MS", 500),
			LogSQL:      dbCfg.MayBool("LOG_SQL", true),
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
		fStart = flag.String("start", "", "UTC start hour YYYY-MM-DDTHH")
		fEnd   = flag.String("end", "", "UTC end hour YYYY-MM-DDTHH inclusive")
	)
	flag.Parse()

	if *fStart == "" || *fEnd == "" {
		l.Panic().Msg("must provide -start and -end or set CORE_BACKFILL_START and CORE_BACKFILL_END")
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

	// Construct shared deps for modules
	deps := modkit.Deps{
		Cfg: root,
		PG:  st.PG,
		Log: *l,
	}

	// Build backfill module and register ports
	bf := backfillmod.New(deps)
	module.Register(bf.Name(), bf.Ports())

	// Invoke the runner port
	ports := bf.Ports().(backfillmod.Ports) // or module.Lookup("backfill").(backfillmod.Ports)
	if err := ports.Runner.RunRange(context.Background(), start.UTC(), end.UTC()); err != nil {
		l.Fatal().Err(err).Msg("backfill failed")
	}
}
