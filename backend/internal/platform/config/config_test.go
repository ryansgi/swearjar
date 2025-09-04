package config

import (
	"net/url"
	"testing"
	"time"

	kit "swearjar/internal/platform/testkit"
)

func TestPrefixAndKey(t *testing.T) {
	root := New()
	api := root.Prefix("API_")
	if got := api.key("PORT"); got != "API_PORT" {
		t.Fatalf("key() = %q, want %q", got, "API_PORT")
	}
	// nested prefix
	apiLog := api.Prefix("LOG_")
	if got := apiLog.key("LEVEL"); got != "API_LOG_LEVEL" {
		t.Fatalf("nested key() = %q, want %q", got, "API_LOG_LEVEL")
	}
}

// Must* panics

func TestMustString(t *testing.T) {
	c := New().Prefix("APP_")
	t.Setenv("APP_NAME", "  swearjar ")
	got := c.MustString("NAME")
	if got != "swearjar" {
		t.Fatalf("MustString = %q, want %q", got, "swearjar")
	}

	kit.MustPanic(t, func() { _ = c.MustString("MISSING") })
}

func TestMustInt(t *testing.T) {
	c := New().Prefix("SVC_")
	t.Setenv("SVC_WORKERS", "  8 ")
	if got := c.MustInt("WORKERS"); got != 8 {
		t.Fatalf("MustInt = %d, want %d", got, 8)
	}
	kit.MustPanic(t, func() { _ = c.MustInt("MISSING") })
	t.Setenv("SVC_BAD", "x")
	kit.MustPanic(t, func() { _ = c.MustInt("BAD") })
}

func TestMustBool(t *testing.T) {
	c := New().Prefix("F_")
	t.Setenv("F_ON", " true ")
	if !c.MustBool("ON") {
		t.Fatalf("MustBool true expected")
	}
	kit.MustPanic(t, func() { _ = c.MustBool("MISSING") })
	t.Setenv("F_BAD", "notabool")
	kit.MustPanic(t, func() { _ = c.MustBool("BAD") })
}

func TestMustDuration(t *testing.T) {
	c := New().Prefix("D_")
	t.Setenv("D_TIMEOUT", " 250ms ")
	if got := c.MustDuration("TIMEOUT"); got != 250*time.Millisecond {
		t.Fatalf("MustDuration = %v, want %v", got, 250*time.Millisecond)
	}
	t.Setenv("D_BAD", "nope")
	kit.MustPanic(t, func() { _ = c.MustDuration("BAD") })
}

func TestMustURL(t *testing.T) {
	c := New().Prefix("U_")
	t.Setenv("U_BASE", "https://example.com/api")
	u := c.MustURL("BASE")
	if _, err := url.Parse("https://example.com/api"); err != nil || !u.IsAbs() {
		t.Fatalf("MustURL returned non-absolute URL")
	}
	t.Setenv("U_BAD1", "://bad")
	kit.MustPanic(t, func() { _ = c.MustURL("BAD1") })
	t.Setenv("U_BAD2", "/relative")
	kit.MustPanic(t, func() { _ = c.MustURL("BAD2") })
}

func TestMustPort(t *testing.T) {
	c := New().Prefix("P_")
	t.Setenv("P_PORT", "4000")
	if got := c.MustPort("PORT"); got != ":4000" {
		t.Fatalf("MustPort = %q, want %q", got, ":4000")
	}
	t.Setenv("P_BAD", "abc")
	kit.MustPanic(t, func() { _ = c.MustPort("BAD") })
	t.Setenv("P_OOB", "70000")
	kit.MustPanic(t, func() { _ = c.MustPort("OOB") })
}

func TestRequire(t *testing.T) {
	c := New().Prefix("REQ_")
	t.Setenv("REQ_A", "x")
	t.Setenv("REQ_B", "y")
	// should not panic
	c.Require("A", "B")

	// missing C should panic
	kit.MustPanic(t, func() { c.Require("A", "C") })
}

// May* fallbacks

