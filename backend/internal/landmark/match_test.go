package landmark

import "testing"

func d(m float64) *float64 { return &m }

// Samarqand markazidagi joylar (Registon atrofi), nomlar bazadagidek uz/ru/en.
func samarkand() []Place {
	return []Place{
		{ID: "registan", CityID: "samarkand", Category: "monument", Names: []string{"Registon", "Registan", "Регистан"},
			Text: "Ulug'bek, Sherdor va Tillakori madrasalari bir maydonda turadi."},
		{ID: "gur-emir", CityID: "samarkand", Category: "monument", Names: []string{"Go'ri Amir maqbarasi", "Gur-Emir Mausoleum", "Мавзолей Гур-Эмир"}},
		{ID: "bibi-khanym", CityID: "samarkand", Category: "mosque", Names: []string{"Bibixonim masjidi", "Bibi-Khanym Mosque", "Мечеть Биби-Ханым"}},
		{ID: "siab-bazaar", CityID: "samarkand", Category: "bazaar", Names: []string{"Siyob bozori", "Siab Bazaar", "Сиабский базар", "Siyob"}},
		{ID: "cafe-x", CityID: "samarkand", Category: "cafe", Names: []string{"Choyxona Registon"}, Bulk: true},
	}
}

func withDist(ps []Place, m map[string]float64) []Place {
	var out []Place
	for _, p := range ps {
		if v, ok := m[p.ID]; ok {
			p.DistanceM = d(v)
			out = append(out, p)
		}
	}
	return out
}

func TestSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		ok   bool
	}{
		{"Registan", "Registon", true},
		{"Registan Square", "Регистан", true},
		{"Bibi-Khanym Mosque", "Bibixonim masjidi", true},
		{"Kalyan Minaret", "Kalon minorasi", true},
		{"Sher-Dor Madrasa", "Sherdor madrasasi", true},
		{"Ulugh Beg Madrasa", "Ulug'bek madrasasi", true},
		{"Ulugh Beg Observatory", "Ulug'bek madrasasi", false},
		{"Registan", "Siab Bazaar", false},
		{"Amir Temur monument", "Bibi-Khanym Mosque", false},
	}
	for _, c := range cases {
		if got := Similarity(c.a, c.b) >= nameMatch; got != c.ok {
			t.Errorf("%q ~ %q: %.2f", c.a, c.b, Similarity(c.a, c.b))
		}
	}
}

// Registon yonida (80 m) rasm: Registon yutadi, ishonch yuqori.
func TestMatchRegistanNearby(t *testing.T) {
	places := withDist(samarkand(), map[string]float64{"registan": 80, "gur-emir": 900, "bibi-khanym": 700, "siab-bazaar": 1100, "cafe-x": 150})
	in := Input{HasLoc: true, Type: "madrasa", City: "Samarkand", Guesses: []Guess{
		{Names: []string{"Registan", "Registon", "Регистан"}, Type: "square", Confidence: 0.9},
		{Names: []string{"Bibi-Khanym Mosque"}, Type: "mosque", Confidence: 0.2},
	}}
	r := Match(in, places)
	if r.Best == nil || r.Best.Place.ID != "registan" || r.Confidence < 0.8 {
		t.Fatalf("Registon kutilgan: %+v", r)
	}
}

// Registon ichidagi madrasa nomi taxmin qilingan: Registon tavsifida bor, 300 m ichida, Registon chiqadi.
func TestMatchMentionNearby(t *testing.T) {
	places := withDist(samarkand(), map[string]float64{"registan": 60, "gur-emir": 900})
	in := Input{HasLoc: true, Type: "madrasa", Guesses: []Guess{{Names: []string{"Sher-Dor Madrasa", "Шердор"}, Type: "madrasa", Confidence: 0.85}}}
	r := Match(in, places)
	if r.Best == nil || r.Best.Place.ID != "registan" || !r.Best.Mention {
		t.Fatalf("Registon (tavsif orqali) kutilgan: %+v", r)
	}
}

