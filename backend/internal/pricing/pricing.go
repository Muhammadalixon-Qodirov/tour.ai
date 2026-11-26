package pricing

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Groq — OpenAI-mos chat (rasm + matn). Bir nechta kalit (vergul bilan): biri limitga urilsa keyingisi.
type Groq struct {
	keys        []string
	BaseURL     string
	VisionModel string // rasmni tushunadigan model
	TextModel   string // saralash uchun tez model
	hc          *http.Client
	next        atomic.Uint32
}

// NewGroq — kalit bo'lmasa nil.
func NewGroq(keys, visionModel string) *Groq {
	var ks []string
	for _, k := range strings.Split(keys, ",") {
		if k = strings.TrimSpace(k); k != "" {
			ks = append(ks, k)
		}
	}
	if len(ks) == 0 {
		return nil
	}
	if visionModel == "" {
		visionModel = "qwen/qwen3.8-27b"
	}
	return &Groq{keys: ks, BaseURL: "https://api.groq.com/openai/v1", VisionModel: visionModel, TextModel: "openai/gpt-oss-20b",
		hc: &http.Client{Timeout: 60 * time.Second}}
}

// chat — content: string yoki multimodal qismlar. Javob matni qaytadi.
func (g *Groq) chat(ctx context.Context, model string, messages []map[string]any, maxTokens int) (string, error) {
	payload := map[string]any{"model": model, "messages": messages, "temperature": 0.1, "max_completion_tokens": maxTokens}
	if strings.HasPrefix(model, "openai/gpt-oss") {
		payload["reasoning_effort"] = "low"
	}
	body, _ := json.Marshal(payload)
	start := int(g.next.Add(1)-1) % len(g.keys)
	var lastErr error
	limited := 0
	for i := 0; i < len(g.keys)*2; i++ {
		if i == len(g.keys) { // hamma kalit urinib ko'rildi
			if limited < len(g.keys) {
				break
			}
			select { // hammasi limitda — qisqa kutib yana bir aylanish
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(6 * time.Second):
			}
		}
		key := g.keys[(start+i)%len(g.keys)]
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := g.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("groq %d: %.200s", resp.StatusCode, data)
			if resp.StatusCode == 429 && i < len(g.keys) {
				limited++
			}
			continue // keyingi kalit
		}
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("groq %d: %.300s", resp.StatusCode, data)
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
			return "", fmt.Errorf("groq javobi bo'sh")
		}
		return out.Choices[0].Message.Content, nil
	}
	if limited > 0 {
		return "", fmt.Errorf("%w: %v", ErrBusy, lastErr)
	}
	return "", lastErr
}

// jsonObject — javobdan birinchi JSON obyektini ajratadi (```json fencelari bo'lsa ham).
func jsonObject(s string, v any) error {
	a, b := strings.IndexByte(s, '{'), strings.LastIndexByte(s, '}')
	if a < 0 || b <= a {
		return errors.New("JSON topilmadi")
	}
	return json.Unmarshal([]byte(s[a:b+1]), v)
}

// Identified — rasmdan aniqlangan buyum.
type Identified struct {
	Item       map[string]string `json:"item"`  // uz/ru/en nomi
	Query      string            `json:"query"` // marketlarda qidiruv so'rovi
	AltQuery   string            `json:"altQuery"`
	Category   string            `json:"category"` // electronics, clothing, souvenir ...
	Handmade   bool              `json:"handmade"`
	Confidence float64           `json:"confidence"`
}

