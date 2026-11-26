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
			Fetch: func(ctx context.Context, c *pricing.Client, q string) ([]pricing.Listing, error) { return ls, nil }}
	}
	broken := pricing.Market{ID: "down", Name: "DOWN", SearchURL: func(string) string { return "" },
		Fetch: func(context.Context, *pricing.Client, string) ([]pricing.Listing, error) {
			return nil, fmt.Errorf("timeout")
		}}
	sr := pricing.NewSearcher("", "")
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
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "selfie"}, testJPEG()); code != 501 {
		t.Fatalf("noma'lum rejim: %d", code)
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

func testJPEGn(n int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	img.Set(n%64, n/64%64, color.RGBA{uint8(n), 200, 0, 255})
	var b bytes.Buffer
	jpeg.Encode(&b, img, &jpeg.Options{Quality: 100})
	return b.Bytes()
}

// fakeLensGroq — soxta Groq: prompt turiga qarab javob (auto, landmark, translate, price); chaqiruvlar sanaladi.
type fakeLensGroq struct {
	auto, landmark, item, match string
	calls                       map[string]int
}

func (f *fakeLensGroq) handler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Content any `json:"content"`
		} `json:"messages"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	b, _ := json.Marshal(req.Messages[0].Content)
	prompt := string(b)
	kind := "match"
	switch {
	case strings.Contains(prompt, "You classify a photo"):
		kind = "auto"
	case strings.Contains(prompt, "You recognise landmarks"):
		kind = "landmark"
	case strings.Contains(prompt, "You identify a product"):
		kind = "item"
	}
	f.calls[kind]++
	content := map[string]string{"auto": f.auto, "landmark": f.landmark, "item": f.item, "match": f.match}[kind]
	json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
}

func TestLensLandmark(t *testing.T) {
	testAPI.limiter = NewLimiter()
	testAPI.lensCache = newLensMemo()
	registan := `{"is_place":true,"candidates":[{"name":"Registan","alt":["Registon","Регистан"],"type":"square","confidence":0.9}],"city":"Samarkand","type":"madrasa","cues":["three madrasas"],"confidence":0.85}`
	fake := &fakeLensGroq{calls: map[string]int{}, landmark: registan, match: `{"match":[0]}`}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	g := pricing.NewGroq("k1", "vision-test")
	g.BaseURL = srv.URL
	oldL, oldM := testAPI.lens, testAPI.markets
	testAPI.lens = g
	sr := pricing.NewSearcher("", "")
	sr.Markets = []pricing.Market{{ID: "m1", Name: "M1", SearchURL: func(q string) string { return "https://m1/?q=" + q },
		Fetch: func(context.Context, *pricing.Client, string) ([]pricing.Listing, error) {
			return []pricing.Listing{{Market: "m1", Title: "Do'ppi", Price: 150_000}}, nil
		}}}
	testAPI.markets = sr
	defer func() { testAPI.lens, testAPI.markets = oldL, oldM }()
	c := newClient(t, "lens-device-0002")
	noTells := func(s string) {
		t.Helper()
		if strings.ContainsAny(s, "—≈~") {
			t.Errorf("yozish qoidasi buzildi: %q", s)
		}
	}

	// 1) Registon yonida (≈ 60 m): Registon, tarix va manbalar bazadan, suhbat ochiladi
	code, out := lensPost(t, c.token, map[string]string{"mode": "landmark", "lat": "39.6545", "lng": "66.9755"}, testJPEGn(1))
	if code != 200 || out["kind"] != "landmark" || out["placeId"] != "registan" || out["historyVerified"] != true {
		t.Fatalf("registon: %d %v", code, out)
	}
	if out["confidence"].(float64) < 0.8 || out["title"] != "Registon" || !strings.Contains(out["subtitle"].(string), "Tarixiy obida") {
		t.Fatalf("sarlavha/ishonch: %v %v %v", out["confidence"], out["title"], out["subtitle"])
	}
	srcs := out["sources"].([]any)
	if len(srcs) == 0 || len(out["facts"].([]any)) == 0 || len(out["followUps"].([]any)) != 3 || out["conversationId"] == nil {
		t.Fatalf("manba/fakt/savol/suhbat: %v", out)
	}
	noTells(out["summary"].(string))
	for _, f := range out["followUps"].([]any) {
		noTells(f.(string))
	}
	if out["place"].(map[string]any)["id"] != "registan" {
		t.Fatalf("place: %v", out["place"])
	}
	// suhbat joyga bog'langan va birinchi xabar bor
	var placeID string
	var count int
	if err := testAPI.store.DB.QueryRow(context.Background(), `SELECT coalesce(place_id,''), message_count FROM guide_conversations WHERE id = $1`,
		out["conversationId"]).Scan(&placeID, &count); err != nil || placeID != "registan" || count != 1 {
		t.Fatalf("suhbat: %v %q %d", err, placeID, count)
	}
	// chatda davom: placeId yuborilmasa ham suhbat Registonga bog'langan
	var chat struct {
		ConversationID  string `json:"conversationId"`
		NewConversation bool   `json:"newConversation"`
	}
	if code := c.do("POST", "/api/v1/guide/chat?stream=false", map[string]any{"message": "Registon qachon ochiq?",
		"conversationId": out["conversationId"]}, &chat); code != 200 || chat.NewConversation || chat.ConversationID != out["conversationId"] {
		t.Fatalf("chat davomi: %d %+v", code, chat)
	}
	testAPI.store.DB.QueryRow(context.Background(), `SELECT message_count FROM guide_conversations WHERE id = $1`, out["conversationId"]).Scan(&count)
	if count != 3 {
		t.Fatalf("xabarlar soni: %d", count)
	}
	// kesh: o'sha rasm va joy — vision qayta chaqirilmaydi, lekin yangi suhbat
	conv1 := out["conversationId"]
	_, out = lensPost(t, c.token, map[string]string{"mode": "landmark", "lat": "39.6545", "lng": "66.9755"}, testJPEGn(1))
	if fake.calls["landmark"] != 1 || out["placeId"] != "registan" || out["conversationId"] == conv1 {
		t.Fatalf("kesh: %d chaqiruv, %v", fake.calls["landmark"], out["conversationId"])
	}

	// 2) mos kelmadi (Registon yonida, lekin model boshqa bino deydi): placeId null, yaqin joylar ro'yxati
	fake.landmark = `{"is_place":true,"candidates":[{"name":"Hotel Grand Palace","type":"building","confidence":0.5}],"type":"building","confidence":0.5}`
	code, out = lensPost(t, c.token, map[string]string{"mode": "landmark", "lat": "39.6545", "lng": "66.9755"}, testJPEGn(2))
	if code != 200 || out["placeId"] != nil || out["confidence"].(float64) >= 0.5 || out["conversationId"] != nil {
		t.Fatalf("aniq emas: %d %v", code, out)
	}
	cands := out["candidates"].([]any)
	if len(cands) == 0 || cands[0].(map[string]any)["id"] != "registan" || cands[0].(map[string]any)["placeId"] != "registan" || cands[0].(map[string]any)["distanceM"] == nil {
		t.Fatalf("nomzodlar: %v", cands)
	}
	noTells(out["summary"].(string))
	// joylashuvsiz: nom bo'yicha bazadan (uz "Go'ri Amir", en "Gur-e-Amir")
	fake.landmark = `{"is_place":true,"candidates":[{"name":"Gur-Emir Mausoleum","alt":["Go'ri Amir","Гур-Эмир"],"type":"mausoleum","confidence":0.9}],"city":"Samarkand","type":"mausoleum","confidence":0.9}`
	code, out = lensPost(t, c.token, map[string]string{"mode": "landmark"}, testJPEGn(10))
	if code != 200 || out["placeId"] != "gur-emir" {
		t.Fatalf("nom bo'yicha: %d %v %v", code, out["placeId"], out["candidates"])
	}
	fake.landmark = `{"is_place":true,"candidates":[{"name":"Hotel Grand Palace","type":"building","confidence":0.5}],"type":"building","confidence":0.5}`
	// joylashuvsiz va mos kelmadi: bo'sh ro'yxat, joylashuvni yoqish maslahati
	code, out = lensPost(t, c.token, map[string]string{"mode": "landmark"}, testJPEGn(3))
	if code != 200 || out["placeId"] != nil || len(out["candidates"].([]any)) != 0 || !strings.Contains(out["summary"].(string), "Joylashuv") {
		t.Fatalf("joylashuvsiz: %d %v", code, out)
	}

	testAPI.limiter = NewLimiter()
	// 3) ro'yxatdan tanlangan joy (placeId): rasm va AI kerak emas; tarixi yo'q joy: halol izoh + joyning o'z manbasi
	before := fake.calls["landmark"]
	code, out = lensPost(t, c.token, map[string]string{"mode": "landmark", "placeId": "amir-temur-monument-samarkand"}, nil)
	if code != 200 || out["placeId"] != "amir-temur-monument-samarkand" || out["historyVerified"] != false ||
		!strings.Contains(out["summary"].(string), "tekshirilgan tarix yo'q") || len(out["facts"].([]any)) != 0 {
		t.Fatalf("tarixsiz: %d %v", code, out)
	}
	src := out["sources"].([]any)
	if len(src) != 1 || !strings.Contains(src[0].(map[string]any)["url"].(string), "openstreetmap.org") || fake.calls["landmark"] != before {
		t.Fatalf("o'z manbasi: %v", src)
	}
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "landmark", "placeId": "yoq-joy"}, nil); code != 404 {
		t.Fatalf("yo'q joy: %d", code)
	}
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "price", "placeId": "registan"}, testJPEG()); code != 400 {
		t.Fatalf("placeId faqat landmark: %d", code)
	}
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "landmark", "lat": "39.65"}, testJPEGn(1)); code != 400 {
		t.Fatalf("faqat lat: %d", code)
	}
	// rasm joy emas
	fake.landmark = `{"is_place":false,"confidence":0}`
	if code, _ := lensPost(t, c.token, map[string]string{"mode": "landmark"}, testJPEGn(4)); code != 422 {
		t.Fatalf("joy emas: %d", code)
	}

	// 4) auto (mode yo'q): bitta chaqiruv turini aniqlaydi
	testAPI.limiter = NewLimiter() // lens: 10/daq
	before = fake.calls["landmark"]
	fake.auto = `{"kind":"landmark",` + registan[1:]
	code, out = lensPost(t, c.token, map[string]string{"lat": "39.6545", "lng": "66.9755"}, testJPEGn(5))
	if code != 200 || out["kind"] != "landmark" || out["placeId"] != "registan" || fake.calls["landmark"] != before {
		t.Fatalf("auto landmark: %d %v", code, out)
	}
	fake.auto = `{"kind":"price","item_uz":"Do'ppi","item_ru":"Тюбетейка","item_en":"Skullcap","query":"do'ppi","category":"handicraft","handmade":true,"confidence":0.8}`
	code, out = lensPost(t, c.token, map[string]string{"mode": "auto", "quotedPrice": "150000"}, testJPEGn(6))
	if code != 200 || out["kind"] != "price" || out["item"] != "Do'ppi" || out["verdict"] != "fair" || fake.calls["item"] != 0 {
		t.Fatalf("auto price: %d %v", code, out)
	}
	fake.auto = `{"kind":"translate"}`
	if code, _ := lensPost(t, c.token, map[string]string{}, testJPEGn(7)); code != 501 {
		t.Fatalf("auto translate hali yo'q: %d", code)
	}
	fake.auto = `{"kind":"none"}`
	if code, _ := lensPost(t, c.token, map[string]string{}, testJPEGn(8)); code != 422 {
		t.Fatalf("auto none: %d", code)
	}

	if code, _ := lensPost(t, c.token, map[string]string{"mode": "translate"}, testJPEGn(9)); code != 501 {
		t.Fatalf("translate: %d", code)
	}
}
