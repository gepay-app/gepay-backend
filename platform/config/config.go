// Package config memuat seluruh konfigurasi aplikasi dari environment
// variables SEKALI saja di composition root (internal/bootstrap/app.go).
//
// Konvensi:
//   - Jangan ada os.Getenv() tersebar di package lain — semua lewat sini.
//   - Semua field memakai struct-tag caarlos0/env, sehingga mudah di-override
//     per environment (shell, docker compose, CI) dan mudah di-test.
//   - Sub-struct memakai envPrefix (DB_, REDIS_) supaya nama env-nya
//     berkelompok dan tidak bertabrakan.
//   - "Wajib diisi" divalidasi di Validate(), bukan di tag, supaya pesan
//     errornya jelas dan mudah dites.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Load membaca semua env ke struct App lalu memvalidasinya.
func Load() (*App, error) {
	var app App
	if err := env.Parse(&app); err != nil {
		return nil, err
	}
	if err := app.Validate(); err != nil {
		return nil, err
	}
	return &app, nil
}

// App adalah root config. Setiap sub-struct mewakili satu infrastruktur
// dan memakai envPrefix agar key-nya berkelompok.
type App struct {
	Environment string `env:"APP_ENV"  envDefault:"development"`
	Name        string `env:"APP_NAME" envDefault:"app"`
	Port        int    `env:"APP_PORT" envDefault:"8080"`
	// LogLevel: debug | info | warn | error (case-insensitive).
	// Kosong = otomatis: production → info, environment lain → debug.
	LogLevel string `env:"APP_LOG_LEVEL"`
	// LogFormat: json | text. Kosong = otomatis: production → json,
	// environment lain → text.
	LogFormat string `env:"APP_LOG_FORMAT"`

	Database Database `envPrefix:"DB_"`
	Redis    Redis    `envPrefix:"REDIS_"`
	Firebase Firebase `envPrefix:"FIREBASE_"`
}

// Firebase berisi kredensial service account untuk memverifikasi ID token
// Firebase Auth.
//
// Nilainya di-encode base64 supaya kredensial JSON (multi-baris, berisi private
// key) muat dalam SATU env var / secret manager entry — bukan file yang harus
// di-mount. Cara mengisinya:
//
//	make firebase-credentials   # base64 -w 0 secrets/service-account.json
type Firebase struct {
	ServiceAccountBase64 string `env:"SERVICE_ACCOUNT_BASE64"` // wajib
}

// Database berisi kredensial & tuning connection pool pgx.
type Database struct {
	URL string `env:"URL"` // wajib

	// MaxConns = jumlah maksimum koneksi di pool.
	MaxConns int32 `env:"MAX_CONNS" envDefault:"25"`
	// MinConns = jumlah minimum koneksi idle yang dipertahankan pool.
	// (pgxpool tidak punya konsep "max idle" — idle dibatasi MaxConns dan
	// dibuang setelah ConnMaxIdleTime.)
	MinConns int32 `env:"MIN_CONNS" envDefault:"2"`

	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME"  envDefault:"30m"`
	ConnMaxIdleTime time.Duration `env:"CONN_MAX_IDLE_TIME" envDefault:"10m"`
}

// Redis berisi koneksi ke cache / pub-sub / distributed lock.
// Redis WAJIB hidup saat startup; kalau belum dipakai module mana pun,
// tetap di-wire supaya module baru tinggal memakai tanpa mengubah bootstrap.
type Redis struct {
	Addr     string `env:"ADDR" envDefault:"localhost:6379"`
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`
	DB       int    `env:"DB"   envDefault:"0"`
}

// Validate memastikan nilai wajib terisi dan nilai berenumerasi valid.
// Sengaja dilakukan setelah env.Parse supaya pesan errornya bisa spesifik.
func (a *App) Validate() error {
	if strings.TrimSpace(a.Database.URL) == "" {
		return fmt.Errorf("DB_URL is required")
	}
	if a.Port <= 0 || a.Port > 65535 {
		return fmt.Errorf("APP_PORT must be between 1 and 65535, got %d", a.Port)
	}
	switch strings.ToLower(strings.TrimSpace(a.LogLevel)) {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("APP_LOG_LEVEL must be one of debug|info|warn|error (or empty for auto), got %q", a.LogLevel)
	}
	switch strings.ToLower(strings.TrimSpace(a.LogFormat)) {
	case "", "json", "text":
	default:
		return fmt.Errorf("APP_LOG_FORMAT must be one of json|text (or empty for auto), got %q", a.LogFormat)
	}
	if strings.TrimSpace(a.Firebase.ServiceAccountBase64) == "" {
		return fmt.Errorf("FIREBASE_SERVICE_ACCOUNT_BASE64 is required")
	}
	return nil
}
