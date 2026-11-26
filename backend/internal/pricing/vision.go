package pricing

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strings"
)

// Lens: joy tanish (mode=landmark) va rasm turini aniqlash (mode=auto).
// Hammasi Identify bilan bir xil Groq vision modeli (LENS_VISION_MODEL). Model faqat "nima ko'rinyapti" deydi:
// joy haqidagi ma'lumot (tarix, faktlar, manbalar) faqat bazadan olinadi.

// LandmarkGuess — modelning joy nomi haqidagi taxmini.
type LandmarkGuess struct {
	Names      []string // birinchisi asosiy nom, qolganlari boshqa yozilishlari (uz/ru/mahalliy)
	Type       string   // madrasa, mosque, mausoleum, square ...
	Confidence float64
}

// LandmarkVision — rasmda qanday joy ko'rinishi (modelning xom xulosasi).
type LandmarkVision struct {
	IsPlace    bool
	Guesses    []LandmarkGuess
	City       string
	Type       string
	Cues       []string
	Confidence float64
}

// AutoVision — mode=auto: rasm turi va shu turga kerakli ma'lumot (bitta chaqiruvda).
type AutoVision struct {
	Kind     string // landmark | price | translate | none
	Landmark LandmarkVision
	Item     Identified
}

// SiteTypes — model qaytaradigan joy turlari.
const siteTypes = "madrasa, mosque, mausoleum, minaret, square, monument, statue, museum, bazaar, park, theatre, fortress, palace, gate, bridge, building, other"

const landmarkFields = ` "is_place": true if the photo shows a building, monument, square, park or other site (not a product, not a menu),
 "candidates": up to 4 guesses of WHICH specific site this is, most likely first, each
     {"name": name as commonly written in English, "alt": [other spellings: Uzbek Latin, Russian, local],
      "type": one of ` + siteTypes + `, "confidence": 0..1},
 "city": the city if the cues suggest one (e.g. "Samarkand"), else "",
 "type": the site type (same list),
 "cues": up to 5 short visual cues in English (e.g. "blue ribbed dome", "tiger mosaic on portal"),
 "confidence": 0..1 overall`

const landmarkPrompt = `You recognise landmarks, historic buildings and public sites in photos for a travel app about Uzbekistan
(Samarkand, Bukhara, Tashkent, Namangan, Khiva and other cities).
Return ONLY a JSON object (no markdown) with keys:
` + landmarkFields + `.
Only name a specific site when the visual cues support it. Never invent a proper name; returning no candidates is fine.
If the photo shows no building or site, return {"is_place":false,"confidence":0}.`

const autoPrompt = `You classify a photo for a travel app in Uzbekistan and extract what is needed in ONE answer.
"kind" is one of:
 "landmark"  — a building, monument, mosque, madrasa, square, park or other site,
 "price"     — a product, souvenir, food item or goods someone might buy,
 "translate" — mainly text to read: a menu, sign, label or notice,
 "none"      — anything else.
Return ONLY a JSON object (no markdown) with "kind" and:
for "landmark":
` + landmarkFields + `;
for "price":
 "item_uz", "item_ru", "item_en": short product name (Uzbek Latin, Russian, English),
 "query": best search query for Uzbek online stores, brand + model as sellers write it, 2-5 words; generic noun phrase for generic items,
 "alt_query": a broader fallback query,
 "category": one of electronics, phone, computer, appliance, clothing, shoes, cosmetics, household, toy, book,
             souvenir, handicraft, food, jewelry, other,
 "handmade": true if it looks like a handmade/traditional souvenir,
 "confidence": 0..1 how sure you are about the exact product.
Never invent a proper name of a site; returning no candidates is fine.`

func (g *Groq) vision(ctx context.Context, prompt string, img []byte, mime string, maxTokens int) (string, error) {
	img, mime = shrink(img, mime)
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img)
	msgs := []map[string]any{{"role": "user", "content": []map[string]any{
		{"type": "text", "text": prompt},
		{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
	}}}
	return g.chat(ctx, g.VisionModel, msgs, maxTokens)
}

