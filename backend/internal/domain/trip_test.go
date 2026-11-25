package domain

import (
	"slices"
	"testing"
)

func st(novelty Novelty, partner bool, arrive, dur int) PlanStop {
	return PlanStop{Kind: KindPlace, PlaceID: "p", Novelty: novelty, IsPartner: partner,
		ArriveMin: arrive, DurationMin: dur, ReasonCode: "top_rated"}
}

func codes(v []Violation) []string {
	var out []string
	for _, x := range v {
		out = append(out, x.Code)
	}
	return out
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		days []PlanDay
		want []string
	}{
		{"valid", []PlanDay{{0, []PlanStop{st(NoveltyIconic, false, 540, 60), st(NoveltyNew, false, 610, 45)}}}, nil},
		{"day without novelty", []PlanDay{
			{0, []PlanStop{st(NoveltyNew, false, 540, 60), st(NoveltyHidden, false, 610, 45)}},
			{1, []PlanStop{st(NoveltyIconic, false, 540, 60)}},
		}, []string{"day_without_novelty"}},
		{"partner novel does not count", []PlanDay{{0, []PlanStop{st(NoveltyIconic, false, 540, 60), st(NoveltyNew, true, 610, 45)}}},
			[]string{"day_without_novelty", "novelty_share_low"}},
		{"share exactly 0.25 passes", []PlanDay{{0, []PlanStop{
			st(NoveltyNew, false, 540, 30), st(NoveltyIconic, false, 580, 30), st(NoveltyIconic, false, 620, 30), st(NoveltyIconic, false, 660, 30)}}}, nil},
		{"share 0.2 fails", []PlanDay{{0, []PlanStop{
			st(NoveltyNew, false, 540, 30), st(NoveltyIconic, false, 580, 30), st(NoveltyIconic, false, 620, 30),
			st(NoveltyIconic, false, 660, 30), st(NoveltyIconic, false, 700, 30)}}}, []string{"novelty_share_low"}},
		{"overlap", []PlanDay{{0, []PlanStop{st(NoveltyNew, false, 540, 60), st(NoveltyIconic, false, 590, 30)}}}, []string{"overlap"}},
		{"empty reason", []PlanDay{{0, []PlanStop{{Kind: KindPlace, Novelty: NoveltyNew, ArriveMin: 540, DurationMin: 30}}}}, []string{"empty_reason"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := codes(Validate(tc.days))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOpeningHours(t *testing.T) {
	h := []OpeningHours{{Day: 1, Open: "09:00", Close: "18:00"}}
	if !IsOpenAt(h, 1, 9*60) || IsOpenAt(h, 1, 18*60) || IsOpenAt(h, 2, 600) {
		t.Fatal("IsOpenAt noto'g'ri")
	}
	if !IsOpenAt(nil, 3, 23*60) {
		t.Fatal("bo'sh jadval = doim ochiq")
	}
}

func TestDistance(t *testing.T) {
	// Registon → Go'ri Amir ≈ 0.9 km
	d := DistanceM(LatLng{39.6547, 66.9758}, LatLng{39.6484, 66.9690})
	if d < 800 || d > 1000 {
		t.Fatalf("masofa %v", d)
	}
}
