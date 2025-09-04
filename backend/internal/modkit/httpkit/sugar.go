package httpkit

import (
	"net/http"

	phttp "swearjar/internal/platform/net/http"
)

// GetJSON mounts a pure JSON handler under GET
func GetJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Get(path, phttp.JSONHandler(h))
}

// PostJSON mounts a pure JSON handler under POST
func PostJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Post(path, JSON(h))
}

// PutJSON mounts a pure JSON handler under PUT
func PutJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Put(path, phttp.JSONHandler(h))
}

// PatchJSON mounts a pure JSON handler under PATCH
func PatchJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Patch(path, phttp.JSONHandler(h))
}

// DeleteJSON mounts a pure JSON handler under DELETE
func DeleteJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Delete(path, phttp.JSONHandler(h))
}

// OptionsJSON mounts a pure JSON handler under OPTIONS
func OptionsJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Options(path, phttp.JSONHandler(h))
}

// Body-less JSON endpoints

// zeroBody adapts a 0-arg handler to JSONHandler by ignoring a zero-value payload
type zeroBody struct{}

func adapt0(h func(*http.Request) (any, error)) func(*http.Request, zeroBody) (any, error) {
	return func(r *http.Request, _ zeroBody) (any, error) { return h(r) }
}

// Get registers a no-body handler and uses the envelope adapter
func Get(r Router, path string, h func(*http.Request) (any, error)) {
	r.Get(path, Call(h))
}

// Post registers a no-body handler and uses the envelope adapter
func Post(r Router, path string, h func(*http.Request) (any, error)) {
	r.Post(path, Call(h))
}

// Put mounts a body-less JSON handler under PUT
func Put(r Router, path string, h func(*http.Request) (any, error)) {
	r.Put(path, phttp.JSONHandler(adapt0(h)))
}

// Patch mounts a body-less JSON handler under PATCH
func Patch(r Router, path string, h func(*http.Request) (any, error)) {
	r.Patch(path, phttp.JSONHandler(adapt0(h)))
}

// Delete mounts a body-less JSON handler under DELETE
func Delete(r Router, path string, h func(*http.Request) (any, error)) {
	r.Delete(path, phttp.JSONHandler(adapt0(h)))
}

// Options mounts a body-less JSON handler under OPTIONS
func Options(r Router, path string, h func(*http.Request) (any, error)) {
	r.Options(path, phttp.JSONHandler(adapt0(h)))
}
