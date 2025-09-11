// @title         Swearjar API
// @version       0.1.0
// @description   Read only endpoints for stats and samples

package main

import (
	"context"

	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"
	phttp "swearjar/internal/platform/net/http"
	"swearjar/internal/platform/store"

	"swearjar/internal/services/api"
)

func main() {
	// service-scoped config for HTTP etc (CORE_API_*)
	root := config.New()
	apiCfg := root.Prefix("CORE_API_")

	pgCfg := root.Prefix("SERVICE_PGSQL_")      // pgCfg lives under SERVICE_PGSQL_*
	chCfg := root.Prefix("SERVICE_CLICKHOUSE_") // chCfg lives under SERVICE_CLICKHOUSE_*
	// bring up logging early
	l := logger.Get()

	// open the platform store (postgres + CH adapter)
	st, err := store.Open(
		context.Background(),
		store.Config{
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
				ClientName: "swearjar",
				ClientTag:  "api",
			},
		},
		store.WithLogger(*logger.Get()),
	)
	if err != nil {
		l.Panic().Err(err).Msg("store.Open failed")
	}
	defer func() {
		if err := st.Close(context.Background()); err != nil {
			l.Error().Err(err).Msg("failed to close store")
		}
	}()

	// http server (reads CORE_API_PORT / CORE_API_ADDR)
	srv := phttp.NewServer(apiCfg)

	// mount our API
	api.Mount(
		srv.Router(),
		api.Options{
			Config:         apiCfg,
			Store:          st,
			Logger:         l,
			EnableSwagger:  apiCfg.MayBool("SWAGGER", true),
			EnableProfiler: apiCfg.MayBool("PROFILER", true),
		},
	)

	// run
	if err := srv.Run(context.Background()); err != nil {
		l.Panic().Err(err).Msg("http server stopped")
	}
}
