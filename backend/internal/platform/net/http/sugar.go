package http

import "net/http"

// GetJSON mounts a pure JSON handler for GET
func GetJSON(r Router, path string, h func(*http.Request) (any, error)) {
	r.Get(path, JSONHandlerNoBody(h))
}

// DeleteJSON mounts a pure JSON handler for DELETE (no request body)
func DeleteJSON(r Router, path string, h func(*http.Request) (any, error)) {
	r.Delete(path, JSONHandlerNoBody(h))
}

// PostJSON mounts a pure JSON handler for POST
func PostJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Post(path, JSONHandler(h))
}

// PutJSON mounts a pure JSON handler for PUT
func PutJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Put(path, JSONHandler(h))
}

// PatchJSON mounts a pure JSON handler for PATCH
func PatchJSON[T any](r Router, path string, h func(*http.Request, T) (any, error)) {
	r.Patch(path, JSONHandler(h))
}
