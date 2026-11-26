// lens-try — Lens'ni terminalda sinash.
//
//	GROQ_API_KEY=... go run ./cmd/lens-try rasm.jpg                       # narx: AI tanish → marketlar → saralash → statistika
//	GROQ_API_KEY=... go run ./cmd/lens-try -q "iPhone 13 128GB"           # rasmsiz, to'g'ridan-to'g'ri so'rov
//	GROQ_API_KEY=... go run ./cmd/lens-try -auto rasm.jpg                 # rasm turi (landmark | price | translate | none)
//	GROQ_API_KEY=... DATABASE_URL=... go run ./cmd/lens-try -landmark rasm.jpg [lat lng]
//	    # joy taxminlari; DATABASE_URL bo'lsa bazadagi joylar bilan moslash natijasi ham
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/landmark"
	"yoldosh/internal/platform"
	"yoldosh/internal/pricing"
	"yoldosh/internal/store"
)

func main() {
	g := pricing.NewGroq(os.Getenv("GROQ_API_KEY"), os.Getenv("LENS_VISION_MODEL"))
	if g == nil || len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: GROQ_API_KEY=... lens-try rasm.jpg | -q \"so'rov\" | -auto rasm.jpg | -landmark rasm.jpg [lat lng]")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch os.Args[1] {
	case "-auto":
		img := read(os.Args[2])
		t0 := time.Now()
		v, err := g.Classify(ctx, img, http.DetectContentType(img))
		check(err)
		fmt.Printf("TUR: %s (%.1f s)\n", v.Kind, time.Since(t0).Seconds())
		dump(v)
		return
	case "-landmark":
		landmarkTry(ctx, g)
		return
	}
	var id pricing.Identified
	if os.Args[1] == "-q" {
		id = pricing.Identified{Query: os.Args[2], Item: map[string]string{"en": os.Args[2]}, Confidence: 1}
	} else {
		img := read(os.Args[1])
		var err error
		id, err = g.Identify(ctx, img, http.DetectContentType(img))
		check(err)
	}
	fmt.Printf("BUYUM: %v | so'rov=%q alt=%q | %s | ishonch %.2f\n", id.Item, id.Query, id.AltQuery, id.Category, id.Confidence)
	res := pricing.NewSearcher(os.Getenv("MARKET_PROXY")).Search(ctx, id.Query)
	var flat []pricing.Listing
	for _, r := range res {
		fmt.Printf("== %s: %d taklif, xato=%v\n", r.Market.Name, len(r.Listings), r.Err)
		for j, l := range r.Listings {
			if j == 15 {
				break
			}
			flat = append(flat, l)
		}
	}
	keep := g.Relevant(ctx, id, flat)
	var prices []int64
	for i, l := range flat {
		mark := "  "
		if keep[i] {
			mark = "✓ "
			prices = append(prices, l.Price)
		}
		fmt.Printf("%s[%s] %d — %.70s\n", mark, l.Market, l.Price, l.Title)
	}
	st, _ := pricing.Compute(prices)
	fmt.Printf("NATIJA: o'rtacha %d, oraliq %d–%d, eng arzon %d, eng qimmat %d, namuna %d\n", st.Average, st.Min, st.Max, st.Lowest, st.Highest, st.Count)
}

func landmarkTry(ctx context.Context, g *pricing.Groq) {
	img := read(os.Args[2])
	t0 := time.Now()
	v, err := g.IdentifyLandmark(ctx, img, http.DetectContentType(img))
	check(err)
	fmt.Printf("VISION (%.1f s):\n", time.Since(t0).Seconds())
	dump(v)
	var loc *domain.LatLng
	if len(os.Args) > 4 {
		lat, _ := strconv.ParseFloat(os.Args[3], 64)
		lng, _ := strconv.ParseFloat(os.Args[4], 64)
		loc = &domain.LatLng{Lat: lat, Lng: lng}
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Println("(DATABASE_URL yo'q: bazadagi joylar bilan moslash o'tkazib yuborildi)")
		return
	}
	db, err := platform.OpenDB(ctx, url)
	check(err)
	defer db.Close()
	in := landmark.Input{Type: v.Type, City: v.City, HasLoc: loc != nil}
	for _, x := range v.Guesses {
		in.Guesses = append(in.Guesses, landmark.Guess{Names: x.Names, Type: x.Type, Confidence: x.Confidence})
	}
	rows, err := store.New(db).LandmarkPlaces(ctx, loc, landmark.RadiusM, landmark.SearchWords(in.Guesses), 300)
	check(err)
	var places []landmark.Place
	for _, p := range rows {
		places = append(places, landmark.Place{ID: p.ID, CityID: p.CityID, Category: p.Category, Names: p.Names, Text: p.Text,
			DistanceM: p.DistanceM, Bulk: p.Source == "osm" || p.Source == "wikidata"})
	}
	res := landmark.Match(in, places)
	fmt.Printf("NOMZODLAR: %d ta joy ko'rildi\n", len(places))
	for i, s := range res.Ranked {
		if i == 5 {
			break
		}
		d := -1.0
		if s.Place.DistanceM != nil {
			d = *s.Place.DistanceM
		}
		fmt.Printf("  %-36s nom %.2f  tavsifda=%v  tur=%v  masofa %.0f m  ishonch %.2f\n", s.Place.ID, s.NameSim, s.Mention, s.TypeFit, d, s.Confidence)
	}
	if res.Best != nil {
		fmt.Printf("NATIJA: %s (ishonch %.2f)\n", res.Best.Place.ID, res.Confidence)
	} else {
		fmt.Printf("NATIJA: aniq emas (ishonch %.2f), ilova ro'yxatdan tanlatadi\n", res.Confidence)
	}
}

func read(path string) []byte {
	b, err := os.ReadFile(path)
	check(err)
	return b
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
}

func dump(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