// Uzoqdagi (1.4 km) nomi mos joy yaqindagi (90 m) nomi mos joyga yutqazadi; 5 km dagi aynan nom umuman ko'rilmaydi.
func TestMatchNearbyBeatsFar(t *testing.T) {
	places := []Place{
		{ID: "temur-far", CityID: "tashkent", Category: "monument", Names: []string{"Amir Temur haykali", "Amir Timur Monument"}, DistanceM: d(1400)},
		{ID: "temur-near", CityID: "tashkent", Category: "monument", Names: []string{"Amir Temur xiyoboni haykali", "Amir Timur statue"}, DistanceM: d(90)},
		{ID: "temur-5km", CityID: "tashkent", Category: "monument", Names: []string{"Amir Temur Monument"}, DistanceM: d(5000)},
	}
	in := Input{HasLoc: true, Type: "statue", Guesses: []Guess{{Names: []string{"Amir Timur Monument", "Amir Temur haykali"}, Type: "statue", Confidence: 0.9}}}
	r := Match(in, places)
	if r.Best == nil || r.Best.Place.ID != "temur-near" {
		t.Fatalf("yaqindagi joy kutilgan: %+v", r)
	}
	for _, s := range r.Ranked {
		if s.Place.ID == "temur-5km" {
			t.Fatal("1.5 km dan uzoqdagi joy ko'rilmasligi kerak")
		}
	}
}

// Hech narsa mos kelmadi: Best yo'q, ishonch 0.
func TestMatchNoMatch(t *testing.T) {
	places := withDist(samarkand(), map[string]float64{"registan": 400, "siab-bazaar": 1200})
	in := Input{HasLoc: true, Type: "building", Guesses: []Guess{{Names: []string{"Hotel Uzbekistan"}, Type: "building", Confidence: 0.6}}}
	r := Match(in, places)
	if r.Best != nil || len(r.Ranked) != 0 || r.Confidence >= MinConfidence {
		t.Fatalf("mos kelmasligi kerak: %+v", r)
	}
	if r := Match(Input{HasLoc: true}, places); r.Best != nil {
		t.Fatal("taxminsiz natija bo'lmaydi")
	}
}

// Joylashuvsiz: ikki shaharda bir xil nom bo'lsa aniq emas; model aytgan shahar tanlovni hal qiladi.
func TestMatchNoLocationAmbiguous(t *testing.T) {
	places := []Place{
		{ID: "temur-tashkent", CityID: "tashkent", Category: "monument", Names: []string{"Amir Temur haykali"}},
		{ID: "temur-samarkand", CityID: "samarkand", Category: "monument", Names: []string{"Amir Temur haykali"}},
	}
	g := []Guess{{Names: []string{"Amir Temur haykali"}, Type: "statue", Confidence: 0.9}}
	if r := Match(Input{Guesses: g, Type: "statue"}, places); r.Best != nil {
		t.Fatalf("ikki shahar: aniq emas bo'lishi kerak: %+v", r.Best)
	}
	r := Match(Input{Guesses: g, Type: "statue", City: "Samarqand"}, places)
	if r.Best == nil || r.Best.Place.ID != "temur-samarkand" {
		t.Fatalf("shahar bo'yicha tanlanishi kerak: %+v", r)
	}
}

func TestSearchWords(t *testing.T) {
	w := SearchWords([]Guess{{Names: []string{"Gur-Emir Mausoleum", "Go'ri Amir", "Мавзолей Гур-Эмир"}}})
	want := map[string]bool{"gur": true, "emir": true, "gori": true, "amir": true, "гур": true, "эмир": true}
	for _, x := range w {
		if !want[x] {
			t.Errorf("kutilmagan so'z %q (%v)", x, w)
		}
		delete(want, x)
	}
	if len(want) > 0 {
		t.Errorf("yetishmaydi: %v", want)
	}
}
