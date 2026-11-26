// discover — yangi joylarni yig'ish va tadqiqot nomzodlarini chop etish.
//
//	discover wikidata [--dry-run]   # Wikidata (CC0) dan O'zbekistondagi obidalar, muzeylar, masjidlar, bog'lar...
//	discover research [--dry-run]   # seed/candidates dan kelgan pending nomzodlarni tekshirib chop etish
//	discover stats                  # topilmalar holati
//
// API serveri DISCOVERY_EVERY (masalan 24h) bilan Wikidata yig'ishni o'zi ham muntazam bajaradi.
// OSM yangilash: deploy/osm-refresh.sh (haftalik).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"yoldosh/internal/discover"
	"yoldosh/internal/platform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: discover wikidata|research|stats [--dry-run]")
		os.Exit(2)
	}
	dry := slices.Contains(os.Args[2:], "--dry-run")
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		fail(err)
	}
	if ua := os.Getenv("DISCOVERY_USER_AGENT"); ua != "" {
		discover.UserAgent = ua
	}
	var rep discover.Report
	switch os.Args[1] {
	case "wikidata":
		rep, err = discover.RunWikidata(ctx, db, dry)
	case "research":
		rep, err = discover.PublishResearch(ctx, db, dry)
	case "stats":
		rows, err := db.Query(ctx, `SELECT source, status, count(*), round(avg(confidence)::numeric, 2) FROM discoveries GROUP BY 1, 2 ORDER BY 1, 2`)
		if err != nil {
			fail(err)
		}
		defer rows.Close()
		for rows.Next() {
			var src, st string
			var n int
			var avg float64
			if err := rows.Scan(&src, &st, &n, &avg); err != nil {
				fail(err)
			}
			fmt.Printf("%-10s %-12s %5d  ishonch %.2f\n", src, st, n, avg)
		}
		return
	default:
		fail(fmt.Errorf("noma'lum buyruq %q", os.Args[1]))
	}
	if err != nil {
		fail(err)
	}
	if dry {
		fmt.Println("DRY RUN — bazaga yozilmadi")
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(b))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "xato:", err)
	os.Exit(1)
}
