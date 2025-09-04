package modkit

import (
	"net/http"
	"testing"

	phttp "swearjar/internal/platform/net/http"
)

func TestWithName(t *testing.T) {
	t.Parallel()
	var c buildCfg
	WithName("auth")(&c)
	if c.name != "auth" {
		t.Fatalf("expected name=auth got=%q", c.name)
	}
}

func TestWithPrefix(t *testing.T) {
	t.Parallel()
	var c buildCfg
	WithPrefix("/api/v1")(&c)
	if c.prefix != "/api/v1" {
		t.Fatalf("expected prefix=/api/v1 got=%q", c.prefix)
	}
}

func TestWithMiddlewares_AccumulatesAndOrder(t *testing.T) {
	t.Parallel()

	log := []string{}
	mw := func(tag string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log = append(log, tag)
				if next != nil {
					next.ServeHTTP(w, r)
				}
			})
		}
	}

	var c buildCfg
	WithMiddlewares(mw("a"), mw("b"))(&c)
	WithMiddlewares(mw("c"))(&c)

	if len(c.mw) != 3 {
		t.Fatalf("expected 3 middlewares got=%d", len(c.mw))
	}

	// Build chain in the usual order: the first added should run first
	var h http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for i := len(c.mw) - 1; i >= 0; i-- {
		h = c.mw[i](h)
	}

	// Invoke once
	h.ServeHTTP(nil, nil)

	want := []string{"a", "b", "c"}
	if len(log) != len(want) {
		t.Fatalf("unexpected call count got=%d want=%d", len(log), len(want))
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("middleware order mismatch at %d: got=%q want=%q", i, log[i], want[i])
		}
	}
}

func TestWithPorts_GenericStoresConcreteType(t *testing.T) {
	t.Parallel()

	type Ports struct {
		Hello string
		N     int
	}

	var c buildCfg
	WithPorts(Ports{Hello: "world", N: 7})(&c)

	ps, ok := c.ports.(Ports)
	if !ok {
		t.Fatalf("expected ports of type Ports got %T", c.ports)
	}
	if ps.Hello != "world" || ps.N != 7 {
		t.Fatalf("unexpected ports value: %+v", ps)
	}
}

func TestWithSwagger(t *testing.T) {
	t.Parallel()
	var c buildCfg
	if c.swaggerOn {
		t.Fatal("zero-value swaggerOn should be false")
	}
	WithSwagger(true)(&c)
	if !c.swaggerOn {
		t.Fatal("expected swaggerOn=true after option")
	}
	WithSwagger(false)(&c)
	if c.swaggerOn {
		t.Fatal("expected swaggerOn=false after toggle")
	}
}

func TestWithSubrouter_SetsFactory(t *testing.T) {
	t.Parallel()

	called := false
	var got phttp.Router

	// a tiny factory that records invocation and returns the input unchanged
	factory := func(r phttp.Router) phttp.Router {
		called = true
		got = r
		return r
	}

	var c buildCfg
	WithSubrouter(factory)(&c)

	if c.subrouter == nil {
		t.Fatal("expected subrouter to be set")
	}

	var r phttp.Router = nil
	out := c.subrouter(r)

	if !called {
		t.Fatal("expected subrouter factory to be called")
	}
	if got != r || out != r {
		t.Fatalf("subrouter factory should be identity: got=%v out=%v want=%v", got, out, r)
	}
}

func TestOptions_Compose(t *testing.T) {
	t.Parallel()

	log := []string{}
	mw := func(tag string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log = append(log, tag)
				if next != nil {
					next.ServeHTTP(w, r)
				}
			})
		}
	}

	opts := []Option{
		WithName("tenants"),
		WithPrefix("/t"),
		WithSwagger(true),
		WithMiddlewares(mw("x")),
		WithPorts(map[string]int{"ok": 1}),
	}

	var c buildCfg
	for _, opt := range opts {
		opt(&c)
	}

	if c.name != "tenants" || c.prefix != "/t" || !c.swaggerOn {
		t.Fatalf("unexpected cfg: %+v", c)
	}
	if len(c.mw) != 1 {
		t.Fatalf("expected 1 middleware got=%d", len(c.mw))
	}
	if _, ok := c.ports.(map[string]int); !ok {
		t.Fatalf("expected ports to be map[string]int got %T", c.ports)
	}
}

func TestWithRegister_SetsAndCalls(t *testing.T) {
	t.Parallel()

	var c buildCfg
	called := false
	var got phttp.Router

	fn := func(r phttp.Router) {
		called = true
		got = r
	}

	WithRegister(fn)(&c)

	if c.register == nil {
		t.Fatal("expected register to be set")
	}

	var r phttp.Router
	c.register(r)

	if !called {
		t.Fatal("expected register function to be called")
	}
	if got != r {
		t.Fatalf("expected register to receive the same router value")
	}
}
