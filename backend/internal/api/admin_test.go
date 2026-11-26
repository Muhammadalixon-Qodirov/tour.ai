package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"yoldosh/internal/auth"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
)

const adminEmail, adminPassword = "moderator@test.local", "correct-horse-battery"

func adminClient(t *testing.T) *client {
	t.Helper()
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, testDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, _ := auth.HashPassword(adminPassword)
	if err := store.New(db).UpsertAdmin(ctx, NewAdminID(), adminEmail, "Test", hash, "moderator"); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, lang: "uz"}
	var res struct{ Token string }
	if code := c.do("POST", "/api/v1/admin/auth/login", map[string]string{"email": adminEmail, "password": adminPassword}, &res); code != 200 {
		t.Fatalf("login: %d", code)
	}
	c.token = res.Token
	return c
}

func TestAdminAuth(t *testing.T) {
	adminClient(t)
	c := &client{t: t}
	var e struct{ Error apiError }
	if code := c.do("POST", "/api/v1/admin/auth/login", map[string]string{"email": adminEmail, "password": "wrong-password"}, &e); code != 401 || e.Error.Code != "invalid_credentials" {
		t.Fatalf("noto'g'ri parol: %d %+v", code, e)
	}
	if code := c.do("POST", "/api/v1/admin/auth/login", map[string]string{"email": "yoq@test.local", "password": "x"}, &e); code != 401 {
		t.Fatalf("yo'q email: %d", code)
	}
	tourist := newClient(t, "tourist-admin-try-01")
	if code := tourist.do("GET", "/api/v1/admin/places", nil, &e); code != 403 {
		t.Fatalf("turist tokeni admin yo'liga kirdi: %d", code)
	}
}

type adminPlaceResp struct {
	Place store.PlaceRecord          `json:"place"`
	Texts map[string]store.PlaceText `json:"texts"`
	Rules []adminRuleDTO             `json:"rules"`
}

