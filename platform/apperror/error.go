// Package apperror menyediakan SATU tipe error untuk seluruh aplikasi
// (service, handler, middleware) beserta bentuk JSON yang dikirim ke client.
//
// Bentuk response selalu sesederhana ini:
//
//	{ "status": 404, "message": "product not found" }
//
// Khusus error validasi (422) ditambah daftar penyebab per-field:
//
//	{
//	  "status": 422,
//	  "message": "validation failed",
//	  "errors": [
//	    {"field": "discount", "message": "discount must not be set when coupon_id is used"},
//	    {"field": "name",     "message": "name is required"}
//	  ]
//	}
//
// Jadi:
//   - `status`  → kode HTTP (satu-satunya pembeda jenis error bagi client).
//   - `message` → penjelasan singkat yang aman dibaca user (bahasa Inggris).
//   - `errors`  → opsional, hanya untuk validasi; berisi field + alasannya.
//
// Error validasi boleh dibuat dari HANDLER (hasil go-playground/validator)
// maupun dari SERVICE (validasi kompleks / lintas-field). Bentuknya sama,
// jadi client tidak perlu membedakan asalnya.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// FieldError menjelaskan SATU field yang gagal validasi.
//
// `message` sebaiknya sudah menyebut nama field-nya, mis.
// "email must be a valid email address" — supaya frontend bisa menampilkannya
// langsung tanpa parsing.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Field adalah helper singkat untuk membuat FieldError.
//
//	if req.CouponID != nil && req.Discount != nil {
//		errs = append(errs, apperror.Field("discount", "discount must not be set when coupon_id is used"))
//	}
func Field(field, message string) FieldError {
	return FieldError{Field: field, Message: message}
}

// =============================================================================
// Error — error aplikasi dari service/repository/handler
//
// Implementasi: error, Unwrap, Cause, dan Echo v5 HTTPStatusCoder (StatusCode).
// Handler TIDAK perlu parsing — cukup `return err`.
// =============================================================================

type Error struct {
	// Status adalah HTTP status code (400, 401, 404, 409, 422, 500, ...).
	Status int
	// Message adalah penjelasan singkat untuk client (bahasa Inggris).
	Message string
	// Errors diisi HANYA untuk error validasi (biasanya status 422).
	Errors []FieldError

	cause error
	// RetryAfter opsional (detik). Kalau > 0, error handler set header
	// Retry-After pada response (mis. 429 rate limit).
	RetryAfter int
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s (cause: %s)", e.Status, e.Message, e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.Status, e.Message)
}

// Unwrap membuat error ini kompatibel dengan errors.Is/errors.As.
func (e *Error) Unwrap() error { return e.cause }

// StatusCode memenuhi kontrak echo.HTTPStatusCoder.
func (e *Error) StatusCode() int { return e.Status }

// Cause mengembalikan error internal yang dibungkus (untuk logging, TIDAK
// pernah dikirim ke client).
func (e *Error) Cause() error { return e.cause }

// Response mengubah error ini menjadi body JSON siap kirim.
func (e *Error) Response() Response {
	return Response{
		Status:  e.Status,
		Message: e.Message,
		Errors:  e.Errors,
	}
}

// =============================================================================
// Response — bentuk JSON yang dikirim ke client
// =============================================================================

type Response struct {
	Status  int          `json:"status"`
	Message string       `json:"message"`
	Errors  []FieldError `json:"errors,omitempty"`
}

// =============================================================================
// Konstruktor — satu-satunya cara membuat Error
// =============================================================================

// New membuat error dengan status + message apa adanya.
func New(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, message)
}

func TooManyRequests(message string) *Error {
	return New(http.StatusTooManyRequests, message)
}

// Validation membuat error 422 dengan daftar field yang gagal.
// Kalau message kosong, dipakai "validation failed".
func Validation(message string, fields ...FieldError) *Error {
	if message == "" {
		message = "validation failed"
	}
	return &Error{
		Status:  http.StatusUnprocessableEntity,
		Message: message,
		Errors:  fields,
	}
}

// Internal membuat error 500. Message-nya akan diganti pesan generik oleh
// error handler, jadi isi bebas (dipakai untuk log/cause), bukan untuk client.
func Internal(message string) *Error {
	return New(http.StatusInternalServerError, message)
}

// As mengambil *Error dari rantai error. Dipakai error handler.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// =============================================================================
// Chain helpers — menambah informasi tanpa mengubah konstruktor
// =============================================================================

// WithFields menambahkan field error (append, bukan replace) sehingga
// pemanggil bisa mengumpulkan beberapa sebab sekaligus.
func (e *Error) WithFields(fields ...FieldError) *Error {
	e.Errors = append(e.Errors, fields...)
	return e
}

// WithField menambahkan satu field error.
func (e *Error) WithField(field, message string) *Error {
	return e.WithFields(Field(field, message))
}

// WithCause membungkus error internal. Hanya untuk logging — detailnya
// disanitasi error handler sebelum sampai ke client.
func (e *Error) WithCause(cause error) *Error {
	e.cause = cause
	return e
}

// WithRetryAfter mengisi header Retry-After (detik) pada response.
func (e *Error) WithRetryAfter(seconds int) *Error {
	e.RetryAfter = seconds
	return e
}
