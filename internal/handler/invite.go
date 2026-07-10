package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/middleware"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type InviteHandler struct {
	inviteService *service.InviteService
	authService   *service.AuthService
	jwtSecret     string
}

func NewInviteHandler(
	inviteService *service.InviteService,
	authService *service.AuthService,
	jwtSecret string,
) *InviteHandler {
	return &InviteHandler{
		inviteService,
		authService,
		jwtSecret,
	}
}

// Request Body Structures

type createInviteRequest struct {
	Email      string       `json:"email"`
	LocationId string       `json:"location_id"`
	TargetRole domain.LRole `json:"target_role"`
}

// Response Structures

type CreateInviteResponse struct {
	InviteToken string `json:"invite_token"`
}

// Handlers

// CreateInvite uses context to fetch businessId.
// The email, locationId and targetRole values are fetched from
// the request body.
func (h *InviteHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	inviteResult, err := h.inviteService.CreateInvite(r.Context(), req.Email, req.LocationId, req.TargetRole)
	if err != nil {
		handleServiceError(w, err, "create invite")
		return
	}

	response.WriteJSON(w, CreateInviteResponse{InviteToken: inviteResult.InviteToken}, http.StatusOK)
}

// GetInviteByToken fetches the raw token from params
func (h *InviteHandler) GetInviteByToken(w http.ResponseWriter, r *http.Request) {
	inviteToken := chi.URLParam(r, "token")
	inv, err := h.inviteService.GetInviteByToken(r.Context(), inviteToken)
	if err != nil {
		handleServiceError(w, err, "get invite by token")
		return
	}

	response.WriteJSON(w, inv, http.StatusOK)
}

// AcceptInvite fetches the raw token from params
func (h *InviteHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	inviteToken := chi.URLParam(r, "token")
	businessId, err := h.inviteService.AcceptInvite(r.Context(), inviteToken)
	if err != nil {
		handleServiceError(w, err, "accept invite")
		return
	}

	authResult, err := h.authService.SelectBusiness(r.Context(), businessId)
	if err != nil {
		handleServiceError(w, err, "accept invite")
		return
	}

	response.WriteJSON(w, authResult, http.StatusOK)
}

// Routes
func (h *InviteHandler) SetupRoutes(r chi.Router) {
	r.Route("/invites", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
			r.Post("/", h.CreateInvite)
			r.Post("/{token}/accept", h.AcceptInvite)
		})
		r.Get("/{token}", h.GetInviteByToken)
	})
}
