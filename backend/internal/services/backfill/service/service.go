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

	"swearjar/internal/modkit/repokit"
	perr "swearjar/internal/platform/errors"
	"swearjar/internal/services/backfill/domain"
	"swearjar/internal/services/backfill/guardrails"
)

// DetectWriterPort is an alias to the detect writer port so module wiring
// does not need to import the detect domain directly from here if undesired.
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
	// DB timeout is handled with SET LOCAL lock/statement/deadlock (per-tx)

	// Range guard
	MaxRangeHours int // 0 = unlimited

	// Distributed lease for an hour (optional)
	EnableLeases bool

	// Insert tuning: per-TX insert chunk size; 0 -> default
	InsertChunk int

	// Detection toggle: if true, run detector writer after inserts
	DetectEnabled bool
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
	if db == nil {
		panic("backfill.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("backfill.Service requires a non nil Repo binder")
	}
	return &Service{
		DB: db, Binder: binder,
		Fetch: f, Reader: rf, Extract: ex, Norm: n,
		Cfg:    cfg,
		Detect: detectWriter,
		Lease:  lease,
	}
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

	hrs := enumerate(start, end)
	w := max(s.Cfg.Workers, 1)

	// Sequential path
	if w == 1 {
		var fails int64
		for _, hr := range hrs {
			if err := s.runHourWithRetry(ctx, hr); err != nil {
				atomic.AddInt64(&fails, 1)
			}
			if s.Cfg.DelayPerHour > 0 {
				if err := sleepCtx(ctx, s.Cfg.DelayPerHour); err != nil {
					return err
				}
			}
		}
		if fails > 0 {
			return errors.New("some hours failed")
		}
		return nil
	}

	// Worker pool
	var fails int64
	sem := make(chan struct{}, w)
	var wg sync.WaitGroup
	for _, hr := range hrs {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := s.runHourWithRetry(ctx, hr); err != nil {
				atomic.AddInt64(&fails, 1)
			}
			if s.Cfg.DelayPerHour > 0 {
				_ = sleepCtx(ctx, s.Cfg.DelayPerHour)
			}
		}()
	}
	wg.Wait()
	if fails > 0 {
		return errors.New("some hours failed")
	}
	return nil
}

func (s *Service) runHourWithRetry(ctx context.Context, hr domain.HourRef) error {
	attempts := max(s.Cfg.MaxRetries, 1)
	base := s.Cfg.RetryBase
	if base <= 0 {
		base = 500 * time.Millisecond
	}

	var last error
	for i := 0; i < attempts; i++ {
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
		// If another worker holds the hour, treat as clean skip.
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
			applyTxTuning(ctx, q, false)
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
			applyTxTuning(ctx, q, false)
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

	// Detection (optional)
	if s.Cfg.DetectEnabled && s.Detect != nil && len(all) > 0 {
		type kkey struct{ eventID, source string }
		counts := make(map[kkey]int, len(all))
		keys := make([]domain.UKey, 0, len(all))
		for _, u := range all {
			k := kkey{eventID: u.EventID, source: u.Source}
			counts[k]++
			ord := counts[k] - 1
			keys = append(keys, domain.UKey{
				EventID: u.EventID,
				Source:  u.Source,
				Ordinal: ord,
			})
		}

		var idMap map[domain.UKey]string
		if err := s.DB.Tx(hrCtx, func(q repokit.Queryer) error {
			var e error
			idMap, e = s.Binder.Bind(q).LookupIDs(hrCtx, keys)
			return e
		}); err != nil {
			retErr = err
			return
		}

		buildLangPtr := func(s string) *string {
			if s == "" {
				return nil
			}
			v := s
			return &v
		}
		wbatch := make([]detectdom.WriteInput, 0, len(all))
		for i, u := range all {
			k := keys[i]
			uid := idMap[k]
			if uid == "" || u.TextNormalized == "" {
				continue
			}
			wbatch = append(wbatch, detectdom.WriteInput{
				UtteranceID: uid,
				TextNorm:    u.TextNormalized,

				// denorms for hot filters / correct created_at
				CreatedAt: u.CreatedAt,
				Source:    u.Source, // coarse()'d above
				RepoName:  u.Repo,
				// RepoHID / ActorHID can be added here if your extractor populates them
				LangCode: buildLangPtr(u.LangCode),
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
		dbCtx, dbCancel := guardrails.ForDB(c, guardrails.Timeouts{})
		defer dbCancel()

		err := s.DB.Tx(dbCtx, func(q repokit.Queryer) error {
			// Session tuning: no server-side statement timeouts, quicker deadlock detect,
			// bounded lock waits. Avoid changing the transaction isolation here.
			applyTxTuning(ctx, q, true)

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

func enumerate(start, end time.Time) []domain.HourRef {
	var out []domain.HourRef
	for cur := start; !cur.After(end); cur = cur.Add(time.Hour) {
		out = append(out, domain.HourRef{
			Year:  cur.Year(),
			Month: int(cur.Month()),
			Day:   cur.Day(),
			Hour:  cur.Hour(),
		})
	}
	return out
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
func applyTxTuning(ctx context.Context, q repokit.Queryer, heavy bool) {
	_, _ = q.Exec(ctx, "SET LOCAL statement_timeout = 0")
	_, _ = q.Exec(ctx, "SET LOCAL idle_in_transaction_session_timeout = 0")
	if heavy {
		_, _ = q.Exec(ctx, "SET LOCAL deadlock_timeout = '200ms'")
		_, _ = q.Exec(ctx, "SET LOCAL lock_timeout = '5s'")
	}
}

// treat "hour lease already held" as a contention signal
func isLeaseHeld(err error) bool {
	return errors.Is(err, guardrails.ErrLeaseHeld)
}
