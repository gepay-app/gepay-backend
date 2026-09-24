// Package bootstrap adalah composition root aplikasi: satu-satunya tempat
// yang tahu bagaimana seluruh dependency dirakit, urutan startup, dan
// lifecycle-nya.
//
// Aturan penting:
//   - Package ini boleh meng-import semua module & platform.
//   - Package lain TIDAK boleh meng-import bootstrap (tidak ada siklus).
//   - Tidak ada os.Exit() di sini: Run() mengembalikan error, dan
//     cmd/app/main.go yang menerjemahkannya menjadi exit code. Dengan begitu
//     seluruh defer cleanup tetap berjalan.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"gepay/db/postgres"
	"gepay/db/redis"
	"gepay/internal/modules/identity"
	"gepay/platform/config"
	"gepay/platform/logger"
	"gepay/platform/server"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
)

// shutdownTimeout membatasi berapa lama server diberi kesempatan
// menyelesaikan request yang sedang berjalan sebelum dipaksa berhenti.
const (
	shutdownTimeout = 10 * time.Second
	lbDrainDelay    = 5 * time.Second
)

// Run merakit seluruh aplikasi lalu menjalankannya sampai menerima sinyal
// SIGINT/SIGTERM (atau sampai server gagal listen).
//
// Urutan startup:
//  1. load .env (opsional, hanya untuk dev)
//  2. baca + validasi config
//  3. bangun logger
//  4. koneksi Postgres & Redis (fail-fast)
//  5. rakit module (kalau ada) di sini
//  6. pasang route & jalankan HTTP server
//  7. tunggu sinyal → graceful shutdown → tutup koneksi
func Run() error {
	if err := loadDotenv(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	base, err := logger.FromConfig(cfg.Environment, cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	slog.SetDefault(base)

	// ctx dibatalkan otomatis saat SIGINT/SIGTERM diterima.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop() // reset notify supaya signal berikutnya balik ke default handler (langsung kill)
	}()

	pg, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pg.Close()

	rdb, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			base.Error("failed to close redis", slog.Any("error", err))
		}
	}()

	base.Info("infrastructure ready",
		slog.String("app", cfg.Name),
		slog.String("env", cfg.Environment),
		slog.String("db_host", pg.Config().ConnConfig.Host),
		slog.String("redis_addr", rdb.Options().Addr),
	)

	e := server.New(base)

	// Composition root: rakit module DI SINI dan suntikkan dependency-nya.
	// Module lain yang butuh login menerima identityModule.Middleware dari sini,
	identityModule, err := identity.New(ctx, &cfg.Firebase, pg, rdb)
	if err != nil {
		return fmt.Errorf("init identity module: %w", err)
	}

	api := e.Group("/api/v1")
	identityModule.RegisterRoutes(api)

	var shuttingDown atomic.Bool

	e.GET("/health", func(c *echo.Context) error {
		if shuttingDown.Load() {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "shutting_down"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.NewHttpServer(e, addr)

	// Server jalan di goroutine terpisah supaya goroutine utama bisa
	// menunggu sinyal shutdown. Buffer 1 mencegah goroutine bocor kalau
	// tidak ada yang membaca channel setelah shutdown.
	serverErr := make(chan error, 1)
	go func() {
		base.Info("server starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		base.Info("shutdown signal received")
		shuttingDown.Store(true)
		if cfg.Environment == "production" || cfg.Environment == "prod" {
			time.Sleep(lbDrainDelay) // beri waktu LB/k8s deregister endpoint
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	base.Info("graceful shutdown complete")
	return nil
}

// loadDotenv memuat file .env untuk pengembangan lokal.
//
// File .env yang TIDAK ADA bukan error: di production env disuntik oleh
// orchestrator (Docker/Kubernetes), jadi aplikasi harus tetap start. Yang
// error hanyalah file .env yang ada tapi tidak bisa dibaca/di-parse.
func loadDotenv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}
