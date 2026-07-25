package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/middleware"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type ShiftHandler struct {
	shiftService *service.ShiftService
	jwtSecret    string
}

func NewShiftHandler(shiftService *service.ShiftService, jwtSecret string) *ShiftHandler {
	return &ShiftHandler{
		shiftService,
		jwtSecret,
	}
}

// Request Body Structures

type createShiftRequest struct {
	PositionId string             `json:"position_id"`
	UserId     *string            `json:"user_id"`
	Status     domain.ShiftStatus `json:"status"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
}

type updateShiftRequest struct {
	Status    *domain.ShiftStatus `json:"status"`
	StartTime *time.Time          `json:"start_time"`
	EndTime   *time.Time          `json:"end_time"`
}

type assignShiftRequest struct {
	UserId string `json:"user_id"`
}

// Response Structures

type SingleShiftResponse struct {
	Shift domain.Shift `json:"shift"`
}

type MultipleShiftResponse struct {
	Shifts []domain.Shift `json:"shifts"`
}

// Handlers

// CreateShift uses context to fetch the LocationId.
// positionId, an optional userId, status, startTime, and endTime are
// retrieved via the request body.
func (h *ShiftHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req createShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	s, err := h.shiftService.CreateShift(
		r.Context(),
		req.PositionId,
		req.UserId,
		req.Status,
		req.StartTime,
		req.EndTime,
	)
	if err != nil {
		handleServiceError(w, err, "create shift")
		return
	}

	response.WriteJSON(w, SingleShiftResponse{Shift: s}, http.StatusOK)
}

// GetShift retrieves the shiftId via the params.
func (h *ShiftHandler) GetShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	s, err := h.shiftService.GetShift(r.Context(), shiftId)
	if err != nil {
		handleServiceError(w, err, "get shift")
		return
	}

	response.WriteJSON(w, SingleShiftResponse{Shift: s}, http.StatusOK)
}

// GetShiftsByLocation uses context to fetch the LocationId.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *ShiftHandler) GetShiftsByLocation(w http.ResponseWriter, r *http.Request) {
	ss, err := h.shiftService.GetShiftsByLocation(r.Context())
	if err != nil {
		handleServiceError(w, err, "get shifts by location")
		return
	}

	response.WriteJSON(w, MultipleShiftResponse{Shifts: ss}, http.StatusOK)
}

// UpdateShift retrieves the shiftId via the params.
// This retrieves optional values from the request body: status, start_time,
// end_time. This is a partial update operation.
func (h *ShiftHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req updateShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.shiftService.UpdateShift(r.Context(), shiftId, domain.ShiftUpdate{
		Status:    req.Status,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}); err != nil {
		handleServiceError(w, err, "update shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift updated"}, http.StatusOK)
}

// AssignShift retrieves the shiftId via the params, and the target userId via
// the request body.
func (h *ShiftHandler) AssignShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req assignShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.shiftService.AssignShift(r.Context(), shiftId, req.UserId); err != nil {
		handleServiceError(w, err, "assign shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift assigned"}, http.StatusOK)
}

// UnassignShift retrieves the shiftId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *ShiftHandler) UnassignShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	if err := h.shiftService.UnassignShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "unassign shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift unassigned"}, http.StatusOK)
}

// CancelShift retrieves the shiftId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *ShiftHandler) CancelShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	if err := h.shiftService.CancelShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "cancel shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift cancelled"}, http.StatusOK)
}

// DeleteShift retrieves the shiftId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *ShiftHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "id")

	if err := h.shiftService.DeleteShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "delete shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift deleted"}, http.StatusOK)
}

func (h *ShiftHandler) SetupRoutes(r chi.Router) {
	r.Route("/shifts", func(r chi.Router) {
		r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
		r.Post("/", h.CreateShift)
		r.Get("/", h.GetShiftsByLocation)
		r.Get("/{id}", h.GetShift)
		r.Patch("/{id}", h.UpdateShift)
		r.Patch("/{id}/assign", h.AssignShift)
		r.Patch("/{id}/unassign", h.UnassignShift)
		r.Patch("/{id}/cancel", h.CancelShift)
		r.Delete("/{id}", h.DeleteShift)
	})
}
