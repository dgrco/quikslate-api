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

type createShiftRequest struct {
	PositionId string             `json:"position_id"`
	UserId     *string            `json:"user_id"`
	Status     domain.ShiftStatus `json:"status"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
}

type updateShiftRequest struct {
	Status     *domain.ShiftStatus `json:"status"`
	PositionId *string             `json:"position_id"`
	StartTime  *time.Time          `json:"start_time"`
	EndTime    *time.Time          `json:"end_time"`
}

type assignShiftRequest struct {
	UserId string `json:"user_id"`
}

type SingleShiftResponse struct {
	Shift domain.Shift `json:"shift"`
}

type MultipleShiftResponse struct {
	Shifts []domain.ShiftDetail `json:"shifts"`
}

// CreateShift creates a new shift under the location identified by locationId.
//
//	@Summary		Create shift
//	@Description	Create a new shift at a location, optionally pre-assigned to a user. Status must be "draft", "assigned" (requires user_id), or "uncovered" (requires no user_id). Returns 409 if a pre-assigned user already has an overlapping shift. Admin, Manager, or LocationLead. A pre-assigned user must be able to see the shift: they need a role at this location, or business-admin status.
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
//	@Failure		409			{object}	response.errorResponse
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

// GetShiftsByLocation returns the shifts at locationId overlapping the
// requested time range.
//
//	@Summary		List shifts
//	@Description	List the shifts at a location overlapping [from, to), joined with the assignee's and position's names. Both range params are required (there is no unbounded listing), and the range may not exceed 90 days. Cancelled shifts are included. Draft and uncovered shifts are returned only to Admin, LocationLead, and Manager: an unpublished plan and an unfilled slot are both part of building a schedule rather than reading one, so Employees see neither. Any member of the location may call this.
//	@Tags			shifts
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			locationId	path		string	true	"Location ID"
//	@Param			from		query		string	true	"Range start, RFC3339 (inclusive)"
//	@Param			to			query		string	true	"Range end, RFC3339 (exclusive)"
//	@Success		200			{object}	MultipleShiftResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/locations/{locationId}/shifts [get]
func (h *ShiftHandler) GetShiftsByLocation(w http.ResponseWriter, r *http.Request) {
	// Both params are required rather than defaulting to "all time": a
	// forgotten param should be a loud 400, not a silent full-history dump.
	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		response.WriteError(w, "'from' must be an RFC3339 timestamp", http.StatusBadRequest)
		return
	}
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil {
		response.WriteError(w, "'to' must be an RFC3339 timestamp", http.StatusBadRequest)
		return
	}

	ss, err := h.shiftService.GetShiftsByLocation(r.Context(), from, to)
	if err != nil {
		handleServiceError(w, err, "get shifts by location")
		return
	}

	response.WriteJSON(w, MultipleShiftResponse{Shifts: ss}, http.StatusOK)
}

// UpdateShift partially updates the shift identified by shiftId.
//
//	@Summary		Update shift
//	@Description	Partially update a shift's status, position_id, start_time, and/or end_time. Status cannot be set to "assigned" or "cancelled" here; use the dedicated /assign and /cancel endpoints. Moving an assigned shift's times returns 409 if it would overlap another of that user's shifts. Admin, Manager, or LocationLead; if the shift is assigned, caller must be authorized to act on the assigned user's role (see canActOnRole).
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
//	@Failure		409			{object}	response.errorResponse
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
		Status:     req.Status,
		PositionId: req.PositionId,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	}); err != nil {
		handleServiceError(w, err, "update shift")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "shift updated"}, http.StatusOK)
}

// AssignShift assigns the shift identified by shiftId to a target user.
//
//	@Summary		Assign shift
//	@Description	Assign a shift to a user and set its status to "assigned". Returns 409 if the user already has an overlapping shift. Admin, Manager, or LocationLead. The target must be able to see the shift (they need a role at this location, or business-admin status), and the caller must outrank them (see canActOnRole), except when assigning to themselves.
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
//	@Failure		409			{object}	response.errorResponse
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
