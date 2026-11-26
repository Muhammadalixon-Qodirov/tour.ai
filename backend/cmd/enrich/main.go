// enrich — joylarni ochiq manbalardan boyitadi: Wikimedia Commons rasmlari (faqat ochiq litsenziya, muallif va
// manba bilan) va Vikipediya qisqa tavsiflari (uz/ru/en, CC BY-SA, manba ko'rsatiladi).
//
//	enrich [--dry-run] [--limit N] [--no-geo] [--no-desc]
//
// Takroran ishga tushirish xavfsiz: faqat rasmi yoki tavsifi yo'q joylar to'ldiriladi, qo'lda qo'yilganlar tegilmaydi.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"yoldosh/internal/discover"
	"yoldosh/internal/enrich"
	"yoldosh/internal/platform"
)

func main() {
	dry := flag.Bool("dry-run", false, "bazaga yozmaydi")
	limit := flag.Int("limit", 0, "har bosqichda ko'pi bilan nechta joy (0 — hammasi)")
	noGeo := flag.Bool("no-geo", false, "atrofdagi Commons rasmlarini qidirmaslik")
	noDesc := flag.Bool("no-desc", false, "Vikipediya tavsiflarini olmaslik")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := platform.OpenDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ua := discover.UserAgent
	if v := os.Getenv("DISCOVERY_USER_AGENT"); v != "" {
		ua = v
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	rep, err := enrich.Run(ctx, db, enrich.NewWiki(ua), enrich.Options{Limit: *limit, Geo: !*noGeo, Desc: !*noDesc, DryRun: *dry}, log)
	b, _ := json.MarshalIndent(rep, "", "  ")
	if *dry {
		fmt.Println("DRY RUN — bazaga yozilmadi")
	}
	fmt.Println(string(b))
	if err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
}
