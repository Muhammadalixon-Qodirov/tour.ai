// Package cleaner — "ma'lumot tozalovchi": ichki tarmoqdagi AI model (Ollama/llama.cpp/vLLM/LM Studio — OpenAI-mos)
// yordamida topilgan joylarni tozalaydi: haqiqiy turistik joymi (junk ajratish), to'g'ri kategoriya, qisqa uz/ru/en
// tavsif (faqat berilgan faktlardan). Fakt (koordinata, manba) o'zgarmaydi — model faqat tozalaydi va tartiblaydi.
package cleaner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yoldosh/internal/domain"
)

// PendingItem — asosiy serverdan kelgan xom topilma (GET /internal/clean/pending).
type PendingItem struct {
	ID         int64             `json:"id"`
	Source     string            `json:"source"`
	Ref        string            `json:"ref"`
	Names      map[string]string `json:"names"`
	CityID     *string           `json:"cityId"`
	Lat        float64           `json:"lat"`
	Lng        float64           `json:"lng"`
	Category   *string           `json:"currentCategory"`
	Confidence float64           `json:"confidence"`
	Reasons    []string          `json:"reasons"`
	Desc       map[string]string `json:"desc,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Classes    []string          `json:"classes,omitempty"`
	Sitelinks  int               `json:"sitelinks,omitempty"`
	Website    string            `json:"website,omitempty"`
}

// Result — asosiy serverga qaytariladigan tozalangan natija (POST /internal/clean/results).
type Result struct {
	ID           int64             `json:"id"`
	Decision     string            `json:"decision"` // publish | review | reject
	Category     string            `json:"category"`
	Names        map[string]string `json:"names,omitempty"`
	Descriptions map[string]string `json:"descriptions,omitempty"`
	Confidence   float64           `json:"confidence"`
	Reasons      []string          `json:"reasons,omitempty"`
}

// Model — OpenAI-mos chat modeli (bitta javob matnini qaytaradi).
type Model interface {
	Complete(ctx context.Context, system, user string) (string, error)
	Name() string
}

// AutoPublish — model ishonchi shu darajadan yuqori va manba ishonchli bo'lsa avto-nashr, aks holda admin navbati.
const AutoPublish = 0.8

// trustedSource — avto-nashr qilsa bo'ladigan manbalar (foydalanuvchi taklifi hech qachon avto-nashr emas — serverda ham tekshiriladi).
var trustedSource = map[string]bool{"wikidata": true, "osm": true, "research": true}

func systemPrompt() string {
	return `You are a data-cleaning assistant for Safar, a travel app for Uzbekistan. You receive ONE raw place record
discovered from open data (Wikidata / OpenStreetMap / research / user suggestion). Do all of the following:

1. Decide if it is a genuine TOURIST place worth showing to tourists: monuments, historic sites, museums, mosques,
   madrasahs, mausoleums, bazaars, craft workshops, art galleries, parks, viewpoints, eco-villages, theatres, and
   notable restaurants/cafes. REJECT non-tourist junk: ordinary shops, car-repair/services, offices, schools, clinics,
   banks, construction/hardware markets, phone-repair, generic unnamed buildings.
2. Choose exactly ONE category from this list (use the English keyword): ` + strings.Join(domain.Categories, ", ") + `.
3. Write a SHORT factual description in Uzbek (Latin), Russian and English. Each: ONE sentence, <= 160 characters,
   using ONLY the facts provided (name, category, tags, existing description). Do NOT invent history, dates, prices,
   opening hours or any fact not given. If there is nothing factual to say, return empty strings.
4. You may lightly fix name casing/typos but keep the real name; only include "names" if you changed something.

Output ONLY a compact JSON object, no markdown fences, with keys:
  "touristic" (boolean), "category" (string from the list), "names" ({"uz","ru","en"} optional),
  "descriptions" ({"uz","ru","en"}), "confidence" (number 0..1), "reason" (short string).
Reply with valid JSON only.`
}

type modelReply struct {
	Touristic    bool              `json:"touristic"`
	Category     string            `json:"category"`
	Names        map[string]string `json:"names"`
	Descriptions map[string]string `json:"descriptions"`
	Confidence   float64           `json:"confidence"`
	Reason       string            `json:"reason"`
}

// Clean — bitta topilmani model bilan tozalaydi va qaror (publish/review/reject) bilan qaytaradi.
func Clean(ctx context.Context, m Model, it PendingItem) (Result, error) {
	userMsg, _ := json.Marshal(map[string]any{"names": it.Names, "currentCategory": it.Category, "coordinates": []float64{it.Lat, it.Lng},
		"source": it.Source, "osmTags": it.Tags, "wikidataClasses": it.Classes, "existingDescription": it.Desc,
		"website": it.Website, "wikipediaArticles": it.Sitelinks})
	raw, err := m.Complete(ctx, systemPrompt(), "Clean this place record and return JSON:\n"+string(userMsg))
	if err != nil {
		return Result{}, err
	}
	rep, err := parseReply(raw)
	if err != nil {
		return Result{}, fmt.Errorf("model javobi JSON emas: %w", err)
	}
	cat := strings.ToLower(strings.TrimSpace(rep.Category))
	if !domain.ValidCategory(cat) && it.Category != nil {
		cat = *it.Category
	}
	conf := rep.Confidence
	if conf < 0 {
		conf = 0
	} else if conf > 1 {
		conf = 1
	}
	res := Result{ID: it.ID, Category: cat, Names: pick3(rep.Names), Descriptions: pick3(rep.Descriptions), Confidence: conf,
		Reasons: []string{"ai:" + m.Name()}}
	if r := strings.TrimSpace(rep.Reason); r != "" {
		res.Reasons = append(res.Reasons, r)
	}
	switch {
	case !rep.Touristic:
		res.Decision = "reject"
	case domain.ValidCategory(cat) && conf >= AutoPublish && trustedSource[it.Source]:
		res.Decision = "publish"
	default:
		res.Decision = "review"
	}
	return res, nil
}

// parseReply — model javobidan birinchi JSON obyektini ajratadi (markdown ```json fencelarini ham qabul qiladi).
func parseReply(s string) (modelReply, error) {
	var r modelReply
	s = strings.TrimSpace(s)
	// Qwen3 kabi modellar avval <think>...</think> yozadi — fikrlash qismi tashlanadi
	if i := strings.LastIndex(s, "</think>"); i >= 0 {
		s = s[i+len("</think>"):]
	}
	if i := strings.Index(s, "```"); i >= 0 { // ```json ... ```
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	a, b := strings.IndexByte(s, '{'), strings.LastIndexByte(s, '}')
	if a < 0 || b <= a {
		return r, fmt.Errorf("JSON obyekti topilmadi")
	}
	return r, json.Unmarshal([]byte(s[a:b+1]), &r)
}

func pick3(m map[string]string) map[string]string {
	out := map[string]string{}
	for _, l := range []string{"uz", "ru", "en"} {
		if v := strings.TrimSpace(m[l]); v != "" {
			out[l] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
