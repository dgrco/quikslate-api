package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type EmployeeHandler struct {
	employeeService *service.EmployeeService
	authService     *service.AuthService
	jwtSecret       string
}

func NewEmployeeHandler(
	employeeService *service.EmployeeService,
	authService *service.AuthService,
	jwtSecret string,
) *EmployeeHandler {
	return &EmployeeHandler{
		employeeService,
		authService,
		jwtSecret,
	}
}

type addPositionRequest struct {
	PositionId string `json:"position_id"`
}

type MultipleEmployeePositionResponse struct {
	Positions []domain.EmployeePosition `json:"positions"`
}

// AddPosition assigns a position to the target user identified by userId.
//
//	@Summary		Add employee position
//	@Description	Assign a position to a user. Admin, Manager, or LocationLead (any location in the business).
//	@Tags			employees
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string				true	"Business ID"
//	@Param			userId		path		string				true	"Target user ID"
//	@Param			body		body		addPositionRequest	true	"Position to add"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/employees/{userId}/positions [post]
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

// RemovePosition removes a position from the target user identified by userId.
//
//	@Summary		Remove employee position
//	@Description	Remove a position from a user. Admin only.
//	@Tags			employees
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			userId		path		string	true	"Target user ID"
//	@Param			positionId	path		string	true	"Position ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/employees/{userId}/positions/{positionId} [delete]
func (h *EmployeeHandler) RemovePosition(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")
	positionId := chi.URLParam(r, "positionId")

	if err := h.employeeService.RemovePosition(r.Context(), userId, positionId); err != nil {
		handleServiceError(w, err, "remove position")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "position removed"}, http.StatusOK)
}

// GetAllPositionsByUser returns every position assigned to the target user identified by userId.
//
//	@Summary		List employee positions
//	@Description	List all positions assigned to a user within the business. Admin, Manager, or LocationLead (any location in the business).
//	@Tags			employees
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			userId		path		string	true	"Target user ID"
//	@Success		200			{object}	MultipleEmployeePositionResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/employees/{userId}/positions [get]
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
	r.Route("/businesses/{businessId}/employees", func(r chi.Router) {
		r.Use(RequireBusinessMember(h.authService, h.jwtSecret))
		r.Route("/{userId}/positions", func(r chi.Router) {
			r.Post("/", h.AddPosition)
			r.Get("/", h.GetAllPositionsByUser)
			r.Delete("/{positionId}", h.RemovePosition)
		})
	})
}
