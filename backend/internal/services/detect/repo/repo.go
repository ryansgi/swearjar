// Package repo provides repository implementations for the detect service
package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/detect/domain"
)

// binder implements repokit.Binder[domain.StorageRepo]
type binder struct{}

// NewPG returns a Postgres binder for domain.StorageRepo
func NewPG() repokit.Binder[domain.StorageRepo] { return binder{} }

// Bind implements repokit.Binder
func (binder) Bind(q repokit.Queryer) domain.StorageRepo { return &pg{q: q} }

type pg struct{ q repokit.Queryer }

// ListUtterances returns utterances committed within [since, until), after the
// (afterCommitted, afterID) cursor, up to limit. Ordered by (created_at, id)
func (s *pg) ListUtterances(ctx context.Context,
	since, until time.Time,
	afterCommitted time.Time, afterID int64,
	limit int,
) ([]domain.Utterance, error) {
	const q = `
		SELECT id, COALESCE(text_normalized, ''), text, created_at
		FROM utterances
		WHERE created_at >= $1 AND created_at < $2
			AND (created_at, id) > ($3, $4)
		ORDER BY created_at, id
		LIMIT $5`
	rows, err := s.q.Query(ctx, q, since, until, afterCommitted, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Utterance
	for rows.Next() {
		var u domain.Utterance
		if err := rows.Scan(&u.ID, &u.TextNormalized, &u.TextRaw, &u.CommittedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// WriteHitsBatch writes a batch of hits; ignores duplicates
func (s *pg) WriteHitsBatch(ctx context.Context, xs []domain.Hit) error {
	if len(xs) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`INSERT INTO hits
		(utterance_id, term, category, severity, span_start, span_end, detector_version)
		VALUES `)
	args := make([]any, 0, len(xs)*7)
	for i, h := range xs {
		if i > 0 {
			sb.WriteByte(',')
		}
		base := i*7 + 1
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d)", base, base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, h.UtteranceID, h.Term, h.Category, h.Severity, h.StartOffset, h.EndOffset, h.DetectorVersion)
	}
	sb.WriteString(` ON CONFLICT (utterance_id, term, span_start, span_end, detector_version) DO NOTHING`)
	_, err := s.q.Exec(ctx, sb.String(), args...)
	return err
}
