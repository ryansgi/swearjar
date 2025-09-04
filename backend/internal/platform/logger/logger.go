// Package logger provides a zerolog wrapper with opinionated defaults and
// request-scoped logging support
package logger

import (
	"context"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"swearjar/internal/platform/config/raw"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// Options configures the logger
type Options struct {
	Level        string
	Format       string
	Service      string
	Component    string
	Writer       io.Writer
	WithCaller   bool
	SampleEvery  int
	StaticFields map[string]string
}

// FromEnv builds Options using the logging-free raw config view (no cycles)
func FromEnv() Options {
	rc := raw.New().Prefix("LOG_")
	return Options{
		Level:       strings.ToLower(rc.Get("LEVEL", "debug")),
		Format:      strings.ToLower(rc.Get("FORMAT", "console")),
		Service:     rc.Get("SERVICE", ""),
		Component:   rc.Get("COMPONENT", ""),
		WithCaller:  rc.GetBool("CALLER", false),
		SampleEvery: rc.GetInt("SAMPLE_EVERY", 0),
	}
}

var (
	once   sync.Once
	root   atomic.Pointer[zerolog.Logger] // internal storage of the root logger
	inited atomic.Bool
)

// Logger is the project-wide logging type - today it's just a zerolog.Logger, but it can be swapped later
type Logger = zerolog.Logger

// Get returns the process-wide root logger as a pointer
func Get() *Logger {
	if !inited.Load() {
		Init(FromEnv())
	}
	return root.Load()
}

// Init configures zerolog and builds the root logger, safe to call once
func Init(opt Options) {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		lvl := parseLevel(opt.Level)

		var w io.Writer = os.Stdout
		if opt.Writer != nil {
			w = opt.Writer
		}
		if opt.Format == "console" {
			w = zerolog.ConsoleWriter{Out: w, TimeFormat: time.RFC3339}
		}

		ctx := zerolog.New(w).Level(lvl).With().Timestamp()

		if bi, ok := debug.ReadBuildInfo(); ok && bi != nil {
			ctx = ctx.Str("go_version", bi.GoVersion)
		}
		if opt.Service != "" {
			ctx = ctx.Str("service", opt.Service)
		}
		if opt.Component != "" {
			ctx = ctx.Str("component", opt.Component)
		}
		for k, v := range opt.StaticFields {
			ctx = ctx.Str(k, v)
		}

		log := ctx.Logger()
		if opt.WithCaller {
			log = log.With().Caller().Logger()
		}
		if opt.SampleEvery > 1 {
			log = log.Sample(&zerolog.BasicSampler{N: uint32(opt.SampleEvery)})
		}

		root.Store(&log)
		inited.Store(true)
	})
}

// parseLevel supports string-only levels
func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.DebugLevel
	}
}

type ctxKey struct{ name string }

var (
	keyRequestID = ctxKey{"req_id"}
	keyTenantID  = ctxKey{"tenant_id"}
)

// WithRequest annotates ctx with common request-scoped fields
func WithRequest(ctx context.Context, reqID, tenantID string) context.Context {
	if reqID != "" {
		ctx = context.WithValue(ctx, keyRequestID, reqID)
	}
	if tenantID != "" {
		ctx = context.WithValue(ctx, keyTenantID, tenantID)
	}
	return ctx
}

// C returns a child logger enriched from ctx (request_id, tenant_id)
func C(ctx context.Context) *Logger {
	l := Get()
	// build child off current root
	builder := l.With()
	if v := ctx.Value(keyRequestID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			builder = builder.Str("request_id", s)
		}
	}
	if v := ctx.Value(keyTenantID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			builder = builder.Str("tenant_id", s)
		}
	}
	ll := builder.Logger()
	return &ll
}

// Named returns a child logger with a component field
func Named(component string) *Logger {
	if component == "" {
		return Get()
	}
	ll := Get().With().Str("component", component).Logger()
	return &ll
}
