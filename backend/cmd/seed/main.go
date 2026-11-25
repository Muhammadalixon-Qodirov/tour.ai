// seed — seed/*.json fayllarini bazaga yuklaydi. Idempotent: qayta ishga tushirsa yozuvlar yangilanadi.
//
//	go run ./cmd/seed [seed-papka]
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"yoldosh/internal/platform"
	"yoldosh/internal/seed"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	dir := "seed"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		slog.Error("seed xatosi", "err", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL o'rnatilmagan")
	}
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		return err
	}
	return seed.LoadDir(ctx, db, dir)
}
