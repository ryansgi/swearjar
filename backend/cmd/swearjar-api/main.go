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

	// db config lives under SERVICE_PGSQL_*
	dbCfg := root.Prefix("SERVICE_PGSQL_")

	// bring up logging early
	l := logger.Get()

	// open the platform store (postgres adapter)
	dsn := dbCfg.MayString("DBURL", "")
	if dsn == "" {
		panic("missing SERVICE_PGSQL_DBURL")
	}
	st, err := store.Open(
		context.Background(),
		store.Config{
			PG: store.PGConfig{
				Enabled:     true,
				URL:         dsn,
				MaxConns:    int32(dbCfg.MayInt("MAX_CONNS", 4)),
				SlowQueryMs: dbCfg.MayInt("SLOW_MS", 500),
				LogSQL:      dbCfg.MayBool("LOG_SQL", true),
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
