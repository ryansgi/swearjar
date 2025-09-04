// Package errors provides a structured error type with wrapping and metadata
package errors

// Always import the project errors package as perr (platform/errors)

import (
	stderrs "errors"
	"fmt"
	"net/http"
)

// ErrorCode defines supported error codes used across services
// Values are stable for wire compatibility; add sparingly
type ErrorCode uint16

const (
	// ErrorCodeUnknown is for unclassified errors
	ErrorCodeUnknown ErrorCode = iota

	// ErrorCodePanic is for panics recovered by middleware
	ErrorCodePanic

	// ErrorCodeUnavailable is for transient errors where retry may succeed
	ErrorCodeUnavailable

	// ErrorCodeTooManyRequests is for rate limiting
	ErrorCodeTooManyRequests

	// ErrorCodeConflict is for generic editing conflicts beyond duplicate key
	ErrorCodeConflict

	// ErrorCodeUnauthorized is for auth failures
	ErrorCodeUnauthorized

	// ErrorCodeForbidden is for access control failures
	ErrorCodeForbidden

	// ErrorCodeInvalidArgument is for bad input parameters
	ErrorCodeInvalidArgument

	// ErrorCodeValidation is for validation failures (input data)
	ErrorCodeValidation

	// ErrorCodeJSON is for JSON parsing/validation errors
	ErrorCodeJSON

	// ErrorCodeNotFound is for missing resources
	ErrorCodeNotFound

	// ErrorCodeDuplicateKey is for unique constraint violations
	ErrorCodeDuplicateKey

	// ErrorCodeDB is for general database errors
	ErrorCodeDB
)

// HTTPStatusCode turns an ErrorCode into an http status code
func HTTPStatusCode(c ErrorCode) int {
	switch c {
	case ErrorCodeNotFound:
		return http.StatusNotFound
	case ErrorCodeInvalidArgument:
		return http.StatusUnprocessableEntity
	case ErrorCodeDuplicateKey, ErrorCodeConflict:
		return http.StatusConflict
	case ErrorCodeValidation, ErrorCodeJSON:
		return http.StatusBadRequest
	case ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrorCodeForbidden:
		return http.StatusForbidden
	case ErrorCodeTooManyRequests:
		return http.StatusTooManyRequests
	case ErrorCodeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorCodeDB, ErrorCodePanic, ErrorCodeUnknown:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ErrNotFound is a sentinel not found error for convenience
var ErrNotFound = New(ErrorCodeNotFound, "not found")

// Error is the structured error type with wrapping and metadata
// msg is human/developer facing; code is machine facing
// field is optional (for validation); op is optional operation tag
// orig is the wrapped cause
type Error struct {
	orig  error
	msg   string
	code  ErrorCode
	field string
	op    string
}

// Wire is the JSON-serializable form returned by the API
type Wire struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.orig != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.orig)
	}
	return e.msg
}

// Unwrap returns the wrapped error, if any
func (e *Error) Unwrap() error { return e.orig }

// Code returns the error code
func (e *Error) Code() ErrorCode { return e.code }

// Field returns the offending field, if any
func (e *Error) Field() string { return e.field }

// Op returns the operation label, if set
func (e *Error) Op() string { return e.op }

// ToWire converts an *Error to a Wire payload
func (e *Error) ToWire() Wire { return Wire{Code: e.code, Message: e.msg, Field: e.field} }

// WireFrom converts any error into a Wire payload with best-effort mapping
// If err is nil, returns the zero-value Wire (no error)
func WireFrom(err error) Wire {
	if err == nil {
		return Wire{}
	}
	if e, ok := As(err); ok {
		return e.ToWire()
	}
	return Wire{Code: ErrorCodeUnknown, Message: err.Error()}
}

// Root returns the deepest wrapped cause
func Root(err error) error {
	for err != nil {
		u := stderrs.Unwrap(err)
		if u == nil {
			return err
		}
		err = u
	}
	return nil
}

// CodeOf extracts an ErrorCode from any error, defaulting to Unknown
func CodeOf(err error) ErrorCode {
	if e, ok := As(err); ok {
		return e.code
	}
	return ErrorCodeUnknown
}

// IsCode reports whether err has the given code
func IsCode(err error, code ErrorCode) bool { return CodeOf(err) == code }

