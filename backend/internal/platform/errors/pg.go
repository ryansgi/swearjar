package errors

// Postgres-specific helpers for mapping pgx errors to project ErrorCode, extracting fields, and retry semantics

import (
	"context"
	stderrs "errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Common SQLSTATE codes we care about
const (
	pgErrUniqueViolation           = "23505"
	pgErrForeignKeyViolation       = "23503"
	pgErrNotNullViolation          = "23502"
	pgErrCheckViolation            = "23514"
	pgErrStringDataRightTruncation = "22001"
	pgErrInvalidTextRepresentation = "22P02"

	pgErrSerializationFailure   = "40001"
	pgErrDeadlockDetected       = "40P01"
	pgErrLockNotAvailable       = "55P03"
	pgErrReadOnlySQLTransaction = "25006"
	pgErrCannotConnectNow       = "57P03" // i.e. startup in progress
)

// ExtractPgError returns (*pgconn.PgError, true) if the root cause is a PgError.
func ExtractPgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if stderrs.As(Root(err), &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// IsSQLState reports whether the error is a Postgres error with the given SQLSTATE code
func IsSQLState(err error, code string) bool {
	pgErr, ok := ExtractPgError(err)
	return ok && pgErr.Code == code
}

// Human-friendly predicates for common constraint classes.

// IsDuplicateKey reports whether the error is a unique constraint violation
func IsDuplicateKey(err error) bool { return IsSQLState(err, pgErrUniqueViolation) }

// IsForeignKeyViolation reports whether the error is a foreign key constraint violation
func IsForeignKeyViolation(err error) bool { return IsSQLState(err, pgErrForeignKeyViolation) }

// IsNotNullViolation reports whether the error is a not-null constraint violation
func IsNotNullViolation(err error) bool { return IsSQLState(err, pgErrNotNullViolation) }

// IsCheckViolation reports whether the error is a check constraint violation
func IsCheckViolation(err error) bool { return IsSQLState(err, pgErrCheckViolation) }

// IsSerializationFailure reports whether the error is a serialization failure
func IsSerializationFailure(err error) bool { return IsSQLState(err, pgErrSerializationFailure) }

// IsDeadlock reports whether the error is a deadlock detected error
func IsDeadlock(err error) bool { return IsSQLState(err, pgErrDeadlockDetected) }

// IsLockNotAvailable reports whether the error is a lock not available error
func IsLockNotAvailable(err error) bool { return IsSQLState(err, pgErrLockNotAvailable) }

// IsConnectionUnavailable reports whether the error is a "cannot connect now" error
func IsConnectionUnavailable(err error) bool { return IsSQLState(err, pgErrCannotConnectNow) }

// DBErrorCode maps a Postgres error to an ErrorCode with an ok flag
// !ok means err wasn't a PgError; caller may fall back to generic handling
func DBErrorCode(err error) (ErrorCode, bool) {
	var pgErr *pgconn.PgError
	if !stderrs.As(err, &pgErr) {
		return ErrorCodeUnknown, false
	}

	switch pgErr.Code {
	case pgErrUniqueViolation:
		return ErrorCodeDuplicateKey, true

	case pgErrForeignKeyViolation:
		// Typically this means input referenced a missing row: classify as invalid input
		return ErrorCodeInvalidArgument, true

	case pgErrNotNullViolation, pgErrCheckViolation:
		return ErrorCodeValidation, true

	case pgErrStringDataRightTruncation, pgErrInvalidTextRepresentation:
		return ErrorCodeInvalidArgument, true

	case pgErrSerializationFailure, pgErrDeadlockDetected, pgErrLockNotAvailable:
		// Retryable server-side contention
		return ErrorCodeDB, true

	case pgErrReadOnlySQLTransaction, pgErrCannotConnectNow:
		// Transient/unavailable dependency
		return ErrorCodeUnavailable, true
	}

	// Default: still a DB error
	return ErrorCodeDB, true
}

// FromPostgres wraps a pg error with a mapped ErrorCode and message.
// If err is nil, returns nil
func FromPostgres(err error, msg string) error {
	if err == nil {
		return nil
	}
	if code, ok := DBErrorCode(err); ok {
		return Wrap(err, code, msg)
	}
	return Wrap(err, ErrorCodeDB, msg)
}

// FromPostgresf is the formatted variant of FromPostgres
func FromPostgresf(err error, format string, a ...any) error {
	if err == nil {
		return nil
	}
	if code, ok := DBErrorCode(err); ok {
		return Wrap(err, code, fmt.Sprintf(format, a...))
	}
	return Wrap(err, ErrorCodeDB, fmt.Sprintf(format, a...))
}

// AttachFieldFromPg tries to enrich an error with a field name derived from PgError.
// Priority: ColumnName -> last token of ConstraintName (i.e., users_email_key -> email).
// Returns the original error if no field can be inferred
func AttachFieldFromPg(err error) error {
	var pgErr *pgconn.PgError
	if !stderrs.As(Root(err), &pgErr) {
		return err
	}
	if col := strings.TrimSpace(pgErr.ColumnName); col != "" {
		return WithField(err, col)
	}
	if c := strings.TrimSpace(pgErr.ConstraintName); c != "" {
		//   users_email_key -> email
		//   users_customer_id_fkey -> fkey (not great) so prefer ColumnName when available
		tok := c
		if i := strings.LastIndex(c, "_"); i >= 0 && i+1 < len(c) {
			tok = c[i+1:]
		}
		if tok != "" && tok != "key" {
			return WithField(err, tok)
		}
	}
	return err
}

// FromPostgresWithField wraps the error (like FromPostgres) and then attempts to
// attach a field name if discoverable from the PgError metadata
func FromPostgresWithField(err error, msg string) error {
	return AttachFieldFromPg(FromPostgres(err, msg))
}

// IsRetryable reports whether a database error represents a transient condition
// worth retrying. It handles both structured *pgconn.PgError codes and the
// generic pgx text seen on commit (e.g. "commit unexpectedly resulted in rollback")
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Do not retry local cancellations/timeouts; let the caller decide higher-level retries
	if stderrs.Is(err, context.Canceled) || stderrs.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Unwrap to the root cause so we can see either PgError or the commit text
	root := Root(err)

	// Structured Postgres errors by SQLSTATE
	var pgErr *pgconn.PgError
	if stderrs.As(root, &pgErr) {
		switch pgErr.Code {
		case pgErrSerializationFailure, pgErrDeadlockDetected, pgErrLockNotAvailable:
			return true
		default:
			return false
		}
	}

	// Fallback: text patterns emitted by pgx/driver on commit/abort or lock/timeout cases
	s := strings.ToLower(root.Error())
	switch {
	case strings.Contains(s, "commit unexpectedly resulted in rollback"),
		strings.Contains(s, "deadlock detected"),
		strings.Contains(s, "could not serialize access"),
		strings.Contains(s, "serialization failure"),
		strings.Contains(s, "canceling statement due to statement timeout"),
		strings.Contains(s, "canceling statement due to lock timeout"),
		strings.Contains(s, "could not obtain lock on row"),
		strings.Contains(s, "terminating connection due to administrator command"):
		return true
	default:
		return false
	}
}
