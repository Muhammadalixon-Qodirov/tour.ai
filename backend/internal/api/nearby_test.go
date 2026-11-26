package api

import "testing"

func TestPlaceNearby(t *testing.T) {
	testAPI.limiter = NewLimiter()
	c := newClient(t, "nearby-device-0001")
	c.do("PATCH", "/api/v1/me", map[string]any{"budget": 1}, nil)
	var res struct {
		Origin      map[string]any            `json:"origin"`
		Stays       []map[string]any          `json:"stays"`
		Food        []map[string]any          `json:"food"`
		Sights      []map[string]any          `json:"sights"`
		Services    map[string]map[string]any `json:"services"`
		Attribution string                    `json:"attribution"`
	}
	if code := c.do("GET", "/api/v1/places/registan/nearby", nil, &res); code != 200 {
		t.Fatalf("nearby: %d", code)
	}
	if res.Attribution == "" || len(res.Sights) == 0 || len(res.Food) == 0 {
		t.Fatalf("bo'limlar bo'sh: %+v", res)
	}
	for _, sec := range [][]map[string]any{res.Stays, res.Food, res.Sights} {
		for _, it := range sec {
			if it["distanceM"] == nil || it["walkMin"] == nil || it["reason"] == "" {
				t.Fatalf("masofa/piyoda vaqt/sabab yo'q: %v", it)
			}
			if it["id"] == "registan" {
				t.Fatal("joyning o'zi yaqin joylar ro'yxatida")
			}
		}
	}
	closest := 0
	for _, s := range res.Stays {
		if s["reason"] == "Eng yaqin" {
			closest++
		}
	}
	if len(res.Stays) > 0 && closest != 1 {
		t.Fatalf("'Eng yaqin' bitta bo'lishi kerak: %d", closest)
	}
	var e struct{ Error apiError }
	if code := c.do("GET", "/api/v1/places/yoq-joy/nearby", nil, &e); code != 404 {
		t.Fatalf("yo'q joy: %d", code)
	}
}
