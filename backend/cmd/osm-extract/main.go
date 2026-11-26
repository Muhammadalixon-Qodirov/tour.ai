// osm-extract — Geofabrik .osm.pbf faylidan turizm uchun kerakli obyektlarni JSON ga ajratadi.
//
//	go run ./cmd/osm-extract uzbekistan-latest.osm.pbf > osm-pois.json
//
// Ma'lumot © OpenStreetMap contributors, ODbL. Ilovada atribusiya majburiy.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"yoldosh/internal/osmimport"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: osm-extract file.osm.pbf")
		os.Exit(2)
	}
	pois, err := osmimport.ExtractFile(context.Background(), os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(pois); err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
	counts := map[string]int{}
	for _, p := range pois {
		counts[p.Group]++
	}
	fmt.Fprintln(os.Stderr, "ajratildi:", counts)
}
