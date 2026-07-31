package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type LocationHandler struct {
	locationService *service.LocationService
	authService     *service.AuthService
	jwtSecret       string
}

func NewLocationHandler(
	locationService *service.LocationService,
	authService *service.AuthService,
	jwtSecret string,
) *LocationHandler {
	return &LocationHandler{
		locationService,
		authService,
		jwtSecret,
	}
}

// Request Structures

type createLocationRequest struct {
	Name    string  `json:"name"`
	Address *string `json:"address"`
}

type updateLocationRequest struct {
	Name    *string `json:"name"`
	Address *string `json:"address"`
}

// Response Structures

type SingleLocationResponse struct {
	Location domain.Location `json:"location"`
}

type MultipleLocationResponse struct {
	Locations []domain.Location `json:"locations"`
}

// Handlers

// CreateLocation creates a new location under the business identified by businessId.
//
//	@Summary		Create location
//	@Description	Create a new location under a business. Admin only.
//	@Tags			locations
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			body		body		createLocationRequest	true	"Location details"
//	@Success		200			{object}	SingleLocationResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations [post]
func (h *LocationHandler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req createLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	l, err := h.locationService.CreateLocation(r.Context(), req.Name, req.Address)
	if err != nil {
		handleServiceError(w, err, "create location")
		return
	}

	response.WriteJSON(w, SingleLocationResponse{Location: l}, http.StatusOK)
}

// GetLocation returns the location identified by locationId.
//
//	@Summary		Get location
//	@Description	Get a single location by ID. Business member only (non-admins restricted to their session location).
//	@Tags			locations
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Success		200			{object}	SingleLocationResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId} [get]
func (h *LocationHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")

	l, err := h.locationService.GetLocation(r.Context(), locationId)
	if err != nil {
		handleServiceError(w, err, "get location")
		return
	}

	response.WriteJSON(w, SingleLocationResponse{Location: l}, http.StatusOK)
}

// GetAllLocations returns every location belonging to the business identified by businessId.
//
//	@Summary		List locations
//	@Description	List all locations for a business. Business member only.
//	@Tags			locations
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	MultipleLocationResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations [get]
func (h *LocationHandler) GetAllLocations(w http.ResponseWriter, r *http.Request) {
	ls, err := h.locationService.GetAllLocations(r.Context())
	if err != nil {
		handleServiceError(w, err, "get all locations")
		return
	}

	response.WriteJSON(w, MultipleLocationResponse{Locations: ls}, http.StatusOK)
}

// UpdateLocation partially updates the location identified by locationId.
//
//	@Summary		Update location
//	@Description	Partially update a location's name and/or address. Admin only.
//	@Tags			locations
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			locationId	path		string					true	"Location ID"
//	@Param			body		body		updateLocationRequest	true	"Fields to update"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId} [patch]
func (h *LocationHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req updateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.locationService.UpdateLocation(
		r.Context(),
		locationId,
		domain.LocationUpdate{
			Name:    req.Name,
			Address: req.Address,
		},
	); err != nil {
		handleServiceError(w, err, "get location")
		return
	}

	response.WriteJSON(w, SimpleResponse{"location updated"}, http.StatusOK)
}

// DeleteLocation deletes the location identified by locationId.
//
//	@Summary		Delete location
//	@Description	Delete a location and its dependent shifts/roles. Admin only.
//	@Tags			locations
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId} [delete]
func (h *LocationHandler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "locationId")

	if err := h.locationService.DeleteLocation(r.Context(), locationId); err != nil {
		handleServiceError(w, err, "delete location")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "location deleted"}, http.StatusOK)
}

func (h *LocationHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses/{businessId}/locations", func(r chi.Router) {
		// Identity Only
		r.Group(func(r chi.Router) {
			r.Use(RequireBusinessMember(h.authService, h.jwtSecret))
			r.Get("/", h.GetAllLocations)
			r.Post("/", h.CreateLocation)
		})
		// Location (thus Business!) scoped
		r.Route("/{locationId}", func(r chi.Router) {
			r.Use(RequireLocationMember(h.authService, h.jwtSecret))
			r.Get("/", h.GetLocation)
			r.Patch("/", h.UpdateLocation)
			r.Delete("/", h.DeleteLocation)
		})
	})
}
