// Package response mendefinisikan SATU bentuk JSON untuk semua response API —
// sukses maupun error — supaya client tidak perlu menebak-nebak struktur.
//
// HTTP status code TIDAK diulang di dalam body: status sudah dikirim lewat
// header HTTP oleh server. Body hanya berisi message/data/errors.
//
//	Sukses:           { "message": "OK", "data": { ... } }
//	Error biasa:      { "message": "product not found" }
//	Validation (422): { "message": "validation failed", "errors": [...] }
//
// Error dibuat dari `apperror` (yang memakai Response lewat .Response());
// sukses dibuat langsung lewat constructor di package ini. Bentuknya satu,
// jadi client tidak perlu membedakan asalnya.
package response

import "net/http"

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
//	response.Field("discount", "discount must not be set when coupon_id is used")
func Field(field, message string) FieldError {
	return FieldError{Field: field, Message: message}
}

// Response adalah bentuk JSON yang selalu dikirim ke client.
type Response struct {
	Message string       `json:"message"`
	Data    any          `json:"data,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
}

// =============================================================================
// Constructor sukses
// =============================================================================

// Success membuat response sukses dengan message custom dan data opsional.
func Success(message string, data any) Response {
	return Response{Message: message, Data: data}
}

// OK adalah response sukses 200 standar.
func OK(data any) Response {
	return Success(http.StatusText(http.StatusOK), data)
}

// Created adalah response sukses 201.
func Created(data any) Response {
	return Success(http.StatusText(http.StatusCreated), data)
}
