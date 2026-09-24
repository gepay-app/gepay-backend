package http

import (
	"net/http"
	"uuid"

	"github.com/labstack/echo/v5"

	"gepay/internal/modules/identity/api/domain"
	"gepay/internal/modules/identity/api/middleware"
	"gepay/platform/apperror"
	"gepay/platform/server/validation"
)

type Handler struct {
	svc domain.Service
}

func NewHandler(svc domain.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes hanya mendaftarkan PATH + bentuk request/response + gate role
// per-endpoint. RequireAuth dipasang sekali di level group oleh module.go.
//
// Gate role di sini sengaja dicocokkan dengan aturan di service (defense in
// depth): promote = SUPER_ADMIN saja, ubah KYC = ADMIN atau SUPER_ADMIN.
func (h *Handler) RegisterAdminRoutes(g *echo.Group) {
	g.PUT("/users/:id/role", h.updateUserRole, middleware.RequireRole(domain.RoleSuperAdmin))
	g.PUT("/users/:id/kyc", h.updateKYCStatus, middleware.RequireRole(domain.RoleAdmin, domain.RoleSuperAdmin))
}

// RegisterUserRoutes mendaftarkan route untuk user biasa (hanya butuh login).
func (h *Handler) RegisterUserRoutes(g *echo.Group) {
	g.PUT("/users/me/profile", h.updateProfile)
}

type updateUserRoleRequest struct {
	// Hanya ADMIN: promosi ke SUPER_ADMIN sengaja tidak lewat HTTP.
	Role string `json:"role" validate:"required,oneof=ADMIN"`
}

type updateKYCStatusRequest struct {
	KYCStatus string `json:"kyc_status" validate:"required,oneof=UNVERIFIED PENDING VERIFIED REJECTED"`
}

type updateProfileRequest struct {
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Nickname  string `json:"nickname" validate:"required,max=50"`
}

func (h *Handler) updateUserRole(c *echo.Context) error {
	targetID, err := parseUserID(c)
	if err != nil {
		return err
	}

	var req updateUserRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := validation.New().Struct(&req); err != nil {
		return err
	}

	actor := middleware.FromContext(c)
	if err := h.svc.PromoteToAdmin(c.Request().Context(), actor.UserID, targetID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) updateKYCStatus(c *echo.Context) error {
	targetID, err := parseUserID(c)
	if err != nil {
		return err
	}

	var req updateKYCStatusRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := validation.New().Struct(&req); err != nil {
		return err
	}

	actor := middleware.FromContext(c)
	if err := h.svc.UpdateKYCStatus(c.Request().Context(), actor.UserID, targetID, domain.KYCStatus(req.KYCStatus)); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) updateProfile(c *echo.Context) error {
	var req updateProfileRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := validation.New().Struct(&req); err != nil {
		return err
	}

	actor := middleware.FromContext(c)
	if err := h.svc.UpdateProfile(c.Request().Context(), actor.UserID, req.FirstName, req.LastName, req.Nickname); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// parseUserID menerjemahkan path param :id menjadi UUID; gagal = 422 dengan
// daftar field, bukan 400 polos, supaya bentuknya konsisten dengan validasi body.
func parseUserID(c *echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.UUID{}, apperror.Validation("validation failed",
			apperror.Field("id", "id must be a valid UUID"))
	}
	return id, nil
}
