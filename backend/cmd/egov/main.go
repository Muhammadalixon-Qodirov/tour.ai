// egov — data.egov.uz Ochiq ma'lumotlar portalidan rasmiy datasetlarni import qiladi.
//
//	egov hotels [--dry-run] [--limit N] [--trusted]   # sertifikatlangan mehmonxonalar -> geokodlab discover navbatiga
//	egov export --out founds.json [--limit N]          # oladi + geokodlaydi, DB'siz JSON faylga (bloklangan server uchun)
//	egov ingest-file founds.json [--dry-run] [--trusted] # export faylini discover navbatiga joylaydi (tashqi tarmoqsiz)
//	egov list [--sphere tourism|culture]               # sohadagi datasetlar ro'yxati (tekshirish uchun)
//	egov rows <structId> [--limit N]                   # dataset qatorlarini ko'rish (tekshirish uchun)
//
// data.egov.uz chet el IP'sini bloklaydi. Server bloklangan bo'lsa: mahalliy mashinada `export`,
// faylni serverga ko'chirib `ingest-file`. Aks holda `hotels` bitta buyruqda bajaradi.
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

	"yoldosh/internal/discover"
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
	out := fs.String("out", "", "export: natija JSON fayli (bo'sh — stdout)")
	// Go flag paketi birinchi positional argumentda to'xtaydi. Positional (fayl/structId) flaglardan oldin
	// kelsa ham, keyin kelgan flaglar o'qilishi uchun qolgan argumentlarni qayta tahlil qilamiz.
	_ = fs.Parse(os.Args[2:])
	var positional []string
	for fs.NArg() > 0 {
		positional = append(positional, fs.Arg(0))
		_ = fs.Parse(fs.Args()[1:])
	}

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
		if len(positional) < 1 {
			fail(fmt.Errorf("structId kerak: egov rows <structId>"))
		}
		n := *limit
		if n == 0 {
			n = 5
		}
		rows, count, err := cl.Rows(ctx, positional[0], n, 0)
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

	case "export":
		founds, rep, err := egov.CollectHotels(ctx, cl, egov.NewGeocoder(), egov.HotelOptions{Limit: *limit}, log)
		if err != nil && len(founds) == 0 {
			fail(err)
		}
		payload, _ := json.MarshalIndent(map[string]any{"founds": founds, "report": rep}, "", "  ")
		if *out == "" {
			fmt.Println(string(payload))
		} else if werr := os.WriteFile(*out, payload, 0o644); werr != nil {
			fail(werr)
		}
		fmt.Fprintf(os.Stderr, "export: %d mehmonxona (aniq %d, taxminiy %d, topilmadi %d) -> %s\n",
			len(founds), rep.Geocoded, rep.Approx, rep.NoGeo, orStdout(*out))
		if err != nil {
			fmt.Fprintln(os.Stderr, "ogohlantirish:", err)
		}
		return

	case "ingest-file":
		if len(positional) < 1 {
			fail(fmt.Errorf("fayl kerak: egov ingest-file <founds.json>"))
		}
		raw, err := os.ReadFile(positional[0])
		if err != nil {
			fail(err)
		}
		var payload struct {
			Founds []discover.Found `json:"founds"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			fail(err)
		}
		if *trusted {
			for i := range payload.Founds {
				payload.Founds[i].Trusted = true
			}
		} else {
			for i := range payload.Founds {
				payload.Founds[i].Trusted = false
			}
		}
		db, err := platform.OpenDB(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			fail(err)
		}
		defer db.Close()
		if err := platform.Migrate(ctx, db); err != nil {
			fail(err)
		}
		ing, err := egov.IngestFounds(ctx, db, payload.Founds, *dry)
		b, _ := json.MarshalIndent(ing, "", "  ")
		if *dry {
			fmt.Println("DRY RUN — bazaga yozilmadi")
		}
		fmt.Println(string(b))
		if err != nil {
			fail(err)
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

func orStdout(s string) string {
	if s == "" {
		return "stdout"
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "xato:", err)
	os.Exit(1)
}
