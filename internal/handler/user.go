package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

type UserHandler struct {
	userService *service.UserService
	authService *service.AuthService
	jwtSecret   string
	secure      bool
}

func NewUserHandler(
	userService *service.UserService,
	authService *service.AuthService,
	jwtSecret string,
	secure bool,
) *UserHandler {
	return &UserHandler{
		userService,
		authService,
		jwtSecret,
		secure,
	}
}

// ChangePassword alters the user's password and ends all their sessions.
//
//	@Summary		Change Password
//	@Description	Revoke all refresh tokens, clear refresh_cookie, and changes the user's password.
//	@Tags			user
//	@Security	BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body	changePasswordRequest true	"current and new password"
//	@Success		200	{object}	SimpleResponse
//	@Failure		400 {object} 	response.errorResponse	"invalid body or missing fields"
//	@Failure		401 {object} 	response.errorResponse	"invalid credentials"
//	@Router			/users/me/change-password [post]
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	userId := ctxkeys.GetUserId(ctx)
	if err := h.userService.ChangePassword(ctx, userId, req.CurrentPassword, req.NewPassword); err != nil {
		handleServiceError(w, err, "change password")
		return
	}
	ClearCookie(w, h.secure)

	response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
}

func (h *UserHandler) SetupRoutes(r chi.Router) {
	r.Route("/users/me", func(r chi.Router) {
		r.Use(httprate.LimitByRealIP(60, 1*time.Minute))
		r.Use(RequireIdentity(h.authService, h.jwtSecret))

		r.Post("/change-password", h.ChangePassword)
	})
}
