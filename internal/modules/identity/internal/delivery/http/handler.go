package http

import (
	"net/http"
	"uuid"

	"github.com/labstack/echo/v5"

	"gepay/internal/modules/identity/api/domain"
	"gepay/internal/modules/identity/api/middleware"
	"gepay/platform/apperror"
	"gepay/platform/response"
	"gepay/platform/server/validation"
)

type Handler struct {
	svc domain.Service
}

func NewHandler(svc domain.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterUserRoutes mendaftarkan route untuk user biasa (hanya butuh login).
func (h *Handler) RegisterUserRoutes(g *echo.Group) {
	g.PUT("/users/me/profile", h.updateProfile)
	g.GET("/users/me", h.me)
}

type updateProfileRequest struct {
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Nickname  string `json:"nickname" validate:"required,max=50"`
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

func (h *Handler) me(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK(middleware.FromContext(c)))
}

// parseUserID menerjemahkan path param :id menjadi UUID; gagal = 422 dengan
// daftar field, bukan 400 polos, supaya bentuknya konsisten dengan validasi body.
func parseUserID(c *echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.UUID{}, apperror.Validation("validation failed",
			response.Field("id", "id must be a valid UUID"))
	}
	return id, nil
}
