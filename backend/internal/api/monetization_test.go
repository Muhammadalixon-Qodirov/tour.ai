package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"yoldosh/internal/auth"
	"yoldosh/internal/domain"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
)

const moneyAdminEmail = "money-admin@test.local"

// moneyAdmin — admin roli (pul bilan bog'liq endpointlar moderatorga yopiq).
func moneyAdmin(t *testing.T) *client {
	t.Helper()
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, testDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, _ := auth.HashPassword(adminPassword)
	if err := store.New(db).UpsertAdmin(ctx, NewAdminID(), moneyAdminEmail, "Money", hash, "admin"); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, lang: "uz"}
	var res struct{ Token string }
	if code := c.do("POST", "/api/v1/admin/auth/login", map[string]string{"email": moneyAdminEmail, "password": adminPassword}, &res); code != 200 {
		t.Fatalf("login: %d", code)
	}
	c.token = res.Token
	return c
}

func sqlExec(t *testing.T, q string, args ...any) {
	t.Helper()
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, testDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ctx, q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// redirect — yo'naltirishga ergashmasdan Location ni qaytaradi.
func redirect(t *testing.T, rawURL string, headers map[string]string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", strings.Replace(rawURL, "http://test/", testSrv.URL+"/", 1), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

func resetShared(r *sharedResp) *sharedResp {
	*r = sharedResp{}
	return r
}

func (c *client) doKey(method, path, key string, body, out any) int {
	tok := c.token
	c.token = key // Bearer spk_... ham kalit sifatida qabul qilinadi
	defer func() { c.token = tok }()
	return c.do(method, path, body, out)
}

func TestIntercityRoutes(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := &client{t: t}
	var rs []intercityDTO
	if code := c.do("GET", "/api/v1/routes/intercity?from=tashkent&to=samarkand", nil, &rs); code != 200 || len(rs) != 5 {
		t.Fatalf("%d %+v", code, rs)
	}
	af := rs[1]
	if af.Mode != "afrosiyob" || af.DurationMin != 133 || af.PriceFrom.Amount != 311000 || af.UpdatedAt != "2026-01-16" || af.SourceURL == nil {
		t.Fatalf("afrosiyob: %+v", af)
	}
	if rs[0].Mode != "train" || rs[0].PriceFrom != nil || rs[0].Source != "WE Project" {
		t.Fatalf("poyezd narxsiz bo'lishi kerak: %+v", rs[0])
	}
	if car := rs[4]; car.Mode != "car" || car.DurationMin != 240 || car.PriceFrom.Amount != 360000 || !car.Estimate {
		t.Fatalf("mashina: %+v", car)
	}
	// Toshkent orqali: vaqtlar + 1 soat, narx ikkala bo'lak ma'lum bo'lsa.
	c.do("GET", "/api/v1/routes/intercity?from=namangan&to=samarkand", nil, &rs)
	if len(rs) != 3 || rs[0].Mode != "train" || rs[0].DurationMin != 266+180+60 || rs[0].PriceFrom != nil ||
		rs[0].Source != "Yandex Raspisaniya; WE Project" || rs[0].Via == nil || *rs[0].Via != "tashkent" {
		t.Fatalf("via poyezd: %+v", rs)
	}
	if rs[1].Mode != "plane" || rs[1].DurationMin != 180 || rs[1].PriceFrom.Amount != 470460 || rs[2].PriceFrom.Amount != 710000 {
		t.Fatalf("via samolyot / mashina: %+v", rs)
	}
	c.do("GET", "/api/v1/routes/intercity?from=bukhara&to=bukhara", nil, &rs)
	if len(rs) != 0 {
		t.Fatalf("bir xil shahar: %+v", rs)
	}
	var e struct{ Error apiError }
	if code := c.do("GET", "/api/v1/routes/intercity?from=tashkent", nil, &e); code != 400 || e.Error.Fields["to"] == "" {
		t.Fatalf("validatsiya: %d %+v", code, e)
	}
}

func TestOffersMatching(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := &client{t: t, lang: "uz"}
	one := func(q string) *offerDTO {
		t.Helper()
		var list []offerDTO
		if code := c.do("GET", "/api/v1/offers?"+q, nil, &list); code != 200 || len(list) > 1 {
			t.Fatalf("%s: %d %+v", q, code, list)
		}
		if len(list) == 0 {
			return nil
		}
		return &list[0]
	}
	// Eng tez: Buxoro–Samarqand orasida samolyot yo'q, Afrosiyob — poyezd hamkori; boshqa joyda samolyot hamkori.
	if o := one("slot=transport.fastest&from=bukhara&city=samarkand"); o == nil || o.ID != "so_rail_fast" || o.Partner.ID != "sp_yolchi" || !o.Sample {
		t.Fatalf("fastest bukhara: %+v", o)
	}
	o := one("slot=transport.fastest&from=tashkent&to=samarkand&date=2026-12-01&days=2")
	if o == nil || o.ID != "so_air" || o.Partner.Name != "Osmon Avia" || o.Title != "Ichki reyslar bitta ro'yxatda" || o.CTA != "Reyslarni ko'rish" ||
		!strings.HasPrefix(o.URL, "http://test/r/") {
		t.Fatalf("fastest tashkent: %+v", o)
	}
	if o := one("slot=transport.bus&from=tashkent&to=samarkand"); o != nil {
		t.Fatalf("avtobus hamkori yo'q: %+v", o)
	}
	if o := one("slot=transport.cheapest&city=samarkand"); o != nil {
		t.Fatalf("qayerdan noma'lum: %+v", o)
	}
	if o := one("slot=prep.esim&days=5"); o == nil || o.ID != "so_esim_7" || o.PriceFrom.Amount != 95000 || o.Title != "eSIM 7 kunga, 10 GB" {
		t.Fatalf("esim: %+v", o)
	}
	if o := one("slot=prep.esim&days=45"); o == nil || o.ID != "so_esim_30" {
		t.Fatalf("esim 45: %+v", o)
	}
	if o := one("slot=prep.esim&days=5&resident=true"); o != nil {
		t.Fatalf("rezidentga eSIM: %+v", o)
	}
	o = one("slot=prep.insurance&days=3&resident=true")
	if o == nil || o.ID != "so_ins_local" || o.PriceFrom.Amount != 18000 || o.Title != "Baxtsiz hodisa va bagaj sug'urtasi 3 kunga" {
		t.Fatalf("sug'urta: %+v", o)
	}
	c.lang = "en"
	if o := one("slot=prep.insurance&days=2"); o == nil || o.ID != "so_ins" || o.PriceFrom.Amount != 30000 || o.Title != "Medical insurance for 2 days" {
		t.Fatalf("insurance en: %+v", o)
	}
	var e struct{ Error apiError }
	if code := c.do("GET", "/api/v1/offers?slot=billboard", nil, &e); code != 400 {
		t.Fatalf("slot: %d", code)
	}

	// Ko'rsatish: tana bo'sh, {} yoki {count}.
	for _, body := range []any{nil, map[string]any{}, map[string]any{"count": 3}} {
		if code := c.do("POST", "/api/v1/offers/so_air/impressions", body, nil); code != 204 {
			t.Fatalf("impressions %v: %d", body, code)
		}
	}
	if code := c.do("POST", "/api/v1/offers/yoq/impressions", map[string]any{"count": 1}, &e); code != 404 || e.Error.Code != "offer_not_found" {
		t.Fatalf("yo'q taklif: %d", code)
	}
	if code := c.do("POST", "/api/v1/offers/so_air/impressions", map[string]any{"count": 500}, &e); code != 400 {
		t.Fatalf("count: %d", code)
	}

	// Namuna taklif bosilsa saytdagi xavfsiz sahifa; buzilgan havola — bosh sahifa.
	code, loc := redirect(t, o.URL, map[string]string{"Accept-Language": "ru"})
	if code != 302 || !strings.HasPrefix(loc, platform.DefaultWebBaseURL+"/ru/partner?sample=1&offer=so_ins") {
		t.Fatalf("sample click: %d %s", code, loc)
	}
	code, loc = redirect(t, o.URL[:len(o.URL)-3]+"abc", nil)
	if code != 302 || loc != platform.DefaultWebBaseURL+"/uz" {
		t.Fatalf("buzilgan: %d %s", code, loc)
	}
}

func TestPartnerOffersClicksConversions(t *testing.T) {
	testAPI.limiter = NewLimiter()
	mod := adminClient(t)
	var e struct{ Error apiError }
	if code := mod.do("GET", "/api/v1/admin/partners", nil, &e); code != 403 {
		t.Fatalf("moderator: %d", code)
	}
	a := moneyAdmin(t)
	var p store.Partner
	if code := a.do("POST", "/api/v1/admin/partners", map[string]any{"id": "p_osh_markazi", "name": "Osh markazi", "cityId": "bukhara"}, &p); code != 201 || !p.Active || p.Sample {
		t.Fatalf("partner: %d %+v", code, p)
	}
	if code := a.do("POST", "/api/v1/admin/partners", map[string]any{"id": "p_osh_markazi", "name": "Takror"}, &e); code != 409 {
		t.Fatalf("takror id: %d", code)
	}
	// Jonli taklif havolasiz bo'lmaydi.
	if code := a.do("POST", "/api/v1/admin/offers", map[string]any{"id": "o_osh", "partnerId": "p_osh_markazi", "slot": "food", "cityId": "bukhara",
		"title": map[string]string{"uz": "Osh"}, "cta": map[string]string{"uz": "Ko'rish"}}, &e); code != 400 || e.Error.Fields["linkTemplate"] == "" {
		t.Fatalf("havolasiz: %d %+v", code, e)
	}
	var of store.Offer
	if code := a.do("POST", "/api/v1/admin/offers", map[string]any{"id": "o_osh", "partnerId": "p_osh_markazi", "slot": "food", "cityId": "bukhara",
		"title": map[string]string{"uz": "Bugungi osh", "en": "Today's plov"}, "cta": map[string]string{"uz": "Joy band qilish", "en": "Book a table"},
		"linkTemplate": "https://osh.example/book?ref={subId}", "priceFrom": 45000}, &of); code != 201 || of.PriceUnit != "trip" {
		t.Fatalf("offer: %d %+v", code, of)
	}
	if code := a.do("PATCH", "/api/v1/admin/offers/o_osh", map[string]any{"priceFrom": nil, "priority": 5}, &of); code != 200 || of.PriceFrom != nil || of.Priority != 5 {
		t.Fatalf("patch offer: %d %+v", code, of)
	}
	var key struct{ ID, Key, Prefix string }
	if code := a.do("POST", "/api/v1/admin/partners/p_osh_markazi/keys", nil, &key); code != 201 || !strings.HasPrefix(key.Key, "spk_") || key.Prefix != key.Key[:8] {
		t.Fatalf("key: %d %+v", code, key)
	}

	// Mehmon taklifni ko'radi va bosadi: sub-id bilan hamkor saytiga.
	u := newClient(t, "money-guest-0001")
	var list []offerDTO
	u.do("GET", "/api/v1/offers?slot=food&city=bukhara", nil, &list)
	if len(list) != 1 || list[0].ID != "o_osh" || list[0].Sample || list[0].PriceFrom != nil {
		t.Fatalf("food: %+v", list)
	}
	u.do("POST", "/api/v1/offers/o_osh/impressions", map[string]int{"count": 2}, nil)
	code, loc := redirect(t, list[0].URL, nil)
	if code != 302 || !strings.HasPrefix(loc, "https://osh.example/book?") {
		t.Fatalf("click: %d %s", code, loc)
	}
	lu, _ := url.Parse(loc)
	sub := lu.Query().Get("ref")
	if !strings.HasPrefix(sub, "ck_") || lu.Query().Get("utm_source") != "safar" {
		t.Fatalf("sub-id: %s", loc)
	}

	// Sotuv xabari: kalit bilan; takror externalId ikki marta sanalmaydi.
	h := &client{t: t}
	var conv map[string]any
	if code := h.doKey("POST", "/api/v1/partners/conversions", key.Key, map[string]any{"clickId": sub, "amount": 90000, "currency": "uzs", "externalId": "order-1"}, &conv); code != 201 || conv["duplicate"] != false {
		t.Fatalf("conversion: %d %+v", code, conv)
	}
	if code := h.doKey("POST", "/api/v1/partners/conversions", key.Key, map[string]any{"clickId": sub, "amount": 90000, "currency": "UZS", "externalId": "order-1"}, &conv); code != 200 || conv["duplicate"] != true {
		t.Fatalf("takror: %d %+v", code, conv)
	}
	if code := h.doKey("POST", "/api/v1/partners/conversions", key.Key, map[string]any{"clickId": "ck_begona", "amount": 1, "externalId": "x"}, &e); code != 404 || e.Error.Code != "click_not_found" {
		t.Fatalf("begona click: %d", code)
	}
	if code := h.doKey("POST", "/api/v1/partners/conversions", "spk_notakey", map[string]any{"clickId": sub, "amount": 1, "externalId": "y"}, &e); code != 401 {
		t.Fatalf("noto'g'ri kalit: %d", code)
	}
	// Foydalanuvchi tokeni sotuv xabari uchun yaramaydi (faqat kalit).
	if code := u.do("POST", "/api/v1/partners/conversions", map[string]any{"clickId": sub, "amount": 1, "externalId": "z"}, &e); code != 401 {
		t.Fatalf("token bilan: %d", code)
	}

	var st struct {
		Partner                          struct{ ID string }
		Impressions, Clicks, Conversions int
		Amount                           domain.Money
		Months                           []struct{ Month string }
		Offers                           []struct{ OfferID string }
	}
	if code := h.doKey("GET", "/api/v1/partner/me/stats?from=2026-11-01&to=2026-11-30", key.Key, nil, &st); code != 200 {
		t.Fatalf("stats: %d", code)
	}
	if st.Partner.ID != "p_osh_markazi" || st.Impressions != 2 || st.Clicks != 1 || st.Conversions != 1 || st.Amount.Amount != 90000 ||
		len(st.Offers) != 1 || st.Offers[0].OfferID != "o_osh" {
		t.Fatalf("stats: %+v", st)
	}
	if code := h.doKey("GET", "/api/v1/partner/me/stats?from=2026-12-01&to=2026-11-01", key.Key, nil, &e); code != 400 {
		t.Fatalf("oraliq: %d", code)
	}

	// Kalit bekor qilinsa ishlamaydi.
	if code := a.do("DELETE", "/api/v1/admin/partners/p_osh_markazi/keys/"+key.ID, nil, nil); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if code := h.doKey("GET", "/api/v1/partner/me/stats", key.Key, nil, &e); code != 401 {
		t.Fatalf("bekor kalit: %d", code)
	}
	// Taklif o'chirilsa (active=false) endi chiqmaydi.
	a.do("PATCH", "/api/v1/admin/offers/o_osh", map[string]any{"active": false}, nil)
	u.do("GET", "/api/v1/offers?slot=food&city=bukhara", nil, &list)
	if len(list) != 0 {
		t.Fatalf("o'chirilgan taklif: %+v", list)
	}
}

func TestTripArrivalPrep(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "arrival-device-0001")
	tr := mkTrip(t, c, map[string]any{"cityId": "samarkand", "startDate": "2026-12-03", "days": 2, "interests": []string{"history"}, "companion": "friends"})
	if tr.Arrival != nil || len(tr.Prep) != 0 || tr.Totals.ArrivalCost != nil {
		t.Fatalf("boshlang'ich: %+v %+v", tr.Arrival, tr.Prep)
	}
	base := tr.Totals.EstimatedCost.Amount
	var got tripDTO
	// Mashina (360 000 so'm) 4 kishiga bo'linadi.
	if code := c.do("PATCH", "/api/v1/trips/"+tr.ID, map[string]any{"arrival": map[string]any{"from": "tashkent", "mode": "car", "choice": "comfort", "priceFrom": 360000}}, &got); code != 200 {
		t.Fatalf("arrival: %d", code)
	}
	if got.Arrival == nil || got.Arrival.Mode != "car" || got.Totals.ArrivalCost == nil || got.Totals.ArrivalCost.Amount != 90000 ||
		got.Totals.EstimatedCost.Amount != base+90000 {
		t.Fatalf("arrival cost: %+v %+v base=%d", got.Arrival, got.Totals, base)
	}
	c.do("PATCH", "/api/v1/trips/"+tr.ID, map[string]any{"arrival": map[string]any{"from": "tashkent", "mode": "afrosiyob", "choice": "fastest"}}, &got)
	if got.Totals.ArrivalCost.Amount != 311000 || got.Totals.EstimatedCost.Amount != base+311000 {
		t.Fatalf("afrosiyob: %+v", got.Totals)
	}
	if code := c.do("PATCH", "/api/v1/trips/"+tr.ID, map[string]any{"prep": map[string]string{"esim": "done", "insurance": "skipped"}}, &got); code != 200 ||
		got.Prep["esim"] != "done" || got.Prep["insurance"] != "skipped" || got.Arrival == nil {
		t.Fatalf("prep: %d %+v", code, got.Prep)
	}
	var e struct{ Error apiError }
	for _, body := range []map[string]any{
		{"prep": map[string]string{"visa": "done"}},
		{"prep": map[string]string{"esim": "maybe"}},
		{"arrival": map[string]any{"from": "samarkand", "mode": "car"}},
		{"arrival": map[string]any{"from": "tashkent", "mode": "boat"}},
		{"arrival": map[string]any{"from": "namangan", "mode": "afrosiyob"}},
		{"title": "x"},
	} {
		if code := c.do("PATCH", "/api/v1/trips/"+tr.ID, body, &e); code != 400 {
			t.Fatalf("%v: %d", body, code)
		}
	}
	c.do("PATCH", "/api/v1/trips/"+tr.ID, map[string]any{"arrival": nil}, &got)
	if got.Arrival != nil || got.Totals.EstimatedCost.Amount != base || got.Prep["esim"] != "done" {
		t.Fatalf("tozalash: %+v %+v", got.Arrival, got.Totals)
	}
	other := newClient(t, "arrival-device-0002")
	if code := other.do("PATCH", "/api/v1/trips/"+tr.ID, map[string]any{"prep": nil}, &e); code != 404 {
		t.Fatalf("begona safar: %d", code)
	}
}

func TestStoriesAndCodes(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "story-device-0001")
	var place placeDTO
	c.do("GET", "/api/v1/places/registan", nil, &place)
	var st struct{ ShareID, ShareURL string }
	if code := c.do("POST", "/api/v1/stories", map[string]any{"placeId": "registan", "location": place.Location}, &st); code != 201 ||
		st.ShareURL != platform.DefaultWebBaseURL+"/s/p/registan?ref="+st.ShareID {
		t.Fatalf("story: %d %+v", code, st)
	}
	var res sharedResp
	if code := c.do("POST", "/api/v1/stories/"+st.ShareID+"/shared", nil, resetShared(&res)); code != 200 || res.Code == nil || res.Reason != "" {
		t.Fatalf("shared: %d %+v", code, res)
	}
	code1 := res.Code
	if code1.Partner.ID != "sp_registon_choy" || code1.DiscountPct != 15 || !code1.Sample || !strings.HasPrefix(code1.Value, "SF-") ||
		code1.QRPayload != platform.DefaultWebBaseURL+"/c/"+code1.Value || code1.ExpiresAt != "2026-12-02T10:00:00Z" {
		t.Fatalf("kod: %+v", code1)
	}
	// 7 kun ichida shu hamkor yana kod bermaydi: o'sha kod qaytadi.
	var st2 struct{ ShareID string }
	c.do("POST", "/api/v1/stories", map[string]any{"placeId": "registan", "location": place.Location}, &st2)
	c.do("POST", "/api/v1/stories/"+st2.ShareID+"/shared", map[string]any{}, resetShared(&res))
	if res.Reason != "recent" || res.Code == nil || res.Code.ID != code1.ID {
		t.Fatalf("recent: %+v", res)
	}
	// Joydan uzoqda: kod yo'q.
	far := newClient(t, "story-device-0002")
	far.do("POST", "/api/v1/stories", map[string]any{"placeId": "registan", "location": map[string]float64{"lat": 39.70, "lng": 66.90}}, &st2)
	far.do("POST", "/api/v1/stories/"+st2.ShareID+"/shared", nil, resetShared(&res))
	if res.Reason != "not_on_site" || res.Code != nil {
		t.Fatalf("not_on_site: %+v", res)
	}
	// "Yetib keldim": bugungi marshrutda joy current bo'lsa joylashuvsiz ham joyda deb hisoblanadi.
	tr := mkTrip(t, far, map[string]any{"cityId": "samarkand", "startDate": "2026-11-25", "days": 1, "interests": []string{"history"}})
	var stop stopDTO
	for _, s := range tr.Days[0].Stops {
		if s.Kind == domain.KindPlace {
			stop = s
			break
		}
	}
	far.do("PATCH", "/api/v1/trips/"+tr.ID+"/stops/"+stop.ID, map[string]string{"status": "current"}, nil)
	far.do("POST", "/api/v1/stories", map[string]any{"placeId": stop.Place.ID}, &st2)
	far.do("POST", "/api/v1/stories/"+st2.ShareID+"/shared", nil, resetShared(&res))
	if res.Reason == "not_on_site" {
		t.Fatalf("current to'xtash: %+v", res)
	}
	// Hamkorsiz joy: 1.5 km ichida story hamkori yo'q.
	var places page[placeSummaryDTO]
	c.do("GET", "/api/v1/places?city=bukhara&limit=50", nil, &places)
	labi := domain.LatLng{Lat: 39.7738, Lng: 64.4214}
	lonely := ""
	var lonelyLoc domain.LatLng
	for _, p := range places.Items {
		if domain.DistanceM(p.Location, labi) > 3000 {
			lonely, lonelyLoc = p.ID, p.Location
			break
		}
	}
	if lonely == "" {
		t.Fatal("Buxoroda uzoq joy topilmadi")
	}
	c.do("POST", "/api/v1/stories", map[string]any{"placeId": lonely, "location": lonelyLoc}, &st2)
	c.do("POST", "/api/v1/stories/"+st2.ShareID+"/shared", nil, resetShared(&res))
	if res.Reason != "no_partner" || res.Code != nil {
		t.Fatalf("no_partner: %+v", res)
	}
	var e struct{ Error apiError }
	if code := far.do("POST", "/api/v1/stories/"+st.ShareID+"/shared", nil, &e); code != 404 || e.Error.Code != "story_not_found" {
		t.Fatalf("begona story: %d", code)
	}

	var codes []codeDTO
	if c.do("GET", "/api/v1/me/codes", nil, &codes); len(codes) != 1 || codes[0].ID != code1.ID {
		t.Fatalf("me/codes: %+v", codes)
	}

	// Hamkor kabineti: partner rolidagi foydalanuvchi faqat o'z kodini tekshiradi.
	a := moneyAdmin(t)
	staff := newClient(t, "story-staff-0001")
	var me meDTO
	staff.do("GET", "/api/v1/me", nil, &me)
	if code := a.do("PUT", "/api/v1/admin/users/"+me.ID+"/role", map[string]any{"role": "partner", "partnerId": "sp_registon_choy"}, nil); code != 200 {
		t.Fatalf("role: %d", code)
	}
	var red map[string]any
	if code := staff.do("POST", "/api/v1/partner/codes/redeem", map[string]string{"value": " sf" + strings.ToLower(code1.Value[3:]) + " "}, &red); code != 200 ||
		red["status"] != "ok" || red["discountPct"] != float64(15) {
		t.Fatalf("redeem: %d %+v", code, red)
	}
	staff.do("POST", "/api/v1/partner/codes/redeem", map[string]string{"value": code1.QRPayload}, &red)
	if red["status"] != "used" {
		t.Fatalf("used: %+v", red)
	}
	if c.do("GET", "/api/v1/me/codes", nil, &codes); len(codes) != 0 {
		t.Fatalf("ishlatilgan kod amaldagilar orasida: %+v", codes)
	}
	if c.do("GET", "/api/v1/me/codes?include=all", nil, &codes); len(codes) != 1 || codes[0].UsedAt == nil {
		t.Fatalf("include=all: %+v", codes)
	}
	var key struct{ Key string }
	a.do("POST", "/api/v1/admin/partners/sp_labi_kulol/keys", nil, &key)
	h := &client{t: t}
	red = nil
	h.doKey("POST", "/api/v1/partner/codes/redeem", key.Key, map[string]string{"value": code1.Value}, &red)
	if red["status"] != "not_found" || red["discountPct"] != nil {
		t.Fatalf("begona hamkor kodi: %+v", red)
	}
	// Muddati o'tgan kod.
	far2 := newClient(t, "story-device-0003")
	far2.do("POST", "/api/v1/stories", map[string]any{"placeId": "registan", "location": place.Location}, &st2)
	far2.do("POST", "/api/v1/stories/"+st2.ShareID+"/shared", nil, resetShared(&res))
	sqlExec(t, `UPDATE discount_codes SET expires_at = '2026-11-01' WHERE id = $1`, res.Code.ID)
	staff.do("POST", "/api/v1/partner/codes/redeem", map[string]string{"value": res.Code.Value}, &red)
	if red["status"] != "expired" {
		t.Fatalf("expired: %+v", red)
	}
	if code := c.do("POST", "/api/v1/partner/codes/redeem", map[string]string{"value": res.Code.Value}, &e); code != 403 {
		t.Fatalf("turist kabinetda: %d", code)
	}

	// Story havolasi: ref sanaladi, saytdagi joy sahifasiga.
	code, loc := redirect(t, testSrv.URL+"/api/v1/s/p/registan?ref="+st.ShareID, map[string]string{"Accept-Language": "en-US,en;q=0.9"})
	if code != 302 || loc != platform.DefaultWebBaseURL+"/en/places/registan?ref="+st.ShareID {
		t.Fatalf("s/p: %d %s", code, loc)
	}
	code, loc = redirect(t, testSrv.URL+"/api/v1/s/p/yoq-joy", nil)
	if code != 302 || loc != platform.DefaultWebBaseURL+"/uz/places" {
		t.Fatalf("s/p yo'q joy: %d %s", code, loc)
	}
	ctx := context.Background()
	db, _ := platform.OpenDB(ctx, testDBURL)
	defer db.Close()
	var refs int
	_ = db.QueryRow(ctx, `SELECT ref_count FROM stories WHERE share_id = $1`, st.ShareID).Scan(&refs)
	if refs != 1 {
		t.Fatalf("ref_count = %d", refs)
	}
}

