// Package httpkit provides handler and routing helpers that alias the platform http package
// use these from modules so they do not import internal/platform/net/http directly
package httpkit

import (
	"net/http"

	phttp "swearjar/internal/platform/net/http"
	"swearjar/internal/platform/net/http/bind"
)

type (
	// Envelope is the transport envelope type
	Envelope = phttp.Envelope

	// Page is the pagination metadata type
	Page = phttp.Page

	// Response is the HTTP response type
	Response = phttp.Response

	// Handler is the platform handler type
	Handler = phttp.Handler

	// Router is a re-export of the platform router seam
	Router = phttp.Router
)

// OK returns a 200 response
func OK(data any) Response { return phttp.OK(data) }

// Created returns a 201 response
func Created(data any) Response { return phttp.Created(data) }

// NoContent returns a 204 response
func NoContent() Response { return phttp.NoContent() }

// Data is an alias for OK
func Data(v any) Response { return phttp.Data(v) }

// Error returns a response that maps an error to status and envelope
func Error(err error) Response { return phttp.Error(err) }

// List returns a 200 response with items and pagination
func List(items any, total, page, size int, cursor string) Response {
	return phttp.List(items, total, page, size, cursor)
}

// JSON wraps a JSON handler with the appropriate content type
func JSON[T any](fn func(*http.Request, T) (any, error)) Handler {
	return phttp.Handle(func(r *http.Request) phttp.Response {
		in, err := bind.ParseJSON[T](r)
		if err != nil {
			return phttp.Error(err) // maps to 400 with translated message
		}
		out, err := fn(r, in)
		if err != nil {
			return phttp.Error(err)
		}
		if resp, ok := out.(phttp.Response); ok {
			return resp
		}
		return phttp.OK(out)
	})
}

// Call adapts a handler that takes no JSON body
func Call(fn func(*http.Request) (any, error)) Handler {
	return phttp.Handle(func(r *http.Request) phttp.Response {
		out, err := fn(r)
		if err != nil {
			return phttp.Error(err)
		}
		if resp, ok := out.(phttp.Response); ok {
			return resp
		}
		return phttp.OK(out)
	})
}

// Handle lets you directly adapt a Response-returning function if you prefer
func Handle(fn func(*http.Request) Response) Handler {
	return phttp.Handle(fn)
}
