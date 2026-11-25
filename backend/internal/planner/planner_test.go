package planner

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"yoldosh/internal/domain"
)

var center = domain.LatLng{Lat: 39.6547, Lng: 66.9758}

func daily(open, close string) []domain.OpeningHours {
	var h []domain.OpeningHours
	for d := range 7 {
		h = append(h, domain.OpeningHours{Day: d, Open: open, Close: close})
	}
	return h
}

// genCity — tasodifiy shahar: ~40% yangi/kam tanilgan, bir nechta ovqat joyi.
func genCity(r *rand.Rand, n int) []Candidate {
	cats := []string{"monument", "museum", "mosque", "bazaar", "workshop", "gallery", "park", "viewpoint"}
	var out []Candidate
	for i := range n {
		nov := domain.NoveltyIconic
		if i%5 < 2 {
			nov = []domain.Novelty{domain.NoveltyNew, domain.NoveltyHidden}[r.IntN(2)]
		}
		cat := cats[r.IntN(len(cats))]
		out = append(out, Candidate{
			ID: fmt.Sprintf("p%02d", i), Category: cat, Novelty: nov, IsPartner: r.IntN(10) == 0,
			Interests: domain.CategoryInterests[cat],
			Loc:       domain.LatLng{Lat: center.Lat + (r.Float64()-0.5)*0.05, Lng: center.Lng + (r.Float64()-0.5)*0.05},
			Rating:    3.5 + r.Float64()*1.5, Reviews: r.IntN(500), VisitMin: 30 + r.IntN(60),
			Hours: daily("09:00", "18:00"), PriceLevel: r.IntN(4),
			Wheelchair: r.IntN(2) == 0, FamilyFriendly: true,
		})
	}
	for i := range 4 {
		out = append(out, Candidate{
			ID: fmt.Sprintf("m%d", i), Category: "restaurant", Novelty: domain.NoveltyIconic,
			Loc:    domain.LatLng{Lat: center.Lat + (r.Float64()-0.5)*0.04, Lng: center.Lng + (r.Float64()-0.5)*0.04},
			Rating: 4.2, Reviews: 50, VisitMin: 60, Hours: daily("11:00", "22:00"), FamilyFriendly: true,
		})
	}
	return out
}

var city = City{Center: center, TaxiBase: 10000, TaxiPerKm: 3000}

// Xususiyat testi: tasodifiy profillar × tasodifiy shaharlar → invariantlar doim bajariladi.
func TestPlanInvariantsProperty(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	paces := []string{"relaxed", "balanced", "active"}
	for i := range 500 {
		cands := genCity(r, 30+r.IntN(20))
		var ints []string
		for _, in := range domain.Interests {
			if r.IntN(3) == 0 {
				ints = append(ints, in)
			}
		}
		p := Prefs{
			Days: 1 + r.IntN(3), StartDate: time.Date(2026, 12, 1+r.IntN(7), 0, 0, 0, 0, time.UTC),
			Interests: ints, Pace: paces[r.IntN(3)], Budget: r.IntN(4),
		}
		if r.IntN(2) == 0 {
			p.Start = &domain.LatLng{Lat: center.Lat + 0.01, Lng: center.Lng - 0.01}
		}
		days, err := Plan(p, city, cands)
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if v := domain.Validate(days); len(v) > 0 {
			t.Fatalf("#%d invariant buzildi: %+v\nprefs=%+v", i, v, p)
		}
		seen := map[string]bool{}
		for _, d := range days {
			if len(d.Stops) == 0 {
				t.Fatalf("#%d: bo'sh kun %d", i, d.Index)
			}
			places, iconic := 0, 0
			for _, s := range d.Stops {
				if s.Kind == domain.KindPlace {
					places++
					if !s.CountsForNovelty() {
						iconic++
					}
				}
			}
			if places >= 2 && iconic == 0 {
				t.Fatalf("#%d: %d-kunda mashhur joy yo'q (muvozanat buzildi)", i, d.Index)
			}
			for _, s := range d.Stops {
				if seen[s.PlaceID] {
					t.Fatalf("#%d: %s ikki marta", i, s.PlaceID)
				}
				seen[s.PlaceID] = true
				if s.EndMin() > DayEndMin {
					t.Fatalf("#%d: %s kun oxiridan keyin tugaydi", i, s.PlaceID)
				}
			}
		}
	}
}

func TestPlanRespectsWheelchair(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	cands := genCity(r, 60)
	byID := map[string]Candidate{}
	for _, c := range cands {
		byID[c.ID] = c
	}
	days, err := Plan(Prefs{Days: 2, StartDate: time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), Wheelchair: true}, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range days {
		for _, s := range d.Stops {
			if s.Kind == domain.KindPlace && !byID[s.PlaceID].Wheelchair {
				t.Fatalf("%s aravacha uchun qulay emas", s.PlaceID)
			}
		}
	}
}

// Ikki xil qiziqish → sezilarli farqli marshrut (shaxsiylashtirish ishlayapti).
func TestPersonalizationDiffers(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	cands := genCity(r, 40)
	base := Prefs{Days: 1, StartDate: time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), Pace: "active"}
	a, b := base, base
	a.Interests = []string{"history", "architecture"}
	b.Interests = []string{"nature", "art"}
	da, _ := Plan(a, city, cands)
	db, _ := Plan(b, city, cands)
	set := func(days []domain.PlanDay) map[string]bool {
		m := map[string]bool{}
		for _, s := range days[0].Stops {
			if s.Kind == domain.KindPlace {
				m[s.PlaceID] = true
			}
		}
		return m
	}
	sa, sb := set(da), set(db)
	common := 0
	for id := range sa {
		if sb[id] {
			common++
		}
	}
	if union := len(sa) + len(sb) - common; float64(common)/float64(union) > 0.6 {
		t.Fatalf("marshrutlar juda o'xshash: %v vs %v", sa, sb)
	}
}

func TestImpossible(t *testing.T) {
	if _, err := Plan(Prefs{Days: 2, StartDate: time.Now()}, city, nil); err != ErrImpossible {
		t.Fatalf("got %v", err)
	}
}

func TestTransit(t *testing.T) {
	walk := TransitBetween(center, domain.LatLng{Lat: center.Lat + 0.005, Lng: center.Lng}, city)
	if walk.Mode != "walk" || walk.Fare != nil {
		t.Fatalf("%+v", walk)
	}
	taxi := TransitBetween(center, domain.LatLng{Lat: center.Lat + 0.05, Lng: center.Lng}, city)
	if taxi.Mode != "taxi" || taxi.Fare == nil || taxi.Fare.Amount%1000 != 0 {
		t.Fatalf("%+v", taxi)
	}
}
