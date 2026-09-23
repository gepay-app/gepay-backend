package middleware

import (
	"errors"
	"fmt"
	"gepay/platform/apperror"
	"gepay/platform/logger"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ErrorHandler adalah HTTPErrorHandler Echo: SATU tempat yang mengubah semua
// error menjadi response JSON. Handler cukup `return err` polos.
//
// Tiga cabang:
//  1. *apperror.Error → {status, message, errors?}. Detail 5xx diganti pesan
//     generik supaya cause internal (SQL, koneksi) tidak bocor ke client.
//  2. Error Echo yang membawa status (mis. 404/405) → {status, message}.
//  3. Error lain → 500 generik + dicatat sebagai "unhandled internal error".
//
// Soal logging: 4xx TIDAK dicatat di sini — LoggerMiddleware sudah mencatatnya
// sebagai access log level Warn. Yang perlu detail tambahan hanyalah 5xx
// (cause error internal), dan itu dicatat di sini.
func ErrorHandler(c *echo.Context, err error) {
	ctx := c.Request().Context()
	l := logger.FromContext(ctx)

	if ce, ok := apperror.As(err); ok {
		// Status 0 berarti konstruktor dipakai tanpa status; perlakukan sebagai
		// 500 supaya tidak pernah mengirim response tanpa status valid.
		if ce.Status == 0 {
			ce.Status = http.StatusInternalServerError
		}

		if ce.Status >= 500 {
			attrs := []any{
				slog.Int("status", ce.Status),
				slog.String("message", ce.Message),
			}
			if cause := ce.Cause(); cause != nil {
				attrs = append(attrs, slog.Any("cause", cause))
			}
			l.ErrorContext(ctx, "internal error", attrs...)

			// Pesan 5xx diganti generik; daftar field tidak dibocorkan.
			writeError(c, apperror.Response{
				Status:  ce.Status,
				Message: http.StatusText(http.StatusInternalServerError),
			})
			return
		}

		// 429 (rate limit) butuh header Retry-After supaya client tahu kapan
		// boleh mencoba lagi.
		if ce.RetryAfter > 0 {
			c.Response().Header().Set(echo.HeaderRetryAfter, fmt.Sprint(ce.RetryAfter))
		}

		writeError(c, ce.Response())
		return
	}

	// echo.StatusCode menangkap SEMUA error Echo yang membawa status code —
	// termasuk *echo.HTTPError dan error eksport-tidak-langsung seperti
	// echo.ErrNotFound.
	if status := echo.StatusCode(err); status != 0 {
		message := http.StatusText(status)
		var he *echo.HTTPError
		if errors.As(err, &he) && fmt.Sprint(he.Message) != "" {
			message = fmt.Sprint(he.Message)
		}

		if status >= 500 {
			l.ErrorContext(ctx, "http error", slog.Int("status", status), slog.Any("error", err))
			message = http.StatusText(http.StatusInternalServerError)
		}

		writeError(c, apperror.Response{Status: status, Message: message})
		return
	}

	l.ErrorContext(ctx, "unhandled internal error", slog.Any("error", err))
	writeError(c, apperror.Response{
		Status:  http.StatusInternalServerError,
		Message: http.StatusText(http.StatusInternalServerError),
	})
}

// writeError menulis response error yang konsisten:
//
//	{ "status": ..., "message": ..., "errors": [...] }
func writeError(c *echo.Context, resp apperror.Response) {
	if err := c.JSON(resp.Status, resp); err != nil {
		logger.FromContext(c.Request().Context()).
			ErrorContext(
				c.Request().Context(),
				"failed to write error response",
				slog.Any("error", err),
			)
	}
}
