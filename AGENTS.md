# Backend Starter — Kontrak Kerja Developer & AI Agent

Starter backend Go (Echo v5, pgx, sqlc, Goose, `slog`) dengan arsitektur
**modular monolith + clean architecture**. Repo ini **tanpa auth** dan belum
punya module apa pun; module pertama dibuat dengan mengikuti
[`readme/module-development.md`](readme/module-development.md).

> File ini adalah **kontrak kerja** untuk developer & AI agent. Bagian
> bertanda **[WAJIB]** harus dipatuhi dalam setiap perubahan kode.
> **Sumber kebenaran adalah kode**; dokumen bersifat deskriptif kecuali aturan
> yang ditandai normatif. Jika docs dan kode berbeda, kode yang benar — lalu
> perbarui docs-nya di commit yang sama.

---

## TL;DR — Perintah yang Sering Dipakai

| Perintah | Fungsi |
|---|---|
| `make help` | Daftar semua perintah |
| `make dev` | Dev server dengan hot reload (`air`) |
| `make build` | Compile seluruh package |
| `make test` | Jalankan seluruh unit test |
| `make vet` / `make fmt` | `go vet` / `gofmt -w .` |
| `make check` | Verifikasi sebelum commit (fmt-check + vet + test) |
| `make up` | Terapkan migrasi pending |
| `make down` / `make down-all` / `make reset` | Rollback 1 / semua / reset DB |
| `make migrate name=<nama>` | Buat file migrasi baru |
| `make status` | Status migrasi |
| `make sqlc` | Generate kode repository (sqlc) per module |
| `make install-tools` | Install `goose` & `sqlc` ke `$GOBIN` |

Setup awal:

```bash
cp .env.example .env      # isi DB_URL & pastikan Redis berjalan
make dev
```

**Checklist sebelum commit:** `make check` hijau dan `make build` sukses.

---

## Peta Repo (di mana cari apa)

| Path | Isi | Dipakai saat |
|---|---|---|
| `cmd/app/main.go` | Entrypoint tipis → `bootstrap.Run()` | — |
| `internal/bootstrap/app.go` | Composition root: wiring & lifecycle | menambah dependency / route |
| `internal/config/config.go` | Semua env config (struct-tag `caarlos0/env`) | menambah env var |
| `internal/modules/<mod>/domain/` | Entity murni + interface `Service` | membuat module baru |
| `internal/modules/<mod>/service/` | Implementasi service (panggil sqlc langsung) | business logic |
| `internal/modules/<mod>/repository/` | **Generated sqlc only — JANGAN edit** | — |
| `internal/modules/<mod>/delivery/http/` | Echo handler (parsing/validasi/response) | endpoint baru |
| `internal/modules/<mod>/module.go` | Wiring lokal → struct `Module` | — |
| `db/postgres/migrations/` | Migrasi goose | mengubah skema |
| `db/postgres/queries/<mod>/` | Query SQL sqlc per module | membuat query baru |
| `platform/apperror/` | Error model (`status`/`message`/`errors`) | membuat error |
| `platform/logger/` | `slog` via context + redaction | logging |
| `platform/middleware/` | request_id, logger, recover, error handler | middleware |
| `platform/server/` | Setup Echo + `http.Server` | server |
| `platform/validation/` | Wrapper validator → `*apperror.Error` | validasi |

`internal/modules/` belum ada — folder itu dibuat saat module pertama dibuat.

---

## Aturan Arsitektur [WAJIB]

```
delivery (Echo handler) → service → repository.Queries (sqlc generated)
                                      ↑
                                 domain (entity + Service interface)
```

- `domain/` **tidak boleh import** Echo, sqlc, atau library infrastruktur —
  hanya entity murni + interface `Service` (kontrak publik).
- `service/` mengimplementasikan `domain.Service` dan **langsung** memanggil
  `*repository.Queries`.
- `repository/` berisi **hanya** kode generated sqlc — tanpa wrapper manual,
  tanpa edit manual.
- `delivery/http/` depend ke interface `Service`, tidak boleh berisi business
  logic — hanya parsing, validasi, dan response.
- Module lain hanya diakses lewat interface `Service` (lihat
  `internal/modules/<mod>/domain/service.go`).
- **Jangan JOIN lintas tabel milik module berbeda** — panggil `Service` masing-
  masing, gabungkan hasilnya di layer pemanggil.
- **Jangan write ke tabel module lain langsung** — itu melewati validasi /
  invariant pemilik data. FK antar tabel beda module tetap diperbolehkan.
