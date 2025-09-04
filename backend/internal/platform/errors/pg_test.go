package errors

import (
	stderrs "errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func pg(code, col, constraint string) *pgconn.PgError {
	return &pgconn.PgError{
		Code:           code,
		ColumnName:     col,
		ConstraintName: constraint,
	}
}

func TestDBErrorCodeMappings(t *testing.T) {
	cases := []struct {
		code string
		want ErrorCode
	}{
		{"23505", ErrorCodeDuplicateKey},    // unique violation
		{"23503", ErrorCodeInvalidArgument}, // fk violation -> invalid input
		{"23502", ErrorCodeValidation},      // not null
		{"23514", ErrorCodeValidation},      // check
		{"22001", ErrorCodeInvalidArgument}, // string truncation
		{"22P02", ErrorCodeInvalidArgument}, // invalid text representation
		{"40001", ErrorCodeDB},              // serialization failure (retryable) mapped to DB
		{"40P01", ErrorCodeDB},              // deadlock
		{"55P03", ErrorCodeDB},              // lock not available
		{"25006", ErrorCodeUnavailable},     // read-only
		{"57P03", ErrorCodeUnavailable},     // cannot connect now
		{"XXXXX", ErrorCodeDB},              // default branch
	}
	for _, c := range cases {
		got, ok := DBErrorCode(pg(c.code, "", ""))
		if !ok {
			t.Fatalf("expected ok for PgError code %s", c.code)
		}
		if got != c.want {
			t.Fatalf("DBErrorCode(%s) = %v, want %v", c.code, got, c.want)
		}
	}

	// Non-pg error path
	if _, ok := DBErrorCode(stderrs.New("nope")); ok {
		t.Fatalf("DBErrorCode should return ok=false for non-pg error")
	}
}

func TestFromPostgresVariants(t *testing.T) {
	// nil passthrough
	if FromPostgres(nil, "x") != nil {
		t.Fatalf("FromPostgres(nil) should be nil")
	}
	if FromPostgresf(nil, "x %d", 1) != nil {
		t.Fatalf("FromPostgresf(nil) should be nil")
	}

	// mapped: check codes only (PgError string includes SQLSTATE formatting)
	err := FromPostgres(pg("23505", "", ""), "insert user")
	if CodeOf(err) != ErrorCodeDuplicateKey {
		t.Fatalf("FromPostgres map code = %v", CodeOf(err))
	}
	errf := FromPostgresf(pg("22P02", "", ""), "bad: %s", "email")
	if CodeOf(errf) != ErrorCodeInvalidArgument {
		t.Fatalf("FromPostgresf code = %v, want %v", CodeOf(errf), ErrorCodeInvalidArgument)
	}
}

func TestAttachFieldFromPg(t *testing.T) {
	// prefer ColumnName when present
	withCol := AttachFieldFromPg(Wrap(pg("23502", "email", ""), ErrorCodeValidation, "oops"))
	e, ok := As(withCol)
	if !ok || e.Field() != "email" {
		t.Fatalf("AttachFieldFromPg column name failed: %+v", e)
	}

	// fallback to last token of constraint (must not be "key")
	// use a constraint ending with the field name, i.e. "users_email"
	wrapped := Wrap(pg("23505", "", "users_email"), ErrorCodeDuplicateKey, "dup")
	withField := AttachFieldFromPg(wrapped)
	e2, ok := As(withField)
	if !ok || e2.Field() != "email" {
		t.Fatalf("AttachFieldFromPg constraint token failed: %+v", e2)
	}

	// unknown/undesired token (i.e., ends with "key") -> unchanged
	wrapped2 := Wrap(pg("23505", "", "users_email_key"), ErrorCodeDuplicateKey, "dup")
	if out := AttachFieldFromPg(wrapped2); out != wrapped2 {
		t.Fatalf("AttachFieldFromPg should return input when token is 'key'")
	}

	// non-pg error should be returned as-is
	other := Wrap(stderrs.New("x"), ErrorCodeDB, "wrap")
	if out := AttachFieldFromPg(other); out != other {
		t.Fatalf("AttachFieldFromPg changed non-pg error")
	}
}

func TestFromPostgresWithField(t *testing.T) {
	// constraint that ends with actual field name so AttachFieldFromPg can infer it
	err := FromPostgresWithField(pg("23505", "", "users_email"), "insert")
	e, ok := As(err)
	if !ok || e.Field() != "email" || e.Code() != ErrorCodeDuplicateKey {
		t.Fatalf("FromPostgresWithField failed: %+v", e)
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(pg("40001", "", "")) { // serialization failure
		t.Fatalf("40001 should be retryable")
	}
	if !IsRetryable(pg("40P01", "", "")) { // deadlock
		t.Fatalf("40P01 should be retryable")
	}
	if !IsRetryable(pg("55P03", "", "")) { // lock not available
		t.Fatalf("55P03 should be retryable")
	}
	// non-retryable
	if IsRetryable(pg("23505", "", "")) {
		t.Fatalf("23505 should not be retryable")
	}
	if IsRetryable(stderrs.New("nope")) {
		t.Fatalf("non-pg error should not be retryable")
	}
}

func TestHTTPHelper(t *testing.T) {
	// OK branch
	if st, w := HTTP(nil); st != 200 || w != (Wire{}) {
		t.Fatalf("HTTP(nil) mismatch: %d %+v", st, w)
	}
	// Non-nil maps via HTTPStatus + WireFrom
	err := NotFoundf("x")
	st, w := HTTP(err)
	if st != 404 || w.Code != ErrorCodeNotFound {
		t.Fatalf("HTTP(err) mismatch: %d %+v", st, w)
	}
}
