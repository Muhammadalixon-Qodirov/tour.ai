package api

// Integratsiya testlari: haqiqiy PostgreSQL + seed ma'lumot.
// Ishga tushirish: TEST_DATABASE_URL=postgres://.../yoldosh_test go test ./internal/api/
// O'zgaruvchi bo'lmasa testlar o'tkazib yuboriladi. DIQQAT: test bazasi sxemasi to'liq tozalanadi.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"yoldosh/internal/platform"
	"yoldosh/internal/seed"
	"yoldosh/internal/store"
)

var (
	testSrv   *httptest.Server
	testDBURL string
)

func jsonDecode(resp *http.Response, v any) error { return json.NewDecoder(resp.Body).Decode(v) }

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("TEST_DATABASE_URL yo'q — API integratsiya testlari o'tkazib yuborildi")
		os.Exit(0)
	}
	testDBURL = url
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, url)
	if err != nil {
		panic(err)
	}
	if _, err := db.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		panic(err)
	}
	if err := platform.Migrate(ctx, db); err != nil {
		panic(err)
	}
	if err := seed.LoadDir(ctx, db, "../../seed"); err != nil {
		panic(err)
	}
	uploads, err := os.MkdirTemp("", "yoldosh-uploads")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(uploads)
	cfg := platform.Config{TokenSecret: []byte("test-secret-test-secret-test-secret!"), PublicBaseURL: "http://test", UploadDir: uploads}
	srv := NewServer(cfg, store.New(db))
	srv.now = func() time.Time { return time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC) }
	testSrv = httptest.NewServer(srv.Handler())
	code := m.Run()
	testSrv.Close()
	db.Close()
	os.Exit(code)
}

