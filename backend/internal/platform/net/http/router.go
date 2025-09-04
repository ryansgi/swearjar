package http

import "net/http"

// Handler is the platform handler type used everywhere
type Handler = func(http.ResponseWriter, *http.Request)

// Router is the minimal surface area we mount against
type Router interface {
	Get(path string, h Handler)
	Post(path string, h Handler)
	Put(path string, h Handler)
	Patch(path string, h Handler)
	Delete(path string, h Handler)

	// optional but handy
	Head(path string, h Handler)
	Options(path string, h Handler)

	Handle(path string, h http.Handler)
	Use(mw ...func(http.Handler) http.Handler)
	Group(fn func(Router))
	Route(pattern string, fn func(Router))

	Mux() http.Handler
}
