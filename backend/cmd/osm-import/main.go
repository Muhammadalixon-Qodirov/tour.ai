// osm-import — osm-extract chiqishini (JSON) bazaga yuklaydi: joylar, turar joylar, xizmatlar.
//
//	go run ./cmd/osm-extract uzbekistan-latest.osm.pbf > osm-pois.json
//	go run ./cmd/osm-import osm-pois.json
//
// Takroran ishga tushirish xavfsiz (upsert). Qo'lda kiritilgan yoki tahrirlangan ma'lumotlar ustiga yozilmaydi.
package main

import (
	"context"
	"fmt"
	"os"

	"yoldosh/internal/osmimport"
	"yoldosh/internal/platform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: osm-import osm-pois.json")
		os.Exit(2)
	}
	url := os.Getenv("DATABASE_URL")
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rate := osmimport.USDRate(ctx)
	fmt.Printf("USD kursi (CBU): %.2f\n", rate)
	rep, err := osmimport.Load(ctx, db, os.Args[1], rate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
	fmt.Printf("Joylar: %d (takror sifatida o'tkazildi: %d) · turar joy: %d · xizmatlar: %d · olib tashlangan demo xizmatlar: %d · shaharlardan tashqarida: %d\n",
		rep.Places, rep.PlacesSkippedDup, rep.Stays, rep.Services, rep.DemoServicesRemoved, rep.OutOfArea)
	fmt.Println("Shaharlar bo'yicha:", rep.PerCity)
}
