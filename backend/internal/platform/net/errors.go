package net

import (
	"net/http"

	perr "swearjar/internal/platform/errors"
)

// HTTPStatus maps a project error to http status
func HTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return perr.HTTPStatus(err)
}
