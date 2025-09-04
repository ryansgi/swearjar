package raw

import (
	"testing"
)

// Test Get with prefixing and trimming
func TestConfGet(t *testing.T) {
	t.Setenv("APP_NAME", " swearjar ")
	t.Setenv("API_PORT", " 8080 ")

	root := New()
	api := root.Prefix("API_")

	tests := []struct {
		name   string
		conf   Conf
		key    string
		def    string
		envKey string
		want   string
	}{
		{name: "root no default used", conf: root, key: "APP_NAME", def: "x", envKey: "APP_NAME", want: "swearjar"},
		{name: "prefixed hit", conf: api, key: "PORT", def: "x", envKey: "API_PORT", want: "8080"},
		{name: "missing returns default", conf: api, key: "MISSING", def: "defv", envKey: "", want: "defv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.conf.Get(tt.key, tt.def)
			if got != tt.want {
				t.Fatalf("Get(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// Test GetBool with truthy and falsy variants and defaults
func TestConfGetBool(t *testing.T) {
	api := New().Prefix("API_")

	t.Setenv("API_T1", "true")
	t.Setenv("API_T2", "1")
	t.Setenv("API_T3", "YES")
	t.Setenv("API_F1", "false")
	t.Setenv("API_F2", "0")
	t.Setenv("API_F3", "no")
	t.Setenv("API_WS", "   true   ")

	tests := []struct {
		name string
		key  string
		def  bool
		want bool
	}{
		{name: "true", key: "T1", def: false, want: true},
		{name: "1", key: "T2", def: false, want: true},
		{name: "YES", key: "T3", def: false, want: true},
		{name: "false", key: "F1", def: true, want: false},
		{name: "0", key: "F2", def: true, want: false},
		{name: "no", key: "F3", def: true, want: false},
		{name: "whitespace trimmed", key: "WS", def: false, want: true},
		{name: "missing uses default true", key: "MISSING", def: true, want: true},
		{name: "missing uses default false", key: "MISSING2", def: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := api.GetBool(tt.key, tt.def); got != tt.want {
				t.Fatalf("GetBool(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// Test GetInt with numeric, non numeric, trimming, and defaults
func TestConfGetInt(t *testing.T) {
	sys := New().Prefix("SYS_")

	t.Setenv("SYS_OK", "42")
	t.Setenv("SYS_WS", "  7  ")
	t.Setenv("SYS_NONNUM", "12x")
	t.Setenv("SYS_NEG", "-5") // negative should fall back to default by our simple parser

	tests := []struct {
		name string
		key  string
		def  int
		want int
	}{
		{name: "numeric", key: "OK", def: 0, want: 42},
		{name: "trimmed", key: "WS", def: 1, want: 7},
		{name: "non numeric falls back", key: "NONNUM", def: 9, want: 9},
		{name: "negative falls back", key: "NEG", def: 3, want: 3},
		{name: "missing uses default", key: "MISSING", def: 11, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sys.GetInt(tt.key, tt.def); got != tt.want {
				t.Fatalf("GetInt(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

// Test Prefix composition does not collide and composes correctly
func TestPrefixComposition(t *testing.T) {
	root := New()
	log := root.Prefix("LOG_")
	api := root.Prefix("API_")
	apiLog := api.Prefix("LOG_") // nested

	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("API_LEVEL", "debug")
	t.Setenv("API_LOG_MODE", "console")

	if got := log.Get("LEVEL", ""); got != "info" {
		t.Fatalf("LOG_.Get LEVEL = %q, want %q", got, "info")
	}
	if got := api.Get("LEVEL", ""); got != "debug" {
		t.Fatalf("API_.Get LEVEL = %q, want %q", got, "debug")
	}
	if got := apiLog.Get("MODE", ""); got != "console" {
		t.Fatalf("API_LOG_.Get MODE = %q, want %q", got, "console")
	}
}
