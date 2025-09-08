// Package service implements the detect service
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"swearjar/internal/core/detector"
	"swearjar/internal/core/rulepack"
	str "swearjar/internal/platform/strings"
	hitsdom "swearjar/internal/services/hits/domain"
	utdom "swearjar/internal/services/utterances/domain"
)

// Config for the detect service
type Config struct {
	Version       int
	Workers       int
	PageSize      int
	MaxRangeHours int // 0 = unlimited
	DryRun        bool
}

// Service implements domain.RunnerPort
type Service struct {
	Utters utdom.ReaderPort
	Hits   hitsdom.WriterPort
	Det    *detector.Detector
	Cfg    Config
}

// New constructs a new detect service
func New(utters utdom.ReaderPort, hits hitsdom.WriterPort, rp *rulepack.Pack, cfg Config) *Service {
	w := cfg.Workers
	if w <= 0 {
		w = 1
	}
	ps := cfg.PageSize
	if ps <= 0 {
		ps = 5000
	}
	return &Service{
		Utters: utters,
		Hits:   hits,
		Det:    detector.New(rp, cfg.Version),
		Cfg: Config{
			Version:       cfg.Version,
			Workers:       w,
			PageSize:      ps,
			MaxRangeHours: cfg.MaxRangeHours,
			DryRun:        cfg.DryRun,
		},
	}
}

// RunRange processes utterances in the given time range, detecting hits and writing them to the hits service
func (s *Service) RunRange(ctx context.Context, start, end time.Time) error {
	start = start.Truncate(time.Hour).UTC()
	end = end.Truncate(time.Hour).UTC()
	if end.Before(start) {
		return errors.New("end before start")
	}
	if s.Cfg.MaxRangeHours > 0 && int(end.Sub(start).Hours()) > s.Cfg.MaxRangeHours {
		return errors.New("range exceeds MaxRangeHours")
	}

	after := utdom.AfterKey{}
	for {
		rows, next, err := s.Utters.List(ctx, utdom.ListInput{
			Since: start, Until: end,
			After: after, Limit: s.Cfg.PageSize,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		type chunk struct{ xs []hitsdom.HitWrite }
		out := make([]chunk, len(rows))

		sem := make(chan struct{}, s.Cfg.Workers)
		wg := sync.WaitGroup{}

		for i := range rows {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer func() { <-sem; wg.Done() }()
				u := rows[i]
				if u.TextNorm == "" {
					return // detector will get nothing; we assume backfill populates text_normalized
				}
				matches := s.Det.Scan(u.TextNorm)
				buf := make([]hitsdom.HitWrite, 0, len(matches))
				for _, m := range matches {
					for _, sp := range m.Spans {
						buf = append(buf, hitsdom.HitWrite{
							UtteranceID:     u.ID,
							CreatedAt:       u.CreatedAt,
							Term:            m.Term,
							Category:        mapCategory(m.Category), // must match hit_category_enum
							Severity:        mapSeverity(m.Severity), // must match hit_severity_enum
							SpanStart:       sp[0],
							SpanEnd:         sp[1],
							DetectorVersion: s.Cfg.Version,
							Source:          u.Source,
							RepoHID:         u.RepoHID,
							ActorHID:        u.ActorHID,
							LangCode:        str.Deref(u.LangCode),
						})
					}
				}
				out[i] = chunk{xs: buf}
			}(i)
		}
		wg.Wait()

		if !s.Cfg.DryRun {
			flat := make([]hitsdom.HitWrite, 0, 512)
			for i := range out {
				flat = append(flat, out[i].xs...)
			}
			if len(flat) > 0 {
				if err := s.Hits.WriteBatch(ctx, flat); err != nil {
					return err
				}
			}
		}

		after = next
	}
}
