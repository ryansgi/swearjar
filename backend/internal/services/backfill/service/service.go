// Package service provides the backfill service implementation
package service

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	detectdom "swearjar/internal/services/detect/domain"
	identdom "swearjar/internal/services/ident/domain"

	"swearjar/internal/modkit/repokit"
	perr "swearjar/internal/platform/errors"
	"swearjar/internal/platform/logger"
	"swearjar/internal/services/backfill/domain"
	"swearjar/internal/services/backfill/guardrails"
)

// DetectWriterPort is an alias to the detect writer port so module wiring
// does not need to import the detect domain directly from here if undesired
type DetectWriterPort = detectdom.WriterPort

// Config holds configuration options for the backfill service
type Config struct {
	// Concurrency & pacing
	Workers      int           // number of parallel hours; <=0 -> 1
	DelayPerHour time.Duration // optional sleep after each processed hour (per worker)

	// Hour-level retry
	MaxRetries int           // attempts per hour; <=0 -> 1
	RetryBase  time.Duration // base backoff for hour retries; <=0 -> 500ms

	// Timeouts applied via guardrails
	FetchTimeout time.Duration
	ReadTimeout  time.Duration

	// Range guard
	MaxRangeHours int // 0 = unlimited

	// Distributed lease for an hour (optional)
	EnableLeases bool

	// Insert tuning: per-TX insert chunk size; 0 -> default
	InsertChunk int

	// Detection toggle: if true, run detector writer after inserts
	DetectEnabled bool

	// PrincipalsConcurrency limits concurrent EnsurePrincipalsAndMaps calls; <=0 -> 2
	PrincipalsConcurrency int
}

// Service implements the backfill service
type Service struct {
	DB      repokit.TxRunner
	Binder  repokit.Binder[domain.StorageRepo] // binds q -> domain.StorageRepo
	Fetch   domain.Fetcher
	Reader  domain.ReaderFactory
	Extract domain.Extractor
	Norm    domain.Normalizer
	Cfg     Config

	// Optional detect writer; used when Cfg.DetectEnabled == true
	Detect detectdom.WriterPort

	// Lease(ctx, hourUTC, do) should take an hour-scoped advisory lock and run do()
	Lease func(ctx context.Context, hour time.Time, do func(context.Context) error) error

	principalsSem chan struct{}

	identPort identdom.Ports
}

// WithIdentService wires an ident service port for use in lookups
func (s *Service) WithIdentService(p identdom.Ports) *Service {
	s.identPort = p
	return s
}

