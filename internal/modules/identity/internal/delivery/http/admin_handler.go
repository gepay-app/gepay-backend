package http

import (
	"gepay/internal/modules/identity/api/domain"
	"gepay/internal/modules/identity/api/middleware"
	"gepay/platform/apperror"
	"gepay/platform/server/validation"
	"net/http"

	"github.com/labstack/echo/v5"
)

// RegisterRoutes hanya mendaftarkan PATH + bentuk request/response + gate role
// per-endpoint. RequireAuth dipasang sekali di level group oleh module.go.
//
// Gate role di sini sengaja dicocokkan dengan aturan di service (defense in
// depth): promote = SUPER_ADMIN saja, ubah KYC = ADMIN atau SUPER_ADMIN.
func (h *Handler) RegisterAdminRoutes(g *echo.Group) {
	g.PUT("/users/:id/role", h.updateUserRole, middleware.RequireRole(domain.RoleSuperAdmin))
	g.PUT("/users/:id/kyc", h.updateKYCStatus, middleware.RequireRole(domain.RoleAdmin, domain.RoleSuperAdmin))
}

type updateUserRoleRequest struct {
	// Hanya ADMIN: promosi ke SUPER_ADMIN sengaja tidak lewat HTTP.
	Role string `json:"role" validate:"required,oneof=ADMIN"`
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

type updateKYCStatusRequest struct {
	KYCStatus string `json:"kyc_status" validate:"required,oneof=UNVERIFIED PENDING VERIFIED REJECTED"`
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
