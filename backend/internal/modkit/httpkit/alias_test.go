package httpkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// mkReq builds an *http.Request with an optional body
func mkReq(t *testing.T, method string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "http://x.test/y", body)
	if err != nil {
		t.Fatalf("mkReq: %v", err)
	}
	return req
}

// run executes a Handler and returns status code and body
func run(h Handler, r *http.Request) (int, string) {
	rec := httptest.NewRecorder()
	h(rec, r)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }() // explicitly ignore close error

	b, _ := io.ReadAll(res.Body)
	return rec.Code, string(b)
}

func TestAliases_SimpleConstructors(t *testing.T) {
	// just ensure they return a non-zero Response so the line is executed
	if v := reflect.ValueOf(OK("x")); v.IsZero() {
		t.Fatal("OK returned zero value")
	}
	if v := reflect.ValueOf(Created(123)); v.IsZero() {
		t.Fatal("Created returned zero value")
	}
	if v := reflect.ValueOf(NoContent()); v.IsZero() {
		t.Fatal("NoContent returned zero value")
	}
	if v := reflect.ValueOf(Data("alias")); v.IsZero() {
		t.Fatal("Data returned zero value")
	}
	if v := reflect.ValueOf(Error(errors.New("boom"))); v.IsZero() {
		t.Fatal("Error returned zero value")
	}
	if v := reflect.ValueOf(List([]int{1, 2, 3}, 3, 1, 50, "c")); v.IsZero() {
		t.Fatal("List returned zero value")
	}
}

func TestHandle_PassThrough(t *testing.T) {
	// Handle should pass through the Response we return (e.g., Created)
	h := Handle(func(_ *http.Request) Response {
		return Created("made")
	})
	code, body := run(h, mkReq(t, http.MethodGet, nil))
	if code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, code)
	}
	if !strings.Contains(body, "made") {
		t.Fatalf("expected body to contain %q, got %q", "made", body)
	}
}

func TestCall_PlainValue_OKWrap(t *testing.T) {
	h := Call(func(_ *http.Request) (any, error) {
		return map[string]string{"a": "1"}, nil
	})
	code, body := run(h, mkReq(t, http.MethodGet, nil))
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", code)
	}
	if !strings.Contains(body, `"a":"1"`) {
		t.Fatalf("expected body to contain a=1, got %q", body)
	}
}

func TestCall_ResponsePassthrough(t *testing.T) {
	want := Created("z")
	h := Call(func(_ *http.Request) (any, error) {
		return want, nil
	})
	code, body := run(h, mkReq(t, http.MethodGet, nil))
	if code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", code)
	}
	if !strings.Contains(body, "z") {
		t.Fatalf("expected body to contain %q, got %q", "z", body)
	}
}

func TestCall_ErrorPath(t *testing.T) {
	h := Call(func(_ *http.Request) (any, error) {
		return nil, errors.New("nah")
	})
	code, body := run(h, mkReq(t, http.MethodGet, nil))
	if code < 400 {
		t.Fatalf("expected error status >=400, got %d", code)
	}
	if len(body) == 0 {
		t.Fatal("expected error body, got empty")
	}
}

func TestJSON_SuccessPlainValue(t *testing.T) {
	type in struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	payload := in{A: 7, B: "ok"}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		t.Fatalf("encode: %v", err)
	}

	h := JSON[in](func(r *http.Request, got in) (any, error) {
		if !reflect.DeepEqual(got, payload) {
			t.Fatalf("decoded mismatch: got %#v want %#v", got, payload)
		}
		return map[string]any{"seen": true, "ua": r.UserAgent()}, nil
	})

	req := mkReq(t, http.MethodPost, buf)
	req.Header.Set("User-Agent", "ua/1")
	code, body := run(h, req)
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", code)
	}
	if !strings.Contains(body, `"seen":true`) {
		t.Fatalf("expected body to contain seen=true, got %q", body)
	}
}

func TestJSON_ResponsePassthrough(t *testing.T) {
	type in struct {
		X string `json:"x"`
	}
	body := `{"x":"z"}`
	want := Created("nice")

	h := JSON[in](func(_ *http.Request, _ in) (any, error) {
		return want, nil
	})

	code, gotBody := run(h, mkReq(t, http.MethodPost, strings.NewReader(body)))
	if code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", code)
	}
	if !strings.Contains(gotBody, "nice") {
		t.Fatalf("expected body to contain %q, got %q", "nice", gotBody)
	}
}

func TestJSON_DecodeError_InvalidJSON(t *testing.T) {
	type in struct {
		A int `json:"a"`
	}
	h := JSON[in](func(_ *http.Request, _ in) (any, error) {
		t.Fatal("handler should not be called on decode error")
		return nil, nil
	})
	code, body := run(h, mkReq(t, http.MethodPost, strings.NewReader(`{`))) // malformed
	if code < 400 {
		t.Fatalf("expected error status >=400, got %d", code)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty error body")
	}
}

func TestJSON_DecodeError_UnknownField(t *testing.T) {
	type in struct {
		A int `json:"a"`
	}
	// DisallowUnknownFields is set; "b" should trigger an error
	h := JSON[in](func(_ *http.Request, _ in) (any, error) {
		t.Fatal("handler should not be called on unknown field")
		return nil, nil
	})
	code, body := run(h, mkReq(t, http.MethodPost, strings.NewReader(`{"a":1,"b":2}`)))
	if code < 400 {
		t.Fatalf("expected error status >=400, got %d", code)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty error body")
	}
}

func TestJSON_HandlerError(t *testing.T) {
	type in struct {
		A int `json:"a"`
	}
	h := JSON[in](func(_ *http.Request, _ in) (any, error) {
		return nil, errors.New("nope")
	})
	code, body := run(h, mkReq(t, http.MethodPost, strings.NewReader(`{"a":123}`)))
	if code < 400 {
		t.Fatalf("expected error status >=400, got %d", code)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty error body")
	}
}
