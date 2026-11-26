package api

import (
	"math"
	"strings"
	"testing"

	"yoldosh/internal/domain"
)

// Boshlanish nuqtasi: prefs da qaytadi, har kun startLeg bilan, birinchi to'xtash nuqtaga eng yaqin joy,
// tahrirdan keyin ham shu nuqtadan hisoblanadi.
func TestTripStartPoint(t *testing.T) {
	c := newClient(t, "trip-start-0001")
	start := domain.LatLng{Lat: 39.6620, Lng: 66.9870} // Shohizinda tomonda
	body := map[string]any{"cityId": "samarkand", "startDate": "2026-12-02", "days": 2, "interests": []string{"history", "food"},
		"startLocation": start, "startLabel": "  Hotel   Bibikhanum ", "startKind": "stay"}
	var tr tripDTO
	if code := c.do("POST", "/api/v1/trips", body, &tr); code != 201 {
		t.Fatalf("create: %d", code)
	}
	if tr.Prefs.StartLabel != "Hotel Bibikhanum" || tr.Prefs.StartKind != "stay" || tr.Prefs.StartLocation == nil {
		t.Fatalf("prefs: %+v", tr.Prefs)
	}
	for _, d := range tr.Days {
		if len(d.Stops) == 0 {
			continue
		}
		first := d.Stops[0]
		if d.StartLeg == nil || first.TransitFromPrev == nil || *d.StartLeg != *first.TransitFromPrev || d.StartLeg.DurationMin <= 0 {
			t.Fatalf("kun %d startLeg: %+v / %+v", d.Index, d.StartLeg, first.TransitFromPrev)
		}
		best := math.MaxFloat64
		for _, s := range d.Stops {
			if s.Kind == domain.KindPlace {
				best = math.Min(best, domain.DistanceM(start, s.Place.Location))
			}
		}
		if got := domain.DistanceM(start, first.Place.Location); got > best+1 {
			t.Errorf("kun %d: birinchi to'xtash %s (%.0f m), eng yaqini %.0f m", d.Index, first.Place.ID, got, best)
		}
	}
	// birinchi to'xtash o'chirilsa — yangi birinchisi ham boshlanish nuqtasidan
	var after tripDTO
	if code := c.do("DELETE", "/api/v1/trips/"+tr.ID+"/stops/"+tr.Days[0].Stops[0].ID, nil, &after); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	d0 := after.Days[0]
	if d0.StartLeg == nil || d0.Stops[0].TransitFromPrev == nil || *d0.StartLeg != *d0.Stops[0].TransitFromPrev {
		t.Fatalf("tahrirdan keyin startLeg: %+v", d0.StartLeg)
	}
	if want := domain.DistanceM(start, d0.Stops[0].Place.Location) * 1.35; math.Abs(float64(d0.StartLeg.DistanceM)-want) > 20 {
		t.Fatalf("startLeg masofasi %d, kutilgan %.0f", d0.StartLeg.DistanceM, want)
	}
	// boshlanish nuqtasisiz — startLeg null
	var plain tripDTO
	c.do("POST", "/api/v1/trips", map[string]any{"cityId": "samarkand", "startDate": "2026-12-02", "days": 1, "interests": []string{"history"}}, &plain)
	if plain.Days[0].StartLeg != nil {
		t.Fatalf("startLeg bo'lmasligi kerak: %+v", plain.Days[0].StartLeg)
	}
	// tekshiruv
	var e struct{ Error apiError }
	bad := map[string]any{"cityId": "samarkand", "startDate": "2026-12-02", "days": 1, "startKind": "boat", "startLabel": strings.Repeat("a", 81)}
	if code := c.do("POST", "/api/v1/trips", bad, &e); code != 400 || e.Error.Fields["startKind"] == "" || e.Error.Fields["startLabel"] == "" {
		t.Fatalf("validatsiya: %d %+v", code, e)
	}
}
