// internal/normalize/normalize_test.go
package normalize

import (
	"testing"
)

// Test table covers each stage and combined pipelines.
func TestNormalize_Table(t *testing.T) {
	n := New()

	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "identity ascii",
			in:   "hello world",
			out:  "hello world",
		},
		{
			name: "utf8 repair drops invalid bytes",
			in:   string([]byte{0xff, 'f', 'o', 'o', 0x80, ' ', 'b', 'a', 'r'}),
			out:  "foo bar",
		},
		{
			name: "case fold",
			in:   "FuCk",
			out:  "fuck",
		},
		{
			name: "remove zero-widths",
			in:   "f\u200Bu\u200Dck", // ZERO WIDTH SPACE + ZERO WIDTH JOINER
			out:  "fuck",
		},
		{
			name: "remove combining marks",
			in:   "cafe\u0301", // "café" using combining acute accent
			out:  "cafe",
		},
		{
			name: "width fold fullwidth",
			in:   "ＦＵＣＫ bot", // fullwidth letters
			out:  "fuck bot",
		},
		{
			name: "nfkc ligature",
			in:   "oﬃce", // ﬁ ligature
			out:  "office",
		},
		{
			name: "leet folding basic",
			in:   "5h!t 3lite f@ce 700l",
			out:  "shit elite face tool",
		},
		{
			name: "collapse whitespace",
			in:   "a\t\tb\nc   d",
			out:  "a b c d",
		},
		{
			name: "combined normalization",
			in:   "  ZW\u200B N\u200C B\uFEFF S  \t\n", // zero-widths + spaces + FEFF
			out:  "zw nb s",
		},
		{
			name: "idempotent",
			in:   n.Normalize("Ｆ@N\t\tB\u200Dor  "),
			out:  "fan bor",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := n.Normalize(tc.in)
			if got != tc.out {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.out)
			}
			// Idempotence check: normalize again should be identical
			got2 := n.Normalize(got)
			if got2 != got {
				t.Fatalf("Normalize not idempotent: %q -> %q", got, got2)
			}
		})
	}
}

// Spot-check internal helpers in isolation.
func TestLeetFold(t *testing.T) {
	in := "4b0u7 !$ 3l337"
	want := "about is e llet"
	got := leetFold(in)
	if got != want {
		t.Fatalf("leetFold(%q) = %q, want %q", in, got, want)
	}
}

func TestCollapseSpaces(t *testing.T) {
	in := " \t a \n b   c \r\n "
	want := "a b c"
	got := collapseSpaces(in)
	if got != want {
		t.Fatalf("collapseSpaces(%q) = %q, want %q", in, got, want)
	}
}