type client struct {
	t     *testing.T
	token string
	lang  string
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, testSrv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if c.lang != "" {
		req.Header.Set("Accept-Language", c.lang)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.t.Fatalf("%s %s: javobni o'qib bo'lmadi: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func newClient(t *testing.T, device string) *client {
	c := &client{t: t, lang: "uz"}
	var res struct{ Token string }
	if code := c.do("POST", "/api/v1/auth/device", map[string]string{"deviceId": device, "platform": "android"}, &res); code != 200 {
		t.Fatalf("auth: %d", code)
	}
	c.token = res.Token
	return c
}

func TestCitiesAndLanguages(t *testing.T) {
	c := &client{t: t}
	for _, lang := range []string{"uz", "ru", "en"} {
		c.lang = lang
		var cities []cityDTO
		if code := c.do("GET", "/api/v1/cities", nil, &cities); code != 200 || len(cities) != 2 {
			t.Fatalf("%s: code=%d len=%d", lang, code, len(cities))
		}
		var p placeDTO
		c.do("GET", "/api/v1/places/registan", nil, &p)
		if p.Name == "" || p.History == nil || len(p.Rules) == 0 {
			t.Fatalf("%s: to'liq joy kelmadi: %+v", lang, p)
		}
	}
}

func TestErrorFormat(t *testing.T) {
	c := &client{t: t, lang: "uz"}
	var e struct{ Error apiError }
	if code := c.do("GET", "/api/v1/places/yoq-joy", nil, &e); code != 404 || e.Error.Code != "place_not_found" || e.Error.Message == "" {
		t.Fatalf("%d %+v", code, e)
	}
	if code := c.do("GET", "/api/v1/me", nil, &e); code != 401 || e.Error.Code != "unauthorized" {
		t.Fatalf("%d %+v", code, e)
	}
	u := newClient(t, "err-device-0001")
	if code := u.do("POST", "/api/v1/trips", map[string]any{"cityId": "samarkand", "days": 0}, &e); code != 400 || e.Error.Fields["days"] == "" {
		t.Fatalf("%d %+v", code, e)
	}
}

// Har profil uchun marshrut kontrakt qoidalariga mos (API_CONTRACT.md §5).
func TestTripContractRules(t *testing.T) {
	c := newClient(t, "trip-device-0001")
	profiles := []map[string]any{
		{"cityId": "samarkand", "startDate": "2026-12-01", "days": 2, "interests": []string{"history", "food"}, "pace": "balanced"},
		{"cityId": "samarkand", "startDate": "2026-12-07", "days": 1, "interests": []string{"art", "nature"}, "pace": "active"},
		{"cityId": "bukhara", "startDate": "2026-12-04", "days": 3, "interests": []string{"craft"}, "pace": "relaxed",
			"startLocation": map[string]float64{"lat": 39.7735, "lng": 64.42}},
		{"cityId": "bukhara", "startDate": "2026-12-05", "days": 1, "companion": "family", "budget": 1},
	}
	for i, body := range profiles {
		var tr tripDTO
		if code := c.do("POST", "/api/v1/trips", body, &tr); code != 201 {
			t.Fatalf("#%d: code %d", i, code)
		}
		if tr.NoveltyShare < 0.25 {
			t.Errorf("#%d: noveltyShare %.2f < 0.25", i, tr.NoveltyShare)
		}
		for _, d := range tr.Days {
			novel, prevEnd := 0, -1
			for _, s := range d.Stops {
				if s.Reason == "" {
					t.Errorf("#%d: bo'sh reason", i)
				}
				if s.Place == nil {
					t.Fatalf("#%d: stop.place yo'q", i)
				}
				if s.Kind == "place" && s.Place.Novelty != "iconic" && !s.Place.IsPartner {
					novel++
				}
				start := hhmm(t, s.ArriveAt)
				if start < prevEnd {
					t.Errorf("#%d: ustma-ust vaqt %s", i, s.ArriveAt)
				}
				prevEnd = start + s.DurationMin
			}
			if novel == 0 {
				t.Errorf("#%d: %d-kunda yangi joy yo'q", i, d.Index)
			}
		}
		var again tripDTO
		if code := c.do("GET", "/api/v1/trips/"+tr.ID, nil, &again); code != 200 || len(again.Days) != len(tr.Days) {
			t.Fatalf("#%d: qayta o'qish %d", i, code)
		}
	}
}

func hhmm(t *testing.T, s string) int {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		t.Fatalf("vaqt %q", s)
	}
	return h*60 + m
}

func TestTripEditingAndOwnership(t *testing.T) {
	c := newClient(t, "edit-device-0001")
	var tr tripDTO
	c.do("POST", "/api/v1/trips", map[string]any{"cityId": "bukhara", "startDate": "2026-12-02", "days": 1, "interests": []string{"history"}}, &tr)
	first := tr.Days[0].Stops[0]

	var added tripDTO
	if code := c.do("POST", "/api/v1/trips/"+tr.ID+"/stops", map[string]any{"dayIndex": 0, "placeId": "sitorai-mokhi-khosa", "after": first.ID}, &added); code != 200 {
		t.Fatalf("add: %d", code)
	}
	if added.Days[0].Stops[1].Place.ID != "sitorai-mokhi-khosa" || len(added.Days[0].Stops) != len(tr.Days[0].Stops)+1 {
		t.Fatalf("qo'shilgan joy noto'g'ri o'rinda")
	}
	var e struct{ Error apiError }
	if code := c.do("POST", "/api/v1/trips/"+tr.ID+"/stops", map[string]any{"dayIndex": 0, "placeId": "sitorai-mokhi-khosa"}, &e); code != 409 {
		t.Fatalf("takror: %d", code)
	}
	var deleted tripDTO
	if code := c.do("DELETE", "/api/v1/trips/"+tr.ID+"/stops/"+first.ID, nil, &deleted); code != 200 || len(deleted.Days[0].Stops) != len(tr.Days[0].Stops) {
		t.Fatalf("delete: %d", code)
	}
	var swap struct{ Alternatives []placeSummaryDTO }
	if code := c.do("POST", "/api/v1/trips/"+tr.ID+"/stops/"+deleted.Days[0].Stops[0].ID+"/swap", nil, &swap); code != 200 || len(swap.Alternatives) == 0 {
		t.Fatalf("swap: %d %d", code, len(swap.Alternatives))
	}
	if swap.Alternatives[0].Novelty == "iconic" {
		t.Errorf("swap takliflarida yangi joylar birinchi turishi kerak")
	}

	other := newClient(t, "other-device-0001")
	if code := other.do("GET", "/api/v1/trips/"+tr.ID, nil, &e); code != 404 {
		t.Fatalf("boshqa foydalanuvchi marshrutni ko'rdi: %d", code)
	}
}
