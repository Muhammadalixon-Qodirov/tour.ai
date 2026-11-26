package api

import (
	"slices"
	"testing"
)

func TestResidentAliasesTrustBreaks(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "extras-device-0001")
	var me meDTO
	if code := c.do("PATCH", "/api/v1/me", map[string]any{"isResident": true}, &me); code != 200 || me.IsResident == nil || !*me.IsResident {
		t.Fatalf("rezident: %d %+v", code, me.IsResident)
	}

	var res page[placeSummaryDTO]
	c.do("GET", "/api/v1/places?city=samarkand&q=Siyob", nil, &res)
	if !slices.ContainsFunc(res.Items, func(x placeSummaryDTO) bool { return x.ID == "siab-bazaar" }) {
		t.Fatalf("mahalliy nom bo'yicha qidiruv: %+v", res.Items)
	}

	// Tadqiqot katalogi: joy, tarix va qoida ochiq manbadan (public), ish vaqti va kirish narxi namuna (sample).
	var p placeDTO
	c.do("GET", "/api/v1/places/registan", nil, &p)
	if p.Trust != "public" || p.History == nil || p.History.Trust != "public" || p.Rules[0].Trust != "public" ||
		p.HoursTrust != "sample" || p.EntryFee == nil || p.EntryFee.Trust != "sample" {
		t.Fatalf("ishonch darajasi: %s %+v hours=%s", p.Trust, p.History, p.HoursTrust)
	}
	c.do("GET", "/api/v1/places/magoki-attori", nil, &p)
	friday := false
	for _, h := range p.OpeningHours {
		friday = friday || h.Day == 5 && len(h.Breaks) == 1 && h.Breaks[0].Reason == "prayer"
	}
	if !friday {
		t.Fatalf("juma tanaffusi: %+v", p.OpeningHours)
	}
	c.do("GET", "/api/v1/places/kalon-minaret", nil, &p)
	if !slices.Contains(p.Aliases, "Kalon minorasi") {
		t.Fatalf("aliases: %v", p.Aliases)
	}

	// Juma kuni marshrutda namoz tanaffusiga (12:00–14:00) masjid tashrifi tushmaydi.
	tr := mkTrip(t, c, map[string]any{"cityId": "bukhara", "startDate": "2026-12-04", "days": 1, "interests": []string{"history", "architecture"}, "pace": "active"})
	for _, s := range tr.Days[0].Stops {
		if s.Place.ID == "kalon-mosque" || s.Place.ID == "bolo-hauz" || s.Place.ID == "magoki-attori" {
			st := hhmm(t, s.ArriveAt)
			if st < 14*60 && st+s.DurationMin > 12*60 {
				t.Fatalf("%s namoz vaqtiga to'g'ri keldi: %s", s.Place.ID, s.ArriveAt)
			}
		}
	}

	// theatre kategoriyasi admin orqali
	a := adminClient(t)
	var created adminPlaceResp
	if code := a.do("POST", "/api/v1/admin/places", map[string]any{"cityId": "samarkand", "category": "theatre",
		"location": map[string]float64{"lat": 39.65, "lng": 66.96}, "aliases": []string{"Opera"}, "trust": "official",
		"openingHours": []map[string]any{{"day": 1, "open": "10:00", "close": "22:00", "breaks": []map[string]string{{"from": "14:00", "to": "15:00", "reason": "lunch"}}}},
		"texts":        map[string]any{"uz": map[string]string{"name": "Teatr", "tagline": "Kechki spektakllar"}}}, &created); code != 201 {
		t.Fatalf("theatre: %d", code)
	}
	if created.Place.Trust != "official" || created.Place.Aliases[0] != "Opera" {
		t.Fatalf("admin maydonlari: %+v", created.Place)
	}
	var e struct{ Error apiError }
	if code := a.do("PATCH", "/api/v1/admin/places/"+created.Place.ID, map[string]any{"trust": "ishonchli"}, &e); code != 400 {
		t.Fatalf("noto'g'ri trust: %d", code)
	}
}

func TestSavedPlaces(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "saved-device-0001")
	if code := c.do("PUT", "/api/v1/me/saved/registan", nil, nil); code != 204 {
		t.Fatalf("save: %d", code)
	}
	c.do("PUT", "/api/v1/me/saved/registan", nil, nil) // takror — xatosiz
	var e struct{ Error apiError }
	if code := c.do("PUT", "/api/v1/me/saved/yoq-joy", nil, &e); code != 404 {
		t.Fatalf("yo'q joy: %d", code)
	}
	var saved []placeSummaryDTO
	c.do("GET", "/api/v1/me/saved", nil, &saved)
	if len(saved) != 1 || saved[0].ID != "registan" {
		t.Fatalf("saqlanganlar: %+v", saved)
	}
	c.do("DELETE", "/api/v1/me/saved/registan", nil, nil)
	c.do("GET", "/api/v1/me/saved", nil, &saved)
	if len(saved) != 0 {
		t.Fatal("o'chirilmadi")
	}
}

