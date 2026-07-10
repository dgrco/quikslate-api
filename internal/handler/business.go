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
	jwtSecret       string
}

func NewBusinessHandler(businessService *service.BusinessService, jwtSecret string) *BusinessHandler {
	return &BusinessHandler{
		businessService,
		jwtSecret,
	}
}

// Request Body Structures

type renameBusinessRequest struct {
	BusinessName string `json:"business_name"`
}

// Response Structures

type GetBusinessResponse struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Handlers

// GetBusiness uses context to fetch the Business.
// Therefore, there is NO request body needed.
func (h *BusinessHandler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	b, err := h.businessService.GetBusiness(r.Context())
	if err != nil {
		handleServiceError(w, err, "get business")
		return
	}

	response.WriteJSON(w, GetBusinessResponse{
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

func (h *BusinessHandler) SetupRoutes(r chi.Router) {
	r.Route("/business", func(r chi.Router) {
		r.Use(middleware.AccessAuthMiddleware(h.jwtSecret))
		r.Get("/", h.GetBusiness)
		r.Patch("/", h.RenameBusiness)
		r.Delete("/", h.DeleteBusiness)
	})
}
