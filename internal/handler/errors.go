package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
)

// handleServiceError is the single place that maps domain/service errors to
// HTTP responses. Only the fallback 500 is logged, with fnName identifying
// the handler, since every other case is an expected client-facing outcome.
func handleServiceError(w http.ResponseWriter, err error, fnName string) {
	if valErr, ok := errors.AsType[*domain.ValidationError](err); ok {
		response.WriteError(w, valErr.Message, http.StatusBadRequest)
		return
	}

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

	// 400 rather than 401: the caller is anonymous here, and the remedy is to
	// request a new reset link, not to authenticate and retry.
	if errors.Is(err, domain.ErrInvalidPasswordResetToken) {
		response.WriteError(w, ERR_INVALID_PASSWORD_RESET_TOKEN, http.StatusBadRequest)
		return
	}

	if errors.Is(err, domain.ErrInvalidShiftTimes) {
		response.WriteError(w, ERR_INVALID_SHIFT_TIMES, http.StatusBadRequest)
		return
	}

	if errors.Is(err, domain.ErrShiftOverlap) {
		response.WriteError(w, ERR_SHIFT_OVERLAP, http.StatusConflict)
		return
	}

	if errors.Is(err, domain.ErrLastAdminRemoval) {
		response.WriteError(w, ERR_LAST_ADMIN_REMOVAL, http.StatusBadRequest)
		return
	}

	response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
	log.Printf("%s: %v", fnName, err)
}

const (
	ERR_INVALID_REQ_BODY             = "invalid request body"
	ERR_INTERNAL_SERVER              = "internal server error"
	ERR_NOT_FOUND                    = "not found"
	ERR_FORBIDDEN                    = "forbidden"
	ERR_UNAUTHORIZED                 = "unauthorized"
	ERR_INVALID_REFRESH_TOKEN        = "invalid refresh token"
	ERR_INVALID_PASSWORD_RESET_TOKEN = "invalid password reset token"
	ERR_ALREADY_EXISTS               = "already exists"
	ERR_INVALID_CREDENTIALS          = "invalid credentials"
	ERR_INVALID_SHIFT_TIMES          = "invalid shift times"
	ERR_SHIFT_OVERLAP                = "shift overlaps an existing shift for this user"
	ERR_LAST_ADMIN_REMOVAL           = "cannot remove last admin"
)
