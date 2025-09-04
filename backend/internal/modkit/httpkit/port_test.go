package httpkit

import (
	"errors"
	"net/http"
	"testing"

	perrs "swearjar/internal/platform/errors"
)

func TestPort_Parse_MissingHeader(t *testing.T) {
	t.Parallel()

	p := NewPortFunc(func(string) (string, string, error) {
		t.Fatalf("parser should not be called when header is missing")
		return "", "", nil
	})

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	uid, tid, err := p.Parse(req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if uid != "" || tid != "" {
		t.Fatalf("expected empty ids, got %q %q", uid, tid)
	}

	var pe *perrs.Error
	if !errors.As(err, &pe) || pe.Code() != perrs.ErrorCodeUnauthorized {
		t.Fatalf("expected unauthorized perrs error, got %#v", err)
	}
}

func TestPort_Parse_WrongSchemeAndEmptyToken(t *testing.T) {
	t.Parallel()

	p := NewPortFunc(func(string) (string, string, error) {
		t.Fatalf("parser should not be called on malformed header")
		return "", "", nil
	})

	// wrong scheme
	req1, _ := http.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("Authorization", "Basic abc")
	_, _, err := p.Parse(req1)
	if err == nil {
		t.Fatalf("expected error for wrong scheme")
	}

	// empty token after Bearer
	req2, _ := http.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Bearer   \t ")
	_, _, err = p.Parse(req2)
	if err == nil {
		t.Fatalf("expected error for empty token")
	}
}

func TestPort_Parse_InvalidToken(t *testing.T) {
	t.Parallel()

	calls := 0
	p := NewPortFunc(func(tok string) (string, string, error) {
		calls++
		if tok != "bad.token" {
			t.Fatalf("expected raw token bad.token, got %q", tok)
		}
		return "", "", errors.New("parse failed")
	})

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad.token")

	uid, tid, err := p.Parse(req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if uid != "" || tid != "" {
		t.Fatalf("expected empty ids on invalid token, got %q %q", uid, tid)
	}
	if calls != 1 {
		t.Fatalf("expected parser called once, got %d", calls)
	}
}

func TestPort_Parse_ValidToken_CaseInsensitiveAndTrim(t *testing.T) {
	t.Parallel()

	calls := 0
	p := NewPortFunc(func(tok string) (string, string, error) {
		calls++
		if tok != "abc123" {
			t.Fatalf("expected trimmed token abc123, got %q", tok)
		}
		return "user-1", "ten-2", nil
	})

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "   BEARER   abc123   ")

	uid, tid, err := p.Parse(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != "user-1" || tid != "ten-2" {
		t.Fatalf("unexpected ids, got %q %q", uid, tid)
	}
	if calls != 1 {
		t.Fatalf("expected parser called once, got %d", calls)
	}
}

func TestPort_Parse_NilParser(t *testing.T) {
	t.Parallel()

	// zero value friendly guard when parse is nil
	var p Port

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")

	_, _, err := p.Parse(req)
	if err == nil {
		t.Fatalf("expected error when parser is nil")
	}
}
