# Backend Go — Kontrak Kerja Developer & AI Agent

Dokumen ini adalah **aturan normatif** untuk setiap penulisan/perubahan kode di
repo ini. Bagian bertanda **[WAJIB]** tidak bisa ditawar.

- Isi dokumen ini hanya **aturan**, bukan catatan kondisi repo saat ini.
- Kalau kode dan dokumen berbeda: **kode adalah sumber kebenaran** — lalu
  perbarui dokumen ini di commit yang sama.
- Aturan berlaku untuk **semua module**, termasuk module yang belum dibuat.

Stack: Go — Echo v5, pgx + sqlc, Goose, `slog`; arsitektur **modular monolith +
clean architecture**.

---

## TL;DR — Perintah yang Sering Dipakai

| Perintah | Fungsi |
|---|---|
| `make help` | Daftar semua perintah |
| `make dev` | Dev server dengan hot reload (`air`) |
| `make build` | Compile seluruh package |
| `make test` | Jalankan seluruh unit test |
| `make vet` / `make fmt` | `go vet` / `gofmt -w .` |
| `make fmt-check` | Gagal kalau masih ada file yang belum diformat |
| `make check` | Verifikasi sebelum commit (fmt-check + vet + test) |
| `make up` / `up-one` | Terapkan semua / satu migrasi pending |
| `make down` / `down-all` / `reset` | Rollback 1 / semua / reset DB |
| `make migrate name=<nama>` | Buat file migrasi baru |
| `make status` | Status migrasi |
| `make sqlc` | Generate kode repository (sqlc) per module |
| `make firebase-credentials` | Cetak service account JSON sebagai base64 satu baris |
| `make install-tools` | Install `goose` & `sqlc` ke `$GOBIN` |

Setup lokal:

```bash
cp .env.example .env      # isi DB_URL, FIREBASE_SERVICE_ACCOUNT_BASE64, Redis jalan
make dev
```

**Sebelum commit:** `make check` hijau dan `make build` sukses.

---

## Peta Repo (di mana menaruh apa)

| Path | Isi | Dipakai saat |
|---|---|---|
| `cmd/app/main.go` | Entrypoint tipis → `bootstrap.Run()` | — |
| `internal/bootstrap/app.go` | Composition root: wiring & lifecycle | menambah dependency / route |
| `platform/config/` | Semua env config (struct-tag `caarlos0/env`) | menambah env var |
| `platform/apperror/` | Error model (status HTTP + `message`/`errors`) | membuat error |
| `platform/response/` | SATU bentuk response JSON (sukses + error) | mengirim response |
| `platform/logger/` | `slog` via context + redaction | logging |
| `platform/server/` | Setup Echo + `http.Server` | server |
| `platform/server/middleware/` | request_id, logger, recover, error handler | middleware global |
| `platform/server/validation/` | Wrapper validator → `*apperror.Error` | validasi input |
| `db/postgres/`, `db/redis/` | Koneksi pool/klien infrastruktur | menambah infrastruktur |
| `db/postgres/migrations/` | Migrasi goose | mengubah skema |
| `db/postgres/queries/<mod>/` | Query SQL sqlc per module | membuat query baru |
| `internal/modules/<mod>/api/` | Kontrak **PUBLIK** module | module lain butuh tipe/interface-nya |
| `internal/modules/<mod>/api/domain/` | Entity murni + interface `Service` | membuat module baru |
| `internal/modules/<mod>/api/middleware/` | Middleware HTTP publik | module lain butuh auth |
| `internal/modules/<mod>/internal/service/` | Implementasi service (panggil sqlc langsung) | business logic |
| `internal/modules/<mod>/internal/repository/` | **Generated sqlc only — JANGAN edit** | — |
| `internal/modules/<mod>/internal/delivery/http/` | Echo handler (parsing/validasi/response) | endpoint baru |
| `internal/modules/<mod>/internal/{cache,provider}/` | Adapter infra privat (Redis, provider auth, dst) | menambah adapter |
| `internal/modules/<mod>/module.go` | Facade + wiring lokal → struct `Module` | — |

---

## Aturan Arsitektur [WAJIB]

```
delivery (Echo handler) → service → repository.Queries (sqlc generated)
                                      ↑
                                 domain (entity + Service interface)
```

### Boundary module: `api/` (publik) vs `internal/` (privat)