// New constructs the backfill service
func New(
	db repokit.TxRunner,
	binder repokit.Binder[domain.StorageRepo],
	f domain.Fetcher,
	rf domain.ReaderFactory,
	ex domain.Extractor,
	n domain.Normalizer,
	cfg Config,
	lease func(context.Context, time.Time, func(context.Context) error) error,
	detectWriter detectdom.WriterPort, // optional detect writer
) *Service {
	ps := cfg.PrincipalsConcurrency
	if ps <= 0 {
		ps = 2
	}

	if db == nil {
		panic("backfill.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("backfill.Service requires a non nil Repo binder")
	}
	return &Service{
		DB: db, Binder: binder,
		Fetch: f, Reader: rf, Extract: ex, Norm: n,
		Cfg:           cfg,
		Detect:        detectWriter,
		Lease:         lease,
		principalsSem: make(chan struct{}, ps),
	}
}

// PlanRange seeds ingest_hours without processing
func (s *Service) PlanRange(ctx context.Context, start, end time.Time) error {
	start = start.Truncate(time.Hour).UTC()
	end = end.Truncate(time.Hour).UTC()
	if end.Before(start) {
		return errors.New("end before start")
	}
	return s.DB.Tx(ctx, func(q repokit.Queryer) error {
		applyTxTuning(ctx, q)
		_, err := s.Binder.Bind(q).PreseedHours(ctx, start, end)
		return err
	})
}

// RunResume drains any pending/error hours globally, ignoring bounds
func (s *Service) RunResume(ctx context.Context) error {
	w := max(s.Cfg.Workers, 1)
	var fails int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, w)

	worker := func() {
		defer func() { <-sem; wg.Done() }()
		for {
			hr, ok, err := s.nextHourAny(ctx)
			if err != nil {
				logger.C(ctx).Error().Err(err).Msg("backfill: NextHourToProcessAny failed")
				atomic.AddInt64(&fails, 1)
				_ = sleepCtx(ctx, 500*time.Millisecond)
				continue
			}
			if !ok {
				return // nothing left
			}
			if err := s.runHourWithRetry(ctx, domain.HourRef{
				Year:  hr.Year(),
				Month: int(hr.Month()),
				Day:   hr.Day(),
				Hour:  hr.Hour(),
			}); err != nil {
				logger.C(ctx).Error().Time("hour", hr).Err(err).Msg("backfill: runHour failed")
				atomic.AddInt64(&fails, 1)
			}
			if s.Cfg.DelayPerHour > 0 {
				_ = sleepCtx(ctx, s.Cfg.DelayPerHour)
			}
		}
	}

	for i := 0; i < w; i++ {
		select {
		case <-ctx.Done():
			wg.Wait()
			if fails > 0 {
				return ctx.Err()
			}
			return nil
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go worker()
	}
	wg.Wait()
	if fails > 0 {
		return errors.New("some hours failed")
	}
	return nil
}

// helper: claim next hour anywhere
func (s *Service) nextHourAny(ctx context.Context) (time.Time, bool, error) {
	var hr time.Time
	var ok bool
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		applyTxTuning(ctx, q)
		h, claimed, e := s.Binder.Bind(q).NextHourToProcessAny(ctx)
		if e != nil {
			return e
		}
		hr = h
		ok = claimed
		return nil
	})
	return hr, ok, err
}

// RunRange implements domain.RunnerPort
func (s *Service) RunRange(ctx context.Context, start, end time.Time) error {
	start = start.Truncate(time.Hour).UTC()
	end = end.Truncate(time.Hour).UTC()
	if end.Before(start) {
		return errors.New("end before start")
	}
	if s.Cfg.MaxRangeHours > 0 && int(end.Sub(start).Hours())+1 > s.Cfg.MaxRangeHours {
		return errors.New("range exceeds MaxRangeHours")
	}

	// Pre-seed all hours into ingest_hours up front
	if err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		applyTxTuning(ctx, q)
		_, err := s.Binder.Bind(q).PreseedHours(ctx, start, end)
		return err
	}); err != nil {
		return err
	}

	// Start workers that repeatedly claim the next hour and process it
	w := max(s.Cfg.Workers, 1)
	var fails int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, w)

	worker := func() {
		defer func() { <-sem; wg.Done() }()
		for {
			// Claim next hour; break when none left
			hr, ok, err := s.nextHour(ctx, start, end)
			if err != nil {
				logger.C(ctx).Error().Err(err).Msg("backfill: NextHourToProcess failed")
				atomic.AddInt64(&fails, 1)
				// Small pause on coordinator error (avoid hot loop)
				_ = sleepCtx(ctx, 500*time.Millisecond)
				continue
			}
			if !ok {
				return // no more work in range
			}
			// Process with retry; honors advisory Lease if configured
			if err := s.runHourWithRetry(ctx, domain.HourRef{
				Year:  hr.Year(),
				Month: int(hr.Month()),
				Day:   hr.Day(),
				Hour:  hr.Hour(),
			}); err != nil {
				logger.C(ctx).Error().Time("hour", hr).Err(err).Msg("backfill: runHour failed")
				atomic.AddInt64(&fails, 1)
			}
			// Optional pacing per worker
			if s.Cfg.DelayPerHour > 0 {
				_ = sleepCtx(ctx, s.Cfg.DelayPerHour)
			}
		}
	}

	// Launch the pool
	for range w {
		select {
		case <-ctx.Done():
			wg.Wait()
			if fails > 0 {
				return ctx.Err()
			}
			return nil
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	if fails > 0 {
		return errors.New("some hours failed")
	}
	return nil
}

func (s *Service) nextHour(ctx context.Context, start, end time.Time) (time.Time, bool, error) {
	var hr time.Time
	var ok bool
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		applyTxTuning(ctx, q)
		h, claimed, e := s.Binder.Bind(q).NextHourToProcess(ctx, start, end)
		if e != nil {
			return e
		}
		hr = h
		ok = claimed
		return nil
	})
	return hr, ok, err
}

