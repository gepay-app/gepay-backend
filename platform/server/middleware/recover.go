package middleware

import (
	"gepay/platform/apperror"
	"gepay/platform/logger"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v5"
)

// RecoverMiddleware menangkap panic di handler/middleware setelahnya,
// mencatatnya beserta stack trace, lalu mengubahnya menjadi error 500 yang
// aman. Tanpa ini, panic membuat koneksi mati tanpa response yang jelas
// dan tanpa jejak log yang rapi.
//
// WAJIB dipasang SETELAH RequestIDMiddleware & LoggerMiddleware supaya log
// panic otomatis membawa request_id/method/path.
func RecoverMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}

				// http.ErrAbortHandler adalah sinyal internal net/http supaya
				// koneksi dibatalkan — jangan ditelan.
				if r == http.ErrAbortHandler {
					panic(r)
				}

				ctx := c.Request().Context()
				logger.FromContext(ctx).ErrorContext(ctx, "panic recovered",
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)

				// Detail 5xx otomatis disanitasi oleh ErrorHandler sebelum
				// dikirim ke client.
				err = apperror.Internal("internal server error")
			}()

			return next(c)
		}
	}
}
