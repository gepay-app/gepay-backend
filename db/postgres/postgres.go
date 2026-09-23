// Package postgres membuat dan memverifikasi connection pool PostgreSQL
// (pgx/v5).
//
// Package ini TIDAK memanggil os.Exit(): kegagalan dikembalikan sebagai error
// dan keputusan "berhenti atau lanjut" ada di composition root
// (internal/bootstrap). Ini membuat package infrastruktur tetap bisa dites.
package postgres

import (
	"context"
	"fmt"
	"gepay/platform/config"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pingTimeout membatasi berapa lama startup menunggu DB merespons.
const pingTimeout = 5 * time.Second

// New membuat pool pgx dan memastikan koneksinya benar-benar hidup (Ping).
// Pool hanya dikembalikan saat sudah lolos Ping, sehingga caller tidak pernah
// memegang pool yang tidak bisa dipakai.
func New(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Override tuning pool dari config (0 = pakai default pgxpool).
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}
	if cfg.ConnMaxIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