func (s *Service) runHourWithRetry(ctx context.Context, hr domain.HourRef) error {
	attempts := max(s.Cfg.MaxRetries, 1)
	base := s.Cfg.RetryBase
	if base <= 0 {
		base = 500 * time.Millisecond
	}

	var last error
	for i := range attempts {
		err := s.runHour(ctx, hr)
		if err == nil {
			return nil
		}
		last = err

		// Stop early on non-retryable errors
		if !perr.Retryable(err) && perr.CodeOf(err) != perr.ErrorCodeUnavailable {
			return last
		}

		// Last attempt -> return
		if i == attempts-1 {
			break
		}

		// Exponential backoff with jitter, cap at 30s
		d := min(base<<i, 30*time.Second)
		j := d/2 + time.Duration(rand.Int63n(int64(d/2)))
		if se := sleepCtx(ctx, j); se != nil {
			return se
		}
	}
	return last
}

func (s *Service) runHour(ctx context.Context, hr domain.HourRef) (retErr error) {
	hourUTC := hr.UTC()
	if s.Lease != nil && s.Cfg.EnableLeases {
		// If another worker holds the hour, treat as clean skip
		if err := s.Lease(ctx, hourUTC, func(ctx context.Context) error { return s.runHourUnlocked(ctx, hr) }); err != nil {
			if isLeaseHeld(err) {
				return nil
			}
			return err
		}
		return nil
	}
	return s.runHourUnlocked(ctx, hr)
}

