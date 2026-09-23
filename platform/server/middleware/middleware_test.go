package middleware

import (
	"context"
	"gepay/platform/logger"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordHandler menangkap semua log record untuk di-assert di test.
type shareState struct {
	mu      sync.Mutex
	records []slog.Record
}

type recordHandler struct {
	state *shareState
}

func newRecordHandler() *recordHandler {
	return &recordHandler{state: &shareState{}}
}

func (h *recordHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	h.state.records = append(h.state.records, r)
	return nil
}

func (h *recordHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(_ string) slog.Handler { return h }

func (h *recordHandler) lastMsg() string {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	if len(h.state.records) == 0 {
		return ""
	}
	return h.state.records[len(h.state.records)-1].Message
}

func (h *recordHandler) allMsgs() []string {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	msgs := make([]string, len(h.state.records))
	for i, r := range h.state.records {
		msgs[i] = r.Message
	}
	return msgs
}

// TestRequestChain membuktikan alur request ID + logger request-scoped:
// request ID dibuat, logger masuk ke context, dan "request completed" tercatat.
func TestRequestChain(t *testing.T) {
	e := echo.New()
	h := newRecordHandler()
	base := slog.New(h)

	e.Use(RequestIDMiddleware(), LoggerMiddleware(base))

	e.GET("/ping", func(c *echo.Context) error {
		l := logger.FromContext(c.Request().Context())
		l.InfoContext(c.Request().Context(), "inside handler")
		assert.NotEmpty(t, RequestIDFromContext(c.Request().Context()), "request id ada di context")
		return c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(RequestIDHeader), "X-Request-ID response header")
	assert.Equal(t, "request completed", h.lastMsg())
}

// TestRequestIDHonorHeader memastikan request ID dari caller dipakai apa
// adanya supaya tracing end-to-end nyambung.
func TestRequestIDHonorHeader(t *testing.T) {
	e := echo.New()
	e.Use(RequestIDMiddleware())
	e.GET("/", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "trace-from-gateway-123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, "trace-from-gateway-123", rec.Header().Get(RequestIDHeader))
}

// TestRequestIDGenerated memastikan tanpa header, request ID tetap dibuat dan
// konsisten antara header response & context.
func TestRequestIDGenerated(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	e.Use(RequestIDMiddleware())
	e.GET("/", func(c *echo.Context) error {
		assert.NotEmpty(t, RequestIDFromContext(c.Request().Context()))
		assert.Equal(t, rec.Header().Get(RequestIDHeader), RequestIDFromContext(c.Request().Context()))
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	e.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get(RequestIDHeader))
}

// TestRequestIDRejectsUnsafeHeader memastikan header dari luar yang tidak wajar
// (kepanjangan / mengandung karakter kontrol) tidak dipakai apa adanya.
func TestRequestIDRejectsUnsafeHeader(t *testing.T) {
	cases := map[string]string{
		"kepanjangan":        strings.Repeat("a", maxRequestIDLength+1),
		"mengandung newline": "trace\ninjected",
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			e := echo.New()
			e.Use(RequestIDMiddleware())
			e.GET("/", func(c *echo.Context) error {
				assert.NotEmpty(t, RequestIDFromContext(c.Request().Context()))
				return c.NoContent(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(RequestIDHeader, header)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			got := rec.Header().Get(RequestIDHeader)
			assert.NotEqual(t, header, got, "header tidak wajar harus diganti ID baru")
			assert.NotEmpty(t, got)
		})
	}
}

// TestLoggerFromContextFallback: tanpa middleware, helper harus tetap aman
// (fallback ke slog.Default), tidak panic.
func TestLoggerFromContextFallback(t *testing.T) {
	l := logger.FromContext(context.Background())
	assert.NotNil(t, l, "fallback logger")
	assert.Empty(t, RequestIDFromContext(context.Background()))
}

// TestSensitiveRedaction membuktikan key sensitif di-sensor oleh ReplaceAttr.
func TestSensitiveRedaction(t *testing.T) {
	var buf strings.Builder
	base := logger.NewWriter(&buf, logger.DefaultConfig())

	base.Info("login attempt", slog.String("password", "supersecret"))

	out := buf.String()
	assert.NotContains(t, out, "supersecret", "password plaintext bocor ke log")
	assert.Contains(t, out, `"***"`, "nilai tidak di-sensor")
}

// TestLoggerStatus memastikan LoggerMiddleware mencatat status HTTP yang benar
// (lewat echo.ResolveResponseStatus) saat handler error.
func TestLoggerStatus(t *testing.T) {
	e := echo.New()
	h := newRecordHandler()
	base := slog.New(h)

	e.Use(RequestIDMiddleware(), LoggerMiddleware(base))
	e.GET("/fail", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "I'm a teapot")
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Contains(t, h.allMsgs(), "request failed")
}

// TestRecoverMiddleware memastikan panic ditangkap, dicatat beserta stack, dan
// diterjemahkan menjadi error 500 yang aman.
func TestRecoverMiddleware(t *testing.T) {
	e := echo.New()
	h := newRecordHandler()
	base := slog.New(h)

	e.Use(RequestIDMiddleware(), LoggerMiddleware(base), RecoverMiddleware())
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/boom", func(c *echo.Context) error {
		panic("boom rahasia")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, h.allMsgs(), "panic recovered")
	assert.NotContains(t, rec.Body.String(), "boom rahasia", "detail panic bocor ke client")
}
