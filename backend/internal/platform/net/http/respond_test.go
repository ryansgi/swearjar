package http_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	perr "swearjar/internal/platform/errors"
	lumnet "swearjar/internal/platform/net"
	phttp "swearjar/internal/platform/net/http"
)

// helper to build a request with a request_id in context
func reqWithReqID(method, path, rid string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(lumnet.WithRequest(req.Context(), rid, "")) // tenant empty
	return req
}

func TestJSONAndStatusHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	phttp.JSON(rec, http.StatusTeapot, map[string]any{"k": "v"})
	if rec.Code != http.StatusTeapot {
		t.Fatalf("JSON status: expected 418, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("expected content-type set")
	}

	rec2 := httptest.NewRecorder()
	phttp.JSONStatus(rec2, http.StatusAccepted)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("JSONStatus: expected 202, got %d", rec2.Code)
	}
}

func TestRespondOKCreatedNoContent(t *testing.T) {
	// OK
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/x", "rid-1")
	phttp.RespondOK(rec, req, map[string]string{"a": "b"})
	if rec.Code != http.StatusOK {
		t.Fatalf("RespondOK code: %d", rec.Code)
	}
	var env phttp.Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.StatusCode != 200 || env.RequestID != "rid-1" || env.Data == nil {
		t.Fatalf("bad envelope: %+v", env)
	}

	// Created
	recC := httptest.NewRecorder()
	phttp.RespondCreated(recC, req, map[string]int{"id": 7})
	if recC.Code != http.StatusCreated {
		t.Fatalf("RespondCreated code: %d", recC.Code)
	}

	// NoContent should not write a JSON body
	recN := httptest.NewRecorder()
	phttp.RespondNoContent(recN, req)
	if recN.Code != http.StatusNoContent {
		t.Fatalf("RespondNoContent code: %d", recN.Code)
	}
	if recN.Body.Len() != 0 {
		t.Fatalf("RespondNoContent should have empty body, got %q", recN.Body.String())
	}
}

func TestRespondList(t *testing.T) {
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/list", "rid-2")
	items := []int{1, 2, 3}
	phttp.RespondList(rec, req, items, 30, 2, 15, "cur123")
	if rec.Code != http.StatusOK {
		t.Fatalf("RespondList code: %d", rec.Code)
	}
	var env phttp.Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Page == nil ||
		env.Page.Total != 30 ||
		env.Page.Page != 2 ||
		env.Page.PageSize != 15 ||
		env.Page.Cursor != "cur123" {
		t.Fatalf("bad page: %+v", env.Page)
	}
}

func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/err", "rid-3")

	err := perr.New(perr.ErrorCodeNotFound, "nope")
	phttp.RespondError(rec, req, err)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var env phttp.Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != perr.ErrorCodeNotFound || env.Error == "" || env.RequestID != "rid-3" {
		t.Fatalf("bad error envelope: %+v", env)
	}
}

func TestReturnStyle_Handle_OKCreatedNoContent(t *testing.T) {
	// OK
	h := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.OK(map[string]any{"x": 1})
	})
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/ok", "rid-4")
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("handle OK code: %d", rec.Code)
	}

	// Created
	hc := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.Created(map[string]any{"id": 99})
	})
	recC := httptest.NewRecorder()
	reqC := reqWithReqID("POST", "/created", "rid-5")
	hc(recC, reqC)
	if recC.Code != http.StatusCreated {
		t.Fatalf("handle Created code: %d", recC.Code)
	}

	// NoContent
	hn := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.NoContent()
	})
	recN := httptest.NewRecorder()
	reqN := reqWithReqID("DELETE", "/no", "rid-6")
	hn(recN, reqN)
	if recN.Code != http.StatusNoContent || recN.Body.Len() != 0 {
		t.Fatalf("handle NoContent code=%d body=%q", recN.Code, recN.Body.String())
	}
}

func TestReturnStyle_ErrorAndHeaders(t *testing.T) {
	// Error mapping
	hErr := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.Error(perr.New(perr.ErrorCodeForbidden, "nope"))
	})
	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/err", "rid-7")
	hErr(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("handle error code: %d", rec.Code)
	}

	// headers override
	hHdr := phttp.Handle(func(r *http.Request) phttp.Response {
		resp := phttp.OK("hello")
		resp.Header = http.Header{}
		resp.Header.Set("X-Thing", "yup")
		return resp
	})
	rec2 := httptest.NewRecorder()
	req2 := reqWithReqID("GET", "/hdr", "rid-8")
	hHdr(rec2, req2)
	if got := rec2.Header().Get("X-Thing"); got != "yup" {
		t.Fatalf("expected header override, got %q", got)
	}

	// ensure generic error body path (non-project error) is mapped as unknown 500
	hGen := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.Error(errors.New("boom"))
	})
	rec3 := httptest.NewRecorder()
	req3 := reqWithReqID("GET", "/gen", "rid-9")
	hGen(rec3, req3)
	if rec3.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for generic error, got %d", rec3.Code)
	}
}

func TestReturnStyle_List(t *testing.T) {
	h := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.List([]int{1, 2}, 10, 2, 5, "abc")
	})

	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/list", "rid-list")
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Unmarshal the envelope first...
	var env phttp.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.StatusCode != 200 || env.RequestID != "rid-list" {
		t.Fatalf("bad envelope: %+v", env)
	}

	// ...then assert data shape: {"items":[...], "page":{...}}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", env.Data)
	}

	items, ok := data["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items, got %#v", data["items"])
	}

	page, ok := data["page"].(map[string]any)
	if !ok {
		t.Fatalf("expected page map, got %#v", data["page"])
	}

	// numbers in interface{} come back as float64 from encoding/json
	if total, _ := page["total"].(float64); int(total) != 10 {
		t.Fatalf("page.total = %#v", page["total"])
	}
	if p, _ := page["page"].(float64); int(p) != 2 {
		t.Fatalf("page.page = %#v", page["page"])
	}
	if ps, _ := page["page_size"].(float64); int(ps) != 5 {
		t.Fatalf("page.page_size = %#v", page["page_size"])
	}
	if cursor, _ := page["cursor"].(string); cursor != "abc" {
		t.Fatalf("page.cursor = %#v", page["cursor"])
	}
}

func TestReturnStyle_DataAlias(t *testing.T) {
	h := phttp.Handle(func(r *http.Request) phttp.Response {
		return phttp.Data("hello") // alias for OK
	})

	rec := httptest.NewRecorder()
	req := reqWithReqID("GET", "/data", "rid-data")
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var env phttp.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.StatusCode != http.StatusOK || env.RequestID != "rid-data" {
		t.Fatalf("bad envelope: %+v", env)
	}

	// Data should be the literal string "hello"
	if s, ok := env.Data.(string); !ok || s != "hello" {
		t.Fatalf("expected data \"hello\", got %#v (%T)", env.Data, env.Data)
	}
}
