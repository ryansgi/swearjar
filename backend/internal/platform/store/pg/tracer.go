package pg

import (
	"context"
	// temp: use zerolog here; we can hide it later behind logger package if you want
	"swearjar/internal/platform/logger"

	"github.com/rs/zerolog"
)

type QueryEvent struct {
	SQL       string
	Args      any
	ElapsedUS int64
	Err       error
	Slow      bool
}

type QueryTracer interface {
	OnQuery(ctx context.Context, ev QueryEvent)
}

// Tracer returns a logger that ALWAYS prints SQL when LogSQL=true,
// independent of the process-wide root level
func Tracer(root logger.Logger) QueryTracer {
	ll := root.Level(zerolog.DebugLevel).With().Str("component", "pg").Logger()
	return &zlTracer{log: ll}
}

type zlTracer struct{ log logger.Logger }

func (z *zlTracer) OnQuery(_ context.Context, ev QueryEvent) {
	// log normal queries at Info so they’re visible even if someone changes .Level above
	elapsedMs := float64(ev.ElapsedUS) / 1000.0
	evt := z.log.Info()
	if ev.Slow {
		evt = z.log.Warn()
	}

	evt.Float64("elapsed_ms", elapsedMs).
		Bool("slow", ev.Slow).
		Str("sql", compact(ev.SQL)).
		Interface("args", ev.Args).
		Err(ev.Err).
		Msg("pg query")
}

func compact(s string) string {
	out := make([]rune, 0, len(s))
	space := false
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' || r == ' ' {
			if !space {
				out = append(out, ' ')
				space = true
			}
			continue
		}
		space = false
		out = append(out, r)
	}
	return string(out)
}
