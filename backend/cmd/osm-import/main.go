// osm-import — osm-extract chiqishini (JSON) bazaga yuklaydi: joylar, turar joylar, xizmatlar.
//
//	go run ./cmd/osm-extract uzbekistan-latest.osm.pbf > osm-pois.json
//	go run ./cmd/osm-import osm-pois.json
//
// Takroran ishga tushirish xavfsiz (upsert). Qo'lda kiritilgan yoki tahrirlangan ma'lumotlar ustiga yozilmaydi.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"yoldosh/internal/osmimport"
	"yoldosh/internal/platform"
)

// usdRate — Markaziy bank kursi (narx bahosini so'mga o'tkazish uchun).
func usdRate() float64 {
	c := http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get("https://cbu.uz/uz/arkhiv-kursov-valyut/json/USD/")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var body []struct {
		Rate string `json:"Rate"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil || len(body) == 0 {
		return 0
	}
	r, _ := strconv.ParseFloat(body[0].Rate, 64)
	return r
}

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
	rate := usdRate()
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
