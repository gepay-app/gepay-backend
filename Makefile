# ============================================================
# Makefile — Go backend starter
#
# Jalankan `make` atau `make help` untuk melihat daftar perintah.
# Alur membuat module baru: readme/module-development.md
# ============================================================

# goose membaca DSN/driver/folder migrasi dari .env (GOOSE_*).
GOOSE := goose -env .env

# 1 = sudah ada file migrasi; 0 = folder masih kosong (starter).
HAS_MIGRATIONS := $(shell ls db/postgres/migrations/*.sql >/dev/null 2>&1 && echo 1 || echo 0)
NO_MIGRATIONS_MSG := "Belum ada file migrasi (db/postgres/migrations/ kosong). Buat yang pertama: make migrate name=nama_migrasi (lihat readme/module-development.md)."

.DEFAULT_GOAL := help

.PHONY: help dev build test vet fmt fmt-check tidy check \
        up up-one down down-all reset migrate status sqlc install-tools

# ------------------------------------------------------------
# Help
# ------------------------------------------------------------
help: ## Tampilkan daftar perintah
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------
# Development & verifikasi
# ------------------------------------------------------------
dev: ## Jalankan dev server dengan hot reload (air)
	go tool air

build: ## Compile seluruh package
	go build ./...

test: ## Jalankan seluruh unit test
	go test ./...

vet: ## Jalankan go vet
	go vet ./...

fmt: ## Format kode dengan gofmt
	gofmt -w .

fmt-check: ## Gagal kalau masih ada file yang belum diformat
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "File berikut belum diformat (jalankan: make fmt):"; \
		echo "$$out"; \
		exit 1; \
	fi

tidy: ## Rapikan go.mod & go.sum
	go mod tidy

check: fmt-check vet test ## Verifikasi sebelum commit (format + vet + test)

install-tools: ## Install goose & sqlc ke $GOBIN
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# ------------------------------------------------------------
# Migrasi (goose)
# ------------------------------------------------------------
up: ## Terapkan semua migrasi pending
	@if [ "$(HAS_MIGRATIONS)" = "1" ]; then $(GOOSE) up; else echo $(NO_MIGRATIONS_MSG); fi

up-one: ## Naik 1 migrasi
	@if [ "$(HAS_MIGRATIONS)" = "1" ]; then $(GOOSE) up-by-one; else echo $(NO_MIGRATIONS_MSG); fi

down: ## Rollback tepat 1 migrasi (terbaru)
	@if [ "$(HAS_MIGRATIONS)" = "1" ]; then $(GOOSE) down; else echo $(NO_MIGRATIONS_MSG); fi

down-all: ## Rollback SEMUA migrasi — hati-hati, data terhapus
	@if [ "$(HAS_MIGRATIONS)" = "1" ]; then $(GOOSE) reset; else echo $(NO_MIGRATIONS_MSG); fi

reset: down-all up ## Reset database (down-all + up)

migrate: ## Buat file migrasi baru: make migrate name=nama_migrasi
	@test -n "$(name)" || { echo "Gunakan: make migrate name=nama_migrasi"; exit 1; }
	$(GOOSE) -s create $(name) sql

status: ## Tampilkan status migrasi
	@if [ "$(HAS_MIGRATIONS)" = "1" ]; then $(GOOSE) status; else echo $(NO_MIGRATIONS_MSG); fi

# ------------------------------------------------------------
# Codegen (sqlc)
# ------------------------------------------------------------
# Jalankan SETELAH menambah query di db/postgres/queries/<module>/ dan entry di
# sqlc.yaml. Kalau belum ada module, target ini hanya memberi pesan (bukan error).
sqlc: ## Generate kode repository dari query SQL (sqlc)
	@if ls db/postgres/queries/*/ >/dev/null 2>&1; then \
		sqlc generate; \
	else \
		echo "Tidak ada query module (db/postgres/queries kosong)."; \
		echo "Tambah module dulu: db/postgres/queries/<module>/*.sql + entry di sqlc.yaml — lihat readme/module-development.md."; \
	fi
