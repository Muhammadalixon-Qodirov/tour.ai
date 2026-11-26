package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/weather"
)

type fakeWeather struct {
	f   weather.Forecast
	err error
}

func (f *fakeWeather) Daily(context.Context, domain.LatLng, time.Time, string) (weather.Forecast, error) {
	return f.f, f.err
}

func setWeather(f weather.Forecast, err error) { testAPI.weather = &fakeWeather{f, err} }

func mkTrip(t *testing.T, c *client, body map[string]any) tripDTO {
	t.Helper()
	var tr tripDTO
	if code := c.do("POST", "/api/v1/trips", body, &tr); code != 201 {
		t.Fatalf("trip: %d", code)
	}
	return tr
}

func replan(t *testing.T, c *client, id string, body map[string]any, wantCode int) replanResp {
	t.Helper()
	var res replanResp
	if code := c.do("POST", "/api/v1/trips/"+id+"/replan", body, &res); code != wantCode {
		t.Fatalf("replan %v: %d (kutilgan %d)", body, code, wantCode)
	}
	return res
}

func contains(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestReplan(t *testing.T) {
	c := newClient(t, "replan-device-0001")
	base := map[string]any{"cityId": "samarkand", "startDate": "2026-12-02", "days": 1, "interests": []string{"nature", "history"}, "pace": "active"}

	tr := mkTrip(t, c, base)
	res := replan(t, c, tr.ID, map[string]any{"reason": "late", "minutes": 60}, 200)
	if !contains(res.Changes, "daqiqaga surildi") || res.Days[0].Stops[0].ArriveAt <= tr.Days[0].Stops[0].ArriveAt {
		t.Fatalf("late: %v", res.Changes)
	}

	tr = mkTrip(t, c, base)
	res = replan(t, c, tr.ID, map[string]any{"reason": "tired"}, 200)
	if len(res.Days[0].Stops) >= len(tr.Days[0].Stops) {
		t.Fatalf("tired: %d → %d", len(tr.Days[0].Stops), len(res.Days[0].Stops))
	}

	// ob-havo: yomg'ir → ochiq joylar qolmaydi
	tr = mkTrip(t, c, base)
	outdoor := func(tr tripDTO) int {
		n := 0
		for _, s := range tr.Days[0].Stops {
			if domain.IsOutdoorCategory(s.Place.Category) {
				n++
			}
		}
		return n
	}
	if outdoor(tr) == 0 {
		t.Fatal("test sharti: marshrutda ochiq havodagi joy bo'lishi kerak")
	}
	setWeather(weather.Forecast{MaxTempC: 12, PrecipMM: 8, PrecipProb: 90}, nil)
	res = replan(t, c, tr.ID, map[string]any{"reason": "weather"}, 200)
	if outdoor(res.tripDTO) != 0 || !contains(res.Changes, "Yomg'ir") {
		t.Fatalf("weather: %d ochiq joy qoldi, %v", outdoor(res.tripDTO), res.Changes)
	}
	setWeather(weather.Forecast{MaxTempC: 22}, nil)
	res = replan(t, c, tr.ID, map[string]any{"reason": "weather"}, 200)
	if !contains(res.Changes, "Ob-havo yaxshi") {
		t.Fatalf("yaxshi ob-havo: %v", res.Changes)
	}
	setWeather(weather.Forecast{}, weather.ErrUnavailable)
	replan(t, c, tr.ID, map[string]any{"reason": "weather"}, 503)

	// bo'sh vaqt: Registon yonida 2 soat
	tr = mkTrip(t, c, base)
	res = replan(t, c, tr.ID, map[string]any{"reason": "free_time", "minutes": 120, "location": map[string]float64{"lat": 39.6547, "lng": 66.9758}}, 200)
	if !contains(res.Changes, "qo'shildi") || res.Days[0].Stops[0].Reason != "Bo'sh vaqtingiz uchun, yaqin atrofda" {
		t.Fatalf("free_time: %v", res.Changes)
	}

	// "bordim" belgisi bor to'xtash o'zgarmaydi (yangi foydalanuvchi: marshrut yaratish limiti 6/daq)
	c = newClient(t, "replan-device-0002")
	tr = mkTrip(t, c, base)
	first := tr.Days[0].Stops[0]
	var marked tripDTO
	if code := c.do("PATCH", "/api/v1/trips/"+tr.ID+"/stops/"+first.ID, map[string]string{"status": "visited"}, &marked); code != 200 || marked.Days[0].Stops[0].Status != "visited" {
		t.Fatalf("status: %d", code)
	}
	res = replan(t, c, tr.ID, map[string]any{"reason": "late", "minutes": 45}, 200)
	if got := res.Days[0].Stops[0]; got.ID != first.ID || got.ArriveAt != first.ArriveAt {
		t.Fatalf("borilgan to'xtash o'zgardi: %+v", got)
	}

	// validatsiya va o'tgan kun
	replan(t, c, tr.ID, map[string]any{"reason": "late"}, 400)
	replan(t, c, tr.ID, map[string]any{"reason": "free_time", "minutes": 60}, 400)
	past := mkTrip(t, c, map[string]any{"cityId": "bukhara", "startDate": "2026-11-24", "days": 1})
	replan(t, c, past.ID, map[string]any{"reason": "tired"}, 422)
}
