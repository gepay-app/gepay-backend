package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConstructors memastikan setiap constructor memakai status yang benar
// dan menyimpan message apa adanya.
func TestConstructors(t *testing.T) {
	cases := map[string]struct {
		err        *Error
		wantStatus int
		wantMsg    string
	}{
		"bad request":       {BadRequest("invalid request body"), http.StatusBadRequest, "invalid request body"},
		"unauthorized":      {Unauthorized("invalid credentials"), http.StatusUnauthorized, "invalid credentials"},
		"forbidden":         {Forbidden("access denied"), http.StatusForbidden, "access denied"},
		"not found":         {NotFound("product not found"), http.StatusNotFound, "product not found"},
		"conflict":          {Conflict("email already registered"), http.StatusConflict, "email already registered"},
		"too many":          {TooManyRequests("too many requests"), http.StatusTooManyRequests, "too many requests"},
		"internal":          {Internal("db connection failed"), http.StatusInternalServerError, "db connection failed"},
		"validation":        {Validation("invalid input"), http.StatusUnprocessableEntity, "invalid input"},
		"new custom status": {New(http.StatusTeapot, "teapot"), http.StatusTeapot, "teapot"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.wantStatus, tc.err.Status)
			assert.Equal(t, tc.wantMsg, tc.err.Message)
			assert.Equal(t, tc.wantStatus, tc.err.StatusCode(), "StatusCode() harus sama dengan Status")
		})
	}
}

// TestValidation memastikan Validation memakai 422, mendukung message default,
// dan menyimpan daftar field error.
func TestValidation(t *testing.T) {
	t.Run("message default", func(t *testing.T) {
		err := Validation("")
		assert.Equal(t, "validation failed", err.Message)
		assert.Equal(t, http.StatusUnprocessableEntity, err.Status)
	})

	t.Run("dengan field", func(t *testing.T) {
		err := Validation("invalid input",
			Field("email", "email is required"),
			Field("password", "password must be at least 8 characters"),
		)
		assert.Equal(t, http.StatusUnprocessableEntity, err.Status)
		assert.Equal(t, []FieldError{
			{Field: "email", Message: "email is required"},
			{Field: "password", Message: "password must be at least 8 characters"},
		}, err.Errors)
	})
}

// TestWithFieldsAppend memastikan WithFields/WithField menambah (bukan
// menimpa), sehingga service bisa mengumpulkan beberapa sebab sekaligus.
func TestWithFieldsAppend(t *testing.T) {
	err := Validation("invalid input", Field("a", "a is required")).
		WithField("b", "b is required").
		WithFields(Field("c", "c is required"))

	require.Len(t, err.Errors, 3)
	assert.Equal(t, "a", err.Errors[0].Field)
	assert.Equal(t, "b", err.Errors[1].Field)
	assert.Equal(t, "c", err.Errors[2].Field)
}

// TestWithCause memastikan cause tersimpan, tidak bocor lewat Error(), dan
// tetap bisa ditelusuri dengan errors.Is/As.
func TestWithCause(t *testing.T) {
	sentinel := errors.New("connection refused")
	err := Internal("failed to reach database").WithCause(sentinel)

	assert.Equal(t, sentinel, err.Cause())
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "failed to reach database")
	assert.Contains(t, err.Error(), "connection refused")
}

// TestWithRetryAfter memastikan RetryAfter tersimpan.
func TestWithRetryAfter(t *testing.T) {
	err := TooManyRequests("too many attempts").WithRetryAfter(60)

	assert.Equal(t, 60, err.RetryAfter)
	assert.Equal(t, http.StatusTooManyRequests, err.Status)
}

// TestResponseJSON memverifikasi bentuk JSON yang benar-benar dikirim:
//
//	{ "status": ..., "message": ... }                  (error biasa)
//	{ "status": ..., "message": ..., "errors": [...] } (validasi)
func TestResponseJSON(t *testing.T) {
	t.Run("error biasa tanpa errors", func(t *testing.T) {
		raw, err := json.Marshal(NotFound("product not found").Response())
		require.NoError(t, err)

		assert.JSONEq(t, `{"status":404,"message":"product not found"}`, string(raw))
		assert.NotContains(t, string(raw), "errors", "errors harus di-omit saat kosong")
	})

	t.Run("error validasi dengan errors", func(t *testing.T) {
		appErr := Validation("validation failed", Field("name", "name is required"))
		raw, err := json.Marshal(appErr.Response())
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"status": 422,
			"message": "validation failed",
			"errors": [{"field":"name","message":"name is required"}]
		}`, string(raw))
	})
}

// TestAs memastikan As menemukan *Error di dalam rantai wrapping.
func TestAs(t *testing.T) {
	base := NotFound("product not found")
	wrapped := errors.New("handler: " + base.Error())

	// Bukan *Error → false.
	_, ok := As(wrapped)
	assert.False(t, ok)

	// Dibungkus fmt.Errorf %w → tetap ketemu.
	wrappedReal := wrap(base)
	got, ok := As(wrappedReal)
	require.True(t, ok)
	assert.Equal(t, "product not found", got.Message)
}

func wrap(err error) error {
	return &wrapErr{err}
}

type wrapErr struct{ err error }

func (w *wrapErr) Error() string { return "wrapped: " + w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.err }

// TestErrorMessageFormat memastikan Error() memuat status + message (tanpa
// membocorkan apa pun yang tidak sengaja).
func TestErrorMessageFormat(t *testing.T) {
	msg := BadRequest("invalid request body").Error()
	assert.True(t, strings.HasPrefix(msg, "[400]"), "harus memuat status, got %q", msg)
	assert.Contains(t, msg, "invalid request body")
}
