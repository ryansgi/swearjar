// Package repo provides Postgres bindings for domain.Repo
package repo

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/ident/domain"
)

type (
	// PG is a Postgres binder for domain.Repo
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// Compile-time assertion: queries implements domain.Repo
var _ domain.Repo = (*queries)(nil)

// NewPG returns a Postgres binder for Repo
func NewPG() repokit.Binder[domain.Repo] { return PG{} }

// Bind implements repokit.Binder
func (PG) Bind(q repokit.Queryer) domain.Repo { return &queries{q: q} }

// EnsurePrincipalsAndMaps inserts missing principals and GH maps via temp-table + upsert.
// Race-safe: final inserts use ON CONFLICT DO NOTHING so concurrent writers can't violate PKs
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

	// Principals: repos
	if len(repos) > 0 {
		hs := keysSorted(repos)
		hexes := makeHexes(hs)

		// Stage and upsert -> principals_repos (race-safe, idempotent)
		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_repo(
				stage_hid public.hid_bytes PRIMARY KEY
			) ON COMMIT DROP;
			TRUNCATE _hid_repo;
		`); err != nil {
			return fmt.Errorf("stage repos: create temp: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO _hid_repo(stage_hid)
			SELECT DISTINCT decode(x,'hex')::public.hid_bytes
			FROM unnest($1::text[]) AS t(x) ON CONFLICT (stage_hid) DO NOTHING;
		`, hexes); err != nil {
			return fmt.Errorf("stage repos: load: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO principals_repos (repo_hid) SELECT s.stage_hid FROM _hid_repo s ON CONFLICT (repo_hid) DO NOTHING;
		`); err != nil {
			return fmt.Errorf("ensure principals_repos: %w", err)
		}

		// ident map (bulk shim; runs as SECURITY DEFINER)
		hexes, ids := makeHexesAndIDs(repos, hs)
		if _, err := r.q.Exec(ctx,
			`SELECT ident.bulk_upsert_gh_repo_map($1::text[], $2::bigint[])`,
			hexes, ids,
		); err != nil {
			return fmt.Errorf("repo map bulk upsert: %w", err)
		}
	}

	// Principals: actors
	if len(actors) > 0 {
		hs := keysSorted(actors)
		hexes := makeHexes(hs)

		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_actor(
				stage_hid public.hid_bytes PRIMARY KEY
			) ON COMMIT DROP;
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
		if _, err := r.q.Exec(ctx, `
			INSERT INTO principals_actors (actor_hid) SELECT s.stage_hid FROM _hid_actor s ON CONFLICT (actor_hid) DO NOTHING;
		`); err != nil {
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
