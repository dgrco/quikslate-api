package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/middleware"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

type AuthHandler struct {
	authService *service.AuthService
	secure      bool // should be true in production and false in development (set in Config.SecureMode)
	jwtSecret string
}

// NewAuthHandler creates an AuthHandler object.
// The secure parameter refers to whether we are in a prod or dev environment.
func NewAuthHandler(authService *service.AuthService, secure bool, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService,
		secure,
		jwtSecret,
	}
}

// Request Body Structures

type registerRequest struct {
	Email        string `json:"email"`
	Name				 string `json:"name"`
	Password     string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type selectBusinessRequest struct {
	BusinessId string `json:"business_id"`
}

type selectLocationRequest struct {
	BusinessId string `json:"business_id"`
	LocationId string `json:"location_id"`
}

// Response Structures

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

type LocationSelectionResponse struct {
	BusinessId string   `json:"business_id"`
	Locations  []string `json:"locations"`
}

// Helpers

func isEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}

// Handlers

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

	if authResult.RequiresBusinessSelection {
		// return businesses list + UserId, do NOT set cookie
		response.WriteJSON(w, authResult, http.StatusOK)
		return
	}
	if authResult.RequiresLocationSelection {
		// return locations list, do NOT set cookie
		response.WriteJSON(w, authResult, http.StatusOK)
		return
	}
	// set cookie if fully authenticated
	setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
}

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

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

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
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req selectBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.BusinessId) {
		response.WriteError(w, "business_id is required", http.StatusBadRequest)
		return
	}

	authResult, err := h.authService.SelectBusiness(r.Context(), req.BusinessId)
	if err != nil {
		handleServiceError(w, err, "select business")
		return
	}

	if !authResult.RequiresLocationSelection {
		// no need to select a location
		setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
		response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
		return
	}

	// return a location selection response
	locationList := []string{}
	for _, lr := range authResult.Locations {
		locationList = append(locationList, lr.LocationId)
	}

	response.WriteJSON(
		w,
		LocationSelectionResponse{
			BusinessId: req.BusinessId,
			Locations:  locationList,
		},
		http.StatusOK,
	)
}

func (h *AuthHandler) SelectLocation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req selectLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	// validate request
	if isEmpty(req.BusinessId) || isEmpty(req.LocationId) {
		response.WriteError(w, "business_id and location_id are required", http.StatusBadRequest)
		return
	}

	authResult, err := h.authService.SelectLocation(r.Context(), req.BusinessId, req.LocationId)
	if err != nil {
		handleServiceError(w, err, "select location")
		return
	}

	setRefreshTokenCookie(w, authResult.RefreshToken, h.secure)
	response.WriteJSON(w, TokenResponse{AccessToken: authResult.AccessToken}, http.StatusOK)
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

		r.Group(func(r chi.Router) {
			r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
			r.Post("/select-business", h.SelectBusiness)
			r.Post("/select-location", h.SelectLocation)
		})
	})
}
