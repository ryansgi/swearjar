//go:build integration_pg
// +build integration_pg

package pg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostgres unchanged except: give more timeouts for first image pull
func startPostgres(t *testing.T) (dsn string, stop func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	req := tc.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		).WithDeadline(2 * time.Minute),
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		cancel()
		t.Fatalf("failed to start postgres container: %v", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(context.Background())
		cancel()
		t.Fatalf("failed to get container host: %v", err)
	}
	mapped, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = c.Terminate(context.Background())
		cancel()
		t.Fatalf("failed to get mapped port: %v", err)
	}

	dsn = fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, mapped.Port())
	stop = func() {
		_ = c.Terminate(context.Background())
		cancel()
	}
	return dsn, stop
}

func TestOpen_And_BasicQueries_Integration(t *testing.T) {
	dsn, stop := startPostgres(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	appName := "swearjar-pg-integration"

	WithTestDB(t, dsn, func(pc *pgxpool.Config) {
		if pc.ConnConfig.RuntimeParams == nil {
			pc.ConnConfig.RuntimeParams = map[string]string{}
		}
		pc.ConnConfig.RuntimeParams["application_name"] = appName
		pc.MinConns = 1
	}, func(p *PG) {
		// Keep TEMP table on a single session
		conn := AcquireConn(t, p, ctx)

		// sanity
		var one int
		if err := conn.QueryRow(ctx, "select 1").Scan(&one); err != nil {
			t.Fatalf("select failed: %v", err)
		}
		if one != 1 {
			t.Fatalf("unexpected value: %d", one)
		}

		// TEMP table WITHOUT ON COMMIT DROP (autocommit would drop it immediately)
		if _, err := conn.Exec(ctx, `create temporary table t (id int primary key, name text)`); err != nil {
			t.Fatalf("create temp table failed: %v", err)
		}
		defer func() { _, _ = conn.Exec(ctx, `drop table if exists t`) }()

		batch := &pgx.Batch{}
		batch.Queue(`insert into t (id, name) values ($1,$2)`, 1, "alpha")
		br := conn.SendBatch(ctx, batch)
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			t.Fatalf("insert failed: %v", err)
		}
		if err := br.Close(); err != nil {
			t.Fatalf("batch close: %v", err)
		}

		type row struct {
			ID   int
			Name string
		}
		rows, err := conn.Query(ctx, `select id, name from t order by id`)
		if err != nil {
			t.Fatalf("query rows: %v", err)
		}
		defer rows.Close()

		got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if len(got) != 1 || got[0].ID != 1 || got[0].Name != "alpha" {
			t.Fatalf("unexpected rows: %#v", got)
		}

		var gotApp string
		if err := conn.QueryRow(ctx, `select current_setting('application_name')`).Scan(&gotApp); err != nil {
			t.Fatalf("check app name: %v", err)
		}
		if gotApp != appName {
			t.Fatalf("application_name mismatch: got %q want %q", gotApp, appName)
		}
	})
}
