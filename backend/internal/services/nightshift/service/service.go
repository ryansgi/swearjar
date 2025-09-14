// Package service provides the Nightshift implementation
package service

import (
	"context"
	"errors"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/logger"
	nsdom "swearjar/internal/services/nightshift/domain"
	"swearjar/internal/services/nightshift/guardrails"
)

// Config controls concurrency and retention behavior
type Config struct {
	Workers int

	// DetectorVersion is stamped on archives/rollups for this run
	DetectorVersion int

	// RetentionMode is advisory for the repo: "full", "aggressive", "timebox:Nd"
	RetentionMode string

	// EnableLeases uses the shared advisory lease (optional)
	EnableLeases bool
}

// Service wires TxRunner + Binder into the domain operations
type Service struct {
	DB     repokit.TxRunner
	Binder repokit.Binder[nsdom.StorageRepo]
	Cfg    Config

	// Lease(ctx, hourUTC, do) should take an hour-scoped advisory lock and run do()
	Lease func(ctx context.Context, hour time.Time, do func(context.Context) error) error
}

// New constructs the Nightshift service
func New(
	db repokit.TxRunner,
	binder repokit.Binder[nsdom.StorageRepo],
	cfg Config,
	lease func(context.Context, time.Time, func(context.Context) error) error,
) *Service {
	if db == nil {
		panic("nightshift.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("nightshift.Service requires a non nil Repo binder")
	}
	return &Service{DB: db, Binder: binder, Cfg: cfg, Lease: lease}
}

// ApplyHour runs Nightshift for exactly one hour (idempotent)
func (s *Service) ApplyHour(ctx context.Context, hour time.Time) error {
	l := logger.C(ctx).With().Str("mod", "nightshift").Time("hour", hour.UTC()).Logger()
	l.Info().Msg("nightshift: apply-hour start")

	hour = hour.Truncate(time.Hour).UTC()

	run := func(ctx context.Context) error {
		// Transition to 'running' under the same critical section as the work
		if err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
			return s.Binder.Bind(q).Start(ctx, hour)
		}); err != nil {
			// Treat cooperative "no work" as clean skip
			if errors.Is(err, guardrails.ErrLeaseHeld) {
				l.Debug().Msg("nightshift: hour not eligible; clean skip")
				return nil
			}
			return err
		}
		return s.applyHourUnlocked(ctx, hour)
	}

	if s.Lease != nil && s.Cfg.EnableLeases {
		if err := s.Lease(ctx, hour, run); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// Another worker has the lease -> clean skip
			if errors.Is(err, guardrails.ErrLeaseHeld) {
				l.Debug().Msg("nightshift: lease not acquired; clean skip")
				return nil
			}
			l.Error().Err(err).Msg("nightshift: apply-hour failed")
			return err
		}
		return nil
	}

	// Single-process / tests
	return run(ctx)
}

func (s *Service) applyHourUnlocked(ctx context.Context, hour time.Time) (retErr error) {
	start := time.Now()
	var insertedCC, delRaw, sparedRaw int
	var loadCCMS, pruneMS int
	var errText string

	// Always record finish/clear lease, even on error
	defer func() {
		_ = s.DB.Tx(ctx, func(q repokit.Queryer) error {
			return s.Binder.Bind(q).Finish(ctx, hour, nsdom.FinishInfo{
				Status:       map[bool]string{true: "error", false: "retention_applied"}[retErr != nil],
				DetVer:       s.Cfg.DetectorVersion,
				HitsArchived: insertedCC,
				DeletedRaw:   delRaw,
				SparedRaw:    sparedRaw,
				ArchiveMS:    loadCCMS,
				PruneMS:      pruneMS,
				TotalMS:      int(time.Since(start).Milliseconds()),
				ErrText:      errText,
			})
		})
	}()

	// Populate commit_crimes (idempotent per hour+detver)
	{
		t0 := time.Now()
		err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
			n, e := s.Binder.Bind(q).WriteArchives(ctx, hour, s.Cfg.DetectorVersion)
			insertedCC = n
			return e
		})
		loadCCMS = int(time.Since(t0).Milliseconds())
		if err != nil {
			errText = err.Error()
			retErr = err
			return retErr // defer will handle Finish/lease clear
		}
	}

	// Prune per policy
	{
		t1 := time.Now()
		err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
			d, ssp, e := s.Binder.Bind(q).PruneRaw(ctx, hour, s.Cfg.RetentionMode)
			delRaw, sparedRaw = d, ssp
			return e
		})
		pruneMS = int(time.Since(t1).Milliseconds())
		if err != nil {
			errText = err.Error()
			retErr = err
			return retErr // defer will handle Finish/lease clear
		}
	}

	return nil
}

// RunRange simply loops ApplyHour across the interval
func (s *Service) RunRange(ctx context.Context, start, end time.Time) error {
	start = start.Truncate(time.Hour).UTC()
	end = end.Truncate(time.Hour).UTC()
	if end.Before(start) {
		return errors.New("end before start")
	}
	cur := start
	for !cur.After(end) {
		if err := s.ApplyHour(ctx, cur); err != nil {
			logger.C(ctx).Error().Time("hour", cur).Err(err).Msg("nightshift: ApplyHour failed")
		}
		cur = cur.Add(time.Hour)
	}
	return nil
}

// RunResume drains any hours that still need Nightshift
func (s *Service) RunResume(ctx context.Context) error {
	w := s.Cfg.Workers
	if w <= 0 {
		w = 2
	}
	sem := make(chan struct{}, w)
	errs := make(chan error, w)

	worker := func() {
		defer func() { <-sem }()
		for {
			var hr time.Time
			var ok bool
			err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
				h, claimed, e := s.Binder.Bind(q).NextHourNeedingWork(ctx)
				hr, ok = h, claimed
				return e
			})
			if err != nil {
				errs <- err
				time.Sleep(250 * time.Millisecond)
				continue
			}
			if !ok {
				return
			}
			if e := s.ApplyHour(ctx, hr); e != nil {
				errs <- e
			}
		}
	}

	for i := 0; i < w; i++ {
		sem <- struct{}{}
		go worker()
	}

	// Best-effort: wait for workers to drain. Since we don't track completion here,
	// just sleep briefly; callers usually run this under a supervisor
	time.Sleep(100 * time.Millisecond)
	close(errs)
	return nil
}
