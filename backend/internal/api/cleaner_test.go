package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// cleanerReq — X-Cleaner-Key bilan ichki so'rov yuboradi.
func cleanerDo(t *testing.T, method, path, key string, body, out any) int {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, testSrv.URL+path, rd)
	if key != "" {
		req.Header.Set("X-Cleaner-Key", key)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestCleanerInternal(t *testing.T) {
	testAPI.limiter = NewLimiter()
	const key = "test-cleaner-key-123456"
	testAPI.cfg.CleanerKey = key
	defer func() { testAPI.cfg.CleanerKey = "" }()
	ctx := context.Background()
	db := testAPI.store.DB

	// uchta navbatdagi topilma: wikidata muzey, osm (past ishonch), foydalanuvchi taklifi
	db.Exec(ctx, `DELETE FROM discoveries WHERE ref LIKE 'clean-test-%'`)
	ins := func(source, ref, city, cat string, conf float64, name, data string) int64 {
		var id int64
		err := db.QueryRow(ctx, `INSERT INTO discoveries (source, ref, name, lat, lng, city_id, category, confidence, data, status)
			VALUES ($1,$2,$3::jsonb,39.66,66.98,$4,$5,$6,$7::jsonb,'queued') RETURNING id`, source, ref, name, city, cat, conf, data).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	wiki := ins("wikidata", "clean-test-wd1", "samarkand", "museum", 0.6,
		`{"uz":"Test muzeyi","en":"Test Museum"}`, `{"desc":{"en":"a history museum"},"tags":{"tourism":"museum"}}`)
	junk := ins("osm", "clean-test-osm1", "samarkand", "workshop", 0.6, `{"uz":"Avto xizmat"}`, `{"tags":{"craft":"car_repair"}}`)
	usr := ins("user", "clean-test-user1", "samarkand", "cafe", 0.6, `{"uz":"Yangi kafe"}`, `{"note":"mazali"}`)

	// auth
	if code := cleanerDo(t, "GET", "/api/v1/internal/clean/pending", "", nil, nil); code != 401 {
		t.Fatalf("kalitsiz: %d", code)
	}
	if code := cleanerDo(t, "GET", "/api/v1/internal/clean/pending", "noto'g'ri", nil, nil); code != 401 {
		t.Fatalf("noto'g'ri kalit: %d", code)
	}
	// pending
	var pend struct {
		Items []cleanPendingItem `json:"items"`
	}
	if code := cleanerDo(t, "GET", "/api/v1/internal/clean/pending?limit=50", key, nil, &pend); code != 200 {
		t.Fatalf("pending: %d", code)
	}
	got := map[int64]cleanPendingItem{}
	for _, it := range pend.Items {
		got[it.ID] = it
	}
	if w := got[wiki]; w.Source != "wikidata" || w.Desc["en"] != "a history museum" || w.Tags["tourism"] != "museum" {
		t.Fatalf("wikidata item: %+v", w)
	}
	if _, ok := got[wiki]; !ok {
		t.Fatal("wikidata navbatda yo'q")
	}

	// results: publish wikidata, reject junk, user -> publish so'ralsa ham review ga tushadi
	res := map[string]any{"results": []map[string]any{
		{"id": wiki, "decision": "publish", "category": "museum", "confidence": 0.95,
			"descriptions": map[string]string{"uz": "Tarix muzeyi", "en": "History museum"}},
		{"id": junk, "decision": "reject", "category": "workshop", "confidence": 0.9},
		{"id": usr, "decision": "publish", "category": "cafe", "confidence": 0.99},
	}}
	var rep struct{ Published, Rejected, Review, Skipped int }
	if code := cleanerDo(t, "POST", "/api/v1/internal/clean/results", key, res, &rep); code != 200 {
		t.Fatalf("results: %d", code)
	}
	if rep.Published != 1 || rep.Rejected != 1 || rep.Review != 1 {
		t.Fatalf("hisobot: %+v", rep)
	}

	// wikidata joy yaratildi va chop etildi
	c := &client{t: t, lang: "uz"}
	var p placeDTO
	if code := c.do("GET", "/api/v1/places/wd-clean-test-wd1", nil, &p); code != 200 || p.Category != "museum" || p.Description != "Tarix muzeyi" {
		t.Fatalf("yaratilgan joy: %d %+v desc=%q", code, p.placeSummaryDTO, p.Description)
	}
	// holatlar
	status := func(id int64) (string, *string) {
		var st string
		var pid *string
		db.QueryRow(ctx, `SELECT status, place_id FROM discoveries WHERE id = $1`, id).Scan(&st, &pid)
		return st, pid
	}
	if st, pid := status(wiki); st != "published" || pid == nil || *pid != "wd-clean-test-wd1" {
		t.Fatalf("wiki holat: %s %v", st, pid)
	}
	if st, _ := status(junk); st != "rejected" {
		t.Fatalf("junk holat: %s", st)
	}
	if st, _ := status(usr); st != "queued" { // user publish so'raldi -> review (navbatda qoladi)
		t.Fatalf("user holat: %s (review bo'lishi kerak)", st)
	}
	// AI tavsiyasi saqlandi
	var aiCat string
	db.QueryRow(ctx, `SELECT data->'ai'->>'category' FROM discoveries WHERE id = $1`, usr).Scan(&aiCat)
	if aiCat != "cafe" {
		t.Fatalf("AI tavsiya: %q", aiCat)
	}

	// pending endi tozalanganlarni qaytarmaydi (ai_cleaned_at o'rnatilgan)
	pend.Items = nil
	cleanerDo(t, "GET", "/api/v1/internal/clean/pending?limit=50", key, nil, &pend)
	for _, it := range pend.Items {
		if it.ID == wiki || it.ID == junk || it.ID == usr {
			t.Fatalf("tozalangan topilma yana navbatda: %d", it.ID)
		}
	}
	// qayta yuborish — allaqachon hal qilingan, o'tkazib yuboriladi
	if code := cleanerDo(t, "POST", "/api/v1/internal/clean/results", key, map[string]any{"results": []map[string]any{
		{"id": wiki, "decision": "reject"}}}, &rep); code != 200 || rep.Skipped != 1 {
		t.Fatalf("qayta yuborish: %d %+v", code, rep)
	}
}
