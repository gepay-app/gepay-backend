// Package middleware adalah kontrak HTTP PUBLIK module identity: verifikasi
// bearer token + pembatasan role.
//
// Module lain boleh meng-import package ini (biasanya lewat bootstrap yang
// menyuntikkan Auth), tetapi TIDAK boleh meng-import `internal/` module ini.
package middleware

import (
	"context"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"

	"gepay/internal/modules/identity/api/domain"
	"gepay/platform/apperror"
)

// ctxAuthKey adalah key context bertipe (unexported) supaya module lain tidak
// bisa menebak atau menimpa nilai auth — pola yang sama dipakai platform/logger.
type ctxAuthKey struct{}

// Auth adalah sekumpulan middleware auth yang diekspos module identity. Dibuat
// sekali di module.go lalu disuntikkan bootstrap ke module lain.
type Auth struct {
	RequireAuth echo.MiddlewareFunc
	RequireRole func(roles ...domain.Role) echo.MiddlewareFunc
	FromContext func(c *echo.Context) *domain.AuthContext
}

// NewAuth merakit kontrak middleware dari service identity.
func NewAuth(svc domain.Service) Auth {
	return Auth{
		RequireAuth: RequireAuth(svc),
		RequireRole: RequireRole,
		FromContext: FromContext,
	}
}

// RequireAuth memverifikasi bearer token lalu menyimpan *domain.AuthContext ke
// context request. Route yang butuh login WAJIB memakai ini lebih dulu;
// RequireRole hanya membaca hasilnya.
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
			if authCtx.Status == domain.StatusSuspended {
				return apperror.Forbidden("account suspended")
			}

			ctx := context.WithValue(c.Request().Context(), ctxAuthKey{}, authCtx)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// RequireRole membatasi route ke role tertentu. Dipasang SETELAH RequireAuth:
// request yang belum terautentikasi dibalas 401, bukan 403.
func RequireRole(roles ...domain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			authCtx, ok := authContext(c)
			if !ok {
				return apperror.Unauthorized("missing authorization token")
			}
			if slices.Contains(roles, authCtx.Role) {
				return next(c)
			}
			return apperror.Forbidden("insufficient role")
		}
	}
}

// FromContext dipakai module LAIN (mis. donation) untuk membaca AuthContext —
// bukan lewat repository/service identity. Request anonim mengembalikan
// AuthContext{Role: RoleAnonymous} supaya pemanggil tidak perlu nil-check.
func FromContext(c *echo.Context) *domain.AuthContext {
	if authCtx, ok := authContext(c); ok {
		return authCtx
	}
	return &domain.AuthContext{Role: domain.RoleAnonymous}
}

func authContext(c *echo.Context) (*domain.AuthContext, bool) {
	authCtx, ok := c.Request().Context().Value(ctxAuthKey{}).(*domain.AuthContext)
	return authCtx, ok && authCtx != nil
}

// extractBearerToken membaca header Authorization dengan skema "Bearer"
// case-insensitive (RFC 9110: nama skema auth tidak case-sensitive).
func extractBearerToken(c *echo.Context) string {
	h := c.Request().Header.Get(echo.HeaderAuthorization)
	const prefix = "bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
