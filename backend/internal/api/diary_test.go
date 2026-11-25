package api

import (
	"bytes"
	"context"
	"html"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func uploadPhoto(t *testing.T, c *client, tripID string, day int, stopID string) photoDTO {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for x := range 640 {
		img.Set(x, x%480, color.RGBA{30, 140, 120, 255})
	}
	var pngBuf, body bytes.Buffer
	png.Encode(&pngBuf, img)
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "a.png")
	fw.Write(pngBuf.Bytes())
	mw.WriteField("tripId", tripID)
	mw.WriteField("dayIndex", "0")
	if stopID != "" {
		mw.WriteField("stopId", stopID)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/diary/photos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("photo: %d %s", resp.StatusCode, b)
	}
	var p photoDTO
	jsonDecode(resp, &p)
	return p
}

func get(t *testing.T, path string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", testSrv.URL+path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestDiaryToFeedFlow(t *testing.T) {
	c := newClient(t, "diary-device-0001")
	var e struct{ Error apiError }
	tr := mkTrip(t, c, map[string]any{"cityId": "bukhara", "startDate": "2026-11-25", "days": 1, "interests": []string{"history", "craft"}})
	stops := tr.Days[0].Stops
	for _, st := range stops[:2] {
		if code := c.do("PATCH", "/api/v1/trips/"+tr.ID+"/stops/"+st.ID, map[string]string{"status": "visited"}, nil); code != 200 {
			t.Fatalf("visited: %d", code)
		}
	}
	ph := uploadPhoto(t, c, tr.ID, 0, stops[0].ID)

	// 1. hikoya yaratish (takroriy chaqiruv o'sha yozuvni qaytaradi)
	var d diaryDTO
	if code := c.do("POST", "/api/v1/trips/"+tr.ID+"/days/0/diary", nil, &d); code != 201 {
		t.Fatalf("diary: %d", code)
	}
	if d.Title == "" || len(d.Places) != 2 || len(d.Photos) != 1 || d.Photos[0].ID != ph.ID || d.GeneratedBy != "template" {
		t.Fatalf("diary mazmuni: %+v", d)
	}
	if !strings.Contains(d.Body, stops[0].Place.Name) || strings.Contains(d.Body, stops[2].Place.Name) {
		t.Fatalf("hikoya faqat borilgan joylar haqida bo'lishi kerak: %s", d.Body)
	}
	var again diaryDTO
	if code := c.do("POST", "/api/v1/trips/"+tr.ID+"/days/0/diary", nil, &again); code != 200 || again.ID != d.ID {
		t.Fatalf("takror: %d", code)
	}
	future := mkTrip(t, c, map[string]any{"cityId": "bukhara", "startDate": "2026-12-20", "days": 1})
	if code := c.do("POST", "/api/v1/trips/"+future.ID+"/days/0/diary", nil, &e); code != 422 || e.Error.Code != "diary_day_not_started" {
		t.Fatalf("kelajak kun: %d %+v", code, e)
	}

	// 2. tahrir va qayta yozish
	if code := c.do("PATCH", "/api/v1/diary/"+d.ID, map[string]string{"body": "Kanalimga obuna bo'ling: https://t.me/spam"}, &e); code != 400 {
		t.Fatalf("havola o'tib ketdi: %d", code)
	}
	var edited diaryDTO
	c.do("PATCH", "/api/v1/diary/"+d.ID, map[string]string{"title": "Buxorodagi eng yaxshi kunim"}, &edited)
	if !edited.Edited || edited.Title != "Buxorodagi eng yaxshi kunim" {
		t.Fatalf("patch: %+v", edited)
	}
	var regen diaryDTO
	if code := c.do("POST", "/api/v1/diary/"+d.ID+"/regenerate", map[string]string{"tone": "short"}, &regen); code != 200 || regen.Tone != "short" || regen.RegenerationsLeft != 4 || regen.Edited {
		t.Fatalf("regenerate: %d %+v", code, regen)
	}

	// 3. ulashish kartasi
	auth := map[string]string{"Authorization": "Bearer " + c.token}
	resp, body := get(t, "/api/v1/diary/"+d.ID+"/share-card?format=story", auth)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("card: %d", resp.StatusCode)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(body)); err != nil || cfg.Height != 1920 {
		t.Fatalf("card o'lchami: %+v %v", cfg, err)
	}
	auth["If-None-Match"] = resp.Header.Get("ETag")
	if resp, _ := get(t, "/api/v1/diary/"+d.ID+"/share-card?format=story", auth); resp.StatusCode != 304 {
		t.Fatalf("etag: %d", resp.StatusCode)
	}

	// 4. lentaga post
	var pub diaryDTO
	if code := c.do("POST", "/api/v1/diary/"+d.ID+"/publish", map[string]string{"visibility": "public"}, &pub); code != 200 || pub.Post == nil {
		t.Fatalf("publish: %d", code)
	}
	anon := &client{t: t, lang: "uz"}
	var feed page[postDTO]
	anon.do("GET", "/api/v1/feed?city=bukhara", nil, &feed)
	if len(feed.Items) == 0 || feed.Items[0].ID != pub.Post.ID || len(feed.Items[0].Places) != 2 {
		t.Fatalf("lentada yo'q: %+v", feed.Items)
	}
	anon.do("GET", "/api/v1/feed?city=samarkand", nil, &feed)
	for _, p := range feed.Items {
		if p.ID == pub.Post.ID {
			t.Fatal("shahar filtri ishlamadi")
		}
	}

	// 5. ochiq sahifa va karta
	resp, body = get(t, "/s/"+pub.Post.Slug, nil)
	if resp.StatusCode != 200 || !strings.Contains(string(body), `property="og:image"`) || !strings.Contains(string(body), html.EscapeString(regen.Title)) {
		t.Fatalf("ochiq sahifa: %d", resp.StatusCode)
	}
	if resp, _ := get(t, "/s/"+pub.Post.Slug+"/card.png", nil); resp.StatusCode != 200 {
		t.Fatalf("ochiq karta: %d", resp.StatusCode)
	}

	// 6. like
	fan := newClient(t, "diary-fan-0001")
	var like struct {
		LikesCount int  `json:"likesCount"`
		LikedByMe  bool `json:"likedByMe"`
	}
	fan.do("POST", "/api/v1/posts/"+pub.Post.ID+"/like", nil, &like)
	fan.do("POST", "/api/v1/posts/"+pub.Post.ID+"/like", nil, &like)
	if like.LikesCount != 1 {
		t.Fatalf("like takrorlanmasligi kerak: %d", like.LikesCount)
	}
	var fp postDTO
	fan.do("GET", "/api/v1/posts/"+pub.Post.ID, nil, &fp)
	if !fp.LikedByMe || fp.IsMine {
		t.Fatalf("post ko'rinishi: %+v", fp)
	}

	// 7. 3 ta shikoyat → yashiriladi → admin tiklaydi
	for i := range 3 {
		r := newClient(t, "diary-reporter-00"+string(rune('1'+i)))
		if code := r.do("POST", "/api/v1/posts/"+pub.Post.ID+"/report", map[string]string{"reason": "spam"}, nil); code != 202 {
			t.Fatalf("report: %d", code)
		}
	}
	if code := anon.do("GET", "/api/v1/posts/"+pub.Post.ID, nil, &e); code != 404 {
		t.Fatalf("yashirilgan post ko'rinmoqda: %d", code)
	}
	if code := c.do("POST", "/api/v1/diary/"+d.ID+"/publish", nil, &e); code != 409 || e.Error.Code != "post_hidden" {
		t.Fatalf("yashirilgan postni qayta chiqarish: %d", code)
	}
	a := adminClient(t)
	var reported page[map[string]any]
	a.do("GET", "/api/v1/admin/posts?status=reported", nil, &reported)
	if len(reported.Items) == 0 {
		t.Fatal("admin ro'yxatida shikoyat qilingan post yo'q")
	}
	if code := a.do("POST", "/api/v1/admin/posts/"+pub.Post.ID+"/restore", nil, nil); code != 200 {
		t.Fatalf("restore: %d", code)
	}
	if code := anon.do("GET", "/api/v1/posts/"+pub.Post.ID, nil, &fp); code != 200 {
		t.Fatalf("tiklangan post: %d", code)
	}

	// 8. muallif o'chiradi
	if code := c.do("DELETE", "/api/v1/posts/"+pub.Post.ID, nil, nil); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if code := anon.do("GET", "/api/v1/posts/"+pub.Post.ID, nil, &e); code != 404 {
		t.Fatalf("o'chirilgan post: %d", code)
	}
}

