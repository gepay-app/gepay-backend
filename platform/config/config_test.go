package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDBURL = "postgres://root:root@localhost:5432/app?sslmode=disable"

// testFirebaseCreds hanya perlu NON-KOSONG: config tidak mem-parse isi
// kredensial — itu tugas provider Firebase saat startup.
const testFirebaseCreds = "e30="

// TestLoad_Defaults memastikan nilai default keluar dengan benar ketika env
// (selain DB_URL yang wajib) belum di-set.
func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DB_URL", testDBURL)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", testFirebaseCreds)

	app, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "development", app.Environment)
	assert.Equal(t, "app", app.Name)
	assert.Equal(t, 8080, app.Port)
	assert.Empty(t, app.LogLevel, "kosong = otomatis oleh logger.FromConfig")
	assert.Empty(t, app.LogFormat)
	assert.Equal(t, "localhost:6379", app.Redis.Addr)
	assert.Equal(t, int32(25), app.Database.MaxConns)
	assert.Equal(t, int32(2), app.Database.MinConns)
	assert.Equal(t, 30*time.Minute, app.Database.ConnMaxLifetime)
	assert.Equal(t, 10*time.Minute, app.Database.ConnMaxIdleTime)
}

// TestLoad_Override membuktikan config gampang di-override per environment —
// ini pola yang dipakai di staging/prod maupun saat test mengganti nilai.
func TestLoad_Override(t *testing.T) {
	env := map[string]string{
		"APP_ENV":        "production",
		"APP_PORT":       "9000",
		"APP_LOG_LEVEL":  "warn",
		"APP_LOG_FORMAT": "text",
		"DB_URL":         "postgres://root:root@db:5432/app?sslmode=disable",
		"DB_MAX_CONNS":   "10",
		"DB_MIN_CONNS":   "0",
		"REDIS_ADDR":     "redis:6379",
		"REDIS_PASSWORD": "rahasia",

		"FIREBASE_SERVICE_ACCOUNT_BASE64": testFirebaseCreds,
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	app, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "production", app.Environment)
	assert.Equal(t, 9000, app.Port)
	assert.Equal(t, "warn", app.LogLevel)
	assert.Equal(t, "text", app.LogFormat)
	assert.Equal(t, "postgres://root:root@db:5432/app?sslmode=disable", app.Database.URL)
	assert.Equal(t, int32(10), app.Database.MaxConns)
	assert.Equal(t, int32(0), app.Database.MinConns)
	assert.Equal(t, "redis:6379", app.Redis.Addr)
	assert.Equal(t, "rahasia", app.Redis.Password)
}

// TestLoad_Validation memastikan nilai wajib/salah ketik ketahuan saat startup,
// bukan saat runtime.
func TestLoad_Validation(t *testing.T) {
	cases := map[string]struct {
		env     map[string]string
		wantErr string
	}{
		"DB_URL kosong": {
			env:     map[string]string{"DB_URL": ""},
			wantErr: "DB_URL is required",
		},
		"APP_PORT di luar rentang": {
			env:     map[string]string{"DB_URL": testDBURL, "APP_PORT": "70000"},
			wantErr: "APP_PORT must be between 1 and 65535",
		},
		"APP_LOG_LEVEL tidak dikenal": {
			env:     map[string]string{"DB_URL": testDBURL, "APP_LOG_LEVEL": "verbose"},
			wantErr: "APP_LOG_LEVEL must be one of",
		},
		"APP_LOG_FORMAT tidak dikenal": {
			env:     map[string]string{"DB_URL": testDBURL, "APP_LOG_FORMAT": "xml"},
			wantErr: "APP_LOG_FORMAT must be one of",
		},
		"FIREBASE_SERVICE_ACCOUNT_BASE64 kosong": {
			env:     map[string]string{"DB_URL": testDBURL, "FIREBASE_SERVICE_ACCOUNT_BASE64": ""},
			wantErr: "FIREBASE_SERVICE_ACCOUNT_BASE64 is required",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