func (s *Service) runHourUnlocked(ctx context.Context, hr domain.HourRef) (retErr error) {
	// Build timeouts bundle (Hour/DB optional -> zero)
	tos := guardrails.Timeouts{
		Hour:  0,
		Fetch: s.Cfg.FetchTimeout,
		Read:  s.Cfg.ReadTimeout,
		DB:    0,
	}

	// Hour-scoped context
	hrCtx, hrCancel := guardrails.WithHour(ctx, tos)
	defer hrCancel()

	hourUTC := hr.UTC()
	startWall := time.Now()
	var fetchMS, readMS, dbMS, elapsedMS int
	var cacheHit bool
	var events, utts, inserted, deduped int
	var bytesUncompressed int64
	var errText string

	// Start (best-effort, DB-bounded)
	{
		dbCtx, dbCancel := guardrails.ForDB(hrCtx, tos)
		_ = s.DB.Tx(dbCtx, func(q repokit.Queryer) error {
			applyTxTuning(ctx, q)
			return s.Binder.Bind(q).StartHour(dbCtx, hourUTC)
		})
		dbCancel()
	}

	// Ensure Finish even on error
	defer func() {
		elapsedMS = int(time.Since(startWall).Milliseconds())
		if retErr != nil && errText == "" {
			errText = retErr.Error()
		}
		dbCtx, dbCancel := guardrails.ForDB(hrCtx, tos)
		_ = s.DB.Tx(dbCtx, func(q repokit.Queryer) error {
			applyTxTuning(ctx, q)
			return s.Binder.Bind(q).FinishHour(dbCtx, hourUTC, domain.HourFinish{
				Status:            map[bool]string{true: "error", false: "ok"}[retErr != nil],
				CacheHit:          cacheHit,
				BytesUncompressed: bytesUncompressed,
				Events:            events,
				Utterances:        utts,
				Inserted:          inserted,
				Deduped:           deduped,
				FetchMS:           fetchMS,
				ReadMS:            readMS,
				DBMS:              dbMS,
				ElapsedMS:         elapsedMS,
				ErrText:           errText,
			})
		})
		dbCancel()
	}()

	// Fetch (timeoutable)
	t0 := time.Now()
	fetchCtx, fetchCancel := guardrails.ForFetch(hrCtx, tos)
	rc, err := s.Fetch.Fetch(fetchCtx, hr)
	fetchCancel()
	fetchMS = int(time.Since(t0).Milliseconds())
	if err != nil {
		retErr = err
		return
	}

	// Best-effort cache-hit detection for metrics only
	if _, ok := any(rc).(interface{ Name() string }); ok {
		cacheHit = true
	}

	rd, err := s.Reader.New(rc)
	if err != nil {
		_ = rc.Close()
		retErr = err
		return
	}
	defer func() {
		if cerr := rd.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()

	// Read + extract (timeoutable)
	t1 := time.Now()
	var all []domain.Utterance
	readCtx, readCancel := guardrails.ForRead(hrCtx, tos)
	rerr := func() error {
		for {
			if err := readCtx.Err(); err != nil {
				return err
			}
			env, e := rd.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return e
			}
			events++
			uttsSlice := s.Extract.FromEvent(env, s.Norm)
			if len(uttsSlice) == 0 {
				continue
			}
			for i := range uttsSlice {
				if uttsSlice[i].SourceDetail == "" {
					uttsSlice[i].SourceDetail = uttsSlice[i].Source
				}
				uttsSlice[i].Source = coarse(uttsSlice[i].Source)
			}
			all = append(all, uttsSlice...)
		}
		return nil
	}()
	readCancel()
	readMS = int(time.Since(t1).Milliseconds())
	if rerr != nil {
		retErr = rerr
		return
	}
	utts = len(all)

	if statser, ok := any(rd).(interface{ Stats() (int, int64) }); ok {
		_, bytesUncompressed = statser.Stats()
	}

	// Batched insert with robust fallback
	t2 := time.Now()
	chunk := s.Cfg.InsertChunk
	if chunk <= 0 {
		chunk = 1000 // production default
	}
	for i := 0; i < len(all); i += chunk {
		end := min(i+chunk, len(all))
		ins, dd, err := s.insertBatchRobust(hrCtx, all[i:end])
		inserted += ins
		deduped += dd
		if err != nil {
			retErr = err
			dbMS += int(time.Since(t2).Milliseconds())
			return
		}
	}
	dbMS += int(time.Since(t2).Milliseconds())

	// Detection (optional) - uses utterance IDs directly; no CH lookups
	if s.Cfg.DetectEnabled && s.Detect != nil && len(all) > 0 {
		wbatch := make([]detectdom.WriteInput, 0, len(all))
		for _, u := range all {
			if u.UtteranceID == "" || u.TextNormalized == "" {
				continue
			}
			var lang *string
			if u.LangCode != nil {
				if v := strings.TrimSpace(*u.LangCode); v != "" {
					lang = &v
				}
			}

			wbatch = append(wbatch, detectdom.WriteInput{
				UtteranceID: u.UtteranceID,
				TextNorm:    u.TextNormalized,
				CreatedAt:   u.CreatedAt,
				Source:      u.Source,
				RepoHID:     identdom.RepoHID32(u.RepoID).Bytes(),
				ActorHID:    identdom.ActorHID32(u.ActorID).Bytes(),
				LangCode:    lang, // if nil, detect pipeline can infer or CH defaults will handle
			})
		}

		if len(wbatch) > 0 {
			wchunk := chunk
			if wchunk <= 0 {
				wchunk = 1000
			}
			for i := 0; i < len(wbatch); i += wchunk {
				end := min(i+wchunk, len(wbatch))
				if _, err := s.Detect.Write(hrCtx, wbatch[i:end]); err != nil {
					retErr = err
					return
				}
			}
		}
	}

	return nil
}