func TestGroupTrip(t *testing.T) {
	testAPI.limiter = NewLimiter()
	owner := newClient(t, "group-owner-0001")
	friend := newClient(t, "group-friend-0001")
	outsider := newClient(t, "group-outsider-01")
	var e struct{ Error apiError }

	var g groupDTO
	if code := owner.do("POST", "/api/v1/groups", map[string]any{"name": "Do'stlar safari", "cityId": "samarkand"}, &g); code != 201 || len(g.InviteCode) != 8 {
		t.Fatalf("create: %d %+v", code, g)
	}
	if code := friend.do("POST", "/api/v1/groups/join", map[string]string{"inviteCode": "NOTVALID"}, &e); code != 404 {
		t.Fatalf("noto'g'ri kod: %d", code)
	}
	if code := friend.do("POST", "/api/v1/groups/join", map[string]string{"inviteCode": g.InviteCode}, &g); code != 200 || g.MembersCount != 2 {
		t.Fatalf("join: %d %d", code, g.MembersCount)
	}
	owner.do("PATCH", "/api/v1/groups/"+g.ID+"/me", map[string]any{"interests": []string{"history"}, "budget": 3, "pace": "active"}, nil)
	friend.do("PATCH", "/api/v1/groups/"+g.ID+"/me", map[string]any{"interests": []string{"nature", "history"}, "budget": 1, "pace": "relaxed"}, nil)
	if code := outsider.do("GET", "/api/v1/groups/"+g.ID, nil, &e); code != 404 {
		t.Fatalf("begona guruhni ko'rdi: %d", code)
	}

	// Ikkalasi ham Registonga qarshi → marshrutdan chiqadi; Shohi Zinda ga ikkalasi "ha".
	for _, c := range []*client{owner, friend} {
		c.do("POST", "/api/v1/groups/"+g.ID+"/votes", map[string]any{"placeId": "registan", "vote": -1}, nil)
		c.do("POST", "/api/v1/groups/"+g.ID+"/votes", map[string]any{"placeId": "shah-i-zinda", "vote": 1}, &g)
	}
	if len(g.Votes) != 2 || g.Votes[0].Place.ID != "shah-i-zinda" || g.Votes[0].Up != 2 || g.Votes[0].MyVote != 1 {
		t.Fatalf("ovozlar: %+v", g.Votes)
	}

	var tr tripDTO
	if code := friend.do("POST", "/api/v1/groups/"+g.ID+"/trip", map[string]any{"startDate": "2026-12-02", "days": 1}, &tr); code != 201 {
		t.Fatalf("group trip: %d", code)
	}
	has := func(id string) bool {
		return slices.ContainsFunc(tr.Days[0].Stops, func(s stopDTO) bool { return s.Place.ID == id })
	}
	if has("registan") || !has("shah-i-zinda") {
		t.Fatal("ovozlar marshrutga ta'sir qilmadi")
	}
	// relaxed temp (eng sokin a'zo) → kuniga 3 ta diqqatga sazovor joy
	places := 0
	for _, s := range tr.Days[0].Stops {
		if s.Kind == "place" {
			places++
		}
	}
	if places > 3 {
		t.Fatalf("eng sokin a'zoning tempi hisobga olinmadi: %d joy", places)
	}

	var seen tripDTO
	if code := owner.do("GET", "/api/v1/trips/"+tr.ID, nil, &seen); code != 200 {
		t.Fatalf("a'zo umumiy marshrutni ko'rmadi: %d", code)
	}
	if code := outsider.do("GET", "/api/v1/trips/"+tr.ID, nil, &e); code != 404 {
		t.Fatalf("begona umumiy marshrutni ko'rdi: %d", code)
	}
	if code := owner.do("POST", "/api/v1/trips/"+tr.ID+"/days/0/diary", nil, &e); code != 404 {
		t.Fatalf("boshqa a'zoning kundaligiga yozildi: %d", code)
	}
	var mine []tripListDTO
	owner.do("GET", "/api/v1/trips", nil, &mine)
	if !slices.ContainsFunc(mine, func(x tripListDTO) bool { return x.ID == tr.ID }) {
		t.Fatal("guruh marshruti a'zoning ro'yxatida yo'q")
	}

	if code := owner.do("DELETE", "/api/v1/groups/"+g.ID+"/members/me", nil, nil); code != 204 {
		t.Fatalf("leave (egasi → guruh o'chadi): %d", code)
	}
	if code := friend.do("GET", "/api/v1/groups/"+g.ID, nil, &e); code != 404 {
		t.Fatalf("o'chirilgan guruh: %d", code)
	}
	if code := friend.do("GET", "/api/v1/trips/"+tr.ID, nil, &seen); code != 200 {
		t.Fatalf("yaratuvchi o'z marshrutini yo'qotmasligi kerak: %d", code)
	}
}
