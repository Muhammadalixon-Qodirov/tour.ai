package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"yoldosh/internal/osmimport"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
)

func TestOSMImportAndStays(t *testing.T) {
	testAPI.limiter = NewLimiter()
	pois := []osmimport.POI{
		{OSM: "node/1001", Group: "stay", Kind: "hostel", Lat: 39.6550, Lng: 66.9700, Tags: map[string]string{"name": "Test Hostel", "internet_access": "wlan"}},
		{OSM: "way/1002", Group: "stay", Kind: "hotel", Lat: 39.6560, Lng: 66.9750, Tags: map[string]string{"name": "Grand Test", "stars": "4", "phone": "+998 66 000 00 00"}},
		{OSM: "node/1003", Group: "stay", Kind: "guest_house", Lat: 39.6570, Lng: 66.9760, Tags: map[string]string{}},
		{OSM: "node/1004", Group: "food", Kind: "cafe", Lat: 39.6580, Lng: 66.9770, Tags: map[string]string{"name": "OSM kafe", "opening_hours": "Mo-Su 08:00-22:00", "cuisine": "uzbek"}},
		// mavjud "Registon maydoni" yonida o'xshash nom — takror sifatida o'tkazilishi kerak
		{OSM: "way/1005", Group: "sight", Kind: "historic:building", Lat: 39.6548, Lng: 66.9757, Tags: map[string]string{"name": "Registon"}},
		{OSM: "node/1006", Group: "service", Kind: "pharmacy", Lat: 39.6551, Lng: 66.9701, Tags: map[string]string{"opening_hours": "24/7"}},
		{OSM: "node/1007", Group: "stay", Kind: "hotel", Lat: 30.0, Lng: 50.0, Tags: map[string]string{"name": "Uzoqdagi"}},
	}
	path := filepath.Join(t.TempDir(), "pois.json")
	b, _ := json.Marshal(pois)
	os.WriteFile(path, b, 0o644)
	db, err := platform.OpenDB(context.Background(), testDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rep, err := osmimport.Load(context.Background(), db, path, 12000)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stays != 3 || rep.Places != 1 || rep.PlacesSkippedDup != 1 || rep.Services != 1 || rep.OutOfArea != 1 {
		t.Fatalf("hisobot: %+v", rep)
	}

	c := &client{t: t, lang: "uz"}
	var res page[stayDTO]
	if code := c.do("GET", "/api/v1/stays?city=samarkand&limit=50", nil, &res); code != 200 {
		t.Fatalf("stays: %d", code)
	}
	var hostel, grand *stayDTO
	for i := range res.Items {
		switch res.Items[i].Name {
		case "Test Hostel":
			hostel = &res.Items[i]
		case "Grand Test":
			grand = &res.Items[i]
		case "Nomsiz guesthouse":
			t.Fatal("nomsiz joy tekshiruvsiz chiqmasligi kerak")
		}
	}
	if hostel == nil || grand == nil {
		t.Fatalf("import qilingan joylar yo'q: %+v", res.Items)
	}
	if !hostel.PricePerNight.Estimated || hostel.PricePerNight.Min.Amount != 180000 || hostel.Amenities[0] != "wifi" {
		t.Fatalf("hostel narx bahosi: %+v", hostel.PricePerNight)
	}
	if *grand.Stars != 4 || grand.PricePerNight.Min.Amount != 840000 || grand.Contacts["phone"] == "" {
		t.Fatalf("4 yulduzli: %+v", grand)
	}
	c.do("GET", "/api/v1/stays?near=39.6550,66.9700&radiusM=300&kind=hostel", nil, &res)
	if len(res.Items) == 0 || res.Items[0].Name != "Test Hostel" || res.Items[0].DistanceM == nil {
		t.Fatalf("near filtri: %+v", res.Items)
	}

	// Admin aniq narx va E-Mehmon kiritadi → qayta import ustiga yozmaydi
	a := adminClient(t)
	var saved store.Stay
	if code := a.do("PATCH", "/api/v1/admin/stays/"+grand.ID, map[string]any{"priceMin": 950000, "priceMax": 1200000, "emehmon": true}, &saved); code != 200 {
		t.Fatalf("admin patch: %d", code)
	}
	if *saved.PriceTrust != "official" || saved.Source != "manual" || !*saved.EMehmon {
		t.Fatalf("admin tahriri: %+v", saved)
	}
	if _, err := osmimport.Load(context.Background(), db, path, 12000); err != nil {
		t.Fatal(err)
	}
	var after stayDTO
	c.do("GET", "/api/v1/stays/"+grand.ID, nil, &after)
	if after.PricePerNight.Min.Amount != 950000 || after.PricePerNight.Estimated {
		t.Fatalf("qayta import qo'lda kiritilgan narxni o'chirdi: %+v", after.PricePerNight)
	}
	// Shahar muqovasi (seed/city_covers.json) OSM importidan keyin ham saqlanadi
	var coverURL, coverCredit string
	if err := db.QueryRow(context.Background(), `SELECT cover_url, coalesce(cover_credit, '') FROM cities WHERE id = 'samarkand'`).
		Scan(&coverURL, &coverCredit); err != nil || coverURL != "/static/cities/samarkand.jpg" || coverCredit == "" {
		t.Fatalf("OSM importi shahar muqovasini o'zgartirdi: %q %q %v", coverURL, coverCredit, err)
	}

	// OSM kafe: ish vaqti o'qilgan, ommaviy import joyi
	var p placeDTO
	if code := c.do("GET", "/api/v1/places/osm-node-1004", nil, &p); code != 200 || len(p.OpeningHours) != 7 || p.Tagline != "Kafe · uzbek" {
		t.Fatalf("OSM kafe: %d %+v", code, p.placeSummaryDTO)
	}
	// SOS: demo xizmatlar haqiqiylariga almashdi
	var list []serviceDTO
	c.do("GET", "/api/v1/services?city=samarkand&kind=pharmacy", nil, &list)
	for _, s := range list {
		if s.IsDemo {
			t.Fatalf("demo xizmat qoldi: %+v", s)
		}
	}
	if len(list) == 0 || !list[0].Open24h {
		t.Fatalf("OSM dorixona: %+v", list)
	}
}