// Asosiy demo: adminda yangi joy chop etiladi → keyingi marshrutda paydo bo'ladi.
func TestAdminPublishAppearsInTrip(t *testing.T) {
	a := adminClient(t)
	var e struct{ Error apiError }

	if code := a.do("POST", "/api/v1/admin/places", map[string]any{"cityId": "samarkand", "category": "gallery"}, &e); code != 400 || e.Error.Fields["texts.uz"] == "" {
		t.Fatalf("uz matnsiz yaratildi: %d %+v", code, e)
	}

	body := map[string]any{
		"cityId": "samarkand", "category": "gallery", "novelty": "new", "openedAt": "2026-11-01",
		"location": map[string]float64{"lat": 39.6560, "lng": 66.9770}, "interests": []string{"nightlife", "art"},
		"visitDurationMin": 50,
		"openingHours":     []map[string]any{{"day": 0, "open": "10:00", "close": "21:00"}, {"day": 1, "open": "10:00", "close": "21:00"}, {"day": 2, "open": "10:00", "close": "21:00"}, {"day": 3, "open": "10:00", "close": "21:00"}, {"day": 4, "open": "10:00", "close": "21:00"}, {"day": 5, "open": "10:00", "close": "21:00"}, {"day": 6, "open": "10:00", "close": "21:00"}},
		"texts": map[string]any{
			"uz": map[string]string{"name": "Tungi galereya", "tagline": "Kechki ko'rgazmalar"},
			"en": map[string]string{"name": "Night gallery", "tagline": "Evening exhibitions"},
		},
	}
	var created adminPlaceResp
	if code := a.do("POST", "/api/v1/admin/places", body, &created); code != 201 {
		t.Fatalf("yaratish: %d", code)
	}
	id := created.Place.ID
	if !strings.HasPrefix(id, "tungi-galereya-") || created.Place.Status != "draft" {
		t.Fatalf("id/status: %s %s", id, created.Place.Status)
	}

	tourist := newClient(t, "tourist-demo-flow-01")
	tripHas := func() bool {
		var tr tripDTO
		if code := tourist.do("POST", "/api/v1/trips", map[string]any{"cityId": "samarkand", "startDate": "2026-12-03", "days": 1,
			"interests": []string{"nightlife"}, "pace": "relaxed"}, &tr); code != 201 {
			t.Fatalf("trip: %d", code)
		}
		for _, s := range tr.Days[0].Stops {
			if s.Place.ID == id {
				return true
			}
		}
		return false
	}
	if tripHas() {
		t.Fatal("chop etilmagan joy marshrutga tushdi")
	}
	if code := tourist.do("GET", "/api/v1/places/"+id, nil, &e); code != 404 {
		t.Fatalf("chop etilmagan joy ochiq: %d", code)
	}

	var pub adminPlaceResp
	if code := a.do("POST", "/api/v1/admin/places/"+id+"/publish", nil, &pub); code != 200 || pub.Place.Status != "published" {
		t.Fatalf("publish: %d %s", code, pub.Place.Status)
	}
	if !tripHas() {
		t.Fatal("chop etilgan yangi joy marshrutda chiqmadi")
	}
	if code := a.do("POST", "/api/v1/admin/places/"+id+"/reject", map[string]string{"reason": "test"}, &e); code != 409 {
		t.Fatalf("published → reject ruxsat etilmasligi kerak: %d", code)
	}

	// qoidalar → geofence versiyasi oshadi
	var ver struct{ Version int }
	rules := map[string]any{"rules": []map[string]any{{"code": "no_flash", "severity": "warn", "priority": 1,
		"text": map[string]string{"uz": "Chaqnoqsiz suratga oling.", "en": "No flash, please."}}}}
	if code := a.do("PUT", "/api/v1/admin/places/"+id+"/rules", rules, &ver); code != 200 || ver.Version != 2 {
		t.Fatalf("rules: %d v%d", code, ver.Version)
	}
	var fences []geofenceDTO
	tourist.do("GET", "/api/v1/geofences?city=samarkand", nil, &fences)
	found := false
	for _, g := range fences {
		found = found || g.PlaceID == id && g.Version == 2 && g.TopRules[0].Text == "Chaqnoqsiz suratga oling."
	}
	if !found {
		t.Fatal("geofence ro'yxatida yangi qoida yo'q")
	}

	// qisman tahrir: faqat ru matn qo'shiladi, boshqa maydonlar o'zgarmaydi
	var patched adminPlaceResp
	if code := a.do("PATCH", "/api/v1/admin/places/"+id, map[string]any{"texts": map[string]any{"ru": map[string]string{"name": "Ночная галерея", "tagline": "Вечерние выставки"}}}, &patched); code != 200 {
		t.Fatalf("patch: %d", code)
	}
	if len(patched.Texts) != 3 || patched.Place.VisitMin != 50 || patched.Place.Status != "published" {
		t.Fatalf("patch boshqa maydonlarni buzdi: %+v", patched.Place)
	}

	// unpublish → turistdan yashirinadi
	if code := a.do("POST", "/api/v1/admin/places/"+id+"/unpublish", nil, &pub); code != 200 {
		t.Fatalf("unpublish: %d", code)
	}
	if tripHas() {
		t.Fatal("yashirilgan joy marshrutda qoldi")
	}

	var list page[store.AdminPlaceRow]
	if code := a.do("GET", "/api/v1/admin/places?status=draft&city=samarkand", nil, &list); code != 200 || len(list.Items) == 0 {
		t.Fatalf("ro'yxat: %d %d", code, len(list.Items))
	}
	var kpi struct {
		Cities []store.NoveltyKPI `json:"cities"`
	}
	if code := a.do("GET", "/api/v1/admin/analytics/novelty", nil, &kpi); code != 200 || len(kpi.Cities) != 4 {
		t.Fatalf("analytics: %d", code)
	}
}

func TestAdminImageUpload(t *testing.T) {
	a := adminClient(t)
	upload := func(data []byte) (int, map[string]any) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("image", "photo.png")
		fw.Write(data)
		mw.WriteField("credit", "Test")
		mw.Close()
		req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/admin/places/registan/images", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+a.token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = jsonDecode(resp, &out)
		return resp.StatusCode, out
	}
	img := image.NewRGBA(image.Rect(0, 0, 2400, 1200))
	for x := range 2400 {
		img.Set(x, x%1200, color.RGBA{200, 50, 50, 255})
	}
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)

	code, out := upload(pngBuf.Bytes())
	if code != 201 {
		t.Fatalf("upload: %d %v", code, out)
	}
	url := out["image"].(map[string]any)["url"].(string)
	resp, err := http.Get(testSrv.URL + strings.TrimPrefix(url, "http://test"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("yuklangan rasm berilmadi: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if code, _ := upload([]byte("bu rasm emas")); code != 415 {
		t.Fatalf("rasm bo'lmagan fayl: %d", code)
	}
}