```
internal/modules/<mod>/
├── api/                # PUBLIK — satu-satunya yang boleh di-import module lain
│   ├── domain/         # entity + interface Service
│   └── middleware/     # middleware HTTP publik (mis. tipe Auth)
├── internal/           # PRIVAT — dijaga COMPILER, bukan sekadar konvensi
│   ├── delivery/http/  service/  repository/  cache/  provider/
└── module.go           # facade + wiring; HANYA bootstrap yang meng-import ini
```

- Setiap module **WAJIB** memakai layout ini.
- Module lain **hanya** boleh import `<mod>/api/...`. Import ke
  `<mod>/internal/...` ditolak compiler (`use of internal package ... not
  allowed`); penegakannya otomatis, tidak perlu test/lint tambahan.
- `api/` **tidak boleh** import `internal/` — kalau tidak, muka module bocor dan
  rawan import cycle.
- `api/` **tidak boleh** berisi business logic; hanya kontrak (entity,
  interface, tipe middleware).
- Middleware/injeksi lintas module dilakukan oleh `bootstrap`,
  **bukan** dengan meng-import `internal/delivery/http` module lain. Kontrak
  publiknya disediakan di `api/middleware`.

### Lapisan di dalam module

- `api/domain/` **tidak boleh import** Echo, sqlc, atau library infrastruktur —
  hanya entity murni + interface `Service` (kontrak publik).
- `internal/service/` mengimplementasikan `domain.Service` dan **langsung**
  memanggil `*repository.Queries`.
- `internal/repository/` berisi **hanya** kode generated sqlc — tanpa wrapper
  manual, tanpa edit manual.
- `internal/delivery/http/` depend ke interface `Service`, tidak boleh berisi
  business logic — hanya parsing, validasi, dan response.
- Module lain hanya diakses lewat interface `Service` (lihat
  `internal/modules/<mod>/api/domain/service.go`).
- **Jangan JOIN lintas tabel milik module berbeda** — panggil `Service` masing-
  masing, gabungkan hasilnya di layer pemanggil.
- **Jangan write ke tabel module lain langsung** — itu melewati validasi /
  invariant pemilik data. FK antar tabel beda module tetap diperbolehkan.
- `bootstrap` adalah satu-satunya package yang boleh meng-import semua module.
  Module TIDAK boleh meng-import `bootstrap`.
- Module baru dibuat dengan mengikuti urutan di bagian **Alur Kerja** di bawah.

## Konvensi Kode [WAJIB]

- `gofmt` + `goimports` wajib sebelum commit (`make fmt` / `make check`).
- Tidak ada `os.Getenv` tersebar — semua env lewat `platform/config`
  (struct-tag `caarlos0/env`, `envPrefix` per sub-struct: `DB_`, `REDIS_`,
  `FIREBASE_`).
- Tidak edit file generated sqlc (`repository/*.sql.go`, `models.go`, `db.go`)
  maupun migrasi yang sudah di-apply di environment bersama.
- Logger dari context (`logger.FromContext(ctx)`), **bukan** DI constructor;
  selalu pakai method `*Context` (`InfoContext`, `ErrorContext`, dst).
- Jangan log data sensitif (password, token, secret, private key). Redaction
  `ReplaceAttr` hanyalah pertahanan terakhir, bukan alasan untuk tetap menulis.
- **UUID wajib v7** memakai package **stdlib `uuid`**: `id := uuid.NewV7()`.
  `NewV7()` mengembalikan `UUID` langsung **tanpa error** — jangan tulis
  `id, err := uuid.NewV7()`. Yang dilarang: `uuid.New()` (itu v4) dan
  `uuid.MustParse` untuk input dari luar.
- Echo v5: handler `func(c *echo.Context) error`; status ditulis eksplisit di
  `c.JSON(status, ...)`.
- **Semua pesan error/response user-facing bahasa Inggris.** Komentar/string
  internal boleh bahasa Indonesia.
- Error dibuat lewat constructor `apperror.*` (`BadRequest`, `Unauthorized`,
  `Forbidden`, `NotFound`, `Conflict`, `Validation`, `Internal`,
  `TooManyRequests`, atau `New(status, message)`); handler cukup `return err`.
  Bentuk response ditentukan SATU tempat di `platform/response`: sukses
  `{message, data}`, error biasa `{message}`, validasi `{message, errors[]}` —
  **tidak ada** `code`/`type`/`detail`, dan status HTTP tidak diulang di body
  (dikirim lewat header).
- Jangan kembalikan detail error internal (cause, SQL) ke client — 5xx
  disanitasi otomatis oleh error handler; lampirkan cause lewat `.WithCause(err)`
  untuk kebutuhan log.
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
- Kolom nullable dipetakan ke pointer (`*string`) lewat
  `emit_pointers_for_null_types` di `sqlc.yaml`, bukan `pgtype.Text`.

