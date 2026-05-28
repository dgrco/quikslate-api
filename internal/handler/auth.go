package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

const (
	ERR_INVALID_REQ_BODY      = "invalid request body"
	ERR_INTERNAL_SERVER       = "internal server error"
	ERR_INVALID_REFRESH_TOKEN = "invalid refresh token"
)

type AuthHandler struct {
	authService *service.AuthService
	secure      bool // should be true in production and false in development (set in Config.SecureMode)
}

// NewAuthHandler creates an AuthHandler object.
// The secure parameter refers to whether we are in a prod or dev environment.
func NewAuthHandler(authService *service.AuthService, secure bool) *AuthHandler {
	return &AuthHandler{
		authService,
		secure,
	}
}

// Request Body Structures

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Response Structures

type SimpleResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Handlers

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	authResponse, err := h.authService.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		var validationErr *domain.ValidationError
		switch {
		case errors.As(err, &validationErr):
			response.WriteError(w, validationErr.Message, http.StatusBadRequest)
		case errors.Is(err, domain.ErrAlreadyExists):
			response.WriteError(w, "that email is already in use", http.StatusConflict)
		default:
			// unexpected errors
			log.Printf("register: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	authResponse, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			response.WriteError(w, "invalid login credentials", http.StatusUnauthorized)
		default:
			log.Printf("login: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		response.WriteError(w, "no refresh token cookie", http.StatusBadRequest)
		return
	}
	refreshToken := cookie.Value

	authResponse, err := h.authService.Refresh(r.Context(), refreshToken)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidRefreshToken):
			response.WriteError(w, ERR_INVALID_REFRESH_TOKEN, http.StatusUnauthorized)
		default:
			log.Printf("refresh: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		response.WriteError(w, "no refresh token cookie", http.StatusBadRequest)
		return
	}
	refreshToken := cookie.Value

	err = h.authService.Logout(r.Context(), refreshToken)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			response.WriteError(w, ERR_INVALID_REFRESH_TOKEN, http.StatusUnauthorized)
		default:
			log.Printf("logout: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
}

func setRefreshTokenCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/auth",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	})
}

// SetupRoutes registers the auth route group and its subroutes
func (h *AuthHandler) SetupRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})
}
