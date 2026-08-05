package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type LocationRoleHandler struct {
	locationRoleService *service.LocationRoleService
	authService         *service.AuthService
	jwtSecret           string
}

func NewLocationRoleHandler(
	locationRoleService *service.LocationRoleService,
	authService *service.AuthService,
	jwtSecret string,
) *LocationRoleHandler {
	return &LocationRoleHandler{
		locationRoleService,
		authService,
		jwtSecret,
	}
}

// Request Structures

type assignLocationRoleRequest struct {
	Role domain.LRole `json:"role"`
}

// Response Structures

type LocationRoleDetailDTO struct {
	UserId     string       `json:"user_id"`
	BusinessId string       `json:"business_id"`
	LocationId string       `json:"location_id"`
	Name       string       `json:"name"`
	Email      string       `json:"email"`
	Role       domain.LRole `json:"role"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

type MultipleLocationRoleResponse struct {
	Roles []LocationRoleDetailDTO `json:"roles"`
}

// Handlers

// AssignLocationRole assigns or changes the target user's role at the location identified by locationId.
//
//	@Summary		Assign location role
//	@Description	Assign or change a business member's role at a location. Admin, LocationLead, or Manager (within the role hierarchy).
//	@Tags			location-roles
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string						true	"Business ID"
//	@Param			locationId	path		string						true	"Location ID"
//	@Param			userId		path		string						true	"Target user ID"
//	@Param			body		body		assignLocationRoleRequest	true	"Role to assign"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/roles/{userId} [put]
func (h *LocationRoleHandler) AssignLocationRole(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")
	userId := chi.URLParam(r, "userId")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req assignLocationRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.locationRoleService.AssignRole(r.Context(), locationId, userId, req.Role); err != nil {
		handleServiceError(w, err, "assign location role")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "role assigned"}, http.StatusOK)
}

// RemoveLocationRole removes the target user's role at the location identified by locationId.
//
//	@Summary		Remove location role
//	@Description	Remove a business member's role at a location. Admin, LocationLead, or Manager (within the role hierarchy).
//	@Tags			location-roles
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			userId		path		string	true	"Target user ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/roles/{userId} [delete]
func (h *LocationRoleHandler) RemoveLocationRole(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")
	userId := chi.URLParam(r, "userId")

	if err := h.locationRoleService.RemoveRole(r.Context(), locationId, userId); err != nil {
		handleServiceError(w, err, "remove location role")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "role removed"}, http.StatusOK)
}

// GetLocationRoles returns the roster of every business member with a role at the location identified by locationId.
//
//	@Summary		List location roles
//	@Description	List every business member with a role at a location. Admin, LocationLead, or Manager.
//	@Tags			location-roles
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Success		200			{object}	MultipleLocationRoleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/roles [get]
func (h *LocationRoleHandler) GetLocationRoles(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")

	roster, err := h.locationRoleService.GetLocationRoles(r.Context(), locationId)
	if err != nil {
		handleServiceError(w, err, "get location roles")
		return
	}

	dtos := make([]LocationRoleDetailDTO, 0, len(roster))
	for _, lrd := range roster {
		dtos = append(dtos, LocationRoleDetailDTO{
			UserId:     lrd.UserId,
			BusinessId: lrd.BusinessId,
			LocationId: lrd.LocationId,
			Name:       lrd.Name,
			Email:      lrd.Email,
			Role:       lrd.Role,
			CreatedAt:  lrd.CreatedAt,
			UpdatedAt:  lrd.UpdatedAt,
		})
	}

	response.WriteJSON(w, MultipleLocationRoleResponse{Roles: dtos}, http.StatusOK)
}

func (h *LocationRoleHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses/{businessId}/locations/{locationId}/roles", func(r chi.Router) {
		r.Use(RequireLocationMember(h.authService, h.jwtSecret))
		r.Get("/", h.GetLocationRoles)
		r.Put("/{userId}", h.AssignLocationRole)
		r.Delete("/{userId}", h.RemoveLocationRole)
	})
}
