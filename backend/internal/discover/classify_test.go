package discover

import (
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		s    Signals
		cat  string
		auto bool // ishonch AutoPublish dan yuqori bo'lishi kerakmi
	}{
		{"osm muzey", Signals{OSM: map[string]string{"tourism": "museum"}, Names: map[string]string{"uz": "Afrosiyob muzeyi"}}, "museum", true},
		{"osm kafe", Signals{OSM: map[string]string{"amenity": "cafe"}, Names: map[string]string{"uz": "Bon"}}, "cafe", true},
		{"tarixiy masjid", Signals{OSM: map[string]string{"historic": "yes", "building": "mosque"}, Names: map[string]string{"uz": "Bolo Hovuz masjidi"}}, "mosque", true},
		{"madrasa (wikidata)", Signals{WDClasses: []string{"Q132834"}, Names: map[string]string{"ru": "Медресе Улугбека"}}, "monument", true},
		{"choyxona nomdan", Signals{Names: map[string]string{"uz": "Lyabi Hovuz choyxonasi"}}, "cafe", false},
		{"osh markazi", Signals{Names: map[string]string{"uz": "Besh Qozon osh markazi"}}, "restaurant", false},
		{"kulolchilik ustaxonasi", Signals{Names: map[string]string{"uz": "G'ijduvon kulolchilik ustaxonasi"}}, "workshop", false},
		{"craft teg", Signals{OSM: map[string]string{"craft": "pottery"}, Names: map[string]string{"en": "Narzullaev ceramics"}}, "workshop", true},
		{"bozor", Signals{OSM: map[string]string{"amenity": "marketplace"}, Names: map[string]string{"uz": "Siyob bozori"}}, "bazaar", true},
		{"supermarket bozor emas", Signals{Names: map[string]string{"uz": "Korzinka supermarket"}}, "", false},
		{"teatr ruscha", Signals{Names: map[string]string{"ru": "Театр Ильхом"}}, "theatre", false},
		{"bog'", Signals{OSM: map[string]string{"leisure": "park"}, Names: map[string]string{"uz": "Navoiy bog'i"}}, "park", true},
		{"nomzod hint + nom", Signals{Hint: "gallery", Names: map[string]string{"uz": "Bukhara Art Gallery"}}, "gallery", false},
		{"signal yo'q", Signals{Names: map[string]string{"uz": "Ali aka"}}, "", false},
	}
	for _, c := range cases {
		r := Classify(c.s)
		if r.Category != c.cat {
			t.Errorf("%s: kategoriya %q, kutilgan %q (%v)", c.name, r.Category, c.cat, r.Reasons)
			continue
		}
		if c.auto != (r.Confidence >= AutoPublish) {
			t.Errorf("%s: ishonch %.2f (auto=%v kutilgan)", c.name, r.Confidence, c.auto)
		}
	}
	// qarama-qarshi signallar ishonchni pasaytiradi: OSM restoran, nomi "muzey"
	r := Classify(Signals{OSM: map[string]string{"amenity": "restaurant"}, Names: map[string]string{"uz": "Muzey restorani"}})
	if r.Category != "restaurant" || r.Confidence >= 0.95 {
		t.Errorf("qarama-qarshi: %+v", r)
	}
	if r := Classify(Signals{WDClasses: []string{"Q33506"}, Names: map[string]string{"en": "X"}, Sitelinks: 20}); r.Novelty != "iconic" {
		t.Errorf("mashhur: %+v", r)
	}
	if r := Classify(Signals{OSM: map[string]string{"amenity": "cafe"}, NewSince: true}); r.Novelty != "new" {
		t.Errorf("yangi: %+v", r)
	}
}

func TestParseWikidata(t *testing.T) {
	js := `{"results":{"bindings":[
	 {"item":{"value":"http://www.wikidata.org/entity/Q1"},"coord":{"value":"Point(66.975 39.655)"},"class":{"value":"http://www.wikidata.org/entity/Q132834"},
	  "sl":{"value":"30"},"uz":{"value":"Ulug'bek madrasasi"},"ru":{"value":"Медресе Улугбека"},"heritage":{"value":"http://www.wikidata.org/entity/Q9259"}},
	 {"item":{"value":"http://www.wikidata.org/entity/Q1"},"coord":{"value":"Point(66.975 39.655)"},"class":{"value":"http://www.wikidata.org/entity/Q33506"},"sl":{"value":"30"}},
	 {"item":{"value":"http://www.wikidata.org/entity/Q2"},"coord":{"value":"Point(64.4 39.77)"},"class":{"value":"http://www.wikidata.org/entity/Q33506"},
	  "sl":{"value":"1"},"en":{"value":"New Museum"},"inception":{"value":"2025-03-01T00:00:00Z"}},
	 {"item":{"value":"http://www.wikidata.org/entity/Q3"},"coord":{"value":"Point(64.4 39.77)"},"class":{"value":"http://www.wikidata.org/entity/Q33506"},"sl":{"value":"0"}}
	]}}`
	out, err := parseWikidata(strings.NewReader(js), time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("nomsiz obyekt tashlanishi kerak: %d", len(out))
	}
	if len(out[0].Signals.WDClasses) != 2 || !out[0].Signals.Heritage || out[0].Loc.Lat != 39.655 {
		t.Fatalf("Q1: %+v", out[0])
	}
	if r := Classify(out[0].Signals); r.Category != "monument" || r.Novelty != "iconic" || r.Confidence < AutoPublish {
		t.Fatalf("Q1 klass: %+v", r)
	}
	if !out[1].Signals.NewSince {
		t.Fatalf("Q2 yangi bo'lishi kerak: %+v", out[1])
	}
}

func TestSameName(t *testing.T) {
	same := [][2]string{{"Ulug'bek madrasasi", "Ulugʻbek madrasasi"}, {"Hazrati-Xizr masjidi", "Hazrati Xizr masjidi"},
		{"Медресе Улугбека", "Медресе Улугбек"}, {"Qo'qon Jome masjidi", "Qoʻqon jome masjidi"}, {"Dorut Tilovat majmuasi", "Dorut Tilovat majmuasi va Ko'k Gumbaz"},
		{"Mo'ynoq kemalar qabristoni", "Moynoq kemalar qabristoni"}, {"Chor Chinor bog'i", "Chorchinor bog'i"}}
	diff := [][2]string{{"Said Kamol madrasasi", "Ko'kaldosh madrasasi"}, {"Медресе Абдулазизхана", "Медресе Амир Олимхана"},
		{"Modarixon madrasasi", "Abdullaxon madrasasi"}, {"Bog'bonli masjid", "Said Ota masjidi"}, {"Yunusxon maqbarasi", "Мавзолей"},
		{"Vali masjidi", "Vahid masjidi"}, {"Kalta Minor", "Minor Cafe"}, {"Islom Xo'ja minorasi va madrasasi", "Minor Cafe"},
		{"Mirzaboshi choyxonasi", "Matrasulboy Mirzaboshi madrasasi"}}
	for _, p := range same {
		if !SameName(p[0], p[1]) {
			t.Errorf("bir xil bo'lishi kerak: %q %q (%q %q)", p[0], p[1], NormName(p[0]), NormName(p[1]))
		}
	}
	for _, p := range diff {
		if SameName(p[0], p[1]) {
			t.Errorf("farqli bo'lishi kerak: %q %q (%q %q)", p[0], p[1], NormName(p[0]), NormName(p[1]))
		}
	}
}
