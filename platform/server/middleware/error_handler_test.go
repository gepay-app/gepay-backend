package middleware

import (
	"encoding/json"
	"errors"
	"gepay/platform/apperror"
	"gepay/platform/logger"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorHandler memverifikasi ErrorHandler dengan 3 cabang:
//  1. *apperror.Error → {status, message, errors?} (5xx disanitasi)
//  2. error Echo yang membawa status → {status, message}
//  3. error lain → 500 generik + di-log
func TestErrorHandler(t *testing.T) {
	cases := map[string]struct {
		handler      func(c *echo.Context) error
		wantStatus   int
		wantMessage  string
		wantErrors   []apperror.FieldError
		wantNoErrors bool
		wantLogMsg   string // jika diisi, pastikan log dengan message ini tercatat
	}{
		"apperror 404": {
			handler: func(c *echo.Context) error {
				return apperror.NotFound("product not found")
			},
			wantStatus:   http.StatusNotFound,
			wantMessage:  "product not found",
			wantNoErrors: true,
		},
		"apperror 5xx disanitasi": {
			handler: func(c *echo.Context) error {
				return apperror.Internal("failed to connect to postgres").
					WithCause(errors.New("connection refused"))
			},
			wantStatus:   http.StatusInternalServerError,
			wantMessage:  http.StatusText(http.StatusInternalServerError),
			wantNoErrors: true,
			wantLogMsg:   "internal error",
		},
		"apperror validasi dengan errors": {
			handler: func(c *echo.Context) error {
				return apperror.Validation("validation failed",
					apperror.Field("email", "email must be a valid email address"),
					apperror.Field("name", "name is required"),
				)
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "validation failed",
			wantErrors: []apperror.FieldError{
				{Field: "email", Message: "email must be a valid email address"},
				{Field: "name", Message: "name is required"},
			},
		},
		"echo http error": {
			handler: func(c *echo.Context) error {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
			},
			wantStatus:   http.StatusBadRequest,
			wantMessage:  "invalid request body",
			wantNoErrors: true,
		},
		"echo http error code 0": {
			handler: func(c *echo.Context) error {
				return echo.NewHTTPError(0, "no code")
			},
			wantStatus:  http.StatusInternalServerError,
			wantMessage: http.StatusText(http.StatusInternalServerError),
		},
		"echo http error message kosong": {
			handler: func(c *echo.Context) error {
				return echo.NewHTTPError(http.StatusBadRequest, "")
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: http.StatusText(http.StatusBadRequest),
		},
		"echo not found (ErrNotFound)": {
			handler: func(c *echo.Context) error {
				return echo.ErrNotFound
			},
			wantStatus:  http.StatusNotFound,
			wantMessage: http.StatusText(http.StatusNotFound),
		},
		"unknown error": {
			handler: func(c *echo.Context) error {
				return errors.New("something went very wrong")
			},
			wantStatus:  http.StatusInternalServerError,
			wantMessage: http.StatusText(http.StatusInternalServerError),
			wantLogMsg:  "unhandled internal error",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := echo.New()
			rh := newRecordHandler()
			base := slog.New(rh)

			e.GET("/test", func(c *echo.Context) error {
				ctx := logger.WithContext(c.Request().Context(), base)
				c.SetRequest(c.Request().WithContext(ctx))
				return tc.handler(c)
			})
			e.HTTPErrorHandler = ErrorHandler

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			wantStatus := tc.wantStatus

			assert.Equal(t, wantStatus, rec.Code, "status code")

			var resp apperror.Response
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, wantStatus, resp.Status, "response.status")
			assert.Equal(t, tc.wantMessage, resp.Message, "response.message")

			if tc.wantErrors != nil {
				assert.Equal(t, tc.wantErrors, resp.Errors, "response.errors")
			}
			if tc.wantNoErrors {
				assert.Empty(t, resp.Errors, "errors harus kosong")
				assert.NotContains(t, rec.Body.String(), `"errors"`, "errors harus di-omit")
			}

			if tc.wantLogMsg != "" {
				assert.True(t, hasLogMessage(rh, tc.wantLogMsg), "log %q harus tercatat", tc.wantLogMsg)
			}
		})
	}
}

// TestErrorHandlerRetryAfter memastikan 429 menyertakan header Retry-After.
func TestErrorHandlerRetryAfter(t *testing.T) {
	e := echo.New()
	base := slog.New(newRecordHandler())

	e.GET("/test", func(c *echo.Context) error {
		ctx := logger.WithContext(c.Request().Context(), base)
		c.SetRequest(c.Request().WithContext(ctx))
		return apperror.TooManyRequests("too many login attempts").WithRetryAfter(60)
	})
	e.HTTPErrorHandler = ErrorHandler

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get(echo.HeaderRetryAfter))

	var resp apperror.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "too many login attempts", resp.Message)
}

func hasLogMessage(rh *recordHandler, msg string) bool {
	rh.state.mu.Lock()
	defer rh.state.mu.Unlock()
	for _, r := range rh.state.records {
		if r.Message == msg {
			return true
		}
	}
	return false
}
