package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestBotSuggest(t *testing.T) {
	testAPI.limiter = NewLimiter()
	const key = "bot-key-test-123456"
	testAPI.cfg.BotKey = key
	defer func() { testAPI.cfg.BotKey = "" }()
	do := func(bodyKey string, body any) int {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/internal/suggest", bytes.NewReader(b))
		if bodyKey != "" {
			req.Header.Set("X-Bot-Key", bodyKey)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		return resp.StatusCode
	}
	// auth
	if c := do("", map[string]any{}); c != 401 {
		t.Fatalf("kalitsiz: %d", c)
	}
	if c := do("noto'g'ri", map[string]any{}); c != 401 {
		t.Fatalf("noto'g'ri kalit: %d", c)
	}
	// yangi joy -> navbatga (queued)
	if c := do(key, map[string]any{"name": "Choyxona Navro'z Test", "location": map[string]float64{"lat": 39.66, "lng": 66.98},
		"from": "@tester", "note": "mazali osh"}); c != 202 {
		t.Fatalf("taklif: %d", c)
	}
	var st string
	var data string
	testAPI.store.DB.QueryRow(context.Background(), `SELECT status, data::text FROM discoveries WHERE source='user' AND ref LIKE 'tg-%' AND name->>'uz'='Choyxona Navro''z Test'`).Scan(&st, &data)
	if st != "queued" {
		t.Fatalf("holat: %s (queued kutilgan)", st)
	}
	if !bytes.Contains([]byte(data), []byte("telegram")) || !bytes.Contains([]byte(data), []byte("@tester")) {
		t.Fatalf("data: %s", data)
	}
	// validatsiya
	if c := do(key, map[string]any{"name": "x", "location": map[string]float64{"lat": 39.66, "lng": 66.98}}); c != 400 {
		t.Fatalf("qisqa nom: %d", c)
	}
	if c := do(key, map[string]any{"name": "Yaxshi joy", "location": map[string]float64{"lat": 10, "lng": 10}}); c != 400 {
		t.Fatalf("O'zbekistondan tashqarida: %d", c)
	}
}
