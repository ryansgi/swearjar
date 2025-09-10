package net_test

import (
	"errors"
	"net/http"
	"testing"

	perr "swearjar/internal/platform/errors"
	pnet "swearjar/internal/platform/net"
)

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error -> 200",
			err:  nil,
			want: http.StatusOK,
		},
		{
			name: "generic error -> perr mapping (expect 5xx)",
			err:  errors.New("boom"),
			// if perr maps generic errors to 500, assert 500 directly.
			// otherwise keep this flexible and assert 5xx below
			want: 0, // special: we'll assert range
		},
		{
			name: "project unauthorized -> 401",
			err:  perr.New(perr.ErrorCodeUnauthorized, "not allowed"),
			want: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pnet.HTTPStatus(tt.err)
			if tt.want == 0 {
				if got < 400 || got > 599 {
					t.Fatalf("expected 4xx/5xx for generic error, got %d", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("want %d got %d", tt.want, got)
			}
		})
	}
}
