package http

import (
	"gepay/internal/modules/identity/domain"
	"gepay/platform/apperror"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"
)

const ctxKeyAuth = "auth_ctx"

func RequireAuth(svc domain.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token := extractBearerToken(c)
			if token == "" {
				return apperror.Unauthorized("missing authorization token")
			}

			authCtx, err := svc.ResolveAuthContext(c.Request().Context(), token)
			if err != nil {
				return err
			}
			switch authCtx.Status {
			case domain.StatusSuspended:
				return apperror.Forbidden("account suspended")
			}
			c.Set(ctxKeyAuth, authCtx)
			return next(c)
		}
	}
}

// OptionalAuth dipakai di endpoint yang boleh anonim, mis. donasi.
//func OptionalAuth(svc domain.Service) echo.MiddlewareFunc {
//	return func(next echo.HandlerFunc) echo.HandlerFunc {
//		return func(c *echo.Context) error {
//			token := extractBearerToken(c)
//			if token == "" {
//				c.Set(ctxKeyAuth, &domain.AuthContext{Role: domain.RoleAnonymous})
//				return next(c)
//			}
//
//			authCtx, err := svc.ResolveAuthContext(c.Request().Context(), token)
//			if err != nil {
//				return err
//			}
//			c.Set(ctxKeyAuth, authCtx)
//			return next(c)
//		}
//	}
//}

func RequireRole(roles ...domain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			authCtx := FromContext(c)
			if slices.Contains(roles, authCtx.Role) {
				return next(c)
			}
			return apperror.Forbidden("insufficient role")
		}
	}
}

// FromContext dipakai module LAIN (mis. donation) untuk baca AuthContext — bukan repository/service identity langsung.
func FromContext(c *echo.Context) *domain.AuthContext {
	authCtx, ok := c.Get(ctxKeyAuth).(*domain.AuthContext)
	if !ok || authCtx == nil {
		return &domain.AuthContext{Role: domain.RoleAnonymous}
	}
	return authCtx
}

func extractBearerToken(ctx *echo.Context) string {
	h := ctx.Request().Header.Get(echo.HeaderAuthorization)
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
