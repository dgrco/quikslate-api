package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
)

// handleServiceError performs a error type check and writes the corresponding error to
// the provided http.ResponseWriter.
// On InternalServerError, it also logs to the server (use fnName for specifying)
func handleServiceError(w http.ResponseWriter, err error, fnName string) {
	// 1. Check for custom Type-based errors first (to extract data)
	if valErr, ok := errors.AsType[*domain.ValidationError](err); ok {
		response.WriteError(w, valErr.Message, http.StatusBadRequest)
		return
	}

	// 2. Check for Sentinel errors (value matches)
	if errors.Is(err, domain.ErrUnauthorized) {
		response.WriteError(w, ERR_UNAUTHORIZED, http.StatusUnauthorized)
		return
	}

	if errors.Is(err, domain.ErrForbidden) {
		response.WriteError(w, ERR_FORBIDDEN, http.StatusForbidden)
		return
	}

	if errors.Is(err, domain.ErrNotFound) {
		response.WriteError(w, ERR_NOT_FOUND, http.StatusNotFound)
		return
	}

	if errors.Is(err, domain.ErrAlreadyExists) {
		response.WriteError(w, ERR_ALREADY_EXISTS, http.StatusConflict)
		return
	}

	if errors.Is(err, domain.ErrInvalidCredentials) {
		response.WriteError(w, ERR_INVALID_CREDENTIALS, http.StatusUnauthorized)
		return
	}

	if errors.Is(err, domain.ErrInvalidRefreshToken) {
		response.WriteError(w, ERR_INVALID_REFRESH_TOKEN, http.StatusUnauthorized)
		return
	}

	if errors.Is(err, domain.ErrInvalidShiftTimes) {
		response.WriteError(w, ERR_INVALID_SHIFT_TIMES, http.StatusBadRequest)
		return
	}

	if errors.Is(err, domain.ErrLastAdminRemoval) {
		response.WriteError(w, ERR_LAST_ADMIN_REMOVAL, http.StatusBadRequest)
		return
	}

	// 3. Default case (Internal Server Error)
	response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
	log.Printf("%s: %v", fnName, err)
}

const (
	ERR_INVALID_REQ_BODY      = "invalid request body"
	ERR_INTERNAL_SERVER       = "internal server error"
	ERR_NOT_FOUND             = "not found"
	ERR_FORBIDDEN             = "forbidden"
	ERR_UNAUTHORIZED          = "unauthorized"
	ERR_INVALID_REFRESH_TOKEN = "invalid refresh token"
	ERR_ALREADY_EXISTS        = "already exists"
	ERR_INVALID_CREDENTIALS   = "invalid credentials"
	ERR_INVALID_SHIFT_TIMES   = "invalid shift times"
	ERR_LAST_ADMIN_REMOVAL    = "cannot remove last admin"
)
