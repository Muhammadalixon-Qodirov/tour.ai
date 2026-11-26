package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"yoldosh/internal/platform"
)

// To'liq katalog: ilova va saytdagi 171 joy, 123 narx, 20 tadbir, 28 turar joy, bir xil ID lar.
// OSM importi (stays_test) qo'shgan yozuvlar hisobga kirmaydi.
func TestFullCatalogSeeded(t *testing.T) {
	ctx := context.Background()
	var places, prices, events, stays, oldDemo, publicPlaces, samplePrices, sampleStayPrices int
	err := testAPI.store.DB.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM places WHERE status = 'published' AND source <> 'osm'),
		(SELECT count(*) FROM price_items), (SELECT count(*) FROM events), (SELECT count(*) FROM stays WHERE source = 'research'),
		(SELECT count(*) FROM places WHERE status = 'published' AND id LIKE '%-demo'),
		(SELECT count(*) FROM places WHERE status = 'published' AND source = 'open_data' AND trust = 'public' AND hours_trust = 'sample'),
		(SELECT count(*) FROM price_items WHERE trust = 'sample'),
		(SELECT count(*) FROM stays WHERE source = 'research' AND trust = 'public' AND price_trust = 'sample')`).
		Scan(&places, &prices, &events, &stays, &oldDemo, &publicPlaces, &samplePrices, &sampleStayPrices)
	if err != nil {
		t.Fatal(err)
	}
	if places != 171 || prices != 123 || events != 20 || stays != 28 || oldDemo != 0 {
		t.Fatalf("katalog: places=%d prices=%d events=%d stays=%d demo=%d", places, prices, events, stays, oldDemo)
	}
	if publicPlaces != 171 || samplePrices != 123 || sampleStayPrices != 28 {
		t.Fatalf("ishonch darajasi: places=%d prices=%d stays=%d", publicPlaces, samplePrices, sampleStayPrices)
	}
}

func TestPlaceDetailFields(t *testing.T) {
	testAPI.SetImages(os.DirFS("../../static/places"))
	defer testAPI.SetImages(nil)
	c := &client{t: t, lang: "ru"}
	var p placeDTO
	if code := c.do("GET", "/api/v1/places/registan", nil, &p); code != 200 {
		t.Fatalf("place: %d", code)
	}
	switch {
	case !strings.HasPrefix(p.Cover.URL, "http://test/static/places/") || p.Cover.Credit == "" || p.Cover.SourceURL == "":
		t.Fatalf("rasm: %+v", p.Cover)
	case p.EntryFee == nil || p.EntryFee.Foreigner == nil || p.EntryFee.Foreigner.Amount != 100000 || p.EntryFee.Source == nil:
		t.Fatalf("kirish narxi: %+v", p.EntryFee)
	case p.Crowd == nil || p.Crowd.Level != "high" || p.Crowd.BestTime == nil || !strings.Contains(*p.Crowd.BestTime, "утра"):
		t.Fatalf("crowd: %+v", p.Crowd)
	case p.History == nil || len(p.Rules) == 0 || p.Rules[0].Source == nil:
		t.Fatalf("tarix/qoida: %+v %+v", p.History, p.Rules)
	case len(p.OpeningHours) != 7 || p.OpeningHours[0].Close != "24:00":
		t.Fatalf("ish vaqti: %+v", p.OpeningHours)
	case p.Trust != "public" || p.HoursTrust != "sample" || p.EntryFee.Trust != "sample" || p.History.Trust != "public" ||
		p.Rules[0].Trust != "public":
		t.Fatalf("ishonch: place=%s hours=%s fee=%s history=%s rule=%s", p.Trust, p.HoursTrust, p.EntryFee.Trust, p.History.Trust, p.Rules[0].Trust)
	case len(p.NearbyPrices) == 0 || p.NearbyPrices[0].ID != "p_samarkand_registan":
		t.Fatalf("nearbyPrices: %+v", p.NearbyPrices)
	}
	for _, x := range p.NearbyPrices[1:] {
		if x.Category == "ticket" {
			t.Fatalf("boshqa obidaning chiptasi: %s", x.ID)
		}
	}
	// bozor: bozor, suvenir, ovqat narxlari; chipta yo'q
	var b placeDTO
	c.do("GET", "/api/v1/places/siab-bazaar", nil, &b)
	if len(b.NearbyPrices) == 0 || b.NearbyPrices[0].Category != "market" {
		t.Fatalf("bozor narxlari: %+v", b.NearbyPrices)
	}

	resp, err := http.Get(strings.Replace(p.Cover.URL, "http://test", testSrv.URL, 1))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("rasm berilmadi: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if resp, _ := http.Get(testSrv.URL + "/static/places/..%2f..%2fgo.mod"); resp.StatusCode != 404 {
		t.Fatalf("yo'l tekshiruvi: %d", resp.StatusCode)
	}
}

func TestPricesAndETag(t *testing.T) {
	c := &client{t: t, lang: "uz"}
	var list []priceItemDTO
	if code := c.do("GET", "/api/v1/prices?city=samarkand", nil, &list); code != 200 || len(list) != 35 {
		t.Fatalf("prices: %d %d", code, len(list))
	}
	if list[0].Category != "ticket" || list[0].SampleSize != 0 || list[0].Source == nil || list[0].UpdatedAt == "" || list[0].Trust != "sample" {
		t.Fatalf("narx: %+v", list[0])
	}
	var tickets []priceItemDTO
	c.do("GET", "/api/v1/prices?city=bukhara&category=ticket&q=ark", nil, &tickets)
	if len(tickets) != 1 || tickets[0].ID != "p_bukhara_ark" {
		t.Fatalf("filtr: %+v", tickets)
	}

	req, _ := http.NewRequest("GET", testSrv.URL+"/api/v1/prices?city=samarkand", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	req.Header.Set("If-None-Match", etag)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if etag == "" || resp.StatusCode != 304 {
		t.Fatalf("etag %q → %d", etag, resp.StatusCode)
	}
	req.Header.Set("Accept-Language", "ru")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("boshqa til uchun 304 bo'lmasligi kerak: %d", resp.StatusCode)
	}
}

func TestPriceReports(t *testing.T) {
	anon := &client{t: t, lang: "en"}
	var res priceReportResp
	body := map[string]any{"priceItemId": "p_bukhara_taxi_short", "paid": map[string]any{"amount": 11000, "currency": "UZS"},
		"location": map[string]float64{"lat": 39.77345, "lng": 64.42111}}
	if code := anon.do("POST", "/api/v1/prices/reports", body, &res); code != 201 || res.Status != "approved" ||
		res.PriceItem == nil || res.PriceItem.SampleSize != 1 {
		t.Fatalf("anonim kuzatuv: %d %+v", 0, res)
	}
	// turistga aytilgan 5 baravar narx — moderatsiyaga, oraliq o'zgarmaydi
	u := newClient(t, "price-reporter-01")
	body["paid"] = map[string]any{"amount": 60000}
	if code := u.do("POST", "/api/v1/prices/reports", body, &res); code != 201 || res.Status != "pending" || res.PriceItem.SampleSize != 1 {
		t.Fatalf("g'ayrioddiy narx: %+v", res)
	}
	// 5 ta tasdiqlangan kuzatuv → oraliq kuzatuvlardan
	for _, v := range []int{12000, 12000, 12000, 12000} {
		body["paid"] = map[string]any{"amount": v}
		u.do("POST", "/api/v1/prices/reports", body, &res)
	}
	if res.PriceItem.SampleSize != 5 || res.PriceItem.FairRange.Min.Amount != 11000 || res.PriceItem.FairRange.Max.Amount != 12000 ||
		res.PriceItem.Trust != "community" {
		t.Fatalf("qayta hisoblash: %+v", res.PriceItem)
	}
	// narx bandisiz kuzatuv ham qabul qilinadi (moderatsiyaga)
	if code := anon.do("POST", "/api/v1/prices/reports", map[string]any{"cityId": "tashkent", "name": "Somsa", "category": "food",
		"paid": map[string]any{"amount": 9000}}, &res); code != 201 || res.Status != "pending" {
		t.Fatalf("erkin kuzatuv: %d %+v", code, res)
	}
	var e struct{ Error apiError }
	if code := anon.do("POST", "/api/v1/prices/reports", map[string]any{"name": "x", "category": "nope", "paid": map[string]any{"amount": 1}}, &e); code != 400 ||
		e.Error.Fields["category"] == "" || e.Error.Fields["paid.amount"] == "" {
		t.Fatalf("validatsiya: %d %+v", code, e)
	}
	if code := anon.do("POST", "/api/v1/prices/reports", map[string]any{"priceItemId": "nope", "paid": map[string]any{"amount": 1000}}, &e); code != 404 {
		t.Fatalf("noma'lum band: %d", code)
	}
}

func TestEventsAndStays(t *testing.T) {
	c := &client{t: t, lang: "en"}
	var ev []eventDTO
	if code := c.do("GET", "/api/v1/events?city=tashkent", nil, &ev); code != 200 || len(ev) != 10 {
		t.Fatalf("events: %d %d", code, len(ev))
	}
	if ev[0].StartsAt > ev[len(ev)-1].StartsAt || !strings.HasSuffix(ev[0].StartsAt, "Z") || ev[0].Cover.URL == "" {
		t.Fatalf("tartib/format: %+v", ev[0])
	}
	var dec []eventDTO
	c.do("GET", "/api/v1/events?city=tashkent&from=2026-12-01&to=2026-12-31", nil, &dec)
	for _, e := range dec {
		if e.StartsAt > "2027-01-01" {
			t.Fatalf("to filtri: %s", e.StartsAt)
		}
	}
	// o'tib ketgan tadbirlar ko'rinmaydi
	old := testAPI.now
	testAPI.now = func() time.Time { return time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC) }
	var later []eventDTO
	c.do("GET", "/api/v1/events?from=2026-01-01", nil, &later)
	testAPI.now = old
	for _, e := range later {
		if e.EndsAt < "2027-03-01" {
			t.Fatalf("o'tgan tadbir: %s %s", e.ID, e.EndsAt)
		}
	}

	// Tadqiqot turar joylari upstream /stays shaklida: 5 ballik reyting, manbalar, namuna narx, rasm muallifi.
	var stays page[stayDTO]
	if code := c.do("GET", "/api/v1/stays?city=bukhara&near=39.7747,64.4286&radiusM=20000&limit=50", nil, &stays); code != 200 {
		t.Fatalf("stays: %d", code)
	}
	research := 0
	for i, s := range stays.Items {
		if s.DistanceM == nil || i > 0 && *s.DistanceM < *stays.Items[i-1].DistanceM {
			t.Fatalf("masofa tartibi: %+v", s)
		}
		if s.RatingSource == nil {
			continue
		}
		research++
		switch {
		case s.Rating == nil || *s.Rating > 5 || s.Trust != "public":
			t.Fatalf("reyting: %+v", s)
		case s.PricePerNight.Min == nil || s.PricePerNight.Estimated || s.PricePerNight.Trust == nil ||
			*s.PricePerNight.Trust != "sample" || s.PricePerNight.Source == nil:
			t.Fatalf("narx: %+v", s.PricePerNight)
		case len(s.Amenities) == 0 || s.Amenities[0] != "wifi":
			t.Fatalf("qulayliklar: %+v", s.Amenities)
		case len(s.Images) > 0 && (!strings.HasPrefix(s.Cover.URL, "http://test/static/places/") || s.Cover.Credit == ""):
			t.Fatalf("rasm: %+v", s.Cover)
		}
	}
	if research != 7 {
		t.Fatalf("Buxoro tadqiqot turar joylari: %d", research)
	}
}

func TestTripEditingFull(t *testing.T) {
	c := newClient(t, "edit-device-full-01")
	if code := c.do("PATCH", "/api/v1/me", map[string]any{"isResident": true}, nil); code != 200 {
		t.Fatalf("me: %d", code)
	}
	var me meDTO
	c.do("GET", "/api/v1/me", nil, &me)
	if me.IsResident == nil || !*me.IsResident {
		t.Fatal("isResident saqlanmadi")
	}
	tr := mkTrip(t, c, map[string]any{"cityId": "samarkand", "startDate": "2026-12-02", "days": 2, "interests": []string{"history", "food"},
		"pace": "balanced", "budgetPerDay": 500000, "groupSize": 2})
	if tr.Prefs.CityID != "samarkand" || tr.Prefs.BudgetPerDay == nil || *tr.Prefs.BudgetPerDay != 500000 ||
		tr.Prefs.IsResident == nil || !*tr.Prefs.IsResident || len(tr.Prefs.Interests) != 2 {
		t.Fatalf("prefs: %+v", tr.Prefs)
	}
	if tr.Days[0].Stops[0].Place.ID != "registan" || tr.Totals.EstimatedCost.Amount == 0 {
		t.Fatalf("reja: %s %d", tr.Days[0].Stops[0].Place.ID, tr.Totals.EstimatedCost.Amount)
	}
	path := func(st stopDTO, suffix string) string { return "/api/v1/trips/" + tr.ID + "/stops/" + st.ID + suffix }

	// baho
	var out tripDTO
	first := tr.Days[0].Stops[0]
	if code := c.do("PATCH", path(first, ""), map[string]any{"status": "visited", "rating": 5}, &out); code != 200 ||
		out.Days[0].Stops[0].Rating == nil || *out.Days[0].Stops[0].Rating != 5 || out.Days[0].Stops[0].Status != "visited" {
		t.Fatalf("baho: %+v", out.Days[0].Stops[0])
	}
	// boshqa kunga, boshiga
	mv := tr.Days[0].Stops[1]
	if code := c.do("POST", path(mv, "/move"), map[string]any{"dayIndex": 1, "position": 0}, &out); code != 200 ||
		out.Days[1].Stops[0].ID != mv.ID || len(out.Days[0].Stops) != len(tr.Days[0].Stops)-1 || out.Days[1].Stops[0].ArriveAt != "09:00" {
		t.Fatalf("move: %d", len(out.Days[1].Stops))
	}
	// kun ichida bir pog'ona pastga
	if code := c.do("POST", path(mv, "/move"), map[string]any{"step": 1}, &out); code != 200 || out.Days[1].Stops[1].ID != mv.ID {
		t.Fatal("step")
	}
	for _, d := range out.Days {
		for i := 1; i < len(d.Stops); i++ {
			prev, cur := d.Stops[i-1], d.Stops[i]
			tr := 0
			if cur.TransitFromPrev != nil {
				tr = cur.TransitFromPrev.DurationMin
			}
			if hhmm(t, cur.ArriveAt) < hhmm(t, prev.ArriveAt)+prev.DurationMin+tr {
				t.Fatalf("vaqtlar qayta hisoblanmadi: %+v %+v", prev, cur)
			}
		}
	}
	var e struct{ Error apiError }
	if code := c.do("POST", path(out.Days[1].Stops[0], "/move"), map[string]any{"step": -1}, &e); code != 400 {
		t.Fatalf("chet: %d", code)
	}
	// joyni almashtirish
	target := out.Days[1].Stops[1]
	if code := c.do("PATCH", path(target, ""), map[string]any{"placeId": "afrasiab-museum"}, &out); code != 200 ||
		out.Days[1].Stops[1].Place.ID != "afrasiab-museum" || out.Days[1].Stops[1].ID != target.ID {
		t.Fatalf("almashtirish: %+v", out.Days[1].Stops[1].Place)
	}
	if code := c.do("PATCH", path(target, ""), map[string]any{"placeId": "registan"}, &e); code != 409 {
		t.Fatalf("takror joy: %d", code)
	}
	// o'chirish
	if code := c.do("DELETE", "/api/v1/trips/"+tr.ID, nil, nil); code != 204 {
		t.Fatalf("delete trip: %d", code)
	}
	if code := c.do("GET", "/api/v1/trips/"+tr.ID, nil, &e); code != 404 {
		t.Fatalf("o'chirilgan: %d", code)
	}
}

func TestClientIPTrustedProxy(t *testing.T) {
	p1, _ := platform.ParsePrefix("127.0.0.1")
	p2, _ := platform.ParsePrefix("10.20.0.0/16")
	s := &Server{cfg: platform.Config{TrustedProxies: []netip.Prefix{p1, p2}}}
	cases := []struct{ remote, xff, want string }{
		{"203.0.113.9:1234", "1.2.3.4", "203.0.113.9"},                         // ishonchsiz manba: sarlavha e'tiborsiz
		{"127.0.0.1:5000", "198.51.100.7", "198.51.100.7"},                     // nginx orqali
		{"127.0.0.1:5000", "6.6.6.6, 198.51.100.7, 10.20.1.5", "198.51.100.7"}, // sayt proksisi + nginx; chapdagi soxta
		{"127.0.0.1:5000", "", "127.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := s.clientIP(r); got != c.want {
			t.Fatalf("%+v → %s", c, got)
		}
	}
}
