package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// chiRoot adapts *chi.Mux to Router
type chiRoot struct{ m *chi.Mux }

// chiSub adapts chi.Router to Router while retaining top-level mux for Mux()
type chiSub struct {
	parent *chi.Mux
	r      chi.Router
}

// toStd wraps a platform Handler into a stdlib HandlerFunc
func toStd(h Handler) http.HandlerFunc { return http.HandlerFunc(h) }

// AdaptChi adapts a *chi.Mux to a Router
func AdaptChi(m *chi.Mux) Router { return chiRoot{m: m} }

func (c chiRoot) Get(p string, h Handler)     { c.m.Method(http.MethodGet, p, toStd(h)) }
func (c chiRoot) Post(p string, h Handler)    { c.m.Method(http.MethodPost, p, toStd(h)) }
func (c chiRoot) Put(p string, h Handler)     { c.m.Method(http.MethodPut, p, toStd(h)) }
func (c chiRoot) Patch(p string, h Handler)   { c.m.Method(http.MethodPatch, p, toStd(h)) }
func (c chiRoot) Delete(p string, h Handler)  { c.m.Method(http.MethodDelete, p, toStd(h)) }
func (c chiRoot) Head(p string, h Handler)    { c.m.Method(http.MethodHead, p, toStd(h)) }
func (c chiRoot) Options(p string, h Handler) { c.m.Method(http.MethodOptions, p, toStd(h)) }

func (c chiRoot) Handle(p string, h http.Handler)           { c.m.Handle(p, h) }
func (c chiRoot) Use(mw ...func(http.Handler) http.Handler) { c.m.Use(mw...) }
func (c chiRoot) Group(fn func(Router)) {
	c.m.Group(func(sub chi.Router) { fn(chiSub{parent: c.m, r: sub}) })
}

func (c chiRoot) Route(pattern string, fn func(Router)) {
	c.m.Route(pattern, func(sub chi.Router) { fn(chiSub{parent: c.m, r: sub}) })
}
func (c chiRoot) Mux() http.Handler { return c.m }

// Sub router methods

func (c chiSub) Get(p string, h Handler)     { c.r.Method(http.MethodGet, p, toStd(h)) }
func (c chiSub) Post(p string, h Handler)    { c.r.Method(http.MethodPost, p, toStd(h)) }
func (c chiSub) Put(p string, h Handler)     { c.r.Method(http.MethodPut, p, toStd(h)) }
func (c chiSub) Patch(p string, h Handler)   { c.r.Method(http.MethodPatch, p, toStd(h)) }
func (c chiSub) Delete(p string, h Handler)  { c.r.Method(http.MethodDelete, p, toStd(h)) }
func (c chiSub) Head(p string, h Handler)    { c.r.Method(http.MethodHead, p, toStd(h)) }
func (c chiSub) Options(p string, h Handler) { c.r.Method(http.MethodOptions, p, toStd(h)) }

func (c chiSub) Handle(p string, h http.Handler)           { c.r.Handle(p, h) }
func (c chiSub) Use(mw ...func(http.Handler) http.Handler) { c.r.Use(mw...) }
func (c chiSub) Group(fn func(Router)) {
	c.r.Group(func(sub chi.Router) { fn(chiSub{parent: c.parent, r: sub}) })
}

func (c chiSub) Route(pattern string, fn func(Router)) {
	c.r.Route(pattern, func(sub chi.Router) { fn(chiSub{parent: c.parent, r: sub}) })
}
func (c chiSub) Mux() http.Handler { return c.r } // chi.Router implements http.Handler
