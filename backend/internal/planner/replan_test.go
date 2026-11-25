package planner

import (
	"math/rand/v2"
	"testing"
	"time"

	"yoldosh/internal/domain"
)

func noOverlap(t *testing.T, stops []domain.PlanStop) {
	t.Helper()
	if v := domain.Validate([]domain.PlanDay{{Stops: stops}}); len(v) > 0 {
		for _, x := range v {
			if x.Code == "overlap" || x.Code == "empty_reason" {
				t.Fatalf("invariant: %+v", v)
			}
		}
	}
}

func dayPlan(t *testing.T, seed uint64, pace string) (*Replanner, []domain.PlanStop, int) {
	r := rand.New(rand.NewPCG(seed, seed+1))
	cands := genCity(r, 45)
	p := Prefs{Days: 1, StartDate: time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), Pace: pace, Interests: []string{"history", "craft"}}
	days, err := Plan(p, city, cands)
	if err != nil {
		t.Fatal(err)
	}
	return NewReplanner(p, city, cands), days[0].Stops, int(p.StartDate.Weekday())
}

func TestLate(t *testing.T) {
	for seed := range uint64(50) {
		rp, stops, wd := dayPlan(t, seed, "active")
		out, ch := rp.Late(stops, 1, wd, 0, 90, nil)
		noOverlap(t, out)
		if out[0] != stops[0] {
			t.Fatal("o'tgan to'xtash o'zgardi")
		}
		for _, s := range out {
			if !rp.fits(s, wd) {
				t.Fatalf("sig'maydigan to'xtash qoldi: %+v", s)
			}
		}
		if len(out) > 1 && out[1].ArriveMin < stops[1].ArriveMin+90-stops[1].Transit.DurationMin {
			t.Fatalf("kechikish hisobga olinmadi")
		}
		if len(ch) == 0 || ch[0].Kind != "shifted" {
			t.Fatalf("shifted o'zgarishi yo'q: %+v", ch)
		}
		// juda katta kechikish — panika yo'q, hammasi sig'adi
		out, _ = rp.Late(stops, 0, wd, 0, 700, nil)
		noOverlap(t, out)
	}
}

func TestTiredKeepsNovel(t *testing.T) {
	for seed := range uint64(50) {
		rp, stops, wd := dayPlan(t, seed, "active")
		places := func(s []domain.PlanStop) (n, novel int) {
			for _, x := range s {
				if x.Kind == domain.KindPlace {
					n++
					if x.CountsForNovelty() {
						novel++
					}
				}
			}
			return
		}
		before, novelBefore := places(stops)
		out, _ := rp.Tired(stops, 0, wd, 0, nil)
		after, novelAfter := places(out)
		noOverlap(t, out)
		if after > (before+1)/2 {
			t.Fatalf("charchaganda %d → %d joy", before, after)
		}
		if novelBefore > 0 && novelAfter == 0 {
			t.Fatal("yagona yangi joy ham olib tashlandi")
		}
	}
}

func TestWeatherReplacesOutdoor(t *testing.T) {
	h := daily("08:00", "20:00")
	cands := []Candidate{
		{ID: "park", Category: "park", Novelty: domain.NoveltyNew, Loc: center, Rating: 4.5, VisitMin: 45, Hours: h, FamilyFriendly: true},
		{ID: "mon", Category: "monument", Novelty: domain.NoveltyIconic, Loc: domain.LatLng{Lat: center.Lat + 0.002, Lng: center.Lng}, Rating: 4.8, VisitMin: 45, Hours: h},
		{ID: "museum-old", Category: "museum", Novelty: domain.NoveltyIconic, Loc: domain.LatLng{Lat: center.Lat + 0.001, Lng: center.Lng}, Rating: 4.9, VisitMin: 45, Hours: h},
		{ID: "gallery-new", Category: "gallery", Novelty: domain.NoveltyNew, Loc: domain.LatLng{Lat: center.Lat + 0.004, Lng: center.Lng}, Rating: 4.2, VisitMin: 45, Hours: h},
	}
	rp := NewReplanner(Prefs{}, city, cands)
	stops := rp.RetimeFrom([]domain.PlanStop{
		{Kind: domain.KindPlace, PlaceID: "park", Novelty: domain.NoveltyNew, DurationMin: 45, ReasonCode: "new"},
		{Kind: domain.KindPlace, PlaceID: "mon", Novelty: domain.NoveltyIconic, DurationMin: 45, ReasonCode: "top_rated"},
	}, 3, DayStartMin, nil)
	out, ch := rp.Weather(stops, 0, 3, 0, map[string]bool{"park": true, "mon": true}, nil)
	if len(ch) != 1 || ch[0].PlaceID != "park" || out[0].PlaceID != "gallery-new" {
		t.Fatalf("yangi joy yangi joyga almashishi kerak: %+v %+v", ch, out)
	}
	noOverlap(t, out)
}

func TestFreeTime(t *testing.T) {
	for seed := range uint64(30) {
		rp, stops, wd := dayPlan(t, seed, "balanced")
		exclude := map[string]bool{}
		for _, s := range stops {
			exclude[s.PlaceID] = true
		}
		now := 13 * 60
		loc := domain.LatLng{Lat: center.Lat + 0.003, Lng: center.Lng + 0.003}
		out, ch := rp.FreeTime(stops, 2, wd, now, 120, loc, exclude)
		noOverlap(t, out)
		end := now
		for _, c := range ch {
			if c.Kind != "added" {
				continue
			}
			if exclude[c.PlaceID] {
				t.Fatalf("marshrutdagi joy qayta qo'shildi: %s", c.PlaceID)
			}
		}
		for _, s := range out[2:] {
			if s.ReasonCode == "free_time" {
				if s.ArriveMin < now || domain.DistanceM(loc, rp.byID[s.PlaceID].Loc) > freeTimeRadiusM {
					t.Fatalf("bo'sh vaqt to'xtashi noto'g'ri: %+v", s)
				}
				end = s.EndMin()
			}
		}
		if end > now+120 {
			t.Fatalf("bo'sh vaqtdan oshib ketdi: %d", end-now)
		}
	}
}
