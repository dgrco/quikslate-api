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

type EmployeeHandler struct {
	employeeService *service.EmployeeService
	jwtSecret       string
}

func NewEmployeeHandler(employeeService *service.EmployeeService, jwtSecret string) *EmployeeHandler {
	return &EmployeeHandler{
		employeeService,
		jwtSecret,
	}
}

// Request Body Structures

type addPositionRequest struct {
	PositionId string `json:"position_id"`
}

// Response Structures

type MultipleEmployeePositionResponse struct {
	Positions []domain.EmployeePosition `json:"positions"`
}

// Handlers

// AddPosition retrieves the target userId via the params, and the positionId
// via the request body.
func (h *EmployeeHandler) AddPosition(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req addPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.employeeService.AddPosition(r.Context(), userId, req.PositionId); err != nil {
		handleServiceError(w, err, "add position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position added"}, http.StatusOK)
}

// RemovePosition retrieves the target userId and positionId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *EmployeeHandler) RemovePosition(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")
	positionId := chi.URLParam(r, "positionId")

	if err := h.employeeService.RemovePosition(r.Context(), userId, positionId); err != nil {
		handleServiceError(w, err, "remove position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position removed"}, http.StatusOK)
}

// GetAllPositionsByUser retrieves the target userId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *EmployeeHandler) GetAllPositionsByUser(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")

	positions, err := h.employeeService.GetAllPositionsByUser(r.Context(), userId)
	if err != nil {
		handleServiceError(w, err, "get all positions by user")
		return
	}

	response.WriteJSON(w, MultipleEmployeePositionResponse{Positions: positions}, http.StatusOK)
}

func (h *EmployeeHandler) SetupRoutes(r chi.Router) {
	r.Route("/employees/{userId}/positions", func(r chi.Router) {
		r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
		r.Post("/", h.AddPosition)
		r.Get("/", h.GetAllPositionsByUser)
		r.Delete("/{positionId}", h.RemovePosition)
	})
}
