// Package redis membuat dan memverifikasi koneksi Redis (go-redis/v9).
//
// Seperti db/postgres, package ini mengembalikan error alih-alih memanggil
// os.Exit(). Client hanya dikembalikan setelah Ping berhasil.
package redis

import (
	"context"
	"fmt"
	"gepay/platform/config"
	"time"

	"github.com/redis/go-redis/v9"
)

// pingTimeout membatasi berapa lama startup menunggu Redis merespons.
const pingTimeout = 5 * time.Second

// New membuat client Redis dan memastikan koneksinya hidup.
func New(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		// Jangan tinggalkan client setengah jadi kalau gagal.
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return rdb, nil
}