- `bootstrap` adalah satu-satunya package yang boleh meng-import semua module.
  Module TIDAK boleh meng-import `bootstrap`.

## Konvensi Kode [WAJIB]

- `gofmt` + `goimports` wajib sebelum commit (`make fmt` / `make check`).
- Tidak ada `os.Getenv` tersebar — semua env lewat `internal/config`
  (struct-tag `caarlos0/env`, `envPrefix` per sub-struct: `DB_`, `REDIS_`).
- Tidak edit file generated sqlc (`repository/*.sql.go`, `models.go`, `db.go`)
  maupun migrasi yang sudah di-apply di environment bersama.
- Logger dari context (`logger.FromContext(ctx)`), **bukan** DI constructor;
  selalu pakai method `*Context` (`InfoContext`, `ErrorContext`, dst).
- Jangan log data sensitif (password, token, secret, private key). Redaction
  `ReplaceAttr` hanyalah pertahanan terakhir, bukan alasan untuk tetap menulis.
- **UUID wajib v7**: `id, err := uuid.NewV7()` + cek error — bukan `uuid.Must`,
  bukan `uuid.New()`.
- Echo v5: handler `func(c *echo.Context) error`; status ditulis eksplisit di
  `c.JSON(status, ...)`.
- **Semua pesan error/response user-facing bahasa Inggris.** Komentar/string
  internal boleh bahasa Indonesia.
- Error dibuat lewat constructor `apperror.*` (`BadRequest`, `Unauthorized`,
  `Forbidden`, `NotFound`, `Conflict`, `Validation`, `Internal`,
  `TooManyRequests`, atau `New(status, message)`); handler cukup `return err`.
  Response selalu `{status, message}` dan `{status, message, errors[]}` untuk
  validasi — **tidak ada** `code`/`type`/`detail`.
- Jangan kembalikan detail error internal (cause, SQL) ke client — 5xx
  disanitasi otomatis oleh error handler.
- 429 (rate limit) wajib set `Retry-After` via `.WithRetryAfter(seconds)`.
- Pesan validasi wajib menyertakan nama field dari reflection (mis.
  "email is required"); panggil `validation.New().Struct(&req)` di handler.
- Aturan **lintas-field / butuh data DB** divalidasi di **service**, dan
  dikembalikan sebagai `apperror.Validation("validation failed", fields...)` —
  bentuk response-nya sama dengan validasi handler. Kumpulkan semua pelanggaran
  dulu, jangan `return` di yang pertama.
- Nilai enum/status **jangan** disimpan sebagai enum DB — pakai konstanta
  aplikasi dan set eksplisit saat insert (HURUF BESAR), bukan bergantung
  default DB.

## Logging [WAJIB]

Detail & contoh: [`readme/logging.md`](readme/logging.md).

- Base logger dibuat sekali di bootstrap:
  `logger.FromConfig(cfg.Environment, cfg.LogLevel, cfg.LogFormat)`.
- Middleware `RequestID` → `Logger` → `Recover` (urutan ini penting).
- `LoggerMiddleware` menyisipkan logger request-scoped ke context; service &
  repository membacanya lewat `logger.FromContext(ctx)`.
- Selalu method `*Context`; jangan `Info`/`Error` tanpa context.
- Jangan inject logger lewat constructor.
- Jangan log data sensitif. Key yang mengandung `password`, `token`, `secret`,
  `authorization`, `cookie`, `api_key`, `credential`, `private_key` otomatis
  disensor — tapi itu bukan izin menulisnya.
- Pakai level yang tepat: `Debug` untuk detail diagnosa, `Info` untuk kejadian
  normal, `Warn` untuk kesalahan sisi client, `Error` untuk kegagalan sistem.
- `LoggerMiddleware` mencatat satu access log per request (level mengikuti
  status); `ErrorHandler` hanya menambah detail `cause` untuk 5xx — jangan
  mencatat 4xx lagi di handler/service.

## Error & Validasi [WAJIB]

Detail: [`readme/error-validation.md`](readme/error-validation.md).

- Satu tipe `*apperror.Error` untuk semua lapisan; pembeda utama adalah
  **HTTP `status`**.
- Response: `{status, message}` atau `{status, message, errors[]}` untuk validasi.
- `platform/middleware.ErrorHandler` adalah **satu-satunya** tempat error
  diubah menjadi response JSON.
- Handler tidak parsing error — cukup `return err`.
- Validasi format/tag di handler; validasi lintas-field & aturan bisnis di
  service — keduanya memakai bentuk error validasi yang sama.