## Logging [WAJIB]

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

- Satu tipe `*apperror.Error` untuk semua lapisan; pembeda utama adalah
  **HTTP `status`** (dikirim lewat header, TIDAK diulang di body).
- Bentuk response sukses `{message, data}` via constructor `response.OK`/
  `response.Created`; error `{message}` atau `{message, errors[]}` untuk
  validasi. Semua didefinisikan di `platform/response`.
- `platform/server/middleware.ErrorHandler` adalah **satu-satunya** tempat error
  diubah menjadi response JSON.
- Handler tidak parsing error — cukup `return err`.
- Validasi format/tag di handler; validasi lintas-field & aturan bisnis di
  service — keduanya memakai bentuk error validasi yang sama.
- Test meng-assert `status` (+ `errors`), bukan string `message` yang panjang.

## Anti-Pattern — JANGAN dilakukan

- Jangan tulis business logic di Echo handler.
- Jangan inject `*pgxpool.Pool` konkret ke service — pakai interface `PgPool`
  lokal (agar bisa di-test dengan `pgxmock`).
- Jangan import `<mod>/internal/...` module lain — antar module hanya lewat
  `<mod>/api/domain.Service`, dan middleware disuntikkan bootstrap.
- Jangan panggil `os.Exit()` dari package library/service — kembalikan error,
  biarkan `bootstrap`/`main` yang memutuskan.
- Jangan pakai `time.Sleep` untuk memajukan waktu di test — pakai
  `miniredis.FastForward`.
- Jangan hardcode key Redis — pakai konstanta lokal + config.
- Jangan kembalikan `uuid.Must` atau `uuid.New()` (bukan v7).
- Jangan buat satu package sqlc raksasa untuk semua tabel — SATU entry per
  module di `sqlc.yaml`.
- Jangan biarkan `AGENTS.md` tidak sinkron dengan kode — kode adalah sumber
  kebenaran, perbarui aturannya di commit yang sama.

---

## Alur Kerja: Menambah / Mengubah Kode

1. **Pahami dulu** — baca `api/domain/service.go` (interface) module terkait dan
   test yang ada sebagai referensi pola.
2. **Skema** (jika butuh kolom/tabel baru) — `make migrate name=...`, isi
   `-- +goose Up`/`Down`, lalu `make up`.
3. **Query** — tulis `db/postgres/queries/<mod>/<table>.sql` dengan
   `-- name: <Method> :one|:many|:exec`, tambah entry di `sqlc.yaml`, lalu
   `make sqlc`.
4. **Domain** — tambahkan entity + method di interface `api/domain/service.go`.
5. **Service** — implementasi interface di `internal/service/`, panggil
   `*repository.Queries`, bungkus error lewat `apperror.*` dan
   `fmt.Errorf("...: %w", err)`.
6. **Handler** — tambah endpoint di `internal/delivery/http/`, validasi via
   `validation.New().Struct(&req)`, panggil service, `return err` polos.
7. **Wiring** — rakit dependency di `module.go` (facade) dan daftarkan di
   `internal/bootstrap/app.go`; route `Module.RegisterRoutes(g *echo.Group)`.
8. **Test** — tambah unit test (pola: `pgxmock` + `miniredis`, `httptest` untuk
   handler); assert `status` (+ `errors` untuk validasi).
9. **Verifikasi** — jalankan `make check` + `make build`.

## Checklist Verifikasi (sebelum dianggap selesai)

- [ ] `gofmt -l .` tidak mengeluarkan output (`make fmt-check`)
- [ ] `go vet ./...` bersih
- [ ] `go build ./...` sukses
- [ ] `go test ./...` hijau
- [ ] Layout module mengikuti `api/` + `internal/`; tidak ada import menembus
      `<mod>/internal/...` dari luar module
- [ ] Tidak ada `os.Getenv` baru di luar `platform/config`
- [ ] Tidak ada `os.Exit()` baru di luar `cmd/app/main.go`
- [ ] Tidak mengedit file generated (sqlc) / migrasi yang sudah di-apply
- [ ] Error baru dibuat via `apperror.*` dengan `message` bahasa Inggris
      (validasi → `apperror.Validation` + `errors[]`)
- [ ] UUID memakai `uuid.NewV7()` (stdlib, v7 — bukan `uuid.New()` yang v4)
- [ ] Tidak ada data sensitif yang di-log
- [ ] `AGENTS.md` diperbarui bila ada aturan/konvensi yang berubah
