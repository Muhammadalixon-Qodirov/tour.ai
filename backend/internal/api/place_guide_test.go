package api

import (
	"strings"
	"testing"
)

func TestPlaceGuide(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := &client{t: t, lang: "uz"}
	var g placeGuideDTO
	// shahar ichidan: piyoda + taksi
	if code := c.do("GET", "/api/v1/places/registan/guide?from=39.6600,66.9650", nil, &g); code != 200 {
		t.Fatalf("guide: %d", code)
	}
	modes := map[string]travelEstimate{}
	for _, e := range g.GetThere.Estimates {
		modes[e.Mode] = e
	}
	if modes["walk"].DurationMin == 0 || modes["taxi"].Fare == nil || modes["taxi"].Fare.Min.Amount < 4600 ||
		modes["taxi"].Fare.Max.Amount < modes["taxi"].Fare.Min.Amount {
		t.Fatalf("taxminlar: %+v", g.GetThere.Estimates)
	}
	if len(g.GetThere.Navigation.Options) < 5 || !strings.Contains(*g.GetThere.Navigation.Options[0].Web, "rtext=~39.65") {
		t.Fatalf("navigatsiya: %+v", g.GetThere.Navigation)
	}
	if len(g.Summary) < 4 || len(g.Stays) == 0 || len(g.Food) == 0 {
		t.Fatalf("summary/atrof: %v / %d stays / %d food", g.Summary, len(g.Stays), len(g.Food))
	}
	joined := strings.Join(g.Summary, "\n")
	if !strings.Contains(joined, "Piyoda") || !strings.Contains(joined, "taksi taxminan") || !strings.Contains(joined, "so'm") {
		t.Fatalf("sodda tushuntirish: %s", joined)
	}
	if strings.ContainsAny(joined, "≈~—") {
		t.Fatalf("taqiqlangan belgi: %s", joined)
	}
	// ru: narx izohi va xizmat nomlari ruscha, summary da "примерно"
	var gr placeGuideDTO
	(&client{t: t, lang: "ru"}).do("GET", "/api/v1/places/registan/guide?from=39.6600,66.9650", nil, &gr)
	for _, st := range gr.Stays {
		if n := st.PricePerNight.Note; n != nil && strings.Contains(*n, "narx") {
			t.Fatalf("ru izoh o'zbekcha: %q", *n)
		}
	}
	if !strings.Contains(strings.Join(gr.Summary, "\n"), "примерно") || strings.ContainsAny(strings.Join(gr.Summary, ""), "≈~—") {
		t.Fatalf("ru summary: %v", gr.Summary)
	}
	for kind, sv := range g.Services {
		if sv.Name == "Pharmacy" || sv.Name == "ATM" || sv.Name == "" {
			t.Fatalf("uz xizmat nomi umumiy (%s): %q", kind, sv.Name)
		}
	}
	// boshqa shahardan (Toshkent → Samarqand): poyezd/avtobus variantlari
	var g2 placeGuideDTO
	c.do("GET", "/api/v1/places/registan/guide?from=41.3111,69.2797", nil, &g2)
	if g2.GetThere.FromCityID == nil || *g2.GetThere.FromCityID != "tashkent" || len(g2.GetThere.Intercity) == 0 ||
		!strings.Contains(strings.Join(g2.Summary, "\n"), "Toshkentdan:") {
		t.Fatalf("shaharlararo: %+v / %v", g2.GetThere, g2.Summary)
	}
	// from siz — havolalar bor, taxmin yo'q
	var g3 placeGuideDTO
	c.do("GET", "/api/v1/places/registan/guide", nil, &g3)
	if len(g3.GetThere.Estimates) != 0 || len(g3.GetThere.Navigation.Options) == 0 {
		t.Fatalf("from siz: %+v", g3.GetThere)
	}
	if code := c.do("GET", "/api/v1/places/registan/guide?from=abc", nil, nil); code != 400 {
		t.Fatalf("noto'g'ri from: %d", code)
	}
}
