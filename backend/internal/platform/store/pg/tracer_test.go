package pg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/rs/zerolog"
)

func TestCompact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"select 1", "select 1"},
		{"  select   1  ", " select 1 "},
		{"SELECT\t*\nFROM\r\ttable WHERE  a =  1", "SELECT * FROM table WHERE a = 1"},
		{"\n\nA\n\tB  C\r\nD", " A B C D"},
		{"", ""},
	}
	for i, c := range cases {
		if got := compact(c.in); got != c.want {
			t.Fatalf("case %d: compact(%q) = %q, want %q", i, c.in, got, c.want)
		}
	}
}

func TestTracer_EmitsDebugAndWarn_WithFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zl := zerolog.New(&buf) // minimal logger; keeps JSON tidy for assertions
	lg := zl

	tr := Tracer(lg)

	type logLine struct {
		Level     string      `json:"level"`
		ElapsedMS float64     `json:"elapsed_ms"` // ms now (float)
		Slow      bool        `json:"slow"`
		SQL       string      `json:"sql"`
		Args      interface{} `json:"args"`
		Error     string      `json:"error"`
		Message   string      `json:"message"`
		Component string      `json:"component,omitempty"`
	}

	// info path (Slow=false)
	buf.Reset()
	ev := QueryEvent{
		SQL:       "SELECT  * \n FROM  t\tWHERE x = 1",
		Args:      []any{1, "two"},
		ElapsedUS: 12345, // 12.345 ms
		Err:       errors.New("boom"),
		Slow:      false,
	}
	tr.OnQuery(context.Background(), ev)

	var line logLine
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("unmarshal info log: %v\nraw=%s", err, buf.String())
	}
	if line.Level != "info" {
		t.Fatalf("expected level=info, got %q", line.Level)
	}
	wantMs := float64(ev.ElapsedUS) / 1000.0
	if math.Abs(line.ElapsedMS-wantMs) > 0.0005 {
		t.Fatalf("elapsed_ms mismatch: got %v want %v", line.ElapsedMS, wantMs)
	}
	if line.Slow {
		t.Fatalf("slow should be false")
	}
	if line.SQL != "SELECT * FROM t WHERE x = 1" {
		t.Fatalf("sql not compacted as expected: %q", line.SQL)
	}
	// args should be a JSON array [1, "two"]
	if arr, ok := line.Args.([]interface{}); !ok || len(arr) != 2 || arr[0].(float64) != 1 || arr[1].(string) != "two" {
		t.Fatalf("args unexpected: %#v", line.Args)
	}
	if line.Error != "boom" {
		t.Fatalf("error field mismatch: %q", line.Error)
	}
	if line.Message != "pg query" {
		t.Fatalf("message mismatch: %q", line.Message)
	}
	// optional: ensure component tag is set by Tracer()
	if line.Component != "pg" {
		t.Fatalf("component field mismatch: %q", line.Component)
	}

	// warn path (Slow=true)
	buf.Reset()
	ev.Slow = true
	tr.OnQuery(context.Background(), ev)

	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("unmarshal warn log: %v\nraw=%s", err, buf.String())
	}
	if line.Level != "warn" {
		t.Fatalf("expected level=warn, got %q", line.Level)
	}
	if !line.Slow {
		t.Fatalf("slow should be true")
	}
	// elapsed_ms still present and close to wantMs
	if math.Abs(line.ElapsedMS-wantMs) > 0.0005 {
		t.Fatalf("elapsed_ms mismatch on warn: got %v want %v", line.ElapsedMS, wantMs)
	}
}
