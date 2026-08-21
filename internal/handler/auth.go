package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

// This file implements the auth handler: registration, login, refresh-token
// rotation, and logout, plus the HttpOnly refresh_token cookie helper shared
// by all four.

type AuthHandler struct {
	authService *service.AuthService
	jwtSecret   string
	secure      bool // should be true in production and false in development (set in Config.SecureMode)
}

// NewAuthHandler creates an AuthHandler object.
// The secure parameter refers to whether we are in a prod or dev environment.
func NewAuthHandler(authService *service.AuthService, jwtSecret string, secure bool) *AuthHandler {
	return &AuthHandler{
		authService,
		jwtSecret,
		secure,
	}
}

// Request Body Structures

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// Response Structures

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Helpers

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}

// Handlers

// Register creates a new user account.
//
//	@Summary		Register
//	@Description	Create a new user account. Returns an access_token and sets a refresh_token cookie.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerRequest	true	"Registration details"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	response.errorResponse	"invalid body or missing fields"
//	@Failure		409		{object}	response.errorResponse	"email already registered"
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.Email) || isEmpty(req.Name) || isEmpty(req.Password) {
		response.WriteError(w, "email, name, and password are required", http.StatusBadRequest)
		return
	}

	authResult, err := h.authService.Register(r.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		handleServiceError(w, err, "register")
		return
	}
	setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
}

// Login authenticates with email and password.
//
//	@Summary		Login
//	@Description	Authenticate with email and password. Returns an access_token and sets a refresh_token cookie.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest	true	"Login credentials"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	response.errorResponse	"invalid body or missing fields"
//	@Failure		401		{object}	response.errorResponse	"invalid credentials"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.Email) || isEmpty(req.Password) {
		response.WriteError(w, "email and password are required", http.StatusBadRequest)
		return
	}

	authResult, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		handleServiceError(w, err, "login")
		return
	}

	setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
}

// Refresh rotates the refresh_token cookie and issues a new access token.
//
//	@Summary		Refresh
//	@Description	Rotate the refresh_token cookie (single use) and issue a new access_token.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	TokenResponse
//	@Failure		400	{object}	response.errorResponse	"no refresh token cookie"
//	@Failure		401	{object}	response.errorResponse	"invalid or expired refresh token"
//	@Router			/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		response.WriteError(w, "no refresh token cookie", http.StatusBadRequest)
		return
	}
	refreshToken := cookie.Value

	authResult, err := h.authService.Refresh(r.Context(), refreshToken)
	if err != nil {
		handleServiceError(w, err, "refresh")
		return
	}

	setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
}

func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/v1/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // set the cookie as expired
	})
}

// Logout revokes the current refresh token and clears its cookie.
//
//	@Summary		Logout
//	@Description	Revoke the current refresh token (if present) and clear the refresh_token cookie. Idempotent: always returns 200.
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	SimpleResponse
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		ClearCookie(w, h.secure) // harmless if already absent, ensures consistent response
		response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
		return
	}
	refreshToken := cookie.Value

	err = h.authService.Logout(r.Context(), refreshToken)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		// again, on ErrNotFound we skip error handling since we want a consistent response
		log.Printf("logout: %v", err)
		response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		return
	}

	ClearCookie(w, h.secure)
	response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
}

// ForgotPassword issues a password reset token for an email address.
//
//	@Summary		Forgot Password
//	@Description	Request a password reset link. Always returns 202 whether or not the address has an account, so the response cannot be used to discover which emails are registered.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		forgotPasswordRequest	true	"Email to send the reset link to"
//	@Success		202		{object}	SimpleResponse
//	@Failure		400		{object}	response.errorResponse	"invalid body or missing email"
//	@Router			/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if isEmpty(req.Email) {
		response.WriteError(w, "email is required", http.StatusBadRequest)
		return
	}

	token, err := h.authService.CreatePasswordResetToken(r.Context(), req.Email)
	if err != nil {
		handleServiceError(w, err, "forgot password")
		return
	}

	// An empty token means the address has no account. Fall through to the
	// same 202 rather than branching, so the two cases are indistinguishable.
	if token != "" {
		// TODO: hand the token to a mailer. Until one exists the reset link
		// can only be retrieved out of band, so log it in development only —
		// a reset token in a production log is a credential sitting in
		// plaintext for anyone with log access.
		if !h.secure {
			log.Printf("forgot password: reset token for %s: %s", req.Email, token)
		}
	}

	response.WriteJSON(w, SimpleResponse{Message: "if that email has an account, a reset link has been sent"}, http.StatusAccepted)
}

// ResetPassword consumes a password reset token and sets a new password.
//
//	@Summary		Reset Password
//	@Description	Consume a password reset token, set a new password, revoke every refresh token for the user, and clear the refresh_token cookie. The token is single-use.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		resetPasswordRequest	true	"Reset token and new password"
//	@Success		200		{object}	SimpleResponse
//	@Failure		400		{object}	response.errorResponse	"invalid body, missing fields, unusable token, or password rejected"
//	@Router			/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if isEmpty(req.Token) || isEmpty(req.NewPassword) {
		response.WriteError(w, "token and new_password are required", http.StatusBadRequest)
		return
	}

	if err := h.authService.UsePasswordResetToken(r.Context(), req.NewPassword, req.Token); err != nil {
		handleServiceError(w, err, "reset password")
		return
	}

	// The service revoked every refresh token for the user, so whatever this
	// browser is still holding is dead. Clear it rather than leaving a cookie
	// that only fails on the next refresh.
	ClearCookie(w, h.secure)

	response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
}

func setRefreshTokenCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		// Matched against the path the browser requests, not the path this
		// router sees internally, so it carries the /v1 prefix the routes are
		// mounted under (cmd/server/main.go). Scoped to /auth beneath it so
		// the cookie rides along only on the two endpoints that consume it,
		// refresh and logout, rather than on every API call.
		Path:     "/v1/auth",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
	})
}

// SetupRoutes registers the auth route group and its subroutes
func (h *AuthHandler) SetupRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		// rate-limit by IP (may add per-email rate-limiting via redis in the future)
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByRealIP(60, 1*time.Minute))
			r.Post("/register", h.Register)
			r.Post("/login", h.Login)
		})

		// Tighter limit than register/login: these two are the enumeration and
		// password-guessing surface (a held reset token can otherwise probe the
		// current password indefinitely via the same-password rejection).
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByRealIP(10, 1*time.Minute))
			r.Post("/forgot-password", h.ForgotPassword)
			r.Post("/reset-password", h.ResetPassword)
		})

		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})
}
