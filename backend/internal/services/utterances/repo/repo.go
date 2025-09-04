// Package repo provides repository implementations for utterances
package repo

import (
	"context"
	"fmt"
	"strings"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/utterances/domain"
)

type binder struct{}

// NewPG constructs a new repo binder for Postgres
func NewPG() repokit.Binder[Storage] { return binder{} }

// Bind implements repokit.Binder
func (binder) Bind(q repokit.Queryer) Storage { return &pg{q: q} }

// Storage defines the utterances repository
type Storage interface {
	List(ctx context.Context, in domain.ListInput, hardLimit int) ([]domain.Row, domain.AfterKey, error)
}

type pg struct{ q repokit.Queryer }

// List implements domain.ReaderPort
func (s *pg) List(ctx context.Context, in domain.ListInput, hardLimit int) ([]domain.Row, domain.AfterKey, error) {
	// Dynamic WHERE with numbered args
	var sb strings.Builder
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT
			u.id::text,
			u.created_at,
			u.repo_name,
			u.repo_id,
			u.repo_hid,
			u.actor_login,
			u.actor_id,
			u.actor_hid,
			u.source::text,
			u.source_detail,
			u.lang_code,
			u.script,
			COALESCE(u.text_normalized, '') AS text_norm
		FROM utterances u
		WHERE u.created_at >= ` + arg(in.Since) + ` AND u.created_at < ` + arg(in.Until) + `
			AND NOT EXISTS (SELECT 1 FROM active_deny_repos r WHERE r.principal_hid = u.repo_hid)
			AND NOT EXISTS (SELECT 1 FROM active_deny_actors a WHERE a.principal_hid = u.actor_hid)
	`)

	// Keyset only when AfterKey is set (avoid ""::uuid on first page)
	if in.After.ID != "" {
		sb.WriteString("  AND (u.created_at, u.id) > (" + arg(in.After.CreatedAt) + ", " + arg(in.After.ID) + "::uuid)\n")
	}

	if in.RepoName != "" {
		sb.WriteString("  AND u.repo_name = " + arg(in.RepoName) + "\n")
	}
	if in.Owner != "" {
		// owner = split_part(repo_name,'/',1)
		sb.WriteString("  AND split_part(u.repo_name, '/', 1) = " + arg(in.Owner) + "\n")
	}
	if in.RepoID != nil {
		sb.WriteString("  AND u.repo_id = " + arg(*in.RepoID) + "\n")
	}
	if in.ActorLogin != "" {
		sb.WriteString("  AND u.actor_login = " + arg(in.ActorLogin) + "\n")
	}
	if in.ActorID != nil {
		sb.WriteString("  AND u.actor_id = " + arg(*in.ActorID) + "\n")
	}
	if in.LangCode != "" {
		sb.WriteString("  AND u.lang_code = " + arg(in.LangCode) + "\n")
	}

	sb.WriteString("ORDER BY u.created_at, u.id\nLIMIT " + arg(hardLimit))

	rows, err := s.q.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, domain.AfterKey{}, err
	}
	defer rows.Close()

	out := make([]domain.Row, 0, hardLimit)
	var last domain.AfterKey
	for rows.Next() {
		var r domain.Row
		if err := rows.Scan(
			&r.ID, &r.CreatedAt, &r.RepoName, &r.RepoID, &r.RepoHID,
			&r.ActorLogin, &r.ActorID, &r.ActorHID,
			&r.Source, &r.SourceDetail, &r.LangCode, &r.Script, &r.TextNorm,
		); err != nil {
			return nil, domain.AfterKey{}, err
		}
		out = append(out, r)
		last = domain.AfterKey{CreatedAt: r.CreatedAt, ID: r.ID}
	}
	return out, last, rows.Err()
}