const visionPrompt = `You identify a product from a photo for a price-comparison feature in Uzbekistan.
Return ONLY a JSON object (no markdown) with keys:
 "item_uz": short product name in Uzbek (Latin),
 "item_ru": short product name in Russian,
 "item_en": short product name in English,
 "query": the best search query for Uzbek online stores: brand + model as SELLERS write it (retail name, NOT factory
          codes like "SM-J510FN" or "A2633"), 2-5 words (e.g. "iPhone 13 128GB", "Samsung Galaxy J5 2016",
          "Philips airfryer HD9252");
          for generic items use a generic noun phrase (e.g. "elektr choynak 1.7 l", "erkaklar krossovkasi"),
 "alt_query": a broader fallback query (e.g. without variant),
 "category": one of electronics, phone, computer, appliance, clothing, shoes, cosmetics, household, toy, book,
             souvenir, handicraft, food, jewelry, other,
 "handmade": true if it looks like a handmade/traditional souvenir (do'ppi, suzani, ceramic, carpet...),
 "confidence": 0..1 how sure you are about the exact product.
If the photo shows no identifiable product, return {"confidence":0}.`

// shrink — rasmni uzun tomoni 768 px gacha kichraytirib JPEG (q=80) qiladi: vision tokenlari ~4 baravar kamayadi.
// Dekod qilib bo'lmasa asl rasm qaytadi.
func shrink(img []byte, mime string) ([]byte, string) {
	src, _, err := image.Decode(bytes.NewReader(img))
	if err != nil {
		return img, mime
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	const maxSide = 768
	if w > maxSide || h > maxSide {
		if w >= h {
			h, w = h*maxSide/w, maxSide
		} else {
			w, h = w*maxSide/h, maxSide
		}
		dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
		src = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		return img, mime
	}
	return buf.Bytes(), "image/jpeg"
}

// ErrBusy — barcha kalitlar limitda (foydalanuvchiga "biroz kuting").
var ErrBusy = errors.New("ai busy")

// Identify — rasmdagi buyumni aniqlaydi.
func (g *Groq) Identify(ctx context.Context, img []byte, mime string) (Identified, error) {
	img, mime = shrink(img, mime)
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img)
	msgs := []map[string]any{{"role": "user", "content": []map[string]any{
		{"type": "text", "text": visionPrompt},
		{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
	}}}
	raw, err := g.chat(ctx, g.VisionModel, msgs, 600)
	if err != nil {
		return Identified{}, err
	}
	var r struct {
		Query      string  `json:"query"`
		AltQuery   string  `json:"alt_query"`
		Category   string  `json:"category"`
		Handmade   bool    `json:"handmade"`
		Confidence float64 `json:"confidence"`
	}
	var names struct {
		ItemUz string `json:"item_uz"`
		ItemRu string `json:"item_ru"`
		ItemEn string `json:"item_en"`
	}
	if err := jsonObject(raw, &r); err != nil {
		return Identified{}, fmt.Errorf("vision javobi: %w", err)
	}
	_ = jsonObject(raw, &names)
	id := Identified{Item: map[string]string{"uz": names.ItemUz, "ru": names.ItemRu, "en": names.ItemEn},
		Query: strings.TrimSpace(r.Query), AltQuery: strings.TrimSpace(r.AltQuery), Category: r.Category,
		Handmade: r.Handmade, Confidence: math.Max(0, math.Min(1, r.Confidence))}
	return id, nil
}

// Relevant — qaysi takliflar aynan shu buyum (aksessuar, boshqa model emas). Model ishlamasa — kalit so'z qoidasi.
func (g *Groq) Relevant(ctx context.Context, id Identified, ls []Listing) []bool {
	keep := make([]bool, len(ls))
	if len(ls) == 0 {
		return keep
	}
	var b strings.Builder
	for i, l := range ls {
		fmt.Fprintf(&b, "%d. %s — %d so'm\n", i, l.Title, l.Price)
	}
	prompt := fmt.Sprintf(`Product photographed: %q (search query: %q).
Below are listings from Uzbek online stores. Select listings that are the SAME product model as the search query.
Sellers write names loosely: ignore factory/region codes (e.g. "J510FN" = "J5 2016"), year, colour, memory size,
condition, Uzbek/Russian spelling and extra words like "garantiya bor". EXCLUDE accessories (cases, glass, chargers,
cables, covers, straps, parts), clearly different models (e.g. S21 vs J5, Watch vs phone), and unrelated items.
%sReturn ONLY JSON: {"match":[indices]}.
%s`, id.Item["en"], id.Query, handmadeRule(id), b.String())
	raw, err := g.chat(ctx, g.TextModel, []map[string]any{{"role": "user", "content": prompt}}, 800)
	var r struct {
		Match []int `json:"match"`
	}
	if err == nil && jsonObject(raw, &r) == nil {
		n := 0
		for _, i := range r.Match {
			if i >= 0 && i < len(ls) {
				keep[i] = true
				n++
			}
		}
		if n > 0 {
			return keep
		}
	}
	// model ishlamadi yoki haddan tashqari qattiq bo'ldi — so'rovdagi barcha so'zlar bor va aksessuar emas
	return heuristicRelevant(id.Query, ls)
}

func handmadeRule(id Identified) string {
	if !id.Handmade {
		return ""
	}
	return `The photographed item is HANDMADE / traditional craft (e.g. Rishtan ceramics, suzani, do'ppi, carpet).
Select ONLY listings that are also handmade or traditional craft of the same kind; EXCLUDE mass-produced brand
items, factory dinnerware sets, appliances and unrelated kitchenware.
`
}

var accessoryWords = []string{"chexol", "chehol", "g'ilof", "gilof", "himoya", "oyna", "steklo", "чехол", "стекло", "плёнк", "пленк",
	"case", "cover", "glass", "kabel", "кабель", "cable", "zaryad", "заряд", "charger", "adapter", "адаптер", "ushlagich", "держател", "holder",
	"remeshok", "ремеш", "strap"}

func heuristicRelevant(q string, ls []Listing) []bool {
	keep := make([]bool, len(ls))
	var toks []string
	for _, t := range strings.Fields(strings.ToLower(q)) {
		if len(t) >= 2 {
			toks = append(toks, t)
		}
	}
	for i, l := range ls {
		title := strings.ToLower(l.Title)
		ok := true
		for _, t := range toks {
			if !strings.Contains(title, t) {
				ok = false
				break
			}
		}
		for _, w := range accessoryWords {
			if strings.Contains(title, w) {
				ok = false
				break
			}
		}
		keep[i] = ok
	}
	return keep
}

// Stats — tanlangan narxlar statistikasi.
type Stats struct {
	Average int64
	Min     int64 // oraliq pastki chegarasi
	Max     int64 // oraliq yuqori chegarasi
	Lowest  int64 // eng arzon (chetga chiqqanlar olib tashlangandan keyin)
	Highest int64
	Median  int64
	Count   int
}

func round1000(v float64) int64 { return int64(math.Round(v/1000)) * 1000 }

func percentile(sorted []int64, p float64) float64 {
	if len(sorted) == 1 {
		return float64(sorted[0])
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return float64(sorted[lo]) + (pos-float64(lo))*float64(sorted[hi]-sorted[lo])
}

// Compute — chetga chiqqanlarni (mediandan 0.4x dan past yoki 2.5x dan yuqori) olib tashlab, o'rtacha va oraliq
// (15–85 persentil; kam bo'lsa min–max).
func Compute(prices []int64) (Stats, []int64) {
	if len(prices) == 0 {
		return Stats{}, nil
	}
	s := append([]int64(nil), prices...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	med := percentile(s, 0.5)
	var kept []int64
	for _, p := range s {
		if float64(p) >= 0.4*med && float64(p) <= 2.5*med {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		kept = s
	}
	var sum float64
	for _, p := range kept {
		sum += float64(p)
	}
	st := Stats{Average: round1000(sum / float64(len(kept))), Lowest: kept[0], Highest: kept[len(kept)-1], Count: len(kept),
		Median: round1000(percentile(kept, 0.5))}
	if len(kept) >= 5 {
		st.Min, st.Max = round1000(percentile(kept, 0.15)), round1000(percentile(kept, 0.85))
	} else {
		st.Min, st.Max = round1000(float64(kept[0])), round1000(float64(kept[len(kept)-1]))
	}
	return st, kept
}
