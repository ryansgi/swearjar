package errors

import (
	stderrs "errors"
	"fmt"
	"net/http"
	"testing"
)

func TestHTTPStatusCodeMapping(t *testing.T) {
	cases := []struct {
		code ErrorCode
		want int
	}{
		{ErrorCodeNotFound, http.StatusNotFound},
		{ErrorCodeInvalidArgument, http.StatusUnprocessableEntity},
		{ErrorCodeDuplicateKey, http.StatusConflict},
		{ErrorCodeConflict, http.StatusConflict},
		{ErrorCodeValidation, http.StatusBadRequest},
		{ErrorCodeJSON, http.StatusBadRequest},
		{ErrorCodeUnauthorized, http.StatusUnauthorized},
		{ErrorCodeForbidden, http.StatusForbidden},
		{ErrorCodeTooManyRequests, http.StatusTooManyRequests},
		{ErrorCodeUnavailable, http.StatusServiceUnavailable},
		{ErrorCodeDB, http.StatusInternalServerError},
		{ErrorCodePanic, http.StatusInternalServerError},
		{ErrorCodeUnknown, http.StatusInternalServerError},
		{9999, http.StatusInternalServerError}, // default branch
	}
	for _, c := range cases {
		if got := HTTPStatusCode(c.code); got != c.want {
			t.Fatalf("HTTPStatusCode(%v) = %d, want %d", c.code, got, c.want)
		}
	}
}

func TestErrorTypeAndMethods(t *testing.T) {
	// nil *Error should render "<nil>"
	var e *Error
	if e.Error() != "<nil>" {
		t.Fatalf("nil *Error render = %q, want <nil>", e.Error())
	}

	// New / Newf
	e1 := New(ErrorCodeValidation, "bad stuff")
	if CodeOf(e1) != ErrorCodeValidation {
		t.Fatalf("CodeOf(New) = %v", CodeOf(e1))
	}
	e2 := Newf(ErrorCodeJSON, "bad json %d", 12)
	if got := e2.Error(); got != "bad json 12" {
		t.Fatalf("Newf().Error = %q", got)
	}

	// Wrap / Wrapf / Unwrap
	src := stderrs.New("root")
	e3 := Wrap(src, ErrorCodeDB, "db failed")
	if Unwrap := stderrs.Unwrap(e3); Unwrap == nil || Unwrap.Error() != "root" {
		t.Fatalf("Wrap did not keep orig")
	}
	if CodeOf(e3) != ErrorCodeDB {
		t.Fatalf("CodeOf(Wrap) = %v", CodeOf(e3))
	}
	e4 := Wrapf(src, ErrorCodeForbidden, "nope %s", "here")
	// Error() includes message + ": " + orig
	if want := "nope here: root"; e4.Error() != want {
		t.Fatalf("Wrapf().Error = %q, want %q", e4.Error(), want)
	}

	// As
	if got, ok := As(e4); !ok || got.Code() != ErrorCodeForbidden {
		t.Fatalf("As() failed for our error")
	}
	if _, ok := As(src); ok {
		t.Fatalf("As() true for foreign error")
	}

	// WithField (copy-on-write) and WithOp
	e5 := Wrap(src, ErrorCodeInvalidArgument, "oops")
	e6 := WithField(e5, "email")
	e7 := WithOp(e6, "validate")
	if fe, ok := As(e6); !ok || fe.Field() != "email" {
		t.Fatalf("WithField failed")
	}
	if oe, ok := As(e7); !ok || oe.Op() != "validate" {
		t.Fatalf("WithOp failed")
	}
	// original unchanged
	if fe0, _ := As(e5); fe0.Field() != "" || fe0.Op() != "" {
		t.Fatalf("copy-on-write mutated original")
	}

	// WithFieldChain wraps foreign error
	wrapped := WithFieldChain(src, "name")
	we, ok := As(wrapped)
	if !ok || we.Field() != "name" || we.Code() != ErrorCodeUnknown {
		t.Fatalf("WithFieldChain failed: %+v", we)
	}

	// Wire / WireFrom
	w := (&Error{code: ErrorCodeUnauthorized, msg: "nope", field: "token"}).ToWire()
	if w.Code != ErrorCodeUnauthorized || w.Message != "nope" || w.Field != "token" {
		t.Fatalf("ToWire mismatch: %+v", w)
	}
	if wf := WireFrom(nil); wf != (Wire{}) {
		t.Fatalf("WireFrom(nil) expected zero, got %+v", wf)
	}
	// WireFrom for foreign error -> Unknown with original message
	if wf := WireFrom(src); wf.Code != ErrorCodeUnknown || wf.Message != "root" {
		t.Fatalf("WireFrom(foreign) mismatch: %+v", wf)
	}
	// WireFrom for our error uses only e.msg (not "msg: orig")
	if wf := WireFrom(e4); wf.Code != ErrorCodeForbidden || wf.Message != "nope here" {
		t.Fatalf("WireFrom(ours) mismatch: %+v", wf)
	}

	// HTTP and HTTPStatus
	if st, _ := HTTP(nil); st != http.StatusOK {
		t.Fatalf("HTTP(nil) status = %d", st)
	}
	if st := HTTPStatus(e3); st != http.StatusInternalServerError {
		t.Fatalf("HTTPStatus mismatch")
	}

	// Helpers (sugar) and IsCode
	if !IsCode(NotFoundf("x"), ErrorCodeNotFound) ||
		!IsCode(InvalidArgf("x"), ErrorCodeInvalidArgument) ||
		!IsCode(DuplicateKeyf("x"), ErrorCodeDuplicateKey) ||
		!IsCode(DBf("x"), ErrorCodeDB) ||
		!IsCode(JSONErrf("x"), ErrorCodeJSON) ||
		!IsCode(PanicErrf("x"), ErrorCodePanic) ||
		!IsCode(Unauthorizedf("x"), ErrorCodeUnauthorized) ||
		!IsCode(Forbiddenf("x"), ErrorCodeForbidden) ||
		!IsCode(Conflictf("x"), ErrorCodeConflict) ||
		!IsCode(Unavailablef("x"), ErrorCodeUnavailable) {
		t.Fatalf("sugar helpers code mismatch")
	}

	// WrapIf
	if WrapIf(nil, ErrorCodeDB, "ignored") != nil {
		t.Fatalf("WrapIf(nil) should return nil")
	}
	if WrapIf(src, ErrorCodeDB, "db") == nil {
		t.Fatalf("WrapIf(non-nil) should wrap")
	}

	// Root traversal
	deep := fmt.Errorf("level2: %w", fmt.Errorf("level1: %w", src))
	if got := Root(deep); got == nil || got.Error() != "root" {
		t.Fatalf("Root() failed, got %v", got)
	}

	// ErrNotFound sentinel behavior
	if !IsCode(ErrNotFound, ErrorCodeNotFound) {
		t.Fatalf("ErrNotFound code mismatch")
	}
}
