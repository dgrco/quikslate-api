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

type PositionHandler struct {
	positionService *service.PositionService
	jwtSecret       string
}

func NewPositionHandler(positionService *service.PositionService, jwtSecret string) *PositionHandler {
	return &PositionHandler{
		positionService,
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

// CreatePosition uses context to fetch the BusinessId.
// The position name is retrieved via the request body.
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

// GetPosition uses context to fetch the BusinessId.
// The positionId is retrieved via the params.
func (h *PositionHandler) GetPosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "id")

	p, err := h.positionService.GetPosition(r.Context(), positionId)
	if err != nil {
		handleServiceError(w, err, "get position")
		return
	}

	response.WriteJSON(w, SinglePositionResponse{Position: p}, http.StatusOK)
}

// GetAllPositionsByBusiness uses context to fetch the BusinessId.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *PositionHandler) GetAllPositionsByBusiness(w http.ResponseWriter, r *http.Request) {
	ps, err := h.positionService.GetAllPositionsByBusiness(r.Context())
	if err != nil {
		handleServiceError(w, err, "get all positions by business")
		return
	}

	response.WriteJSON(w, MultiplePositionResponse{Positions: ps}, http.StatusOK)
}

// RenamePosition uses context to fetch the BusinessId.
// The new position name is retrieved via the request body.
func (h *PositionHandler) RenamePosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "id")

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

// DeletePosition uses context to fetch the BusinessId.
// The positionId is retrieved via the params.
func (h *PositionHandler) DeletePosition(w http.ResponseWriter, r *http.Request) {
	positionId := chi.URLParam(r, "id")

	if err := h.positionService.DeletePosition(r.Context(), positionId); err != nil {
		handleServiceError(w, err, "delete position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position deleted"}, http.StatusOK)
}

func (h *PositionHandler) SetupRoutes(r chi.Router) {
	r.Route("/positions", func(r chi.Router) {
		r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
		r.Post("/", h.CreatePosition)
		r.Get("/", h.GetAllPositionsByBusiness)
		r.Get("/{id}", h.GetPosition)
		r.Patch("/{id}", h.RenamePosition)
		r.Delete("/{id}", h.DeletePosition)
	})
}
