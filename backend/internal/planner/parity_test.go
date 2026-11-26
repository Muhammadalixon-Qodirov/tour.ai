package planner

// Ilova va saytdagi rejalashtirgich bilan moslik: haqiqiy seed katalogida (seed/<city>.json) tekshiriladi.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"yoldosh/internal/domain"
)

type seedCatalog struct {
	City struct {
		Center       domain.LatLng `json:"center"`
		TaxiBaseFare int64         `json:"taxiBaseFare"`
		TaxiPerKm    int64         `json:"taxiPerKm"`
	} `json:"city"`
	Places []struct {
		ID           string         `json:"id"`
		Category     string         `json:"category"`
		Novelty      domain.Novelty `json:"novelty"`
		Lat, Lng     float64
		Interests    []string `json:"interests"`
		VisitMin     int      `json:"visitMin"`
		Hours        string   `json:"hours"`
		Rating       float64  `json:"rating"`
		Crowd        string   `json:"crowd"`
		IsPartner    bool     `json:"isPartner"`
		Wheelchair   bool     `json:"wheelchair"`
		FeeLocal     *int64   `json:"feeLocal"`
		FeeForeigner *int64   `json:"feeForeigner"`
	} `json:"places"`
}

func loadCatalog(t *testing.T, cityID string) (City, []Candidate) {
	t.Helper()
	raw, err := os.ReadFile("../../seed/" + cityID + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var sc seedCatalog
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	var out []Candidate
	for _, p := range sc.Places {
		var h []domain.OpeningHours
		if o, c, ok := strings.Cut(p.Hours, "-"); ok {
			h = daily(o, c)
		}
		out = append(out, Candidate{ID: p.ID, Category: p.Category, Novelty: p.Novelty, IsPartner: p.IsPartner,
			Interests: p.Interests, Loc: domain.LatLng{Lat: p.Lat, Lng: p.Lng}, Rating: p.Rating, VisitMin: p.VisitMin,
			Hours: h, Wheelchair: p.Wheelchair, FamilyFriendly: true, Crowd: p.Crowd, FeeLocal: p.FeeLocal, FeeForeigner: p.FeeForeigner})
	}
	return City{Center: sc.City.Center, TaxiBase: sc.City.TaxiBaseFare, TaxiPerKm: sc.City.TaxiPerKm}, out
}

var parityStart = time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)

