package strings

import "testing"

func TestIfEmpty(t *testing.T) {
	t.Parallel()

	// non-empty slice should be returned as-is
	in := []int{1, 2, 3}
	def := []int{9}
	got := IfEmpty(in, def)
	if len(got) != 3 || got[0] != 1 {
		t.Fatalf("IfEmpty returned wrong slice: %#v", got)
	}

	// empty slice should fall back to default
	var empty []string
	def2 := []string{"x"}
	got2 := IfEmpty(empty, def2)
	if len(got2) != 1 || got2[0] != "x" {
		t.Fatalf("IfEmpty did not return default: %#v", got2)
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		s, sub string
		want   bool
	}{
		{"hello", "ell", true},     // mid substring
		{"hello", "h", true},       // prefix
		{"hello", "lo", true},      // suffix
		{"hello", "", true},        // empty always true
		{"hello", "xyz", false},    // not present
		{"short", "longer", false}, // sub longer than s
	}

	for _, c := range cases {
		if got := Contains(c.s, c.sub); got != c.want {
			t.Errorf("Contains(%q,%q)=%v want %v", c.s, c.sub, got, c.want)
		}
	}
}

func TestHasSuffix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		s, suf string
		want   bool
	}{
		{"filename.txt", ".txt", true},
		{"filename.txt", "txt", true},
		{"filename.txt", "name", false},
		{"a", "longer", false},
		{"hello", "", true}, // empty suffix always matches
	}

	for _, c := range cases {
		if got := HasSuffix(c.s, c.suf); got != c.want {
			t.Errorf("HasSuffix(%q,%q)=%v want %v", c.s, c.suf, got, c.want)
		}
	}
}

func TestMustString(t *testing.T) {
	if got := MustString("ok", "name"); got != "ok" {
		t.Fatalf("want ok got %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("want panic for empty name")
		}
	}()
	_ = MustString("   ", "name")
}

func TestMustRoot(t *testing.T) {
	cases := map[string]string{
		"/auth/":   "/auth",
		" auth  ":  "/auth",
		"//auth//": "/auth",
		"/":        "", // should panic
		"":         "", // should panic
	}
	for in, want := range cases {
		if want == "" {
			func() {
				defer func() {
					if recover() == nil {
						t.Fatalf("want panic for %q", in)
					}
				}()
				_ = MustPrefix(in)
			}()
			continue
		}
		if got := MustPrefix(in); got != want {
			t.Fatalf("in %q want %q got %q", in, want, got)
		}
	}
}
