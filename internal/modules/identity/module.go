package identity

import (
	"context"
	"gepay/internal/modules/identity/cache"
	"gepay/internal/modules/identity/domain"
	"gepay/internal/modules/identity/provider/firebase"
	"gepay/internal/modules/identity/repository"
	"gepay/internal/modules/identity/service"
	"gepay/platform/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

type Module struct {
	Service domain.Service
}

func New(cfg *config.App,db *pgxpool.Pool, rdb *redis.Client) *Module {
	authCache := cache.New(rdb)
	queries := repository.New(db)
	authProvider := firebase.New(context.Background(),cfg.)
	return &Module{
		Service: service.New(provider, queries, authCache),
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {

}
