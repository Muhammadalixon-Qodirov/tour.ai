package osmimport

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// Geofabrik .osm.pbf faylidan turizm uchun kerakli obyektlarni ajratish (cmd/osm-extract va haftalik yangilash).
// Ma'lumot © OpenStreetMap contributors, ODbL.

var (
	stayRe    = regexp.MustCompile(`^(hotel|hostel|guest_house|motel|apartment)$`)
	foodRe    = regexp.MustCompile(`^(restaurant|cafe)$`)
	sightRe   = regexp.MustCompile(`^(museum|attraction|viewpoint|gallery|theme_park|zoo)$`)
	serviceRe = regexp.MustCompile(`^(hospital|clinic|pharmacy|police|atm|bureau_de_change|fuel|charging_station|embassy)$`)
	keepTag   = regexp.MustCompile(`^(name(:uz|:ru|:en)?|alt_name|old_name|opening_hours|phone|contact:.*|website|stars|cuisine|addr:.*|wheelchair|payment:.*|historic|tourism|amenity|wikidata|wikipedia|fee|operator|brand|fuel:.*|internet_access|rooms|beds|check_in|check_out|heritage|start_date|leisure|craft|shop|building|artwork_type|description(:uz|:ru|:en)?|healthcare|emergency|religion)$`)
)

// classify — obyekt qaysi guruhga tushadi (bo'sh — kerak emas).
func classify(t osm.Tags) (group, kind string) {
	m := t.Map()
	switch {
	case stayRe.MatchString(m["tourism"]):
		return "stay", m["tourism"]
	case foodRe.MatchString(m["amenity"]) && m["name"] != "":
		return "food", m["amenity"]
	case sightRe.MatchString(m["tourism"]) && m["name"] != "":
		return "sight", m["tourism"]
	case m["historic"] != "" && m["name"] != "":
		return "sight", "historic:" + m["historic"]
	case (m["amenity"] == "theatre" || m["amenity"] == "concert_hall" || m["amenity"] == "arts_centre") && m["name"] != "":
		return "sight", m["amenity"]
	case m["amenity"] == "marketplace" && m["name"] != "":
		return "sight", "marketplace"
	case m["craft"] != "" && m["name"] != "":
		return "sight", "craft:" + m["craft"]
	case (m["leisure"] == "park" || m["leisure"] == "garden") && m["name"] != "":
		return "sight", "leisure:" + m["leisure"]
	// masjidlar: faqat tarixiy/mashhurlari (har mahalla masjidi katalogga kerak emas)
	case m["amenity"] == "place_of_worship" && m["name"] != "" && (m["wikidata"] != "" || m["wikipedia"] != "" || m["heritage"] != ""):
		return "sight", "place_of_worship"
	case serviceRe.MatchString(m["amenity"]):
		return "service", m["amenity"]
	case m["office"] == "diplomatic":
		return "service", "embassy"
	}
	return "", ""
}

func keep(t osm.Tags) map[string]string {
	out := map[string]string{}
	for _, x := range t {
		if keepTag.MatchString(x.Key) {
			out[x.Key] = x.Value
		}
	}
	return out
}

func scan(ctx context.Context, path string, fn func(osm.Object)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := osmpbf.New(ctx, f, runtime.GOMAXPROCS(0))
	defer sc.Close()
	for sc.Scan() {
		fn(sc.Object())
	}
	return sc.Err()
}

// ExtractFile — PBF fayldan POI lar (ikki o'tish: node/way, so'ng way markazlari uchun node koordinatalari).
func ExtractFile(ctx context.Context, path string) ([]POI, error) {
	var pois []POI
	type wayHit struct {
		group, kind string
		tags        map[string]string
		nodes       []osm.NodeID
		id          osm.WayID
	}
	var ways []wayHit
	need := map[osm.NodeID]bool{}

	// 1-o'tish: mos keladigan node lar (to'g'ridan-to'g'ri) va way lar (markaz uchun node ro'yxati)
	err := scan(ctx, path, func(o osm.Object) {
		switch v := o.(type) {
		case *osm.Node:
			if g, k := classify(v.Tags); g != "" {
				pois = append(pois, POI{OSM: fmt.Sprintf("node/%d", v.ID), Group: g, Kind: k, Lat: v.Lat, Lng: v.Lon, Tags: keep(v.Tags)})
			}
		case *osm.Way:
			if g, k := classify(v.Tags); g != "" {
				ids := v.Nodes.NodeIDs()
				for _, id := range ids {
					need[id] = true
				}
				ways = append(ways, wayHit{g, k, keep(v.Tags), ids, v.ID})
			}
		}
	})
	if err != nil {
		return nil, err
	}
	// 2-o'tish: way node larining koordinatalari
	coords := make(map[osm.NodeID][2]float64, len(need))
	err = scan(ctx, path, func(o osm.Object) {
		if n, ok := o.(*osm.Node); ok && need[n.ID] {
			coords[n.ID] = [2]float64{n.Lat, n.Lon}
		}
	})
	if err != nil {
		return nil, err
	}
	for _, w := range ways {
		var la, lo float64
		n := 0
		for _, id := range w.nodes {
			if c, ok := coords[id]; ok {
				la, lo, n = la+c[0], lo+c[1], n+1
			}
		}
		if n == 0 {
			continue
		}
		pois = append(pois, POI{OSM: fmt.Sprintf("way/%d", w.id), Group: w.group, Kind: w.kind, Lat: la / float64(n), Lng: lo / float64(n), Tags: w.tags})
	}
	return pois, nil
}
