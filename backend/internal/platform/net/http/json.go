package http

import (
	"net/http"

	"swearjar/internal/platform/net/http/bind"
)

// JSONHandler adapts a pure JSON handler to a platform Handler
func JSONHandler[T any](fn func(*http.Request, T) (any, error)) Handler {
	return Handle(func(r *http.Request) Response {
		in, err := bind.ParseJSON[T](r)
		if err != nil {
			return Error(err)
		}
		out, err := fn(r, in)
		if err != nil {
			return Error(err)
		}
		return OK(out)
	})
}

// JSONHandlerNoBody calls fn without parsing a request body and wraps the result
func JSONHandlerNoBody(fn func(*http.Request) (any, error)) Handler {
	return Handle(func(r *http.Request) Response {
		out, err := fn(r)
		if err != nil {
			return Error(err)
		}
		return OK(out)
	})
}
