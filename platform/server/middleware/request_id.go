package middleware

import (
	"context"
	"strings"
	"uuid"

	"github.com/labstack/echo/v5"
)

type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

// RequestIDHeader adalah nama header request ID, dipakai konsisten di semua
// tempat (middleware, test, dokumentasi).
const RequestIDHeader = "X-Request-ID"

// maxRequestIDLength membatasi panjang request ID yang diterima dari luar.
// Tanpa batas, header raksasa bisa masuk ke setiap baris log (log flooding)
// dan membebani storage log.
const maxRequestIDLength = 128

// RequestIDMiddleware memastikan setiap request punya Request ID untuk
// tracing lintas log & service.
//
// Aturan:
//   - Pakai UUIDv7 (bukan v4) — punya komponen timestamp di depan, sortable
//     by waktu, ramah index & investigasi log berurutan.
//   - Kalau caller (mis. BFF/gateway) sudah mengirim header X-Request-ID yang
//     wajar, pakai itu supaya trace end-to-end nyambung. Kalau kosong atau
//     tidak wajar, generate baru.
//   - Request ID ditaruh di header response yang sama dan disisipkan ke
//     context.Context untuk dipakai logger, error handler, dan trace.
func RequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqID := sanitizeRequestID(c.Request().Header.Get(RequestIDHeader))
			if reqID == "" {
				reqID = uuid.NewV7().String()
			}

			c.Response().Header().Set(RequestIDHeader, reqID)

			ctx := context.WithValue(c.Request().Context(), ctxKeyRequestID, reqID)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// RequestIDFromContext mengambil Request ID dari context (dipakai logger,
// error handling, dsb). Kosong bila tidak ada.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// sanitizeRequestID menerima ID dari luar hanya kalau bentuknya wajar:
// tidak kosong, tidak kepanjangan, dan tanpa karakter kontrol. ID yang tidak
// lolos akan diganti ID baru oleh middleware.
func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > maxRequestIDLength {
		return ""
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return id
}
