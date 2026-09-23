package server

import (
	"gepay/platform/server/middleware"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

// Timeout default HTTP server — nilai konservatif yang aman untuk API JSON.
//
//	ReadHeaderTimeout : melindungi dari slowloris saat mengirim header.
//	ReadTimeout       : batas total baca request (header + body).
//	WriteTimeout      : batas total tulis response.
//	IdleTimeout       : batas keep-alive connection menganggur.
//
// Kalau nanti ada endpoint streaming/SSE/upload besar, timeout-nya harus
// disesuaikan (mis. lewat route khusus) — jangan dihapus begitu saja.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// New membuat Echo dengan error handler & middleware standar terpasang.
//
// Urutan middleware PENTING:
// RequestID → Logger (menyisipkan logger ke context) → Recover (memakai
// logger tersebut untuk mencatat panic).
func New(base *slog.Logger) *echo.Echo {
	if base == nil {
		base = slog.Default()
	}

	e := echo.NewWithConfig(echo.Config{
		Logger:           base,
		HTTPErrorHandler: middleware.ErrorHandler,
	})

	// global middleware
	e.Use(
		middleware.RequestIDMiddleware(),
		middleware.LoggerMiddleware(base),
		middleware.RecoverMiddleware())

	return e
}

func NewHttpServer(e *echo.Echo, addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
