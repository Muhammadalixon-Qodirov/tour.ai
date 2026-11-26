package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yoldosh/internal/pricing"
)

func testJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i < 64; i++ {
		img.Set(i, i, color.RGBA{200, 0, 0, 255})
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func lensPost(t *testing.T, token string, fields map[string]string, img []byte) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if img != nil {
		fw, _ := mw.CreateFormFile("image", "x.jpg")
		fw.Write(img)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/lens/identify", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Language", "uz")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLensPrice(t *testing.T) {
	testAPI.limiter = NewLimiter()
	// soxta Groq: vision -> buyum JSON; matn -> mos indekslar (aksessuarni chiqarib tashlaydi)
	recognized := true
	groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string           `json:"model"`
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		content := `{"match":[0,1,3,4]}` // 2-chi (indeks 2) — chexol
		if req.Model == "vision-test" {
			if recognized {
				content = `{"item_uz":"iPhone 13 smartfoni","item_ru":"Смартфон iPhone 13","item_en":"iPhone 13 smartphone","query":"iPhone 13 128GB","alt_query":"iPhone 13","category":"phone","handmade":false,"confidence":0.9}`
			} else {
				content = `{"confidence":0}`
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
	}))
	defer groq.Close()
	g := pricing.NewGroq("k1", "vision-test")
	g.BaseURL = groq.URL
	fake := func(id string, ls ...pricing.Listing) pricing.Market {
		return pricing.Market{ID: id, Name: strings.ToUpper(id), SearchURL: func(q string) string { return "https://" + id + "/?q=" + q },
			Fetch: func(ctx context.Context, hc *http.Client, q string) ([]pricing.Listing, error) { return ls, nil }}
	}
	broken := pricing.Market{ID: "down", Name: "DOWN", SearchURL: func(string) string { return "" },
		Fetch: func(context.Context, *http.Client, string) ([]pricing.Listing, error) {
			return nil, fmt.Errorf("timeout")
		}}
	sr := pricing.NewSearcher()
	sr.Markets = []pricing.Market{
		fake("m1", pricing.Listing{Market: "m1", Title: "iPhone 13 128GB", Price: 7_000_000}, pricing.Listing{Market: "m1", Title: "iPhone 13 128 Midnight", Price: 7_400_000},
			pricing.Listing{Market: "m1", Title: "iPhone 13 chexol", Price: 50_000}),
		fake("m2", pricing.Listing{Market: "m2", Title: "Apple iPhone 13 128GB", Price: 7_200_000}, pricing.Listing{Market: "m2", Title: "iPhone 13 (b/u)", Price: 6_000_000}),
		broken,
	}
	oldL, oldM := testAPI.lens, testAPI.markets
	testAPI.lens, testAPI.markets = g, sr
	defer func() { testAPI.lens, testAPI.markets = oldL, oldM }()

	c := newClient(t, "lens-device-0001")
	code, out := lensPost(t, c.token, map[string]string{"mode": "price", "quotedPrice": "14000000"}, testJPEG())
	if code != 200 {
		t.Fatalf("lens: %d %v", code, out)
	}
	online := out["online"].(map[string]any)
	if out["kind"] != "price" || out["found"] != true || out["item"] != "iPhone 13 smartfoni" || online["marketsCount"].(float64) != 2 {
		t.Fatalf("javob: %v", out)
	}
	fr := out["fairRange"].(map[string]any)
	lo, hi := fr["min"].(map[string]any)["amount"].(float64), fr["max"].(map[string]any)["amount"].(float64)
	if lo < 6_000_000 || hi > 7_400_000 || lo > hi || out["sampleSize"].(float64) != 4 { // chexol (50 000) kirmagan
		t.Fatalf("oraliq: %v–%v namuna %v", lo, hi, out["sampleSize"])
	}
	if out["verdict"] != "very_high" || !strings.Contains(out["tip"].(string), "barobar") {
		t.Fatalf("verdict: %v %v", out["verdict"], out["tip"])
	}
	markets := online["markets"].([]any)
	if len(markets) != 3 || markets[2].(map[string]any)["error"] != true {
		t.Fatalf("ishlamagan market belgilanmagan: %v", markets)
	}
	if len(out["phrases"].([]any)) == 0 {
		t.Fatal("iboralar yo'q")
	}
	// adolatli narx
	_, out = lensPost(t, c.token, map[string]string{"mode": "price", "quotedPrice": "7000000"}, testJPEG())
	if out["verdict"] != "fair" {
		t.Fatalf("adolatli: %v", out["verdict"])
	}
	// xato holatlar
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "landmark"}, testJPEG()); code != 501 {
		t.Fatalf("landmark: %d", code)
	}
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "price"}, nil); code != 400 {
		t.Fatalf("rasmsiz: %d", code)
	}
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "price"}, []byte("matn, rasm emas")); code != 400 {
		t.Fatalf("rasm emas: %d", code)
	}
	recognized = false
	if code, out := lensPost(t, c.token, map[string]string{"mode": "price"}, testJPEG()); code != 422 {
		t.Fatalf("tanilmadi: %d %v", code, out)
	}
}
