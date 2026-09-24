// Package validation membungkus go-playground/validator untuk memvalidasi
// BENTUK input (tag `validate:"..."`) dan menerjemahkan error-nya menjadi
// *apperror.Error 422 dengan daftar field.
//
// Dipakai di HANDLER. Untuk aturan lintas-field atau aturan yang butuh data
// dari DB, validasi di SERVICE dan kembalikan apperror.Validation(...)
// langsung — tidak lewat package ini. Lihat readme/error-validation.md.
package validation

import (
	"errors"
	"gepay/platform/apperror"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"gepay/platform/response"
)

// Validator adalah wrapper tipis di atas *validator.Validate.
type Validator struct {
	v *validator.Validate
}

var (
	defaultOnce      sync.Once
	defaultValidator *validator.Validate
)

// New mengembalikan Validator yang memakai instance go-playground/validator
// BERSAMA. Penting: validator menyimpan cache hasil refleksi tiap struct,
// jadi membuat instance baru per request hanya membuang kerja berulang.
// Aman dipakai bersamaan (validator.Validate concurrency-safe setelah setup).
func New() *Validator {
	defaultOnce.Do(func() {
		defaultValidator = newValidator()
	})
	return &Validator{v: defaultValidator}
}

func newValidator() *validator.Validate {
	v := validator.New()
	// Pakai nama field dari json tag supaya pesan error cocok dengan apa yang
	// dikirim client ("email is required", bukan "Email is required").
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// Struct memvalidasi struct dan mengembalikan *apperror.Error 422 dengan
// message "validation failed" + daftar field yang tidak lolos.
func (v *Validator) Struct(s any) error {
	err := v.v.Struct(s)
	if err == nil {
		return nil
	}

	var verr validator.ValidationErrors
	if !errors.As(err, &verr) {
		return err
	}

	fields := make([]response.FieldError, len(verr))
	for i, fe := range verr {
		fields[i] = response.FieldError{
			Field:   fe.Field(),
			Message: tagMessage(fe.Field(), fe.Tag(), fe.Param()),
		}
	}
	return apperror.Validation("validation failed", fields...)
}

// tagMessage membangun pesan validasi yang sudah menyertakan nama field
// (reflection: fe.Field() memakai JSON tag name via RegisterTagNameFunc).
// Frontend tidak perlu parsing — pesan sudah lengkap.
func tagMessage(field, tag, param string) string {
	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " must be at least " + param + " characters"
	case "max":
		return field + " must be at most " + param + " characters"
	case "len":
		return field + " must be exactly " + param + " characters"
	case "url":
		return field + " must be a valid URL"
	case "uuid":
		if param == "7" {
			return field + " must be a valid UUID v7"
		}
		return field + " must be a valid UUID"
	case "gte":
		return field + " must be greater than or equal to " + param
	case "lte":
		return field + " must be less than or equal to " + param
	case "gt":
		return field + " must be greater than " + param
	case "lt":
		return field + " must be less than " + param
	case "oneof":
		return field + " must be one of: " + param
	case "nefield":
		return field + " must not be the same as " + param
	default:
		return "validation failed for field " + field
	}
}
