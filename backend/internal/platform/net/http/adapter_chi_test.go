package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAdaptChi_RootGroupRouteAndMux(t *testing.T) {
	t.Parallel()

	m := chi.NewRouter()
	r := AdaptChi(m)

	// root middleware
	r.Use(func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
			w.Header().Set("X-Root", "1")
			next.ServeHTTP(w, req)
		})
	})

	// root route
	r.Get("/root", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("root"))
	})

	// group route + group middleware
	r.Group(func(gr Router) {
		gr.Use(func(next stdhttp.Handler) stdhttp.Handler {
			return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
				w.Header().Set("X-Group", "1")
				next.ServeHTTP(w, req)
			})
		})
		// ensure chiSub.Mux() compiles/returns a handler (not used further, just sanity)
		if gr.Mux() == nil {
			t.Fatalf("group Mux() returned nil")
		}
		gr.Get("/g/ping", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("g"))
		})
	})

	// route (subrouter) + subrouter middleware
	r.Route("/api", func(sr Router) {
		sr.Use(func(next stdhttp.Handler) stdhttp.Handler {
			return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
				w.Header().Set("X-Route", "1")
				next.ServeHTTP(w, req)
			})
		})
		// ensure chiSub.Mux() present on route, too
		if sr.Mux() == nil {
			t.Fatalf("route Mux() returned nil")
		}
		sr.Get("/ping", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("pong"))
		})
	})

	srv := httptest.NewServer(r.Mux())
	defer srv.Close()

	// helper
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(stdhttp.MethodGet, srv.URL+path, nil)
		rr := httptest.NewRecorder()
		r.Mux().ServeHTTP(rr, req)
		return rr
	}

	// root route
	rr := get("/root")
	if rr.Code != 200 || rr.Body.String() != "root" {
		t.Fatalf("GET /root => code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Root") != "1" {
		t.Fatalf("root middleware header missing")
	}

	// group route
	rr = get("/g/ping")
	if rr.Code != 200 || rr.Body.String() != "g" {
		t.Fatalf("GET /g/ping => code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Root") != "1" {
		t.Fatalf("root middleware not applied to group route")
	}
	if rr.Header().Get("X-Group") != "1" {
		t.Fatalf("group middleware header missing")
	}

	// routed subrouter
	rr = get("/api/ping")
	if rr.Code != 200 || rr.Body.String() != "pong" {
		t.Fatalf("GET /api/ping => code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Root") != "1" {
		t.Fatalf("root middleware not applied to /api route")
	}
	if rr.Header().Get("X-Route") != "1" {
		t.Fatalf("route middleware header missing")
	}
}

func TestAdaptChi_ExtraVerbs_Handle_And_SubrouterNesting(t *testing.T) {
	t.Parallel()

	m := chi.NewRouter()
	r := AdaptChi(m)

	// Head, Options, Handle
	r.Head("/root/h", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		w.Header().Set("X-Head", "1")
	})
	r.Options("/root/o", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		w.Header().Set("X-Options", "1")
		w.WriteHeader(204)
	})
	r.Handle("/root/std", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("std"))
	}))

	// exercise chiSub.* verbs + Handle
	r.Group(func(gr Router) {
		gr.Post("/g/post", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(201) })
		gr.Put("/g/put", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(200) })
		gr.Patch("/g/patch", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(200) })
		gr.Delete("/g/del", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(204) })
		gr.Head("/g/h", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.Header().Set("X-G-Head", "1") })
		gr.Options("/g/o", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(204) })
		gr.Handle("/g/std", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("gstd"))
		}))

		// chiSub.Group (nested)
		gr.Group(func(ngr Router) {
			ngr.Get("/g/nested", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte("nested"))
			})
		})
	})

	// also check chiSub.Route
	r.Route("/api", func(sr Router) {
		sr.Post("/post", func(w stdhttp.ResponseWriter, req *stdhttp.Request) { w.WriteHeader(201) })
		sr.Route("/v1", func(nr Router) {
			nr.Get("/ok", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte("v1ok"))
			})
		})
	})

	srv := httptest.NewServer(r.Mux())
	defer srv.Close()

	do := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, srv.URL+path, nil)
		rr := httptest.NewRecorder()
		r.Mux().ServeHTTP(rr, req)
		return rr
	}

	// root Head
	rr := do(stdhttp.MethodHead, "/root/h")
	if rr.Code != 200 || rr.Body.Len() != 0 || rr.Header().Get("X-Head") != "1" {
		t.Fatalf("HEAD /root/h => code=%d head=%q body_len=%d", rr.Code, rr.Header().Get("X-Head"), rr.Body.Len())
	}
	// root Options
	rr = do(stdhttp.MethodOptions, "/root/o")
	if rr.Code != 204 || rr.Header().Get("X-Options") != "1" {
		t.Fatalf("OPTIONS /root/o => code=%d X-Options=%q", rr.Code, rr.Header().Get("X-Options"))
	}
	// root Handle (std handler)
	rr = do(stdhttp.MethodGet, "/root/std")
	if rr.Code != 200 || rr.Body.String() != "std" {
		t.Fatalf("GET /root/std => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// chiSub verbs under group
	if rr = do(stdhttp.MethodPost, "/g/post"); rr.Code != 201 {
		t.Fatalf("POST /g/post => %d", rr.Code)
	}
	if rr = do(stdhttp.MethodPut, "/g/put"); rr.Code != 200 {
		t.Fatalf("PUT /g/put => %d", rr.Code)
	}
	if rr = do(stdhttp.MethodPatch, "/g/patch"); rr.Code != 200 {
		t.Fatalf("PATCH /g/patch => %d", rr.Code)
	}
	if rr = do(stdhttp.MethodDelete, "/g/del"); rr.Code != 204 {
		t.Fatalf("DELETE /g/del => %d", rr.Code)
	}
	if rr = do(stdhttp.MethodHead, "/g/h"); rr.Code != 200 || rr.Body.Len() != 0 || rr.Header().Get("X-G-Head") != "1" {
		t.Fatalf("HEAD /g/h => code=%d len=%d X-G-Head=%q", rr.Code, rr.Body.Len(), rr.Header().Get("X-G-Head"))
	}
	if rr = do(stdhttp.MethodOptions, "/g/o"); rr.Code != 204 {
		t.Fatalf("OPTIONS /g/o => %d", rr.Code)
	}
	// chiSub.Handle
	rr = do(stdhttp.MethodGet, "/g/std")
	if rr.Code != 200 || rr.Body.String() != "gstd" {
		t.Fatalf("GET /g/std => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// chiSub.Group nested endpoint
	rr = do(stdhttp.MethodGet, "/g/nested")
	if rr.Code != 200 || rr.Body.String() != "nested" {
		t.Fatalf("GET /g/nested => code=%d body=%q", rr.Code, rr.Body.String())
	}

	// chiSub.Route nested under /api
	rr = do(stdhttp.MethodPost, "/api/post")
	if rr.Code != 201 {
		t.Fatalf("POST /api/post => %d", rr.Code)
	}
	rr = do(stdhttp.MethodGet, "/api/v1/ok")
	if rr.Code != 200 || rr.Body.String() != "v1ok" {
		t.Fatalf("GET /api/v1/ok => code=%d body=%q", rr.Code, rr.Body.String())
	}
}
