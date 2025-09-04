package bind

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	perr "swearjar/internal/platform/errors"
)

// shared payload for many tests
type payload struct {
	Name string `json:"name" validate:"required,min=2"`
	Age  int    `json:"age" validate:"min=1"`
}

func TestParseJSON_Success(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Alice","age":3}`))
	got, err := ParseJSON[payload](req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Alice" || got.Age != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseJSON_EmptyBody_Disallow(t *testing.T) {
	req := httptest.NewRequest("POST", "/", http.NoBody)
	_, err := ParseJSON[payload](req)
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON error code, got %v (%v)", perr.CodeOf(err), err)
	}
}

// Covers: AllowEmptyBody true + EOF path in Decode
func TestParseJSON_AllowEmptyBody_EOF_OK(t *testing.T) {
	type emptyOK struct {
		Note string `json:"note"`
	}
	opts := JSONOptions{AllowEmptyBody: true}
	req := httptest.NewRequest("POST", "/", http.NoBody)

	got, err := ParseJSON[emptyOK](req, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != (emptyOK{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

// Covers: AllowEmptyBody true + MaxBytes > 0 branch
func TestParseJSON_AllowEmptyBody_WithMaxBytes(t *testing.T) {
	type emptyOK struct {
		Note string `json:"note"`
	}
	opts := JSONOptions{AllowEmptyBody: true, MaxBytes: 8}
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))

	got, err := ParseJSON[emptyOK](req, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != (emptyOK{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{`))
	_, err := ParseJSON[payload](req)
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON error code, got %v (%v)", perr.CodeOf(err), err)
	}
}

func TestParseJSON_UnknownField(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Al","age":3,"boom":1}`))
	_, err := ParseJSON[payload](req) // DisallowUnknown default true
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON error for unknown field, got %v (%v)", perr.CodeOf(err), err)
	}
}

func TestParseJSON_DisallowUnknownFalse_OK(t *testing.T) {
	opts := JSONOptions{DisallowUnknown: false}
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Al","age":3,"extra":"ok"}`))
	got, err := ParseJSON[payload](req, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Name != "Al" || got.Age != 3 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

// Forces trailing-data branch via seam
func TestParseJSON_TrailingData_Seam(t *testing.T) {
	orig := jsonMore
	jsonMore = func(_ *json.Decoder) bool { return true }
	defer func() { jsonMore = orig }()

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Al","age":3}`))
	_, err := ParseJSON[payload](req)
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON error for trailing data, got %v (%v)", perr.CodeOf(err), err)
	}
}

func TestParseJSON_ValidationError(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"A","age":0}`))
	_, err := ParseJSON[payload](req)
	if perr.CodeOf(err) != perr.ErrorCodeValidation {
		t.Fatalf("expected validation error code, got %v (%v)", perr.CodeOf(err), err)
	}
}

// Covers: peek+combine path with MaxBytes == 0
func TestParseJSON_PeekCombine_NoLimit(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Bob","age":2}`))
	_, err := ParseJSON[payload](req, JSONOptions{MaxBytes: 0})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// Covers: peek+combine path with MaxBytes > 0
func TestParseJSON_PeekCombine_WithLimit(t *testing.T) {
	// limit high enough to succeed, still goes through LimitReader branch
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Bob","age":2}`))
	_, err := ParseJSON[payload](req, JSONOptions{MaxBytes: 64})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestParseJSON_MaxBytes_Fail(t *testing.T) {
	opts := JSONOptions{MaxBytes: 5, DisallowUnknown: true, AllowEmptyBody: false}
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Alice","age":3}`))
	_, err := ParseJSON[payload](req, opts)
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON error due to size limit, got %v (%v)", perr.CodeOf(err), err)
	}
}

// Triggers InvalidValidationError in validator.Struct
func TestParseJSON_InvalidValidationError_Path(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`5`))
	_, err := ParseJSON[int](req) // non-struct validation
	// ParseJSON maps that to a JSON-coded error with message "validation error"
	if perr.CodeOf(err) != perr.ErrorCodeJSON {
		t.Fatalf("expected JSON-coded error, got %v (%v)", perr.CodeOf(err), err)
	}
}

