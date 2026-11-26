package api

import (
	"context"
	"strconv"
	"testing"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
	"yoldosh/internal/platform"
)

func TestDiscovery(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "discover-device-0001")
	type sugResp struct {
		ID                int64   `json:"id"`
		Status            string  `json:"status"`
		PlaceID           *string `json:"placeId"`
		SuggestedCategory *string `json:"suggestedCategory"`
	}
	// 1) yangi choyxona — kategoriya nomdan aniqlanadi, navbatga tushadi
	var s1 sugResp
	if code := c.do("POST", "/api/v1/places/suggest", map[string]any{"name": "Mahalla choyxonasi Test", "location": map[string]float64{"lat": 39.6601, "lng": 66.9655},
		"note": "osh juda mazali", "instagram": "@test_choyxona"}, &s1); code != 202 {
		t.Fatalf("suggest: %d", code)
	}
	if s1.Status != "queued" || s1.SuggestedCategory == nil || *s1.SuggestedCategory != "cafe" {
		t.Fatalf("taklif: %+v", s1)
	}
	// 2) Registon — bazada bor, takror sifatida bog'lanadi
	var s2 sugResp
	c.do("POST", "/api/v1/places/suggest", map[string]any{"name": "Registon", "location": map[string]float64{"lat": 39.6549, "lng": 66.9757}}, &s2)
	if s2.Status != "duplicate" || s2.PlaceID == nil || *s2.PlaceID != "registan" {
		t.Fatalf("takror: %+v", s2)
	}
	// 3) validatsiya
	if code := c.do("POST", "/api/v1/places/suggest", map[string]any{"name": "X", "location": map[string]float64{"lat": 10, "lng": 10}}, nil); code != 400 {
		t.Fatalf("validatsiya: %d", code)
	}

	// Wikidata topilmasi: ishonch yuqori → avtomatik chop etiladi; nomi bazadagi joyga o'xshash → takror
	db, err := platform.OpenDB(context.Background(), testDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	items := []discover.Found{
		{Source: "wikidata", Ref: "Q990001", Names: map[string]string{"uz": "Test Kulol muzeyi", "en": "Test Potter Museum"},
			Loc: domain.LatLng{Lat: 39.6620, Lng: 66.9800}, Trusted: true, Signals: discover.Signals{WDClasses: []string{"Q33506"}},
			Desc: map[string]string{"en": "museum of pottery in Samarkand"}},
		{Source: "wikidata", Ref: "Q990002", Names: map[string]string{"uz": "Registon maydoni"},
			Loc: domain.LatLng{Lat: 39.6551, Lng: 66.9760}, Trusted: true, Signals: discover.Signals{WDClasses: []string{"Q839954"}}},
		{Source: "wikidata", Ref: "Q990003", Names: map[string]string{"uz": "Uzoq qal'a"},
			Loc: domain.LatLng{Lat: 44.0, Lng: 58.0}, Trusted: true, Signals: discover.Signals{WDClasses: []string{"Q23413"}}},
	}
	rep, err := discover.Ingest(context.Background(), db, "wikidata", items, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Published != 1 || rep.Duplicate != 1 || rep.OutOfArea != 1 || rep.PublishedIDs[0] != "wd-q990001" {
		t.Fatalf("ingest: %+v", rep)
	}
	var p placeDTO
	if code := c.do("GET", "/api/v1/places/wd-q990001", nil, &p); code != 200 || p.Category != "museum" || p.Tagline == "" {
		t.Fatalf("wikidata joyi: %d %+v", code, p.placeSummaryDTO)
	}
	// qayta ishga tushirish — yangi joy yaratilmaydi
	rep, _ = discover.Ingest(context.Background(), db, "wikidata", items, false)
	if rep.New != 0 || rep.Published != 0 {
		t.Fatalf("qayta ingest: %+v", rep)
	}

	// Admin navbati: taklifni tasdiqlash (kategoriyani to'g'rilab) va rad etish
	a := adminClient(t)
	var list page[discoveryDTO]
	if code := a.do("GET", "/api/v1/admin/discoveries?status=queued&source=user", nil, &list); code != 200 || len(list.Items) == 0 {
		t.Fatalf("admin ro'yxat: %d %+v", code, list)
	}
	var ok map[string]string
	if code := a.do("POST", "/api/v1/admin/discoveries/"+strconv.FormatInt(s1.ID, 10)+"/approve", map[string]any{"category": "restaurant"}, &ok); code != 200 {
		t.Fatalf("approve: %d", code)
	}
	if code := c.do("GET", "/api/v1/places/"+ok["placeId"], nil, &p); code != 200 || p.Category != "restaurant" || p.Trust != "community" {
		t.Fatalf("tasdiqlangan joy: %d %+v", code, p.placeSummaryDTO)
	}
	if code := a.do("POST", "/api/v1/admin/discoveries/"+strconv.FormatInt(s1.ID, 10)+"/approve", nil, nil); code != 409 {
		t.Fatalf("ikkinchi approve: %d", code)
	}
	var s3 sugResp
	c.do("POST", "/api/v1/places/suggest", map[string]any{"name": "Spam joy", "location": map[string]float64{"lat": 39.70, "lng": 66.90}}, &s3)
	if code := a.do("POST", "/api/v1/admin/discoveries/"+strconv.FormatInt(s3.ID, 10)+"/reject", nil, nil); code != 204 {
		t.Fatalf("reject: %d", code)
	}
}
