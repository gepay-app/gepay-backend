package cache

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"gepay/internal/modules/identity/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 15 * time.Minute

type IdentityCache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) domain.AuthContextCache {
	return &IdentityCache{rdb: rdb}
}

// key satu-satunya tempat format key didefinisikan → tidak mungkin typo
// beda antara Get/Set/Evict karena semua panggil fungsi yang sama.
func (c *IdentityCache) key(id string) string {
	return fmt.Sprintf("identity:authctx:%d", id)
}

func (i *IdentityCache) GetByProviderID(ctx context.Context, id string) (*domain.AuthContext, error) {
	raw, err := i.rdb.Get(ctx, i.key(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("identity cache: %w", err)
	}

	var authContext domain.AuthContext
	if err := json.Unmarshal(raw, &authContext); err != nil {
		return nil, fmt.Errorf("unmarshal user cache: %w", err)
	}

	return &authContext, nil
}

func (i *IdentityCache) SetByProviderID(ctx context.Context, id string, u *domain.AuthContext) error {
	raw, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("marshal identity cache: %w", err)
	}

	if err := i.rdb.Set(ctx, i.key(id), raw, defaultTTL).Err(); err != nil {
		return fmt.Errorf("set identity cache: %w", err)
	}
	return nil
}

func (i *IdentityCache) EvictByProviderID(ctx context.Context, id string) error {
	if err := i.rdb.Del(ctx, i.key(id)).Err(); err != nil {
		return fmt.Errorf("delete identity cache: %w", err)
	}
	return nil
}