// HTTPStatus returns the mapped HTTP status for any error
func HTTPStatus(err error) int { return HTTPStatusCode(CodeOf(err)) }

// As unwraps and returns (*Error, true) if err is one of ours
func As(err error) (*Error, bool) {
	var e *Error
	if stderrs.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Mutators (copy-on-write)

// WithField attaches a field to an *Error (copy-on-write). If err isn't *Error, returns err unchanged
func WithField(err error, field string) error {
	if e, ok := As(err); ok {
		c := *e
		c.field = field
		return &c
	}
	return err
}

// WithOp attaches an operation label to an *Error (copy-on-write). If err isn't *Error, returns err unchanged
func WithOp(err error, op string) error {
	if e, ok := As(err); ok {
		c := *e
		c.op = op
		return &c
	}
	return err
}

// WithFieldChain sets field on *Error or wraps a foreign error into an *Error with Unknown code (copy-on-write)
func WithFieldChain(err error, field string) error {
	if e, ok := As(err); ok {
		c := *e
		c.field = field
		return &c
	}
	return &Error{code: ErrorCodeUnknown, msg: err.Error(), field: field, orig: err}
}

// Constructors

// New returns a new *Error with the given code and message
func New(code ErrorCode, msg string) error { return &Error{code: code, msg: msg} }

// Newf returns a new *Error with code and formatted message
func Newf(code ErrorCode, format string, a ...any) error {
	return &Error{code: code, msg: fmt.Sprintf(format, a...)}
}

// Wrap returns a new *Error that wraps orig with code and message
func Wrap(orig error, code ErrorCode, msg string) error {
	return &Error{code: code, msg: msg, orig: orig}
}

// Wrapf returns a new *Error that wraps orig with code and formatted message
func Wrapf(orig error, code ErrorCode, format string, a ...any) error {
	return &Error{code: code, msg: fmt.Sprintf(format, a...), orig: orig}
}

// WrapIf wraps only when err != nil (helper for 1-liners)
func WrapIf(err error, code ErrorCode, msg string) error {
	if err == nil {
		return nil
	}
	return Wrap(err, code, msg)
}

// Sugar

// NotFoundf returns a not found error
func NotFoundf(format string, a ...any) error { return Newf(ErrorCodeNotFound, format, a...) }

// InvalidArgf returns an invalid argument error
func InvalidArgf(format string, a ...any) error { return Newf(ErrorCodeInvalidArgument, format, a...) }

// DuplicateKeyf returns a duplicate key error
func DuplicateKeyf(format string, a ...any) error { return Newf(ErrorCodeDuplicateKey, format, a...) }

// DBf returns a general database error
func DBf(format string, a ...any) error { return Newf(ErrorCodeDB, format, a...) }

// JSONErrf returns a JSON error
func JSONErrf(format string, a ...any) error { return Newf(ErrorCodeJSON, format, a...) }

// PanicErrf returns a panic error
func PanicErrf(format string, a ...any) error { return Newf(ErrorCodePanic, format, a...) }

// Unauthorizedf returns an unauthorized error
func Unauthorizedf(format string, a ...any) error { return Newf(ErrorCodeUnauthorized, format, a...) }

// Forbiddenf returns a forbidden error
func Forbiddenf(format string, a ...any) error { return Newf(ErrorCodeForbidden, format, a...) }

// Conflictf returns a conflict error
func Conflictf(format string, a ...any) error { return Newf(ErrorCodeConflict, format, a...) }

// Unavailablef returns an unavailable error
func Unavailablef(format string, a ...any) error { return Newf(ErrorCodeUnavailable, format, a...) }

// Internalf returns a generic internal error
func Internalf(format string, a ...any) error { return Newf(ErrorCodeUnknown, format, a...) }

// HTTP bundles status + wire in one shot (nice for handlers)
func HTTP(err error) (int, Wire) {
	if err == nil {
		return http.StatusOK, Wire{}
	}
	return HTTPStatus(err), WireFrom(err)
}

// Retry semantics

// Retryable reports whether the error is retryable. Delegates to backend-specific logic.
// Currently backed by Postgres helpers in pg.go (IsRetryable), and can be extended.
func Retryable(err error) bool { return IsRetryable(err) }
