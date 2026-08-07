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

// Timezone is an IANA name (e.g. "America/New_York"). It's required on both
// requests, not optional-with-a-default: updateLocationRequest is a full
// replacement (see domain.LocationUpdate), so accepting an empty value here
// would silently reset an existing location to UTC.
type createLocationRequest struct {
	Name     string  `json:"name"`
	Address  *string `json:"address"`
	Timezone string  `json:"timezone"`
}

type updateLocationRequest struct {
	Name     string  `json:"name"`
	Address  *string `json:"address"`
	Timezone string  `json:"timezone"`
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

	l, err := h.locationService.CreateLocation(r.Context(), req.Name, req.Address, req.Timezone)
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

// UpdateLocation fully replaces the name, address, and timezone of the location identified by locationId.
//
//	@Summary		Update location
//	@Description	Replace a location's name, address, and timezone. Full replacement — every field is applied as given, so omitting timezone resets it. Admin only.
//	@Tags			locations
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			locationId	path		string					true	"Location ID"
//	@Param			body		body		updateLocationRequest	true	"New location fields"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId} [put]
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
			Name:     req.Name,
			Address:  req.Address,
			Timezone: req.Timezone,
		},
	); err != nil {
		handleServiceError(w, err, "update location")
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
			r.Put("/", h.UpdateLocation)
			r.Delete("/", h.DeleteLocation)
		})
	})
}
