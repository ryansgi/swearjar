package store

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestWithLogger_SetsOnOptions_AndLogs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	lg := zerolog.New(&buf) // write to buffer so we can assert output

	// Apply option to internal options bag (what Open uses)
	o := buildOptions(WithLogger(lg))

	if o.log == nil {
		t.Fatalf("expected options.log to be set")
	}

	// Emit via the logger stored in options and assert it reaches our buffer
	o.log.Info().Str("k", "v").Msg("hello")
	if buf.Len() == 0 {
		t.Fatalf("expected logger to write to buffer, got empty output")
	}

	// Idempotence: re-applying should still work
	prev := buf.Len()
	o = buildOptions(WithLogger(lg))
	o.log.Info().Msg("again")
	if buf.Len() == prev {
		t.Fatalf("expected additional log output after reapply")
	}
}
