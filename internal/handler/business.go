package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
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

type createBusinessRequest struct {
	BusinessName string `json:"business_name"`
}

type renameBusinessRequest struct {
	BusinessName string `json:"business_name"`
}

type setAdminRequest struct {
	Admin bool `json:"admin"`
}

type CreateBusinessResponse struct {
	BusinessId string `json:"business_id"`
}

type BusinessDTO struct {
	Id        string    `json:"business_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MultipleBusinessResponse struct {
	Businesses []BusinessDTO `json:"businesses"`
}

type BusinessMemberDetailDTO struct {
	UserId         string    `json:"user_id"`
	BusinessId     string    `json:"business_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	IsPrimaryAdmin bool      `json:"is_primary_admin"`
	IsAdmin        bool      `json:"is_admin"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MultipleBusinessMemberDetailResponse struct {
	Members []BusinessMemberDetailDTO `json:"members"`
}

type MyLocationRoleDTO struct {
	LocationId string       `json:"location_id"`
	Role       domain.LRole `json:"role"`
}

type MyBusinessMembershipResponse struct {
	UserId         string              `json:"user_id"`
	IsAdmin        bool                `json:"is_admin"`
	IsPrimaryAdmin bool                `json:"is_primary_admin"`
	LocationRoles  []MyLocationRoleDTO `json:"location_roles"`
}

// CreateBusiness creates a new business owned by the caller, who becomes its primary admin.
//
//	@Summary		Create business
//	@Description	Create a new business. The caller becomes its primary admin.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createBusinessRequest	true	"Business details"
//	@Success		200		{object}	CreateBusinessResponse
//	@Failure		400		{object}	response.errorResponse
//	@Failure		401		{object}	response.errorResponse
//	@Router			/businesses [post]
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

	response.WriteJSON(w, CreateBusinessResponse{BusinessId: businessId}, http.StatusOK)
}

// GetBusinessesByUser retrieves all businesses that the authenticated user belongs to.
//
// @Summary Get Businesses By User
// @Description Retrieves all businesses that the currently authenticated user belongs to.
// @Tags businesses
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MultipleBusinessResponse
// @Failure 401 {object} response.errorResponse
// @Router /businesses/me [get]
func (h *BusinessHandler) GetBusinessesByUser(w http.ResponseWriter, r *http.Request) {
	businesses, err := h.businessService.GetBusinessesByUserId(r.Context())
	if err != nil {
		handleServiceError(w, err, "get businesses by user")
		return
	}

	dtos := []BusinessDTO{}
	for _, b := range businesses {
		dto := BusinessDTO{
			Id:        b.Id,
			Name:      b.Name,
			CreatedAt: b.CreatedAt,
			UpdatedAt: b.UpdatedAt,
		}
		dtos = append(dtos, dto)
	}

	response.WriteJSON(w, MultipleBusinessResponse{Businesses: dtos}, http.StatusOK)
}

// GetBusiness returns the business identified by businessId.
//
//	@Summary		Get business
//	@Description	Get a business by ID. Admin only.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	BusinessDTO
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId} [get]
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

// GetBusinessMemberDetails returns the business members and their details by businessId.
//
//	@Summary		Get business member details
//	@Description	Get business member details by business ID
//	@Tags				businesses
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	MultipleBusinessMemberDetailResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/members [get]
func (h *BusinessHandler) GetBusinessMemberDetails(w http.ResponseWriter, r *http.Request) {
	bmds, err := h.businessService.GetBusinessMemberDetailsByBusinessId(r.Context())
	if err != nil {
		handleServiceError(w, err, "get business member details")
		return
	}

	bmdDTOS := []BusinessMemberDetailDTO{}
	for _, bmd := range bmds {
		bmdDTO := BusinessMemberDetailDTO{
			UserId:         bmd.UserId,
			BusinessId:     bmd.BusinessId,
			Name:           bmd.Name,
			Email:          bmd.Email,
			IsPrimaryAdmin: bmd.IsPrimaryAdmin,
			IsAdmin:        bmd.IsAdmin,
			CreatedAt:      bmd.CreatedAt,
			UpdatedAt:      bmd.UpdatedAt,
		}
		bmdDTOS = append(bmdDTOS, bmdDTO)
	}

	response.WriteJSON(w, MultipleBusinessMemberDetailResponse{Members: bmdDTOS}, http.StatusOK)
}

// GetMyBusinessMembership returns the caller's own admin status and location
// roles within the business identified by businessId.
//
//	@Summary		Get my business membership
//	@Description	Get the caller's own admin status and location roles within this business. Lets the frontend decide what admin-only or location-scoped UI to show, without probing each action individually.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	MyBusinessMembershipResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/members/me [get]
func (h *BusinessHandler) GetMyBusinessMembership(w http.ResponseWriter, r *http.Request) {
	m, err := h.businessService.GetMyBusinessMembership(r.Context())
	if err != nil {
		handleServiceError(w, err, "get my business membership")
		return
	}

	roleDTOs := make([]MyLocationRoleDTO, 0, len(m.LocationRoles))
	for _, lr := range m.LocationRoles {
		roleDTOs = append(roleDTOs, MyLocationRoleDTO{LocationId: lr.LocationId, Role: lr.Role})
	}

	response.WriteJSON(w, MyBusinessMembershipResponse{
		UserId:         ctxkeys.GetUserId(r.Context()),
		IsAdmin:        m.IsAdmin,
		IsPrimaryAdmin: m.IsPrimaryAdmin,
		LocationRoles:  roleDTOs,
	}, http.StatusOK)
}

// RenameBusiness updates the business's name.
//
//	@Summary		Rename business
//	@Description	Rename a business. Admin only.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string					true	"Business ID"
//	@Param			body		body		renameBusinessRequest	true	"New name"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId} [patch]
func (h *BusinessHandler) RenameBusiness(w http.ResponseWriter, r *http.Request) {
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

// DeleteBusiness deletes the business identified by businessId.
//
//	@Summary		Delete business
//	@Description	Delete a business and all of its data (locations, positions, shifts, members). Admin only.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId} [delete]
func (h *BusinessHandler) DeleteBusiness(w http.ResponseWriter, r *http.Request) {
	if err := h.businessService.DeleteBusiness(r.Context()); err != nil {
		handleServiceError(w, err, "delete business")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "business deleted"}, http.StatusOK)
}

// RemoveUserFromBusiness removes the target user from the business, including all of their location roles and position assignments.
//
//	@Summary		Remove business member
//	@Description	Remove a user from the business (and all of their location roles/position assignments). Admin only; admins cannot remove other admins.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Produce		json
//	@Param			businessId	path		string	true	"Business ID"
//	@Param			userId		path		string	true	"Target user ID"
//	@Success		200			{object}	SimpleResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Failure		404			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/members/{userId} [delete]
func (h *BusinessHandler) RemoveUserFromBusiness(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")

	if err := h.businessService.RemoveUserFromBusiness(r.Context(), userId); err != nil {
		handleServiceError(w, err, "remove user from business")
		return
	}

	response.WriteJSON(w, SimpleResponse{Message: "user removed from business"}, http.StatusOK)
}

// SetAdminForBusinessMember grants or revokes admin status for the target business member.
//
//	@Summary		Set business member admin status
//	@Description	Grant or revoke admin status for a business member. Admin to grant admin status to a non-admin member; only the primary admin can modify another admin's status.
//	@Tags			businesses
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			businessId	path		string				true	"Business ID"
//	@Param			userId		path		string				true	"Target user ID"
//	@Param			body		body		setAdminRequest		true	"Desired admin status"
//	@Success		200			{object}	SimpleResponse
//	@Failure		400			{object}	response.errorResponse
//	@Failure		401			{object}	response.errorResponse
//	@Failure		403			{object}	response.errorResponse
//	@Router			/businesses/{businessId}/members/{userId}/admin [patch]
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
			r.Use(RequireIdentity(h.authService, h.jwtSecret))
			r.Post("/", h.CreateBusiness)
			r.Get("/me", h.GetBusinessesByUser)
		})
		r.Route("/{businessId}", func(r chi.Router) {
			r.Use(RequireBusinessMember(h.authService, h.jwtSecret))
			r.Get("/", h.GetBusiness)
			r.Patch("/", h.RenameBusiness)
			r.Delete("/", h.DeleteBusiness)

			r.Route("/members", func(r chi.Router) {
				r.Get("/", h.GetBusinessMemberDetails)
				r.Get("/me", h.GetMyBusinessMembership)
				r.Delete("/{userId}", h.RemoveUserFromBusiness)
				r.Patch("/{userId}/admin", h.SetAdminForBusinessMember)
			})
		})
	})
}
