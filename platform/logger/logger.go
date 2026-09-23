// RINGKASAN KONTRAK:
//   - Service/repository TIDAK menerima logger di constructor.
//   - Mereka memanggil logger.FromContext(ctx) saat butuh log.
//   - Middleware (platform/middleware.LoggerMiddleware) yang MENYISIPKAN
//     logger ke context di awal setiap request.
//
// Prinsip:
//   - Selalu pakai method *Context, bukan Info/Error tanpa context.
//   - Jangan pernah log data sensitif: password, token, secret, private key.
//     Redaction di bawah hanya lapisan pertahanan terakhir, bukan izin.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey string

const ctxKeyLogger ctxKey = "logger"

type Config struct {
	// Level minimum yang dicatat. Default Info.
	Level slog.Level
	// JSON true => JSONHandler (mesin), false => TextHandler (manusia).
	JSON bool
	// Source true => sertakan file:baris pemanggil di setiap record.
	// Berguna saat tracing; ada overhead, jadi biasanya hanya di dev.
	Source bool
}

func DefaultConfig() Config {
	return Config{
		Level:  slog.LevelInfo,
		JSON:   true,
		Source: false,
	}
}

func ParseLevel(lvl string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s", lvl)
	}
}

func New(cfg Config) *slog.Logger { return NewWriter(os.Stdout, cfg) }

func NewWriter(w io.Writer, cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:       cfg.Level,
		AddSource:   cfg.Source,
		ReplaceAttr: redact,
	}

	var h slog.Handler
	if cfg.JSON {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

func FromConfig(environment, level, format string) (*slog.Logger, error) {
	production := strings.EqualFold(strings.TrimSpace(environment), "prod")

	cfg := DefaultConfig()
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		cfg.JSON = true
	case "text":
		cfg.JSON = false
	default:
		cfg.JSON = production
	}

	if strings.TrimSpace(level) == "" {
		if production {
			cfg.Level = slog.LevelInfo
		} else {
			cfg.Level = slog.LevelDebug
		}
	} else {
		parsed, err := ParseLevel(level)
		if err != nil {
			return nil, err
		}
		cfg.Level = parsed
	}

	cfg.Source = !production
	return New(cfg), nil
}

// sensitiveKeys adalah daftar substring (lowercase) yang membuat sebuah key
// log dianggap sensitif. Sengaja substring-match supaya turunannya ikut
// tertutup: access_token, refresh_token, password_hash, client_secret,
// authorization, cookie, dsb.
var sensitiveKeys = []string{
	"password", "passwd", "token", "secret", "authorization", "cookie",
	"api_key", "apikey", "credential", "private_key",
}

// redact adalah ReplaceAttr untuk dua keperluan:
//  1. Memendekkan path source supaya log ringkas (internal/... / platform/...).
//  2. Menutupi nilai dari key yang dianggap sensitif dengan "***".
//
// Ini lapisan pertahanan TERAKHIR. Sumbernya tetap tidak boleh di-log.
func redact(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey {
		if src, ok := a.Value.Any().(*slog.Source); ok {
			src.File = trimSourcePath(src.File)
		}
		return a
	}

	key := strings.ToLower(a.Key)
	for _, s := range sensitiveKeys {
		if strings.Contains(key, s) {
			a.Value = slog.StringValue("***")
			return a
		}
	}
	return a
}

// trimSourcePath memotong path absolut build menjadi path relatif terhadap
// root repo (mis. /home/user/app/internal/x.go → internal/x.go).
func trimSourcePath(file string) string {
	for _, marker := range []string{"/internal/", "/platform/", "/cmd/", "/db/"} {
		if i := strings.Index(file, marker); i >= 0 {
			return file[i+1:]
		}
	}
	// Fallback: nama file saja, jangan bocorkan path absolut ke log.
	if i := strings.LastIndex(file, "/"); i >= 0 {
		return file[i+1:]
	}
	return file
}

// FromContext mengambil logger request-scoped dari context.
//
// INI YANG DIPANGGIL SERVICE/REPOSITORY SAAT MAU LOG:
//
//	l := logger.FromContext(ctx)
//	l.InfoContext(ctx, "user created", slog.String("user_id", id))
//
// Fallback ke slog.Default() bila context tidak punya logger — seharusnya
// tidak terjadi di request path karena middleware sudah menyisipkannya.
// Fallback ini menjaga code path non-HTTP (background job) tetap aman.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithContext menyimpan logger ke dalam context.
// Yang MENULIS adalah middleware di awal request; yang MEMBACA adalah
// service/repository via FromContext.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, l)
}
