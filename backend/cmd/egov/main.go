// egov — data.egov.uz Ochiq ma'lumotlar portalidan rasmiy datasetlarni import qiladi.
//
//	egov hotels [--dry-run] [--limit N] [--trusted]   # sertifikatlangan mehmonxonalar -> geokodlab discover navbatiga
//	egov list [--sphere tourism|culture]               # sohadagi datasetlar ro'yxati (tekshirish uchun)
//	egov rows <structId> [--limit N]                   # dataset qatorlarini ko'rish (tekshirish uchun)
//
// Mehmonxonalar har doim admin navbatiga tushadi (manzildan geokodlangani uchun tekshirish kerak),
// --trusted bilan faqat uy darajasida aniq topilganlari avtomatik chop etilishi mumkin.
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

	"yoldosh/internal/egov"
	"yoldosh/internal/platform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: egov hotels|list|rows [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "bazaga yozmaydi")
	limit := fs.Int("limit", 0, "ko'pi bilan nechta (0 — hammasi)")
	trusted := fs.Bool("trusted", false, "aniq geokodlanganini avtomatik chop etish mumkin")
	sphere := fs.String("sphere", "tourism", "soha: tourism|culture (list uchun)")
	_ = fs.Parse(os.Args[2:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cl := egov.New()

	switch cmd {
	case "list":
		sid := egov.SphereTourism
		if *sphere == "culture" {
			sid = egov.SphereCulture
		}
		ds, err := cl.Datasets(ctx, sid)
		if err != nil {
			fail(err)
		}
		for _, d := range ds {
			fmt.Printf("%-14s %s  %s\n", d.Name, d.StructID, d.Title())
		}
		fmt.Fprintf(os.Stderr, "jami: %d\n", len(ds))
		return

	case "rows":
		if fs.NArg() < 1 {
			fail(fmt.Errorf("structId kerak: egov rows <structId>"))
		}
		n := *limit
		if n == 0 {
			n = 5
		}
		rows, count, err := cl.Rows(ctx, fs.Arg(0), n, 0)
		if err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "jami qator: %d\n", count)
		for _, r := range rows {
			m := map[string]string{}
			for k, c := range r.TableColumn {
				m[k] = c.Str()
			}
			b, _ := json.Marshal(m)
			fmt.Println(string(b))
		}
		return

	case "hotels":
		db, err := platform.OpenDB(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			fail(err)
		}
		defer db.Close()
		if err := platform.Migrate(ctx, db); err != nil {
			fail(err)
		}
		gc := egov.NewGeocoder()
		rep, err := egov.ImportHotels(ctx, db, cl, gc, egov.HotelOptions{
			Limit: *limit, DryRun: *dry, Trusted: *trusted,
		}, log)
		b, _ := json.MarshalIndent(rep, "", "  ")
		if *dry {
			fmt.Println("DRY RUN — bazaga yozilmadi")
		}
		fmt.Println(string(b))
		if err != nil {
			fail(err)
		}
		return

	default:
		fail(fmt.Errorf("noma'lum buyruq %q", cmd))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "xato:", err)
	os.Exit(1)
}
