// lens-try — "Adolatli narx" ni terminalda sinash: rasm → AI tanish → marketlar → saralash → statistika.
//
//	GROQ_API_KEY=... go run ./cmd/lens-try rasm.jpg
//	GROQ_API_KEY=... go run ./cmd/lens-try -q "iPhone 13 128GB"   # rasmsiz, to'g'ridan-to'g'ri so'rov
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"yoldosh/internal/pricing"
)

func main() {
	g := pricing.NewGroq(os.Getenv("GROQ_API_KEY"), os.Getenv("LENS_VISION_MODEL"))
	if g == nil || len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: GROQ_API_KEY=... lens-try rasm.jpg | -q \"so'rov\"")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var id pricing.Identified
	if os.Args[1] == "-q" {
		id = pricing.Identified{Query: os.Args[2], Item: map[string]string{"en": os.Args[2]}, Confidence: 1}
	} else {
		img, err := os.ReadFile(os.Args[1])
		if err != nil {
			panic(err)
		}
		id, err = g.Identify(ctx, img, http.DetectContentType(img))
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("BUYUM: %v | so'rov=%q alt=%q | %s | ishonch %.2f\n", id.Item, id.Query, id.AltQuery, id.Category, id.Confidence)
	res := pricing.NewSearcher().Search(ctx, id.Query)
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
