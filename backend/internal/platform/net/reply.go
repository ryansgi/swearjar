package net

import (
	"net/http"

	perr "swearjar/internal/platform/errors"
)

// Wire is a common envelope used by transports
type Wire struct {
	StatusCode int            `json:"status_code"`
	Status     string         `json:"status"`
	Code       perr.ErrorCode `json:"code,omitempty"`
	Error      string         `json:"error,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Data       any            `json:"data,omitempty"`
}

// OK builds a 200 envelope
func OK(data any, reqID string) (int, Wire) {
	return http.StatusOK, Wire{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		RequestID:  reqID,
		Data:       data,
	}
}

// Created builds a 201 envelope
func Created(data any, reqID string) (int, Wire) {
	return http.StatusCreated, Wire{
		StatusCode: http.StatusCreated,
		Status:     http.StatusText(http.StatusCreated),
		RequestID:  reqID,
		Data:       data,
	}
}

// NoContent builds a 204 envelope
func NoContent(reqID string) (int, Wire) {
	return http.StatusNoContent, Wire{
		StatusCode: http.StatusNoContent,
		Status:     http.StatusText(http.StatusNoContent),
		RequestID:  reqID,
	}
}

// Error builds an error envelope
func Error(err error, reqID string) (int, Wire) {
	if err == nil {
		return OK(nil, reqID)
	}
	status := perr.HTTPStatus(err)
	w := perr.WireFrom(err)
	return status, Wire{
		StatusCode: status,
		Status:     http.StatusText(status),
		Code:       w.Code,
		Error:      w.Message,
		RequestID:  reqID,
	}
}
