package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

// This file implements the position handler: creating, reading, listing,
// renaming, and deleting a business's positions (job roles).

type PositionHandler struct {
	positionService *service.PositionService
	authService     *service.AuthService
	jwtSecret       string
}

// NewPositionHandler constructs a PositionHandler backed by positionService
// and authService.
func NewPositionHandler(
	positionService *service.PositionService,
	authService *service.AuthService,
	jwtSecret string,
) *PositionHandler {
	return &PositionHandler{
		positionService,
		authService,
		jwtSecret,
	}
}

// Request Body Structures

type createPositionRequest struct {
	Name string `json:"name"`
}

type renamePositionRequest struct {
	Name string `json:"name"`
}

// Response Structures

type SinglePositionResponse struct {
	Position domain.Position `json:"position"`
}

type MultiplePositionResponse struct {
	Positions []domain.Position `json:"positions"`
}

// Handlers

// CreatePosition creates a new position under the business identified by businessId.
//
//	@Summary		Create position
//	@Description	Create a new position (job role, business-wide) under a business. Admin only.
//	@Tags			positions
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			body		body		createPositionRequest	true	"Position details"
//	@Success		200			{object}	SinglePositionResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/positions [post]
func (h *PositionHandler) CreatePosition(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req createPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	p, err := h.positionService.CreatePosition(r.Context(), req.Name)
	if err != nil {
		handleServiceError(w, err, "create position")
		return
	}

	response.WriteJSON(w, SinglePositionResponse{Position: p}, http.StatusOK)
}

// GetPosition returns the position identified by positionId.
//
//	@Summary		Get position
//	@Description	Get a single position by ID. Business member only.
//	@Tags			positions
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			positionId	path		string	true	"Position ID"
//	@Success		200			{object}	SinglePositionResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/positions/{positionId} [get]
func (h *PositionHandler) GetPosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "positionId")

	p, err := h.positionService.GetPosition(r.Context(), positionId)
	if err != nil {
		handleServiceError(w, err, "get position")
		return
	}

	response.WriteJSON(w, SinglePositionResponse{Position: p}, http.StatusOK)
}

// GetAllPositionsByBusiness returns every position belonging to the business identified by businessId.
//
//	@Summary		List positions
//	@Description	List all positions for a business. Business member only.
//	@Tags			positions
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	MultiplePositionResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/positions [get]
func (h *PositionHandler) GetAllPositionsByBusiness(w http.ResponseWriter, r *http.Request) {
	ps, err := h.positionService.GetAllPositionsByBusiness(r.Context())
	if err != nil {
		handleServiceError(w, err, "get all positions by business")
		return
	}

	response.WriteJSON(w, MultiplePositionResponse{Positions: ps}, http.StatusOK)
}

// RenamePosition updates the position's name.
//
//	@Summary		Rename position
//	@Description	Rename a position. Admin only.
//	@Tags			positions
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			positionId	path		string					true	"Position ID"
//	@Param			body		body		renamePositionRequest	true	"New name"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/positions/{positionId} [patch]
func (h *PositionHandler) RenamePosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "positionId")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req renamePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.positionService.RenamePosition(r.Context(), positionId, req.Name); err != nil {
		handleServiceError(w, err, "rename position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position renamed"}, http.StatusOK)
}

// DeletePosition deletes the position identified by positionId.
//
//	@Summary		Delete position
//	@Description	Delete a position. Admin only.
//	@Tags			positions
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			positionId	path		string	true	"Position ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/positions/{positionId} [delete]
func (h *PositionHandler) DeletePosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "positionId")

	if err := h.positionService.DeletePosition(r.Context(), positionId); err != nil {
		handleServiceError(w, err, "delete position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position deleted"}, http.StatusOK)
}

func (h *PositionHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses/{businessId}/positions", func(r chi.Router) {
		r.Use(RequireBusinessMember(h.authService, h.jwtSecret))
		r.Route("/{positionId}", func(r chi.Router) {
			r.Get("/", h.GetPosition)
			r.Patch("/", h.RenamePosition)
			r.Delete("/", h.DeletePosition)
		})
		r.Post("/", h.CreatePosition)
		r.Get("/", h.GetAllPositionsByBusiness)
	})
}
