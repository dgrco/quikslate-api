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

type ShiftHandler struct {
	shiftService *service.ShiftService
	authService  *service.AuthService
	jwtSecret    string
}

func NewShiftHandler(
	shiftService *service.ShiftService,
	authService *service.AuthService,
	jwtSecret string,
) *ShiftHandler {
	return &ShiftHandler{
		shiftService,
		authService,
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

// CreateShift creates a new shift under the location identified by locationId.
//
//	@Summary		Create shift
//	@Description	Create a new shift at a location, optionally pre-assigned to a user. Admin, Manager, or LocationLead.
//	@Tags			shifts
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string				true	"Business ID"
//	@Param			locationId	path		string				true	"Location ID"
//	@Param			body		body		createShiftRequest	true	"Shift details"
//	@Success		200			{object}	SingleShiftResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts [post]
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

// GetShift returns the shift identified by shiftId.
//
//	@Summary		Get shift
//	@Description	Get a single shift by ID. Any member of the shift's location (Admin, LocationLead, Manager, or Employee).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			shiftId		path		string	true	"Shift ID"
//	@Success		200			{object}	SingleShiftResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId} [get]
func (h *ShiftHandler) GetShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

	s, err := h.shiftService.GetShift(r.Context(), shiftId)
	if err != nil {
		handleServiceError(w, err, "get shift")
		return
	}

	response.WriteJSON(w, SingleShiftResponse{Shift: s}, http.StatusOK)
}

// GetShiftsByLocation returns every shift at the location identified by locationId.
//
//	@Summary		List shifts
//	@Description	List all shifts at a location. Any member of the location (Admin, LocationLead, Manager, or Employee).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Success		200			{object}	MultipleShiftResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts [get]
func (h *ShiftHandler) GetShiftsByLocation(w http.ResponseWriter, r *http.Request) {
	ss, err := h.shiftService.GetShiftsByLocation(r.Context())
	if err != nil {
		handleServiceError(w, err, "get shifts by location")
		return
	}

	response.WriteJSON(w, MultipleShiftResponse{Shifts: ss}, http.StatusOK)
}

// UpdateShift partially updates the shift identified by shiftId.
//
//	@Summary		Update shift
//	@Description	Partially update a shift's status, start_time, and/or end_time. Admin, Manager, or LocationLead; if the shift is assigned, caller must be authorized to act on the assigned user's role (see canActOnRole).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string				true	"Business ID"
//	@Param			locationId	path		string				true	"Location ID"
//	@Param			shiftId		path		string				true	"Shift ID"
//	@Param			body		body		updateShiftRequest	true	"Fields to update"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId} [patch]
func (h *ShiftHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

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

// AssignShift assigns the shift identified by shiftId to a target user.
//
//	@Summary		Assign shift
//	@Description	Assign a shift to a user. Admin, Manager, or LocationLead; caller must be authorized to act on the target user's role (see canActOnRole).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string				true	"Business ID"
//	@Param			locationId	path		string				true	"Location ID"
//	@Param			shiftId		path		string				true	"Shift ID"
//	@Param			body		body		assignShiftRequest	true	"Target user"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId}/assign [patch]
func (h *ShiftHandler) AssignShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

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

// UnassignShift clears the assigned user on the shift identified by shiftId.
//
//	@Summary		Unassign shift
//	@Description	Remove the assigned user from a shift. Admin, Manager, or LocationLead; caller must be authorized to act on the assigned user's role (see canActOnRole).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			shiftId		path		string	true	"Shift ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId}/unassign [patch]
func (h *ShiftHandler) UnassignShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

	if err := h.shiftService.UnassignShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "unassign shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift unassigned"}, http.StatusOK)
}

// CancelShift marks the shift identified by shiftId as cancelled.
//
//	@Summary		Cancel shift
//	@Description	Mark a shift as cancelled. Admin, Manager, or LocationLead; if the shift is assigned, caller must be authorized to act on the assigned user's role (see canActOnRole).
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			shiftId		path		string	true	"Shift ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId}/cancel [patch]
func (h *ShiftHandler) CancelShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

	if err := h.shiftService.CancelShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "cancel shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift cancelled"}, http.StatusOK)
}

// DeleteShift deletes the shift identified by shiftId.
//
//	@Summary		Delete shift
//	@Description	Delete a shift. Admin only.
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			shiftId		path		string	true	"Shift ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts/{shiftId} [delete]
func (h *ShiftHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	shiftId := chi.URLParam(r, "shiftId")

	if err := h.shiftService.DeleteShift(r.Context(), shiftId); err != nil {
		handleServiceError(w, err, "delete shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift deleted"}, http.StatusOK)
}

func (h *ShiftHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses/{businessId}/locations/{locationId}/shifts", func(r chi.Router) {
		r.Use(RequireLocationMember(h.authService, h.jwtSecret))
		r.Route("/{shiftId}", func(r chi.Router) {
			r.Get("/", h.GetShift)
			r.Patch("/", h.UpdateShift)
			r.Patch("/assign", h.AssignShift)
			r.Patch("/unassign", h.UnassignShift)
			r.Patch("/cancel", h.CancelShift)
			r.Delete("/", h.DeleteShift)
		})
		r.Post("/", h.CreateShift)
		r.Get("/", h.GetShiftsByLocation)
	})
}