func TestBindJSON_Success(t *testing.T) {
	mw := JSON[payload]()
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		p := FromContext[payload](r)
		if p == nil {
			t.Fatalf("expected payload in context")
		}
		if p.Name != "Alice" || p.Age != 3 {
			t.Fatalf("unexpected payload: %+v", *p)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Alice","age":3}`))
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	if !nextCalled {
		t.Fatalf("expected next to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBindJSON_Error(t *testing.T) {
	mw := JSON[payload]()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not be called on bind error")
	})
	req := httptest.NewRequest("POST", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) == "" {
		t.Fatalf("expected error body")
	}
}

func TestFromContext_Absent(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if v := FromContext[payload](req); v != nil {
		t.Fatalf("expected nil when no payload present")
	}
}

// TestTagNameFunc_JsonTagNameUsed coverage: json:"foo,omitempty", json:"-", and no json tag
func TestTagNameFunc_JsonTagNameUsed(t *testing.T) {
	Init()
	type s struct {
		Val int `json:"foo,omitempty" validate:"min=1"`
	}
	err := Get().Validator.Struct(s{Val: 0})
	field, msg := ValidationFieldAndMessage(err)
	if field != "foo" { // trimmed before comma
		t.Fatalf("expected field=foo, got %s", field)
	}
	if !strings.Contains(msg, "at least") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestTagNameFunc_DashUsesFieldName(t *testing.T) {
	Init()
	type s struct {
		Secret int `json:"-" validate:"min=1"`
	}
	err := Get().Validator.Struct(s{Secret: 0})
	field, _ := ValidationFieldAndMessage(err)
	if field != "Secret" { // falls back to struct field name
		t.Fatalf("expected field=Secret, got %s", field)
	}
}

func TestTagNameFunc_NoTagUsesFieldName(t *testing.T) {
	Init()
	type s struct {
		Plain int `validate:"min=1"`
	}
	err := Get().Validator.Struct(s{Plain: 0})
	field, _ := ValidationFieldAndMessage(err)
	if field != "Plain" {
		t.Fatalf("expected field=Plain, got %s", field)
	}
}

func TestValidationFieldAndMessage_GenericError(t *testing.T) {
	field, msg := ValidationFieldAndMessage(errors.New("boom"))
	if field != "" || msg != "boom" {
		t.Fatalf("expected generic passthrough, got field=%q msg=%q", field, msg)
	}
}

func TestTranslations_MaxAndCommaInts(t *testing.T) {
	Init()

	// register a basic validator for comma_ints so the tag is legal
	err := RegisterValidation("comma_ints", func(fl FieldLevel) bool {
		s, ok := fl.Field().Interface().(string)
		if !ok {
			return false
		}
		parts := strings.Split(s, ",")
		if len(parts) == 0 {
			return false
		}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				return false
			}
			if _, convErr := strconv.Atoi(p); convErr != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	type s struct {
		Count int    `json:"count" validate:"max=5"`
		IDs   string `json:"ids" validate:"comma_ints"`
	}

	// max message
	err1 := Get().Validator.Struct(s{Count: 6, IDs: "1,2,3"})
	_, msg1 := ValidationFieldAndMessage(err1)
	if msg1 != "count must be at most 5" {
		t.Fatalf("unexpected max message: %q", msg1)
	}

	// comma_ints message
	err2 := Get().Validator.Struct(s{Count: 1, IDs: "1, x, 3"})
	_, msg2 := ValidationFieldAndMessage(err2)
	if msg2 != "ids must be a comma-separated list of integers" {
		t.Fatalf("unexpected comma_ints message: %q", msg2)
	}
}

func TestRegisterValidation_DuplicateTag_Overwrites(t *testing.T) {
	Init()

	// register "dupe_tag" that always fails
	if err := RegisterValidation("dupe_tag", func(fl FieldLevel) bool { return false }); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}
	// overwrite with a version that always succeeds
	if err := RegisterValidation("dupe_tag", func(fl FieldLevel) bool { return true }); err != nil {
		t.Fatalf("unexpected error on second register: %v", err)
	}

	type S struct {
		N int `json:"n" validate:"dupe_tag"`
	}

	// should pass because the second registration returns true
	if err := Get().Validator.Struct(S{N: 0}); err != nil {
		t.Fatalf("expected validation to pass after overwrite, got %v", err)
	}
}
