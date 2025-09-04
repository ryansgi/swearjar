package store

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestWithLogger_SetsOnStore(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	lg := zerolog.New(&buf) // write to buffer so we can assert output

	opt := WithLogger(lg)

	s := &Store{}
	if err := opt(s); err != nil {
		t.Fatalf("WithLogger returned error: %v", err)
	}

	// emit a log line using the store's logger, ensure it reaches our buffer
	s.Log.Info().Str("k", "v").Msg("hello")
	if buf.Len() == 0 {
		t.Fatalf("expected logger to write to buffer, got empty output")
	}

	// idempotence: applying same option again should keep working
	prevLen := buf.Len()
	if err := opt(s); err != nil {
		t.Fatalf("WithLogger second apply error: %v", err)
	}
	s.Log.Info().Msg("again")
	if buf.Len() == prevLen {
		t.Fatalf("expected additional log output after reapply")
	}
}