func TestMayString(t *testing.T) {
	c := New().Prefix("S_")
	if got := c.MayString("MISSING", "def"); got != "def" {
		t.Fatalf("MayString default = %q, want %q", got, "def")
	}
	t.Setenv("S_NAME", " swearjar ")
	if got := c.MayString("NAME", "x"); got != "swearjar" {
		t.Fatalf("MayString value = %q, want %q", got, "swearjar")
	}
}

func TestMayInt(t *testing.T) {
	c := New().Prefix("I_")
	if got := c.MayInt("MISSING", 9); got != 9 {
		t.Fatalf("MayInt default = %d, want %d", got, 9)
	}
	t.Setenv("I_OK", " 7 ")
	if got := c.MayInt("OK", 0); got != 7 {
		t.Fatalf("MayInt ok = %d, want %d", got, 7)
	}
	t.Setenv("I_BAD", "x")
	if got := c.MayInt("BAD", 3); got != 3 {
		t.Fatalf("MayInt bad -> default = %d, want %d", got, 3)
	}
}

func TestMayBool(t *testing.T) {
	c := New().Prefix("B_")
	if got := c.MayBool("MISSING", true); got != true {
		t.Fatalf("MayBool default true expected")
	}
	t.Setenv("B_T", "true")
	if got := c.MayBool("T", false); got != true {
		t.Fatalf("MayBool true expected")
	}
	t.Setenv("B_BAD", "nope")
	if got := c.MayBool("BAD", false); got != false {
		t.Fatalf("MayBool bad -> default false expected")
	}
}

func TestMayDuration(t *testing.T) {
	c := New().Prefix("DUR_")
	if got := c.MayDuration("MISS", 5*time.Second); got != 5*time.Second {
		t.Fatalf("MayDuration default expected")
	}
	t.Setenv("DUR_OK", "150ms")
	if got := c.MayDuration("OK", time.Second); got != 150*time.Millisecond {
		t.Fatalf("MayDuration ok = %v, want %v", got, 150*time.Millisecond)
	}
	t.Setenv("DUR_BAD", "nope")
	if got := c.MayDuration("BAD", time.Minute); got != time.Minute {
		t.Fatalf("MayDuration bad -> default expected")
	}
}

func TestMayCSV(t *testing.T) {
	c := New().Prefix("CSV_")
	def := []string{"a", "b"}
	if got := c.MayCSV("MISS", def); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("MayCSV default mismatch: %#v", got)
	}
	t.Setenv("CSV_VALS", " one, two , ,three ,, ")
	got := c.MayCSV("VALS", nil)
	want := []string{"one", "two", "three"}
	if len(got) != len(want) {
		t.Fatalf("MayCSV len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MayCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMayEnum(t *testing.T) {
	c := New().Prefix("E_")

	// empty uses default and does not panic
	if got := c.MayEnum("MISS", "json", "json", "console"); got != "json" {
		t.Fatalf("MayEnum default = %q, want %q", got, "json")
	}

	t.Setenv("E_FMT", "Console")
	if got := c.MayEnum("FMT", "json", "json", "console"); got != "Console" {
		t.Fatalf("MayEnum allowed value = %q, want %q", got, "Console")
	}

	t.Setenv("E_BAD", "xml")
	kit.MustPanic(t, func() { _ = c.MayEnum("BAD", "json", "json", "console") })
}

func TestRequireWhitespaceIsMissing(t *testing.T) {
	c := New().Prefix("REQ_")
	t.Setenv("REQ_WS", "   ")
	kit.MustPanic(t, func() { c.Require("WS") })
}

func TestMayCSVAllEmptyFallsBackToDefault(t *testing.T) {
	c := New().Prefix("CSV_")
	def := []string{"fallback"}
	t.Setenv("CSV_VALS", " , ,  ,")
	got := c.MayCSV("VALS", def)
	if len(got) != 1 || got[0] != "fallback" {
		t.Fatalf("MayCSV all-empty -> default mismatch: %#v", got)
	}
}

func TestMayEnumEmptyDefaultAndMissingEnv(t *testing.T) {
	c := New().Prefix("E_")
	if got := c.MayEnum("MISSING", "", "json", "console"); got != "" {
		t.Fatalf("MayEnum with empty def and missing env = %q, want empty string", got)
	}
}
