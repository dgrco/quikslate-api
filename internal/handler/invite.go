package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
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

type AcceptInviteResponse struct {
	BusinessId string `json:"business_id"`
}

// Handlers

// CreateInvite creates an outstanding invite for an email address to join the business at a given location and role.
//
//	@Summary		Create invite
//	@Description	Invite an email address to join the business at a location with a target role. Admin, or LocationLead at the target location (may only invite Managers or Employees, see canActOnRole).
//	@Tags			invites
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			body		body		createInviteRequest	true	"Invite details"
//	@Success		200			{object}	CreateInviteResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/invites [post]
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

// GetInviteByToken previews an invite by its raw token. No authentication required.
//
//	@Summary		Preview invite
//	@Description	Preview a pending invite by its raw token (email, business name, role, expiry). No authentication required.
//	@Tags			invites
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			token		path		string	true	"Raw invite token"
//	@Success		200			{object}	service.InviteDTO
//	@Failure		404			{object}	response.errorResponse	"invite not found, expired, or already accepted"
//	@Router			/businesses/{businessId}/invites/{token} [get]
func (h *InviteHandler) GetInviteByToken(w http.ResponseWriter, r *http.Request) {
	inviteToken := chi.URLParam(r, "token")
	inv, err := h.inviteService.PreviewInviteByToken(r.Context(), inviteToken)
	if err != nil {
		handleServiceError(w, err, "get invite by token")
		return
	}

	response.WriteJSON(w, inv, http.StatusOK)
}

// AcceptInvite accepts an invite by its raw token as the authenticated caller.
//
//	@Summary		Accept invite
//	@Description	Accept a pending invite as the authenticated caller, joining the business at the invited location/role.
//	@Tags			invites
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			token		path		string	true	"Raw invite token"
//	@Success		200			{object}	AcceptInviteResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse	"invite not found, expired, or already accepted"
//	@Router			/businesses/{businessId}/invites/{token}/accept [post]
func (h *InviteHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	inviteToken := chi.URLParam(r, "token")
	businessId, err := h.inviteService.AcceptInvite(r.Context(), inviteToken)
	if err != nil {
		handleServiceError(w, err, "accept invite")
		return
	}

	response.WriteJSON(w, AcceptInviteResponse{BusinessId: businessId}, http.StatusOK)
}

// Routes
func (h *InviteHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses/{businessId}/invites", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(RequireBusinessMember(h.authService, h.jwtSecret))
			r.Post("/", h.CreateInvite)
		})
		r.Group(func(r chi.Router) {
			r.Use(RequireIdentity(h.authService, h.jwtSecret))
			r.Post("/{token}/accept", h.AcceptInvite)
		})
		r.Get("/{token}", h.GetInviteByToken)
	})
}
