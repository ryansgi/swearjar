// Package repo provides Postgres bindings for domain.Repo
package repo

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/ident/domain"
)

type (
	// PG is a Postgres binder for domain.Repo
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG returns a Postgres binder for Repo
func NewPG() repokit.Binder[domain.Repo] { return PG{} }

// Bind implements repokit.Binder
func (PG) Bind(q repokit.Queryer) domain.Repo { return &queries{q: q} }

// EnsurePrincipalsAndMaps inserts missing principals and GH maps via temp-table + upsert.
// Final inserts use ON CONFLICT DO NOTHING (no target) so any unique
// index (repo_hid OR hid_hex) conflicts are tolerated. Inserts are ordered to reduce
// lock inversions, and we retry on deadlock
func (r *queries) EnsurePrincipalsAndMaps(
	ctx context.Context,
	repos map[domain.HID32]int64, actors map[domain.HID32]int64,
) error {
	if len(repos) == 0 && len(actors) == 0 {
		return nil
	}

	keysSorted := func(m map[domain.HID32]int64) []domain.HID32 {
		hs := make([]domain.HID32, 0, len(m))
		for h := range m {
			hs = append(hs, h)
		}
		sort.Slice(hs, func(i, j int) bool { return bytes.Compare(hs[i][:], hs[j][:]) < 0 })
		return hs
	}
	makeHexes := func(hs []domain.HID32) []string {
		xs := make([]string, len(hs))
		for i, h := range hs {
			xs[i] = hex.EncodeToString(h[:])
		}
		return xs
	}
	makeHexesAndIDs := func(m map[domain.HID32]int64, hs []domain.HID32) ([]string, []int64) {
		hexes := make([]string, len(hs))
		ids := make([]int64, len(hs))
		for i, h := range hs {
			hexes[i] = hex.EncodeToString(h[:])
			ids[i] = m[h]
		}
		return hexes, ids
	}
	execRetry := func(ctx context.Context, sql string, args ...any) error {
		const maxAttempts = 4
		backoff := 50 * time.Millisecond
		for a := 1; a <= maxAttempts; a++ {
			if _, err := r.q.Exec(ctx, sql, args...); err != nil {
				msg := fmt.Sprint(err)
				if a < maxAttempts && (strings.Contains(msg, "40P01") ||
					strings.Contains(strings.ToLower(msg), "deadlock detected")) {
					j := backoff/2 + time.Duration(rand.Int63n(int64(backoff)))
					timer := time.NewTimer(j)
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
					backoff *= 2
					if backoff > 500*time.Millisecond {
						backoff = 500 * time.Millisecond
					}
					continue
				}
				return err
			}
			return nil
		}
		return fmt.Errorf("unreachable: execRetry exceeded attempts")
	}

	if len(repos) > 0 {
		hs := keysSorted(repos)
		hexes := makeHexes(hs)

		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_repo(stage_hid public.hid_bytes PRIMARY KEY) ON COMMIT DROP;
			TRUNCATE _hid_repo;
		`); err != nil {
			return fmt.Errorf("stage repos: create temp: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO _hid_repo(stage_hid) SELECT DISTINCT decode(x,'hex')::public.hid_bytes
			FROM unnest($1::text[]) AS t(x) ON CONFLICT (stage_hid) DO NOTHING;
		`, hexes); err != nil {
			return fmt.Errorf("stage repos: load: %w", err)
		}

		err := execRetry(ctx, `
			INSERT INTO principals_repos (repo_hid) SELECT s.stage_hid
			FROM _hid_repo s ORDER BY s.stage_hid ON CONFLICT DO NOTHING;
		`)
		if err != nil {
			return fmt.Errorf("ensure principals_repos: %w", err)
		}

		hexes, ids := makeHexesAndIDs(repos, hs)
		if _, err := r.q.Exec(ctx,
			`SELECT ident.bulk_upsert_gh_repo_map($1::text[], $2::bigint[])`,
			hexes, ids,
		); err != nil {
			return fmt.Errorf("repo map bulk upsert: %w", err)
		}
	}

	if len(actors) > 0 {
		hs := keysSorted(actors)
		hexes := makeHexes(hs)

		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_actor(stage_hid public.hid_bytes PRIMARY KEY) ON COMMIT DROP;
			TRUNCATE _hid_actor;
		`); err != nil {
			return fmt.Errorf("stage actors: create temp: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO _hid_actor(stage_hid) SELECT DISTINCT decode(x,'hex')::public.hid_bytes
			FROM unnest($1::text[]) AS t(x) ON CONFLICT (stage_hid) DO NOTHING;
		`, hexes); err != nil {
			return fmt.Errorf("stage actors: load: %w", err)
		}

		err := execRetry(ctx, `
			INSERT INTO principals_actors (actor_hid) SELECT s.stage_hid
			FROM _hid_actor s ORDER BY s.stage_hid ON CONFLICT DO NOTHING;
		`)
		if err != nil {
			return fmt.Errorf("ensure principals_actors: %w", err)
		}

		hexes, ids := makeHexesAndIDs(actors, hs)
		if _, err := r.q.Exec(ctx,
			`SELECT ident.bulk_upsert_gh_actor_map($1::text[], $2::bigint[])`,
			hexes, ids,
		); err != nil {
			return fmt.Errorf("actor map bulk upsert: %w", err)
		}
	}

	return nil
}