// insertBatchRobust writes a slice with retries; if it still fails with a
// retryable commit abort, it bisects the batch and attempts each half.
// Guarantees eventual progress (down to size 1) for retryable failures
// insertBatchRobust writes a slice with retries; if it still fails with a
// retryable commit abort, it bisects the batch and attempts each half.
// Guarantees eventual progress (down to size 1) for retryable failures
func (s *Service) insertBatchRobust(ctx context.Context, batch []domain.Utterance) (int, int, error) {
	if len(batch) == 0 {
		return 0, 0, nil
	}

	const maxAttempts = 4
	base := s.Cfg.RetryBase
	if base <= 0 {
		base = 250 * time.Millisecond
	}

	tryOnce := func(c context.Context, xs []domain.Utterance) (int, int, error) {
		var ins, dd int

		// Build HID sets for this batch (typed map keys, zero-alloc constructors)
		seenRepo, seenActor := map[identdom.HID32]int64{}, map[identdom.HID32]int64{}
		for _, u := range xs {
			if u.RepoID != 0 {
				seenRepo[identdom.RepoHID32(u.RepoID)] = u.RepoID
			}
			if u.ActorID != 0 {
				seenActor[identdom.ActorHID32(u.ActorID)] = u.ActorID
			}
		}

		// Short path: principals + maps (throttled), NO lock_timeout
		if len(seenRepo) > 0 || len(seenActor) > 0 {
			s.principalsSem <- struct{}{}
			upErr := s.identPort.EnsurePrincipalsAndMaps(c, seenRepo, seenActor)
			<-s.principalsSem
			if upErr != nil {
				return 0, 0, upErr
			}
		}

		// Main Tx: utterances only
		dbCtx, dbCancel := guardrails.ForDB(c, guardrails.Timeouts{})
		defer dbCancel()
		err := s.DB.Tx(dbCtx, func(q repokit.Queryer) error {
			applyTxTuning(c, q)
			i, d, e := s.Binder.Bind(q).InsertUtterances(dbCtx, xs)
			if e == nil {
				ins, dd = i, d
			}
			return e
		})
		return ins, dd, err
	}

	// Fixed retries on the whole batch
	var last error
	var totIns, totDd int
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ins, dd, err := tryOnce(ctx, batch)
		totIns += ins
		totDd += dd
		if err == nil {
			return totIns, totDd, nil
		}
		last = err
		if !perr.Retryable(err) || attempt == maxAttempts {
			break
		}
		// backoff with jitter, capped at 10s
		d := min(base<<(attempt-1), 10*time.Second)
		sleep := d/2 + time.Duration(rand.Int63n(int64(d/2)))
		if se := sleepCtx(ctx, sleep); se != nil {
			return totIns, totDd, err
		}
	}

	// Non-retryable -> bubble up
	if !perr.Retryable(last) {
		return totIns, totDd, last
	}

	// Retryable but flaky -> bisect
	if len(batch) == 1 {
		return totIns, totDd, last
	}
	mid := len(batch) / 2
	lIns, lDd, lErr := s.insertBatchRobust(ctx, batch[:mid])
	if lErr != nil {
		return totIns + lIns, totDd + lDd, lErr
	}
	rIns, rDd, rErr := s.insertBatchRobust(ctx, batch[mid:])
	return totIns + lIns + rIns, totDd + lDd + rDd, rErr
}

func coarse(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "push:"):
		return "commit"
	case strings.HasPrefix(l, "issues:"):
		return "issue"
	case strings.HasPrefix(l, "pr:"):
		return "pr"
	}
	if strings.Contains(l, "comment:") {
		return "comment"
	}
	return l
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SET LOCAL only lives for the duration of the current transaction
func applyTxTuning(ctx context.Context, q repokit.Queryer) {
	_, _ = q.Exec(ctx, "SET LOCAL statement_timeout = 0")

	// May no longer be needed
	//_, _ = q.Exec(ctx, "SET LOCAL idle_in_transaction_session_timeout = 0")
}

// treat "hour lease already held" as a contention signal
func isLeaseHeld(err error) bool {
	return errors.Is(err, guardrails.ErrLeaseHeld)
}
