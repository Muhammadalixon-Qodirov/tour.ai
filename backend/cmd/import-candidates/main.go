// import-candidates — seed/candidates/*.json dagi nomzod joylarni bazaga pending holatida yuklaydi
// va o'tkazib yuborilganlar hisobotini chiqaradi.
//
//	go run ./cmd/import-candidates [papka]
package main

import (
	"context"
	"fmt"
	"os"

	"yoldosh/internal/platform"
	"yoldosh/internal/seed"
)

func main() {
	dir := "seed/candidates"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
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
	rep, err := seed.LoadCandidates(ctx, db, dir)
	if err != nil {
		return err
	}
	fmt.Printf("Yangi: %d · yangilandi: %d · o'tkazib yuborildi: %d\n", len(rep.Imported), len(rep.Updated), len(rep.Skipped))
	for _, s := range rep.Skipped {
		fmt.Println("  -", s)
	}
	fmt.Println("Joylar pending holatida — admin panelda tekshirib publish qiling: GET /api/v1/admin/places?status=pending")
	return nil
}
