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

type LocationHandler struct {
	locationService *service.LocationService
	jwtSecret       string
}

func NewLocationHandler(locationService *service.LocationService, jwtSecret string) *LocationHandler {
	return &LocationHandler{
		locationService,
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

// CreateLocation uses context to fetch the BusinessId.
// The location name (and optionally address) is retrieved via the request body.
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

// GetLocation uses context to fetch the BusinessId.
// The locationId is retrieved via the params.
func (h *LocationHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "id")

	l, err := h.locationService.GetLocation(r.Context(), locationId)
	if err != nil {
		handleServiceError(w, err, "get location")
		return
	}

	response.WriteJSON(w, SingleLocationResponse{Location: l}, http.StatusOK)
}

// GetAllLocations uses context to fetch the BusinessId.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *LocationHandler) GetAllLocations(w http.ResponseWriter, r *http.Request) {
	ls, err := h.locationService.GetAllLocations(r.Context())
	if err != nil {
		handleServiceError(w, err, "get all locations")
		return
	}

	response.WriteJSON(w, MultipleLocationResponse{Locations: ls}, http.StatusOK)
}

// UpdateLocation does not use context for state.
// This retrieves optional values from the request body:
// - name
// - address
// This is a partial update operation.
func (h *LocationHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "id")

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

// DeleteLocation uses context to fetch the BusinessId.
// The locationId is retrieved via the params.
func (h *LocationHandler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	locationId := chi.URLParam(r, "id")
 
	if err := h.locationService.DeleteLocation(r.Context(), locationId); err != nil {
		handleServiceError(w, err, "delete location")
		return
	}
 
	response.WriteJSON(w, SimpleResponse{Message: "location deleted"}, http.StatusOK)
}

func (h *LocationHandler) SetupRoutes(r chi.Router) {
	r.Route("/locations", func(r chi.Router) {
		r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
		r.Post("/", h.CreateLocation)
		r.Get("/", h.GetAllLocations)
		r.Get("/{id}", h.GetLocation)
		r.Patch("/{id}", h.UpdateLocation)
		r.Delete("/{id}", h.DeleteLocation)
	})
}
