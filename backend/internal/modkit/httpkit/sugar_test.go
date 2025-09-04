package httpkit

import (
	"net/http"
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

// fakeRouterSugar satisfies the platform Router surface we need here
// it records verb + path + handler for assertions
type fakeRouterSugar struct {
	recs []struct {
		verb string
		path string
		h    phttp.Handler
	}
}

func (f *fakeRouterSugar) Route(_ string, fn func(Router))          { fn(f) }
func (f *fakeRouterSugar) Group(fn func(Router))                    { fn(f) }
func (f *fakeRouterSugar) Use(_ ...func(http.Handler) http.Handler) {}
func (f *fakeRouterSugar) Mux() http.Handler                        { return http.NewServeMux() }
func (f *fakeRouterSugar) Handle(path string, h http.Handler)       { /* not used here */ }
func (f *fakeRouterSugar) Options(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"OPTIONS", path, h})
}

func (f *fakeRouterSugar) Head(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"HEAD", path, h})
}

func (f *fakeRouterSugar) Delete(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"DELETE", path, h})
}

func (f *fakeRouterSugar) Get(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"GET", path, h})
}

func (f *fakeRouterSugar) Post(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"POST", path, h})
}

func (f *fakeRouterSugar) Put(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"PUT", path, h})
}

func (f *fakeRouterSugar) Patch(path string, h phttp.Handler) {
	f.recs = append(f.recs, struct {
		verb, path string
		h          phttp.Handler
	}{"PATCH", path, h})
}

func TestGetJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	GetJSON[req](r, "/a", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "GET" || rec.path != "/a" {
		t.Fatalf("expected GET /a, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestPostJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	PostJSON[req](r, "/b", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "POST" || rec.path != "/b" {
		t.Fatalf("expected POST /b, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestPutJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	PutJSON[req](r, "/c", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "PUT" || rec.path != "/c" {
		t.Fatalf("expected PUT /c, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestPatchJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	PatchJSON[req](r, "/d", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "PATCH" || rec.path != "/d" {
		t.Fatalf("expected PATCH /d, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestDeleteJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	DeleteJSON[req](r, "/e", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "DELETE" || rec.path != "/e" {
		t.Fatalf("expected DELETE /e, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestOptionsJSON_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	type req struct{ A int }
	OptionsJSON[req](r, "/f", func(_ *http.Request, _ req) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "OPTIONS" || rec.path != "/f" {
		t.Fatalf("expected OPTIONS /f, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Get_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Get(r, "/g", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "GET" || rec.path != "/g" {
		t.Fatalf("expected GET /g, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Post_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Post(r, "/h", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "POST" || rec.path != "/h" {
		t.Fatalf("expected POST /h, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Put_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Put(r, "/i", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "PUT" || rec.path != "/i" {
		t.Fatalf("expected PUT /i, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Patch_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Patch(r, "/j", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "PATCH" || rec.path != "/j" {
		t.Fatalf("expected PATCH /j, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Delete_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Delete(r, "/k", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "DELETE" || rec.path != "/k" {
		t.Fatalf("expected DELETE /k, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestBodyless_Options_MountsHandler(t *testing.T) {
	r := &fakeRouterSugar{}
	Options(r, "/l", func(_ *http.Request) (any, error) { return "ok", nil })

	if len(r.recs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(r.recs))
	}
	rec := r.recs[0]
	if rec.verb != "OPTIONS" || rec.path != "/l" {
		t.Fatalf("expected OPTIONS /l, got %s %s", rec.verb, rec.path)
	}
	if rec.h == nil {
		t.Fatalf("expected non-nil handler")
	}
}