- Test meng-assert `status` (+ `errors`), bukan string `message` yang panjang.

## Anti-Pattern — JANGAN dilakukan

- Jangan tulis business logic di Echo handler.
- Jangan inject `*pgxpool.Pool` konkret ke service — pakai interface `PgPool`
  lokal (agar bisa di-test dengan `pgxmock`).
- Jangan import `repository`/`service` internal module lain — hanya lewat
  interface `domain.Service`.
- Jangan panggil `os.Exit()` dari package library/service — kembalikan error,
  biar `bootstrap`/`main` yang memutuskan (pola yang sudah dipakai
  `db/postgres` & `db/redis`).
- Jangan pakai `time.Sleep` untuk memajukan waktu di test — pakai
  `miniredis.FastForward`.
- Jangan hardcode key Redis — pakai konstanta lokal + config.
- Jangan kembalikan `uuid.Must` atau `uuid.New()` (bukan v7).
- Jangan buat satu package sqlc raksasa untuk semua tabel — SATU entry per
  module di `sqlc.yaml`.
- Jangan mengubah isi `readme/` dan `AGENTS.md` agar tidak sinkron dengan kode —
  kode adalah source of truth.

---

## Alur Kerja: Menambah / Mengubah Kode

1. **Pahami dulu** — baca `domain/service.go` (interface) module terkait dan
   test yang ada sebagai referensi pola.
2. **Skema** (jika butuh kolom/tabel baru) — `make migrate name=...`, isi
   `-- +goose Up`/`Down`, lalu `make up`.
3. **Query** — tulis `db/postgres/queries/<mod>/<table>.sql` dengan
   `-- name: <Method> :one|:many|:exec`, tambah entry di `sqlc.yaml`, lalu
   `make sqlc`.
4. **Domain** — tambahkan entity + method di interface `domain/service.go`.
5. **Service** — implementasi interface, panggil `*repository.Queries`, bungkus
   error lewat `apperror.*` dan `fmt.Errorf("...: %w", err)`.
6. **Handler** — tambah endpoint di `delivery/http/`, validasi via
   `validation.New().Struct(&req)`, panggil service, `return err` polos.
7. **Wiring** — daftarkan route di `module.go`, wire dependency di
   `internal/bootstrap/app.go`.
8. **Test** — tambah unit test (pola: `pgxmock` + `miniredis`, `httptest` untuk
   handler); assert `status` (+ `errors` untuk validasi).
9. **Verifikasi** — jalankan `make check` + `make build`.

> Panduan langkah demi langkah lengkap dengan contoh kode:
> [`readme/module-development.md`](readme/module-development.md).

## Checklist Verifikasi (sebelum dianggap selesai)

- [ ] `gofmt -l .` tidak mengeluarkan output (`make fmt-check`)
- [ ] `go vet ./...` bersih
- [ ] `go build ./...` sukses
- [ ] `go test ./...` hijau
- [ ] Tidak ada `os.Getenv` baru di luar `internal/config`
- [ ] Tidak ada `os.Exit()` baru di luar `cmd/app/main.go`
- [ ] Tidak mengedit file generated (sqlc) / migrasi yang sudah di-apply
- [ ] Error baru dibuat via `apperror.*` dengan `message` bahasa Inggris
      (validasi → `apperror.Validation` + `errors[]`)
- [ ] UUID memakai `uuid.NewV7()` dengan cek error
- [ ] Tidak ada data sensitif yang di-log
- [ ] Docs (`readme/`, `AGENTS.md`) diperbarui bila perilaku berubah

---

## Dokumentasi (`readme/`)

Semua dokumen memakai header konsisten (status normatif/deskriptif + link
terkait). Indeks di [`README.md`](README.md).

- [`readme/architecture.md`](readme/architecture.md) — arsitektur, struktur module, boundary **[normatif]**
- [`readme/module-development.md`](readme/module-development.md) — buat module baru (tutorial lengkap), Makefile **[normatif]** (konvensi)
- [`readme/logging.md`](readme/logging.md) — logging **[normatif]** (konvensi)
- [`readme/error-validation.md`](readme/error-validation.md) — error model & validasi **[normatif]** (konvensi)
- [`readme/configuration.md`](readme/configuration.md) — env vars
- [`readme/testing.md`](readme/testing.md) — konvensi test **[normatif]** (konvensi)
- [`readme/best-practices.md`](readme/best-practices.md) — praktik yang dianjurkan & anti-pattern
