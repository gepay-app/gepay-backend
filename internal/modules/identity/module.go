// Package identity adalah module pertama: verifikasi Firebase ID token,
// auto-provision user saat first login, dan authz berbasis role/KYC.
//
// Struktur module (lihat AGENTS.md):
//   - api/      → kontrak PUBLIK; module lain hanya boleh meng-import ini.
//   - internal/ → implementasi PRIVAT; compiler menolak import dari luar module.
//   - module.go → facade + wiring lokal; HANYA bootstrap yang meng-import ini.
package identity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"

	"gepay/internal/modules/identity/api/domain"
	"gepay/internal/modules/identity/api/middleware"
	"gepay/internal/modules/identity/internal/cache"
	identityhttp "gepay/internal/modules/identity/internal/delivery/http"
	"gepay/internal/modules/identity/internal/provider/firebase"
	"gepay/internal/modules/identity/internal/repository"
	"gepay/internal/modules/identity/internal/service"
	"gepay/platform/config"
)

type Module struct {
	// Service dipakai module lain lewat interface domain.IdentityService.
	Service domain.IdentityService
	// Middleware kontrak HTTP publik (tipe-nya di api/middleware) supaya module
	// lain tidak perlu meng-import facade ini.
	Middleware middleware.Auth

	handler *identityhttp.Handler
}

// New merakit seluruh dependency module identity. Error dari provider Firebase
// dikembalikan (bukan di-panic) supaya bootstrap bisa fail-fast saat startup.
func New(ctx context.Context, cfg *config.Firebase, db *pgxpool.Pool, rdb *redis.Client) (*Module, error) {
	provider, err := firebase.New(ctx, cfg.ServiceAccountBase64)
	if err != nil {
		return nil, fmt.Errorf("identity: init firebase provider: %w", err)
	}

	svc := service.New(provider, repository.New(db), cache.New(rdb))

	return &Module{
		Service:    svc,
		Middleware: middleware.NewAuth(svc),
		handler:    identityhttp.NewHandler(svc),
	}, nil
}

// RegisterRoutes mendaftarkan route milik identity pada group yang sudah
// berprefix (biasanya /api/v1). Endpoint admin dikelompokkan di /admin dan
// wajib login; gate role per-endpoint ada di handler supaya sama persis dengan
// aturan service.
func (m *Module) RegisterRoutes(g *echo.Group) {
	admin := g.Group("/admin", m.Middleware.RequireAuth)
	m.handler.RegisterAdminRoutes(admin)

	// User endpoints (hanya butuh login, bukan admin)
	user := g.Group("", m.Middleware.RequireAuth)
	m.handler.RegisterUserRoutes(user)
}
