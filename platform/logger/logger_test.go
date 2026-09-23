package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLevel memastikan string level (case-insensitive) dipetakan dengan
// benar, dan salah ketik mengembalikan error (bukan diam-diam jadi Info).
func TestParseLevel(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		"debug":             {in: "debug", want: slog.LevelDebug},
		"INFO uppercase":    {in: "INFO", want: slog.LevelInfo},
		"kosong = info":     {in: "", want: slog.LevelInfo},
		"warn":              {in: "warn", want: slog.LevelWarn},
		"warning alias":     {in: "warning", want: slog.LevelWarn},
		"error":             {in: "error", want: slog.LevelError},
		"nilai tak dikenal": {in: "verbose", wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseLevel(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestFromConfig memastikan FromConfig menerima kombinasi env yang wajar dan
// menolak level yang salah ketik.
func TestFromConfig(t *testing.T) {
	cases := map[string]struct {
		environment string
		level       string
		format      string
		wantErr     bool
	}{
		"production default (json+info)":   {environment: "production"},
		"development default (text+debug)": {environment: "development"},
		"level eksplisit":                  {environment: "development", level: "warn"},
		"format eksplisit":                 {environment: "development", format: "json"},
		"level salah":                      {environment: "production", level: "nope", wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			l, err := FromConfig(tc.environment, tc.level, tc.format)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, l, "logger tidak boleh nil")
		})
	}
}

// TestNewWriter memastikan output JSON/Text sesuai konfigurasi.
func TestNewWriter(t *testing.T) {
	cases := map[string]struct {
		cfg        Config
		wantPrefix string // "{" untuk JSON, kosong untuk Text
	}{
		"json": {cfg: Config{Level: slog.LevelInfo, JSON: true}, wantPrefix: "{"},
		"text": {cfg: Config{Level: slog.LevelInfo, JSON: false}, wantPrefix: ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf strings.Builder
			l := NewWriter(&buf, tc.cfg)

			l.Info("hello", slog.String("user_id", "u_1"))

			out := strings.TrimSpace(buf.String())
			require.NotEmpty(t, out)
			if tc.wantPrefix != "" {
				assert.True(t, strings.HasPrefix(out, tc.wantPrefix), "bukan JSON output:\n%s", out)
				assert.Contains(t, out, `"user_id":"u_1"`)
			}
		})
	}
}

// TestNewWriterLevelFiltering memastikan level minimum benar-benar menyaring
// record di bawahnya (cfg.Level tidak diabaikan).
func TestNewWriterLevelFiltering(t *testing.T) {
	var buf strings.Builder
	l := NewWriter(&buf, Config{Level: slog.LevelWarn, JSON: true})

	l.Debug("debug harus hilang")
	l.Info("info harus hilang")
	l.Warn("warn harus muncul")

	out := buf.String()
	assert.NotContains(t, out, "debug harus hilang")
	assert.NotContains(t, out, "info harus hilang")
	assert.Contains(t, out, "warn harus muncul")
}

// TestSensitiveRedaction memastikan key sensitif di-sensor oleh ReplaceAttr.
// Sensor memakai substring-match, jadi key turunan seperti access_token,
// refresh_token, password_hash, authorization, dan cookie ikut tertutup.
func TestSensitiveRedaction(t *testing.T) {
	cases := map[string]struct {
		key, val string
	}{
		"password":      {"password", "supersecret"},
		"password_hash": {"password_hash", "hash-abcd"},
		"token":         {"token", "abc"},
		"access_token":  {"access_token", "abc123"},
		"refresh_token": {"refresh_token", "rtef456"},
		"client_secret": {"client_secret", "xyz789"},
		"secret":        {"secret", "xyz"},
		"authorization": {"authorization", "Bearer abc.def.ghi"},
		"cookie":        {"cookie", "session=abc"},
		"api_key":       {"api_key", "key-123"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf strings.Builder
			l := NewWriter(&buf, Config{Level: slog.LevelInfo, JSON: true})

			l.Info("attempt", slog.String(tc.key, tc.val))

			out := buf.String()
			assert.NotContains(t, out, tc.val, "nilai %q tidak boleh bocor", tc.key)
			assert.Contains(t, out, `"***"`, "nilai %q harus di-sensor", tc.key)
		})
	}
}

// TestSensitiveRedactionAllowsSafeKey memastikan key yang TIDAK sensitif
// tetap direkam apa adanya (tidak over-redact).
func TestSensitiveRedactionAllowsSafeKey(t *testing.T) {
	var buf strings.Builder
	l := NewWriter(&buf, Config{Level: slog.LevelInfo, JSON: true})

	l.Info("request", slog.String("user_id", "u_1"), slog.String("path", "/app/login"))

	out := buf.String()
	assert.Contains(t, out, `"user_id":"u_1"`)
	assert.Contains(t, out, `"path":"/app/login"`)
	assert.NotContains(t, out, `"***"`)
}

// TestTrimSourcePath memastikan path absolut dipendekkan jadi relatif repo.
func TestTrimSourcePath(t *testing.T) {
	cases := map[string]string{
		"/home/user/project/internal/modules/x/service.go": "internal/modules/x/service.go",
		"/home/user/project/platform/logger/logger.go":     "platform/logger/logger.go",
		"/home/user/project/cmd/app/main.go":               "cmd/app/main.go",
		"/home/user/project/db/postgres/postgres.go":       "db/postgres/postgres.go",
		"/tmp/generated.go":                                "generated.go",
		"relative.go":                                      "relative.go",
	}
	for in, want := range cases {
		assert.Equal(t, want, trimSourcePath(in), "input %q", in)
	}
}

// TestContextHelper memastikan WithContext/FromContext round-trip, dan
// fallback ke slog.Default saat context kosong.
func TestContextHelper(t *testing.T) {
	cases := map[string]struct {
		ctx      context.Context
		wantSame bool
	}{
		"dengan logger di context": {
			ctx:      WithContext(context.Background(), slog.New(slog.NewTextHandler(&strings.Builder{}, nil))),
			wantSame: true,
		},
		"context kosong fallback default": {
			ctx:      context.Background(),
			wantSame: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := FromContext(tc.ctx)
			assert.NotNil(t, got)
			if tc.wantSame {
				assert.Same(t, tc.ctx.Value(ctxKeyLogger), got, "harus logger yang sama")
			} else {
				assert.Equal(t, slog.Default(), got, "harus fallback slog.Default")
			}
		})
	}
}

// TestDefaultConfig memastikan nilai default konsisten (aman untuk produksi).
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, slog.LevelInfo, cfg.Level)
	assert.True(t, cfg.JSON)
	assert.False(t, cfg.Source)
}
