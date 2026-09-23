package middleware

import (
	"gepay/platform/logger"
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"
)

// LoggerMiddleware membuat logger request-scoped dan menyisipkannya ke
// context, lalu mencatat durasi & status request saat selesai.
//
// INILAH SATU-SATUNYA TEMPAT YANG "MENULIS" logger ke context (sisi writer).
// Service/repository TIDAK menerima logger lewat constructor — mereka
// "membaca" lewat logger.FromContext(ctx) (lihat platform/logger/logger.go).
//
// Kenapa begini? Field log (request_id, method, path) baru bernilai SETELAH
// request masuk, jadi mustahil di-set saat service dibuat sekali di
// composition root. Context yang mengalir bersama request-lah yang membawa
// logger yang sudah "tahu" konteksnya.
//
// Alur:
//  1. Ambil Request ID yang disisipkan RequestIDMiddleware.
//  2. Buat logger baru dari base dengan field kontekstual.
//  3. Simpan ke context supaya handler/service pakai logger.FromContext(ctx).
//  4. Catat "request completed"/"request failed" dengan status + durasi.
//
// PASTIKAN RequestIDMiddleware dipasang SEBELUM middleware ini.
func LoggerMiddleware(base *slog.Logger) echo.MiddlewareFunc {
	if base == nil {
		base = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			// logger request-scoped berisi field yang selalu relevan.
			cLogger := base.With(
				slog.String("request_id", RequestIDFromContext(ctx)),
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
			)

			ctx = logger.WithContext(ctx, cLogger)
			c.SetRequest(c.Request().WithContext(ctx))

			start := time.Now()
			err := next(c)

			// ResolveResponseStatus: ambil status HTTP aktual dari response
			// (memperhitungkan error yang implement echo.StatusCoder, dsb).
			_, status := echo.ResolveResponseStatus(c.Response(), err)

			attrs := []any{
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)),
			}
			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
			}

			// Satu baris per request (access log). Level mengikuti status:
			// 5xx = kegagalan server (Error), 4xx = kesalahan client (Warn),
			// sisanya Info. Detail cause 5xx ditambahkan ErrorHandler.
			switch {
			case status >= 500:
				cLogger.ErrorContext(ctx, "request failed", attrs...)
			case status >= 400:
				cLogger.WarnContext(ctx, "request failed", attrs...)
			default:
				cLogger.InfoContext(ctx, "request completed", attrs...)
			}
			return err
		}
	}
}