type rawLandmark struct {
	IsPlace    *bool `json:"is_place"`
	Candidates []struct {
		Name       string   `json:"name"`
		Alt        []string `json:"alt"`
		Type       string   `json:"type"`
		Confidence float64  `json:"confidence"`
	} `json:"candidates"`
	City       string   `json:"city"`
	Type       string   `json:"type"`
	Cues       []string `json:"cues"`
	Confidence float64  `json:"confidence"`
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func (r rawLandmark) parse() LandmarkVision {
	v := LandmarkVision{City: strings.TrimSpace(r.City), Type: strings.ToLower(strings.TrimSpace(r.Type)),
		Confidence: clamp01(r.Confidence)}
	v.IsPlace = r.IsPlace == nil && (len(r.Candidates) > 0 || v.Type != "") || r.IsPlace != nil && *r.IsPlace
	for _, c := range r.Cues {
		if c = strings.TrimSpace(c); c != "" && len(v.Cues) < 5 {
			v.Cues = append(v.Cues, c)
		}
	}
	for _, c := range r.Candidates {
		var names []string
		for _, n := range append([]string{c.Name}, c.Alt...) {
			if n = strings.TrimSpace(n); n != "" && len(names) < 6 {
				names = append(names, n)
			}
		}
		if len(names) == 0 || len(v.Guesses) == 4 {
			continue
		}
		conf := clamp01(c.Confidence)
		if c.Confidence == 0 { // model ishonchni yozmagan — umumiy ishonch
			conf = v.Confidence
		}
		v.Guesses = append(v.Guesses, LandmarkGuess{Names: names, Type: strings.ToLower(strings.TrimSpace(c.Type)), Confidence: conf})
	}
	if v.Type == "" && len(v.Guesses) > 0 {
		v.Type = v.Guesses[0].Type
	}
	return v
}

// IdentifyLandmark — rasmdagi joy haqida modelning taxminlari (nomlar, tur, shahar).
func (g *Groq) IdentifyLandmark(ctx context.Context, img []byte, mime string) (LandmarkVision, error) {
	raw, err := g.vision(ctx, landmarkPrompt, img, mime, 700)
	if err != nil {
		return LandmarkVision{}, err
	}
	var r rawLandmark
	if err := jsonObject(raw, &r); err != nil {
		return LandmarkVision{}, fmt.Errorf("vision javobi: %w", err)
	}
	return r.parse(), nil
}

// Classify — mode=auto: bitta chaqiruvda rasm turi va joy yoki buyum ma'lumoti.
func (g *Groq) Classify(ctx context.Context, img []byte, mime string) (AutoVision, error) {
	raw, err := g.vision(ctx, autoPrompt, img, mime, 800)
	if err != nil {
		return AutoVision{}, err
	}
	var k struct {
		Kind string `json:"kind"`
	}
	if err := jsonObject(raw, &k); err != nil {
		return AutoVision{}, fmt.Errorf("vision javobi: %w", err)
	}
	out := AutoVision{Kind: strings.ToLower(strings.TrimSpace(k.Kind))}
	switch out.Kind {
	case "landmark":
		var r rawLandmark
		_ = jsonObject(raw, &r)
		out.Landmark = r.parse()
		out.Landmark.IsPlace = true
	case "price":
		out.Item = parseIdentified(raw)
	case "translate":
	default:
		out.Kind = "none"
	}
	return out, nil
}

// parseIdentified — buyum JSON idan Identified (Identify bilan bir xil maydonlar).
func parseIdentified(raw string) Identified {
	var r struct {
		ItemUz     string  `json:"item_uz"`
		ItemRu     string  `json:"item_ru"`
		ItemEn     string  `json:"item_en"`
		Query      string  `json:"query"`
		AltQuery   string  `json:"alt_query"`
		Category   string  `json:"category"`
		Handmade   bool    `json:"handmade"`
		Confidence float64 `json:"confidence"`
	}
	_ = jsonObject(raw, &r)
	return Identified{Item: map[string]string{"uz": r.ItemUz, "ru": r.ItemRu, "en": r.ItemEn},
		Query: strings.TrimSpace(r.Query), AltQuery: strings.TrimSpace(r.AltQuery), Category: r.Category,
		Handmade: r.Handmade, Confidence: clamp01(r.Confidence)}
}
