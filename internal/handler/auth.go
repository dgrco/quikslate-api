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

const (
	ERR_INVALID_REQ_BODY      = "invalid request body"
	ERR_INTERNAL_SERVER       = "internal server error"
	ERR_NOT_FOUND             = "not found"
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
	Email        string `json:"email"`
	Password     string `json:"password"`
	BusinessName string `json:"business_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type selectBusinessRequest struct {
	UserId     string `json:"user_id"`
	BusinessId string `json:"business_id"`
}

type selectLocationRequest struct {
	UserId     string `json:"user_id"`
	BusinessId string `json:"business_id"`
	LocationId string `json:"location_id"`
}

// Response Structures

type SimpleResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

type LocationSelectionResponse struct {
	UserId     string   `json:"user_id"`
	BusinessId string   `json:"business_id"`
	Locations  []string `json:"locations"`
}

// Helpers

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}

// Handlers

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.Email) || isEmpty(req.Password) || isEmpty(req.BusinessName) {
		response.WriteError(w, "email, password, and business_id are required", http.StatusBadRequest)
		return
	}

	authResponse, err := h.authService.Register(r.Context(), req.Email, req.Password, req.BusinessName)
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
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

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

	if authResponse.RequiresBusinessSelection {
		// return businesses list + UserId, do NOT set cookie
		response.WriteJSON(w, authResponse, http.StatusOK)
		return
	}
	if authResponse.RequiresLocationSelection {
		// return locations list, do NOT set cookie
		response.WriteJSON(w, authResponse, http.StatusOK)
		return
	}
	// set cookie if fully authenticated
	setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

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
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	clearCookie := func() {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Path:     "/auth",
			HttpOnly: true,
			Secure:   h.secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1, // set the cookie as expired
		})
	}

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		clearCookie() // harmless if already absent, ensures consistent response
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

	clearCookie()
	response.WriteJSON(w, SimpleResponse{Message: "ok"}, http.StatusOK)
}

func (h *AuthHandler) SelectBusiness(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req selectBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.UserId) || isEmpty(req.BusinessId) {
		response.WriteError(w, "user_id and business_id are required", http.StatusBadRequest)
		return
	}

	authResponse, err := h.authService.SelectBusiness(r.Context(), req.UserId, req.BusinessId)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			response.WriteError(w, ERR_NOT_FOUND, http.StatusNotFound)
		default:
			log.Printf("select business: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	if !authResponse.RequiresLocationSelection {
		// no need to select a location
		setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
		response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
		return
	}

	// return a location selection response
	locationList := []string{}
	for _, lr := range authResponse.Locations {
		locationList = append(locationList, lr.LocationId)
	}

	response.WriteJSON(
		w,
		LocationSelectionResponse{
			UserId:     req.UserId,
			BusinessId: req.BusinessId,
			Locations:  locationList,
		},
		http.StatusOK,
	)
}

func (h *AuthHandler) SelectLocation(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 1 * 1024 * 1024 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req selectLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.UserId) || isEmpty(req.BusinessId) || isEmpty(req.LocationId) {
		response.WriteError(w, "user_id, business_id, and location_id are required", http.StatusBadRequest)
		return
	}

	authResponse, err := h.authService.SelectLocation(r.Context(), req.UserId, req.BusinessId, req.LocationId)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			response.WriteError(w, ERR_NOT_FOUND, http.StatusNotFound)
		default:
			log.Printf("select location: %v", err)
			response.WriteError(w, ERR_INTERNAL_SERVER, http.StatusInternalServerError)
		}
		return
	}

	setRefreshTokenCookie(w, authResponse.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResponse.AccessToken}, http.StatusOK)
}

func setRefreshTokenCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/auth",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
	})
}

// SetupRoutes registers the auth route group and its subroutes
func (h *AuthHandler) SetupRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		// rate-limit by IP (may add per-email rate-limiting via redis in the future)
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(10, 1*time.Minute)) 
			r.Post("/register", h.Register)
			r.Post("/login", h.Login)
		})

		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.Post("/select-business", h.SelectBusiness)
		r.Post("/select-location", h.SelectLocation)
	})
}