// Samarqandga birinchi safar Registonsiz bo'lmaydi: kvotadan keyin mashhur joylar ustun, zanjir markazdan boshlanadi.
func TestSamarkandStartsWithRegistan(t *testing.T) {
	city, cands := loadCatalog(t, "samarkand")
	days, err := Plan(Prefs{Days: 2, StartDate: parityStart, Interests: []string{"history", "food"}, Pace: "balanced"}, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	if got := days[0].Stops[0].PlaceID; got != "registan" {
		t.Fatalf("birinchi to'xtash %s, registan kutilgan", got)
	}
}

// Buxoro eski shahri piyoda: yo'llarning ko'pi yayov, hamma joy markazdan 18 km ichida.
func TestBukharaIsWalkable(t *testing.T) {
	city, cands := loadCatalog(t, "bukhara")
	byID := map[string]Candidate{}
	for _, c := range cands {
		byID[c.ID] = c
	}
	days, err := Plan(Prefs{Days: 2, StartDate: parityStart, Interests: []string{"history", "architecture"}, Pace: "balanced"}, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	walk, legs := 0, 0
	for _, d := range days {
		for _, s := range d.Stops {
			if km := domain.DistanceM(city.Center, byID[s.PlaceID].Loc); km > ReachM {
				t.Fatalf("%s markazdan %.0f m uzoqda", s.PlaceID, km)
			}
			if s.Transit != nil {
				legs++
				if s.Transit.Mode == "walk" {
					walk++
				}
			}
		}
	}
	if legs == 0 || float64(walk)/float64(legs) < 0.6 {
		t.Fatalf("Buxoroda piyoda yo'llar kam: %d/%d", walk, legs)
	}
}

// Har shahar va sur'atda: kuniga kamida 1 ta yangi joy, jami kamida 1/3, tushlik ikkinchi joydan keyin, 20:00 gacha.
func TestCatalogQuotaAndLunch(t *testing.T) {
	for _, cityID := range []string{"samarkand", "bukhara", "tashkent", "namangan"} {
		city, cands := loadCatalog(t, cityID)
		for _, pace := range []string{"relaxed", "balanced", "active"} {
			for days := 1; days <= 3; days++ {
				plan, err := Plan(Prefs{Days: days, StartDate: parityStart, Interests: []string{"history", "craft"}, Pace: pace}, city, cands)
				if err != nil {
					t.Fatalf("%s/%s/%d: %v", cityID, pace, days, err)
				}
				if v := domain.Validate(plan); len(v) > 0 {
					t.Fatalf("%s/%s/%d: %+v", cityID, pace, days, v)
				}
				if share, _ := domain.NoveltyShare(plan); share < TargetNoveltyShare-1e-9 {
					t.Fatalf("%s/%s/%d: yangilik ulushi %.2f < 1/3", cityID, pace, days, share)
				}
				for _, d := range plan {
					meal := -1
					for i, s := range d.Stops {
						if s.Kind == domain.KindMeal {
							meal = i
						}
						if s.EndMin() > DayEndMin {
							t.Fatalf("%s: %s 20:00 dan keyin tugaydi", cityID, s.PlaceID)
						}
					}
					if meal >= 0 && meal != min(2, len(d.Stops)-1) {
						t.Fatalf("%s/%s/%d kun %d: tushlik %d-o'rinda", cityID, pace, days, d.Index, meal)
					}
				}
			}
		}
	}
}

// Byudjet: kun kishi boshiga byudjetga sig'adi, faqat mashhur joylar tushadi.
func TestBudgetDropsIconicFirst(t *testing.T) {
	city, cands := loadCatalog(t, "samarkand")
	byID := map[string]Candidate{}
	for _, c := range cands {
		byID[c.ID] = c
	}
	base := Prefs{Days: 2, StartDate: parityStart, Interests: []string{"history"}, Pace: "active", GroupSize: 1}
	free, err := Plan(base, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	tight := base
	tight.BudgetPerDay = 150000
	plan, err := Plan(tight, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	kept := map[string]bool{}
	for _, d := range plan {
		if c := DayCost(d.Stops, byID, tight); c > tight.BudgetPerDay && len(d.Stops) > 2 {
			t.Fatalf("kun %d: %d so'm > byudjet", d.Index, c)
		}
		for _, s := range d.Stops {
			kept[s.PlaceID] = true
		}
	}
	dropped := 0
	for _, d := range free {
		for _, s := range d.Stops {
			if !kept[s.PlaceID] {
				dropped++
				if s.Novelty != domain.NoveltyIconic {
					t.Fatalf("%s (%s) byudjet uchun olib tashlandi", s.PlaceID, s.Novelty)
				}
			}
		}
	}
	if dropped == 0 {
		t.Fatal("byudjet hech narsani o'zgartirmadi")
	}
}

func TestTransitParity(t *testing.T) {
	c := City{Center: center, TaxiBase: 4600, TaxiPerKm: 2400}
	// 900 m to'g'ri chiziq → ~1 215 m ko'cha (10 m gacha yaxlitlanadi) → piyoda, 16 daqiqa
	w := TransitBetween(center, domain.LatLng{Lat: center.Lat + 900.0/111195, Lng: center.Lng}, c)
	if w.Mode != "walk" || w.DistanceM%10 != 0 || w.DistanceM < 1200 || w.DistanceM > 1230 || w.DurationMin != 16 {
		t.Fatalf("%+v", w)
	}
	// 1 000 m → 1 350 m → taksi: 4 600 + 1.35 × 2 400 = 7 840 → 8 000
	x := TransitBetween(center, domain.LatLng{Lat: center.Lat + 1000.0/111195, Lng: center.Lng}, c)
	if x.Mode != "taxi" || x.Fare == nil || x.Fare.Amount != 8000 || x.DurationMin != 8 {
		t.Fatalf("%+v", x)
	}
}

// Jamoa tanlagan joy OSM importidagi (Bulk) xuddi shunday joydan ustun; guruh ovozi (Boost) esa ko'taradi.
func TestCuratedAboveBulk(t *testing.T) {
	city, cands := loadCatalog(t, "samarkand")
	var reg Candidate
	for _, c := range cands {
		if c.ID == "registan" {
			reg = c
		}
	}
	osm := reg
	osm.ID, osm.Bulk = "osm-way-1", true
	cands = append(cands, osm)
	pl := newPlanner(Prefs{Days: 1, StartDate: parityStart, Interests: []string{"history"}}, city, cands)
	if pl.score["registan"] <= pl.score[osm.ID] {
		t.Fatalf("OSM joyi jamoa tanlaganidan past bo'lishi kerak: %v <= %v", pl.score["registan"], pl.score[osm.ID])
	}
	boosted := newPlanner(Prefs{Days: 1, StartDate: parityStart, Interests: []string{"history"},
		Boost: map[string]float64{osm.ID: 1}}, city, cands)
	if boosted.score[osm.ID] <= boosted.score["registan"] {
		t.Fatal("guruh ovozi ballni oshirishi kerak")
	}
}
