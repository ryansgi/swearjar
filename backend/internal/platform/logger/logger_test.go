package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	kit "swearjar/internal/platform/testkit"

	"github.com/rs/zerolog"
)

func TestParseLevel_AllBranches(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"trace", "trace"},
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"warning", "warn"},
		{"error", "error"},
		{"fatal", "fatal"},
		{"panic", "panic"},
		{"", "debug"},
		{"   nonsense   ", "debug"},
	}
	for _, c := range cases {
		lvl := parseLevel(c.in)
		if strings.ToLower(lvl.String()) != c.want {
			t.Fatalf("parseLevel(%q) = %q, want %q", c.in, lvl, c.want)
		}
	}
}

func TestInit_Get_Named_C_WithRequest(t *testing.T) {
	var buf bytes.Buffer

	// Init with sampling enabled to exercise that branch
	Init(Options{
		Level:       "info",
		Format:      "console",
		Service:     "svc-a",
		Component:   "root",
		Writer:      &buf,
		WithCaller:  true,
		SampleEvery: 2,
		StaticFields: map[string]string{
			"build": "test",
		},
	})

	// Re-sample each logger to N=1 so lines always emit (pointer receivers)
	rv := Get().Sample(&zerolog.BasicSampler{N: 1})
	rp := &rv
	rp.Info().Str("k", "v").Msg("root-msg")

	nv := Named("api").Sample(&zerolog.BasicSampler{N: 1})
	np := &nv
	np.Info().Msg("named-msg")

	ctx := WithRequest(context.Background(), "req-123", "t-abc")
	cv := C(ctx).Sample(&zerolog.BasicSampler{N: 1})
	cp := &cv
	cp.Info().Msg("ctx-msg")

	// background child (exercise only)
	bgv := C(context.Background()).Sample(&zerolog.BasicSampler{N: 1})
	bgp := &bgv
	bgp.Info().Msg("ctx-empty")

	out := buf.String()

	// robust assertions: tolerate "key=value" vs "key= value" spacing
	kit.MustContain(t, out, "root-msg")
	kit.MustContain(t, out, "named-msg")
	kit.MustContain(t, out, "ctx-msg")
	kit.MustContain(t, out, "component=")
	kit.MustContain(t, out, "api")
	kit.MustContain(t, out, "request_id=")
	kit.MustContain(t, out, "req-123")
	kit.MustContain(t, out, "tenant_id=")
	kit.MustContain(t, out, "t-abc")
	kit.MustContain(t, out, "build=")
	kit.MustContain(t, out, "test")
	kit.MustContain(t, out, "service=")
	kit.MustContain(t, out, "svc-a")
}

func TestFromEnv_Independently(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_SERVICE", "svc-b")
	t.Setenv("LOG_COMPONENT", "comp-b")
	t.Setenv("LOG_CALLER", "true")
	t.Setenv("LOG_SAMPLE_EVERY", "5")

	opt := FromEnv()
	if strings.ToLower(opt.Level) != "warn" {
		t.Fatalf("FromEnv Level = %q, want warn", opt.Level)
	}
	if opt.Format != "json" || opt.Service != "svc-b" || opt.Component != "comp-b" {
		t.Fatalf("FromEnv fields mismatch: %+v", opt)
	}
	if !opt.WithCaller || opt.SampleEvery != 5 {
		t.Fatalf("FromEnv caller/sample mismatch: %+v", opt)
	}
}

func TestWithRequest_NoValues(t *testing.T) {
	v := C(context.Background()).Sample(&zerolog.BasicSampler{N: 1})
	p := &v
	p.Debug().Msg("no-fields")
}
