package validation

import (
	"gepay/platform/apperror"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type testRequestBare struct {
	Field string `validate:"required"`
}

type testRequestJSONTag struct {
	UserName string `json:"user_name" validate:"required,min=3"`
}

// TestStruct memverifikasi adapter validation.Struct: input valid → nil,
// input invalid → *apperror.Error 422 dengan message + Errors yang tepat.
func TestStruct(t *testing.T) {
	cases := map[string]struct {
		input       any
		wantErr     bool
		wantMessage string
		wantFields  []apperror.FieldError
	}{
		"valid": {
			input:   testRequest{Email: "a@b.com", Password: "12345678"},
			wantErr: false,
		},
		"email tidak valid + password pendek": {
			input:       testRequest{Email: "invalid", Password: "123"},
			wantErr:     true,
			wantMessage: "validation failed",
			wantFields: []apperror.FieldError{
				{Field: "email", Message: "email must be a valid email address"},
				{Field: "password", Message: "password must be at least 8 characters"},
			},
		},
		"email kosong (required)": {
			input:       testRequest{Email: "", Password: "12345678"},
			wantErr:     true,
			wantMessage: "validation failed",
			wantFields: []apperror.FieldError{
				{Field: "email", Message: "email is required"},
			},
		},
		"field pakai json tag name": {
			input:       testRequestJSONTag{UserName: "ab"}, // min=3 → gagal
			wantErr:     true,
			wantMessage: "validation failed",
			wantFields: []apperror.FieldError{
				{Field: "user_name", Message: "user_name must be at least 3 characters"},
			},
		},
		"tanpa json tag fallback ke struct name": {
			input:       testRequestBare{},
			wantErr:     true,
			wantMessage: "validation failed",
			wantFields: []apperror.FieldError{
				{Field: "Field", Message: "Field is required"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := New()
			err := v.Struct(tc.input)

			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			ce, ok := apperror.As(err)
			require.True(t, ok, "error harus *apperror.Error, got %T", err)
			assert.Equal(t, tc.wantMessage, ce.Message)
			assert.Equal(t, tc.wantFields, ce.Errors)
		})
	}
}

// TestTagMessages memastikan pesan validasi menyertakan nama field (reflection).
func TestTagMessages(t *testing.T) {
	cases := map[string]struct {
		tag   string
		param string
		want  string
	}{
		"required":    {tag: "required", want: "email is required"},
		"email":       {tag: "email", want: "email must be a valid email address"},
		"min":         {tag: "min", param: "8", want: "password must be at least 8 characters"},
		"max":         {tag: "max", param: "100", want: "password must be at most 100 characters"},
		"len":         {tag: "len", param: "16", want: "token must be exactly 16 characters"},
		"url":         {tag: "url", want: "link must be a valid URL"},
		"uuid biasa":  {tag: "uuid", want: "id must be a valid UUID"},
		"uuid v7":     {tag: "uuid", param: "7", want: "id must be a valid UUID v7"},
		"gte":         {tag: "gte", param: "0", want: "age must be greater than or equal to 0"},
		"lte":         {tag: "lte", param: "100", want: "age must be less than or equal to 100"},
		"gt":          {tag: "gt", param: "0", want: "age must be greater than 0"},
		"lt":          {tag: "lt", param: "100", want: "age must be less than 100"},
		"oneof":       {tag: "oneof", param: "a b", want: "role must be one of: a b"},
		"unknown tag": {tag: "unknown_tag", want: "validation failed for field custom"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			field := map[string]string{
				"required": "email", "email": "email", "min": "password", "max": "password",
				"len": "token", "url": "link", "uuid": "id", "gte": "age", "lte": "age",
				"gt": "age", "lt": "age", "oneof": "role", "unknown_tag": "custom",
			}[tc.tag]
			assert.Equal(t, tc.want, tagMessage(field, tc.tag, tc.param))
		})
	}
}
