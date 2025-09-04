// Package http provides helpers for writing JSON responses with a consistent envelope
package http

import (
	"encoding/json"
	stdhttp "net/http"

	perr "swearjar/internal/platform/errors"
	lumnet "swearjar/internal/platform/net"
)

// Envelope is the standard response body for all endpoints
type Envelope struct {
	StatusCode int            `json:"status_code"`
	Status     string         `json:"status"`
	Code       perr.ErrorCode `json:"code,omitempty"`
	Error      string         `json:"error,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Data       any            `json:"data,omitempty"`
	Page       *Page          `json:"page,omitempty"`
}

// Page describes pagination when returning lists
type Page struct {
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Cursor   string `json:"cursor,omitempty"`
}

// JSON writes v as application/json with the given status
func JSON(w stdhttp.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// JSONStatus writes only a status with an empty object body
func JSONStatus(w stdhttp.ResponseWriter, status int) {
	JSON(w, status, map[string]any{})
}

//
// Effectful helpers (Respond*) for classic handlers
//

// RespondOK writes a 200 envelope with data
func RespondOK(w stdhttp.ResponseWriter, r *stdhttp.Request, data any) {
	reqID := lumnet.RequestID(r.Context())
	JSON(w, stdhttp.StatusOK, Envelope{
		StatusCode: stdhttp.StatusOK,
		Status:     stdhttp.StatusText(stdhttp.StatusOK),
		RequestID:  reqID,
		Data:       data,
	})
}

// RespondCreated writes a 201 envelope with data
func RespondCreated(w stdhttp.ResponseWriter, r *stdhttp.Request, data any) {
	reqID := lumnet.RequestID(r.Context())
	JSON(w, stdhttp.StatusCreated, Envelope{
		StatusCode: stdhttp.StatusCreated,
		Status:     stdhttp.StatusText(stdhttp.StatusCreated),
		RequestID:  reqID,
		Data:       data,
	})
}

// RespondNoContent writes a 204 with no body
func RespondNoContent(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	w.WriteHeader(stdhttp.StatusNoContent)
}

// RespondData is an alias for RespondOK
func RespondData(w stdhttp.ResponseWriter, r *stdhttp.Request, data any) {
	RespondOK(w, r, data)
}

// RespondList writes items and a pagination block
func RespondList(w stdhttp.ResponseWriter, r *stdhttp.Request, items any, total, page, pageSize int, cursor string) {
	reqID := lumnet.RequestID(r.Context())
	JSON(w, stdhttp.StatusOK, Envelope{
		StatusCode: stdhttp.StatusOK,
		Status:     stdhttp.StatusText(stdhttp.StatusOK),
		RequestID:  reqID,
		Data:       items,
		Page: &Page{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Cursor:   cursor,
		},
	})
}

// RespondError maps a project error into an envelope and writes it
func RespondError(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
	reqID := lumnet.RequestID(r.Context())
	status := perr.HTTPStatus(err)
	wr := perr.WireFrom(err)
	JSON(w, status, Envelope{
		StatusCode: status,
		Status:     stdhttp.StatusText(status),
		Code:       wr.Code,
		Error:      wr.Message,
		RequestID:  reqID,
	})
}

//
// Return-style helpers for early returns in handlers
//

// Response is a functional response object for return-style handlers
type Response struct {
	Status int
	Body   any
	// optional headers if a handler wants to add any
	Header stdhttp.Header
}

// Handle adapts a Response-returning handler to net/http
func Handle(h func(r *stdhttp.Request) Response) stdhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		h(r).write(w, r)
	}
}

func (resp Response) write(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	status := resp.Status
	if status == 0 {
		status = stdhttp.StatusOK
	}
	// allow header overrides
	if resp.Header != nil {
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}
	if status == stdhttp.StatusNoContent {
		w.WriteHeader(stdhttp.StatusNoContent)
		return
	}

	reqID := lumnet.RequestID(r.Context())

	// If Body is an error, derive status from error *before* building the envelope
	if err, ok := resp.Body.(error); ok && err != nil {
		status = perr.HTTPStatus(err)
		wr := perr.WireFrom(err)
		JSON(w, status, Envelope{
			StatusCode: status,
			Status:     stdhttp.StatusText(status),
			Code:       wr.Code,
			Error:      wr.Message,
			RequestID:  reqID,
		})
		return
	}

	// success path
	JSON(w, status, Envelope{
		StatusCode: status,
		Status:     stdhttp.StatusText(status),
		RequestID:  reqID,
		Data:       resp.Body,
	})
}

// OK returns a 200 response
func OK(data any) Response { return Response{Status: stdhttp.StatusOK, Body: data} }

// Created returns a 201 response
func Created(data any) Response { return Response{Status: stdhttp.StatusCreated, Body: data} }

// NoContent returns a 204 response
func NoContent() Response { return Response{Status: stdhttp.StatusNoContent} }

// Data is an alias for OK
func Data(v any) Response { return OK(v) }

// Error returns a response that maps the error to status and envelope
func Error(err error) Response { return Response{Body: err} }

// List returns a 200 response with items and pagination
func List(items any, total, page, size int, cursor string) Response {
	return OK(struct {
		Items any  `json:"items"`
		Page  Page `json:"page"`
	}{Items: items, Page: Page{Total: total, Page: page, PageSize: size, Cursor: cursor}})
}