type fakeSender struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeSender) Send(_ context.Context, tokens []string, title, _ string, data map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, title+"|"+data["entryId"])
	return nil
}

func TestEveningNudge(t *testing.T) {
	c := newClient(t, "nudge-device-0001")
	tr := mkTrip(t, c, map[string]any{"cityId": "samarkand", "startDate": "2026-11-25", "days": 1})
	if code := c.do("POST", "/api/v1/me/push-token", map[string]string{"deviceId": "nudge-device-0001", "pushToken": "ExponentPushToken[test-123]", "timezone": "Asia/Tashkent"}, nil); code != 204 {
		t.Fatalf("push token: %d", code)
	}
	orig := testAPI.now
	defer func() { testAPI.now = orig }()
	testAPI.now = func() time.Time { return time.Date(2026, 11, 25, 15, 10, 0, 0, time.UTC) } // 20:10 Toshkent

	f := &fakeSender{}
	if n, _ := testAPI.NudgeTick(context.Background(), f); n != 0 {
		t.Fatal("borilgan joy yo'q — push bo'lmasligi kerak")
	}
	for _, st := range tr.Days[0].Stops[:2] {
		c.do("PATCH", "/api/v1/trips/"+tr.ID+"/stops/"+st.ID, map[string]string{"status": "visited"}, nil)
	}
	if n, err := testAPI.NudgeTick(context.Background(), f); n != 1 || err != nil {
		t.Fatalf("push: %d %v", n, err)
	}
	if !strings.HasPrefix(f.sent[0], "Bugungi hikoyangiz tayyor") {
		t.Fatalf("matn: %s", f.sent[0])
	}
	var d diaryDTO
	if code := c.do("GET", "/api/v1/trips/"+tr.ID+"/days/0/diary", nil, &d); code != 200 || !strings.HasSuffix(f.sent[0], d.ID) {
		t.Fatalf("hikoya oldindan yozilmagan: %d", code)
	}
	if n, _ := testAPI.NudgeTick(context.Background(), f); n != 0 {
		t.Fatal("kuniga bir martadan ko'p yuborildi")
	}
	testAPI.now = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) } // 17:00 — vaqti emas
	if n, _ := testAPI.NudgeTick(context.Background(), f); n != 0 {
		t.Fatal("soat 20 dan boshqa vaqtda yuborildi")
	}
}
