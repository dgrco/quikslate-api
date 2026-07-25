package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrco/quikslate/internal/middleware"
	"github.com/dgrco/quikslate/internal/response"
	"github.com/dgrco/quikslate/internal/service"
	"github.com/go-chi/chi/v5"
)

type BusinessHandler struct {
	businessService *service.BusinessService
	authService     *service.AuthService
	jwtSecret       string
}

func NewBusinessHandler(
	businessService *service.BusinessService,
	authService *service.AuthService,
	jwtSecret string,
) *BusinessHandler {
	return &BusinessHandler{
		businessService,
		authService,
		jwtSecret,
	}
}

// Request Body Structures

type createBusinessRequest struct {
	BusinessName string `json:"business_name"`
}

type renameBusinessRequest struct {
	BusinessName string `json:"business_name"`
}

type setAdminRequest struct {
	Admin bool `json:"admin"`
}

// Response Structures

type BusinessDTO struct {
	Id        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Handlers

// CreateBusiness retrieves BusinessName via the request body.
func (h *BusinessHandler) CreateBusiness(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req createBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	businessId, err := h.businessService.CreateBusiness(r.Context(), req.BusinessName)
	if err != nil {
		handleServiceError(w, err, "create business")
		return
	}

	authResult, err := h.authService.SelectBusiness(r.Context(), businessId)
	if err != nil {
		handleServiceError(w, err, "create business")
		return
	}

	response.WriteJSON(w, authResult, http.StatusOK)
}

// GetBusiness uses context to fetch the Business.
// Therefore, there is NO request body needed.
func (h *BusinessHandler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	b, err := h.businessService.GetBusiness(r.Context())
	if err != nil {
		handleServiceError(w, err, "get business")
		return
	}

	response.WriteJSON(w, BusinessDTO{
		Id:        b.Id,
		Name:      b.Name,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}, http.StatusOK)
}

// RenameBusiness uses context to fetch/update the Business.
// The new business name is retrieved via the request body.
func (h *BusinessHandler) RenameBusiness(w http.ResponseWriter, r *http.Request) {
	// prevent DoS risk
	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req renameBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.businessService.RenameBusiness(r.Context(), req.BusinessName); err != nil {
		handleServiceError(w, err, "rename business")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "business renamed"}, http.StatusOK)
}

// DeleteBusiness uses context to fetch/delete the Business.
// Therefore, there is NO request body needed.
func (h *BusinessHandler) DeleteBusiness(w http.ResponseWriter, r *http.Request) {
	if err := h.businessService.DeleteBusiness(r.Context()); err != nil {
		handleServiceError(w, err, "delete business")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "business deleted"}, http.StatusOK)
}

// RemoveUserFromBusiness retrieves the target userId via the params.
// Therefore: no need to wrap the request body in MaxBytesReader.
func (h *BusinessHandler) RemoveUserFromBusiness(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")

	if err := h.businessService.RemoveUserFromBusiness(r.Context(), userId); err != nil {
		handleServiceError(w, err, "remove user from business")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "user removed from business"}, http.StatusOK)
}

// SetAdminForBusinessMember retrieves the target userId via the params.
// The new admin status is retrieved via the request body.
func (h *BusinessHandler) SetAdminForBusinessMember(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")

	r.Body = http.MaxBytesReader(w, r.Body, DEFAULT_MAX_REQUEST_BODY_SIZE)

	var req setAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, ERR_INVALID_REQ_BODY, http.StatusBadRequest)
		return
	}

	if err := h.businessService.SetAdminForBusinessMember(r.Context(), userId, req.Admin); err != nil {
		handleServiceError(w, err, "set admin for business member")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "business member admin status updated"}, http.StatusOK)
}

func (h *BusinessHandler) SetupRoutes(r chi.Router) {
	r.Route("/businesses", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
			r.Post("/", h.CreateBusiness)
			r.Get("/", h.GetBusiness)
			r.Patch("/", h.RenameBusiness)
			r.Delete("/", h.DeleteBusiness)
			r.Delete("/members/{userId}", h.RemoveUserFromBusiness)
			r.Patch("/members/{userId}/admin", h.SetAdminForBusinessMember)
		})
	})
}