func TestInsightsAndOptOut(t *testing.T) {
	testAPI.limiter = NewLimiter()
	// Statistikadan chiqish.
	out := newClient(t, "insights-optout-0001")
	var me meDTO
	if code := out.do("PATCH", "/api/v1/me", map[string]any{"analyticsOptOut": true}, &me); code != 200 || !me.AnalyticsOptOut || me.Role != "tourist" {
		t.Fatalf("opt-out: %d %+v", code, me)
	}
	out.do("GET", "/api/v1/me", nil, &me)
	if !me.AnalyticsOptOut {
		t.Fatalf("GET /me: %+v", me)
	}
	in := newClient(t, "insights-optin-0001")
	var meIn meDTO
	in.do("GET", "/api/v1/me", nil, &meIn)

	// 60 ta safar (rozilik bergan) + 30 ta (chiqqan) Namanganda, 2026-12-10..12. 8 tasi samolyotda (K dan kam).
	sqlExec(t, `
		INSERT INTO trips (id, user_id, city_id, status, start_date, prefs, arrival)
		SELECT gen_random_uuid(), CASE WHEN g <= 60 THEN $1 ELSE $2 END, 'namangan', 'ready', DATE '2026-12-10' + (g % 3),
		  jsonb_build_object('cityId','namangan','startDate','2026-12-10','days',2,'interests', jsonb_build_array('history','food'),
		    'companion', CASE WHEN g % 2 = 0 THEN 'couple' ELSE 'solo' END, 'pace','balanced',
		    'budgetPerDay', CASE WHEN g <= 54 THEN 150000 ELSE 600000 END, 'isResident', g % 10 <> 0),
		  CASE WHEN g <= 8 THEN '{"from":"tashkent","mode":"plane"}'::jsonb ELSE '{"from":"tashkent","mode":"train"}'::jsonb END
		FROM generate_series(1, 90) g`, meIn.ID, me.ID)
	sqlExec(t, `INSERT INTO trip_days (trip_id, idx, date) SELECT id, i, start_date + i FROM trips, generate_series(0, 1) i WHERE city_id = 'namangan' AND user_id IN ($1, $2)`, meIn.ID, me.ID)
	sqlExec(t, `INSERT INTO stops (id, trip_id, day_idx, position, kind, place_id, arrive_min, duration_min, reason_code)
		SELECT gen_random_uuid(), t.id, 0, 0, 'place', p.id, 600, 60, 'top_rated'
		FROM trips t, (SELECT id FROM places WHERE city_id = 'namangan' AND status = 'published' ORDER BY id LIMIT 1) p
		WHERE t.city_id = 'namangan' AND t.user_id IN ($1, $2)`, meIn.ID, me.ID)

	analyst := newClient(t, "insights-analyst-0001")
	var am meDTO
	analyst.do("GET", "/api/v1/me", nil, &am)
	var e struct{ Error apiError }
	if code := analyst.do("GET", "/api/v1/insights/summary?region=namangan", nil, &e); code != 403 {
		t.Fatalf("rolsiz: %d", code)
	}
	a := moneyAdmin(t)
	if code := a.do("PUT", "/api/v1/admin/users/"+am.ID+"/role", map[string]any{"role": "insights", "regions": []string{"atlantida"}}, &e); code != 400 {
		t.Fatalf("noma'lum hudud: %d", code)
	}
	if code := a.do("PUT", "/api/v1/admin/users/"+am.ID+"/role", map[string]any{"role": "insights", "regions": []string{"namangan"}}, nil); code != 200 {
		t.Fatalf("insights role: %d", code)
	}
	if code := analyst.do("GET", "/api/v1/insights/summary?region=bukhara", nil, &e); code != 403 || e.Error.Code != "insights_region_forbidden" {
		t.Fatalf("boshqa hudud: %d %+v", code, e)
	}
	var sm insightsDTO
	if code := analyst.do("GET", "/api/v1/insights/summary?region=namangan&from=2026-12-10&to=2026-12-12", nil, &sm); code != 200 {
		t.Fatalf("summary: %d", code)
	}
	if sm.Trips == nil || *sm.Trips != 60 || sm.TripsPrev != nil || sm.Change != nil || sm.Withheld["tripsPrev"] != belowK {
		t.Fatalf("trips: %+v", sm)
	}
	// (54*150000 + 6*600000) / 60 = 195 000
	if sm.AvgBudgetPerPerson == nil || *sm.AvgBudgetPerPerson != 195000 || sm.AvgDays == nil || *sm.AvgDays != 2 {
		t.Fatalf("budget/days: %+v %+v", sm.AvgBudgetPerPerson, sm.AvgDays)
	}
	if len(sm.TopPlaces) != 1 || sm.TopPlaces[0].Count == nil || *sm.TopPlaces[0].Share != 1 || sm.TopPlaces[0].Label == "" {
		t.Fatalf("topPlaces: %+v", sm.TopPlaces)
	}
	if len(sm.Transport) != 2 || sm.Transport[0].Key != "train" || sm.Transport[0].Count == nil || *sm.Transport[0].Count != 52 ||
		sm.Transport[1].Key != "plane" || sm.Transport[1].Count != nil || sm.Transport[1].Reason != belowK {
		t.Fatalf("transport (K=50): %+v", sm.Transport)
	}
	// 30/30 juftlik va yolg'iz: ikkalasi K dan kam.
	for _, x := range sm.Companion {
		if x.Count != nil {
			t.Fatalf("companion K: %+v", sm.Companion)
		}
	}
	if sm.Residency[0].Key != "resident" || sm.Residency[0].Count == nil || *sm.Residency[0].Count != 54 || sm.Residency[1].Count != nil {
		t.Fatalf("residency: %+v", sm.Residency)
	}
	// Namangan eng arzon turar joy 460 000 (juftlikka 230 000 dan). 150 000 byudjetli 54 ta rejaga sig'maydi (0.4 * 150 000),
	// 600 000 li 6 tadan yolg'iz 3 tasiga ham (460 000 > 240 000): 57 ta.
	if sm.GapNoStay.CheapestStay == nil || *sm.GapNoStay.CheapestStay != 460000 || sm.GapNoStay.Plans == nil || *sm.GapNoStay.Plans != 57 {
		t.Fatalf("gap: %+v", sm.GapNoStay)
	}

	// CSV eksport.
	req, _ := http.NewRequest("GET", testSrv.URL+"/api/v1/insights/export?region=namangan&from=2026-12-10&to=2026-12-12&format=csv", nil)
	req.Header.Set("Authorization", "Bearer "+analyst.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil || resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv: %d %v", resp.StatusCode, err)
	}
	found := false
	for _, r := range rows {
		if r[0] == "kpi" && r[1] == "trips" && r[3] == "60" {
			found = true
		}
		if r[0] == "transport" && r[1] == "plane" && r[3] != "" {
			t.Fatalf("csv K: %v", r)
		}
	}
	if !found {
		t.Fatalf("csv qatorlari: %v", rows)
	}
	// Javobda shaxsiy maydon yo'q.
	raw := fmt.Sprintf("%+v", sm)
	if strings.Contains(raw, meIn.ID) || strings.Contains(raw, me.ID) {
		t.Fatal("javobda foydalanuvchi ID si")
	}
	// Rol olib qo'yilsa darhol yopiladi.
	a.do("PUT", "/api/v1/admin/users/"+am.ID+"/role", map[string]any{"role": "tourist"}, nil)
	if code := analyst.do("GET", "/api/v1/insights/summary?region=namangan", nil, &e); code != 403 {
		t.Fatalf("rol olingan: %d", code)
	}
}
