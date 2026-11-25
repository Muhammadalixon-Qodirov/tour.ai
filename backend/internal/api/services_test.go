package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestServicesAndEmergency(t *testing.T) {
	c := &client{t: t, lang: "en"}
	var list []serviceDTO
	if code := c.do("GET", "/api/v1/services?city=samarkand&kind=pharmacy,atm&near=39.6547,66.9758", nil, &list); code != 200 || len(list) < 3 {
		t.Fatalf("services: %d %d", code, len(list))
	}
	for i := 1; i < len(list); i++ {
		if *list[i].DistanceM < *list[i-1].DistanceM {
			t.Fatal("masofa bo'yicha saralanmagan")
		}
	}
	if !list[0].IsDemo || list[0].Name == "" {
		t.Fatalf("demo belgisi/nom: %+v", list[0])
	}
	var e struct{ Error apiError }
	if code := c.do("GET", "/api/v1/services?city=samarkand&kind=casino", nil, &e); code != 400 {
		t.Fatalf("noto'g'ri tur: %d", code)
	}

	var sos struct {
		Numbers []emergencyNumber     `json:"numbers"`
		Nearest map[string]serviceDTO `json:"nearest"`
		Guide   string                `json:"guide"`
	}
	c.do("GET", "/api/v1/cities/bukhara/emergency?near=39.7740,64.4200", nil, &sos)
	ambulance := false
	for _, n := range sos.Numbers {
		ambulance = ambulance || n.Kind == "ambulance" && n.Number == "103"
	}
	if !ambulance || len(sos.Nearest) != 3 || sos.Guide == "" {
		t.Fatalf("sos: %+v", sos)
	}
}

func TestOfflinePack(t *testing.T) {
	fetch := func(etag string) (*http.Response, []byte) {
		req, _ := http.NewRequest("GET", testSrv.URL+"/api/v1/cities/samarkand/offline-pack", nil)
		req.Header.Set("Accept-Language", "ru")
		req.Header.Set("Accept-Encoding", "gzip")
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		tr := &http.Transport{DisableCompression: true}
		resp, err := (&http.Client{Transport: tr}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, b
	}
	resp, gz := fetch("")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("pack: %d %s", resp.StatusCode, resp.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	if len(raw) > 1<<20 {
		t.Fatalf("paket juda katta: %d bayt", len(raw))
	}
	var pack struct {
		Places    []placeDTO    `json:"places"`
		Geofences []geofenceDTO `json:"geofences"`
		Services  []serviceDTO  `json:"services"`
		Emergency struct {
			Numbers []emergencyNumber `json:"numbers"`
		} `json:"emergency"`
	}
	if err := json.Unmarshal(raw, &pack); err != nil {
		t.Fatal(err)
	}
	if len(pack.Places) < 15 || len(pack.Geofences) == 0 || len(pack.Services) == 0 || len(pack.Emergency.Numbers) == 0 {
		t.Fatalf("paket to'liq emas: %d joy, %d geofence, %d xizmat", len(pack.Places), len(pack.Geofences), len(pack.Services))
	}
	if pack.Places[0].Name == "" || pack.Places[0].Description == "" {
		t.Fatal("joy matnlari yo'q")
	}
	etag := resp.Header.Get("ETag")
	if r2, _ := fetch(etag); r2.StatusCode != 304 {
		t.Fatalf("etag: %d", r2.StatusCode)
	}

	// admin xizmat qo'shsa → versiya o'zgaradi
	a := adminClient(t)
	var created map[string]any
	if code := a.do("POST", "/api/v1/admin/services", map[string]any{"cityId": "samarkand", "kind": "embassy",
		"name": map[string]string{"uz": "Test elchixonasi"}, "location": map[string]float64{"lat": 39.65, "lng": 66.96}, "phone": "+998 66 000 00 00"}, &created); code != 201 {
		t.Fatalf("service create: %d", code)
	}
	if r3, _ := fetch(etag); r3.StatusCode != 200 {
		t.Fatalf("yangi xizmatdan keyin paket yangilanmadi: %d", r3.StatusCode)
	}
	id := created["id"].(string)
	var e struct{ Error apiError }
	if code := a.do("PATCH", "/api/v1/admin/services/"+id, map[string]any{"kind": "casino"}, &e); code != 400 {
		t.Fatalf("validatsiya: %d", code)
	}
	if code := a.do("PATCH", "/api/v1/admin/services/"+id, map[string]any{"open24h": true}, &created); code != 200 || created["open24h"] != true {
		t.Fatalf("patch: %d", code)
	}
	if code := a.do("DELETE", "/api/v1/admin/services/"+id, nil, nil); code != 204 {
		t.Fatalf("delete: %d", code)
	}
}
