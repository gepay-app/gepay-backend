package main

import (
	"gepay/internal/bootstrap"
	"log/slog"
	"os"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		// Run() sudah menyiapkan slog.SetDefault jauh sebelum titik ini
		// (kecuali kalau gagal membaca config, di situ pakai logger bawaan Go).
		slog.Error("application stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}
