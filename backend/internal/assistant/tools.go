// Package assistant — "Safar" AI yordamchisi: foydalanuvchi savolini tushunadi va javobni FAQAT bizning bazadan
// (joylar, turar joylar, narxlar) tuzadi. Model narx, ish vaqti yoki manzilni o'zidan to'qimaydi — hamma fakt
// asboblar (tools) natijasidan keladi. API kalit bo'lmasa — qoidaga asoslangan qidiruv (Fallback).
package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
	"yoldosh/internal/weather"
)

var tashkent = func() *time.Location {
	l, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		return time.FixedZone("UZT", 5*3600)
	}
	return l
}()

// Tools — asboblar bazaga murojaat qiladi. Found — javobda ko'rsatish uchun topilgan joylar (id → joy).
type Tools struct {
	Store   *store.Store
	Lang    string
	Now     func() time.Time
	Found   map[string]store.Place
	Debug   func(tool, input string) // ixtiyoriy: asbob chaqiruvlarini kuzatish (cmd/guide-try)
	Trip    *TripInfo                // so'rovdagi marshrut (egasi tekshirilgan); get_trip shuni qaytaradi
	Weather weather.Source           // nil — get_weather "unavailable"
	order   []string
	preload map[string]bool // marshrutdan oldindan qo'shilgan joylar: faqat javobda tilga olinsa kartochka
	sources []Source
}

func NewTools(st *store.Store, lang string, now func() time.Time) *Tools {
	if now == nil {
		now = time.Now
	}
	return &Tools{Store: st, Lang: lang, Now: now, Found: map[string]store.Place{}}
}

// FoundOrdered — topilgan joylar topilish tartibida.
func (t *Tools) FoundOrdered() []store.Place {
	out := make([]store.Place, 0, len(t.order))
	for _, id := range t.order {
		out = append(out, t.Found[id])
	}
	return out
}

func (t *Tools) remember(p store.Place) {
	if _, ok := t.Found[p.ID]; !ok {
		t.order = append(t.order, p.ID)
	}
	t.Found[p.ID] = p
}

// ---------- search_places ----------

type SearchPlacesInput struct {
	City          string         `json:"city,omitempty"`
	Categories    []string       `json:"categories,omitempty"`
	Words         []string       `json:"words,omitempty"`
	Family        bool           `json:"family,omitempty"`
	Wheelchair    bool           `json:"wheelchair,omitempty"`
	MaxPriceLevel *int           `json:"max_price_level,omitempty"`
	Novelty       string         `json:"novelty,omitempty"`
	OpenNow       bool           `json:"open_now,omitempty"`
	Near          *domain.LatLng `json:"near,omitempty"`
	RadiusM       float64        `json:"radius_m,omitempty"`
	Limit         int            `json:"limit,omitempty"`
}

type placeBrief struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	City       string   `json:"city"`
	Category   string   `json:"category"`
	Tagline    string   `json:"tagline,omitempty"`
	PriceLevel *int     `json:"price_level,omitempty"` // 0 bepul .. 3 qimmat; yo'q — noma'lum
	Rating     *float64 `json:"rating,omitempty"`
	Reviews    int      `json:"reviews,omitempty"`
	Family     bool     `json:"family_friendly"`
	Wheelchair bool     `json:"wheelchair"`
	OpenNow    *bool    `json:"open_now,omitempty"` // null — ish vaqti noma'lum
	Novelty    string   `json:"novelty"`
	DistanceM  *int     `json:"distance_m,omitempty"`
	Trust      string   `json:"trust"` // public | community | sample (taxminiy/namuna)
}

func (t *Tools) brief(p store.Place) placeBrief {
	b := placeBrief{ID: p.ID, Name: p.Name, City: p.CityID, Category: p.Category, Tagline: p.Tagline,
		Reviews: p.Reviews, Family: p.FamilyFriendly, Wheelchair: p.Wheelchair, Novelty: string(p.Novelty), Trust: p.Trust}
	if pl := p.PriceLevel; pl > 0 || !domain.IsMealCategory(p.Category) { // ovqat joyida 0 — "noma'lum", bepul emas
		b.PriceLevel = &pl
	}
	if p.Reviews > 0 || p.Rating > 0 {
		r := p.Rating
		b.Rating = &r
	}
	if len(p.Hours) > 0 {
		now := t.Now().In(tashkent)
		open := domain.IsOpenAt(p.Hours, int(now.Weekday()), now.Hour()*60+now.Minute())
		b.OpenNow = &open
	}
	if p.DistanceM != nil {
		d := int(*p.DistanceM)
		b.DistanceM = &d
	}
	return b
}

func (t *Tools) SearchPlaces(ctx context.Context, in SearchPlacesInput) (any, error) {
	limit := in.Limit
	if limit <= 0 || limit > 10 {
		limit = 6 // token tejash: odatda 3-5 tasi tavsiya qilinadi
	}
	in.City = NormCity(in.City)
	f := store.PlaceFilter{City: in.City, Words: in.Words, Family: in.Family, Wheelchair: in.Wheelchair, MaxPrice: in.MaxPriceLevel,
		Near: in.Near, RadiusM: in.RadiusM, Limit: 60}
	for _, c := range in.Categories {
		if domain.ValidCategory(c) {
			f.Categories = append(f.Categories, c)
		}
	}
	if in.Novelty == "novel" || domain.Novelty(in.Novelty).Valid() {
		f.Novelty = in.Novelty
	}
	if f.Near != nil && f.RadiusM == 0 {
		f.RadiusM = 3000
	}
	list, err := t.Store.ListPlaces(ctx, f, i18n.Chain(t.Lang))
	if err != nil {
		return nil, err
	}
	out := []placeBrief{}
	for _, p := range list {
		b := t.brief(p)
		if in.OpenNow && (b.OpenNow == nil || !*b.OpenNow) {
			continue
		}
		out = append(out, b)
		t.remember(p)
		if len(out) == limit {
			break
		}
	}
	return map[string]any{"count": len(out), "places": out}, nil
}

// ---------- get_place ----------

type GetPlaceInput struct {
	ID string `json:"id"`
}

func (t *Tools) GetPlace(ctx context.Context, in GetPlaceInput) (any, error) {
	p, err := t.Store.Place(ctx, in.ID, i18n.Chain(t.Lang))
	if err != nil || p.Status != "published" {
		return map[string]string{"error": "place_not_found"}, nil
	}
	t.remember(p)
	rules, _ := t.Store.Rules(ctx, []string{p.ID})
	var rs []string
	for _, r := range rules[p.ID] {
		rs = append(rs, r.Severity+": "+i18n.Pick(r.Text, t.Lang))
	}
	hours := []string{}
	days := []string{"Ya", "Du", "Se", "Ch", "Pa", "Ju", "Sh"}
	for _, h := range p.Hours {
		hours = append(hours, fmt.Sprintf("%s %s-%s", days[h.Day%7], h.Open, h.Close))
	}
	desc := p.Description
	if r := []rune(desc); len(r) > 400 { // token tejash
		desc = string(r[:400]) + "…"
	}
	var history map[string]any
	if p.HistorySummary != nil && *p.HistorySummary != "" {
		srcs := parseSources(p.Sources)
		titles := make([]string, 0, len(srcs))
		for _, sr := range srcs {
			titles = append(titles, sr.Title)
			t.addSource(sr, p.ID)
		}
		history = map[string]any{"summary": *p.HistorySummary, "facts": p.HistoryFacts, "sources": titles}
	}
	return map[string]any{"place": t.brief(p), "description": desc, "history": history, "address": p.Address, "opening_hours": hours,
		"hours_note": p.HoursNote, "entry_fee": json.RawMessage(nonEmptyJSON(p.EntryFee)), "fee_note": p.FeeNote,
		"crowd": p.Crowd, "best_time": p.BestTime, "rules": rs, "visit_minutes": p.VisitMin, "card": p.PayCard, "cash_only": p.CashOnly}, nil
}

func nonEmptyJSON(b json.RawMessage) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

// ---------- search_stays ----------

type stayBrief struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Stars     *int     `json:"stars,omitempty"`
	Rating    *float64 `json:"rating,omitempty"`
	PriceMin  *int64   `json:"price_min_uzs,omitempty"`
	PriceMax  *int64   `json:"price_max_uzs,omitempty"`
	Estimated bool     `json:"price_is_estimate"`
	DistanceM *int     `json:"distance_m,omitempty"`
	Address   string   `json:"address,omitempty"`
}

type SearchStaysInput struct {
	City        string         `json:"city,omitempty"`
	Near        *domain.LatLng `json:"near,omitempty"`
	Kinds       []string       `json:"kinds,omitempty"`
	MaxPriceUZS int64          `json:"max_price_uzs,omitempty"`
	Limit       int            `json:"limit,omitempty"`
}

func (t *Tools) SearchStays(ctx context.Context, in SearchStaysInput) (any, error) {
	limit := in.Limit
	if limit <= 0 || limit > 10 {
		limit = 6
	}
	in.City = NormCity(in.City)
	f := store.StayFilter{City: in.City, Near: in.Near, MaxPrice: in.MaxPriceUZS, Limit: limit}
	if f.Near != nil {
		f.RadiusM = 3000
	}
	for _, k := range in.Kinds {
		if slices.Contains([]string{"hotel", "boutique", "guesthouse", "hostel", "motel", "apartment"}, k) {
			f.Kinds = append(f.Kinds, k)
		}
	}
	if f.City == "" && f.Near == nil {
		return map[string]string{"error": "city or near required"}, nil
	}
	list, err := t.Store.Stays(ctx, f)
	if err != nil {
		return nil, err
	}
	note := ""
	if len(list) == 0 && f.MaxPrice > 0 { // chegara juda past bo'lsa — chegarasiz eng arzonlarini ko'rsatamiz
		f.MaxPrice = 0
		if list, err = t.Store.Stays(ctx, f); err != nil {
			return nil, err
		}
		note = "nothing under max_price_uzs; these are the nearest/best options without the price limit; compare prices yourself"
	}
	out := []stayBrief{}
	for _, v := range list {
		s := stayBrief{ID: v.ID, Name: i18n.Pick(v.Name, t.Lang), Kind: v.Kind, Stars: v.Stars, Rating: v.Rating, PriceMin: v.PriceMin,
			PriceMax: v.PriceMax, Estimated: v.PriceTrust != nil && *v.PriceTrust == "estimate", Address: v.Address}
		if v.DistanceM != nil {
			d := int(*v.DistanceM)
			s.DistanceM = &d
		}
		out = append(out, s)
	}
	res := map[string]any{"count": len(out), "stays": out}
	if note != "" {
		res["note"] = note
	}
	return res, nil
}

// ---------- search_prices ----------

type SearchPricesInput struct {
	City     string `json:"city"`
	Category string `json:"category,omitempty"`
	Q        string `json:"q,omitempty"`
}

func (t *Tools) SearchPrices(ctx context.Context, in SearchPricesInput) (any, error) {
	list, err := t.Store.Prices(ctx, store.PriceFilter{City: NormCity(in.City), Category: in.Category, Q: strings.TrimSpace(in.Q)})
	if err != nil {
		return nil, err
	}
	type price struct {
		Source  string `json:"source,omitempty"`
		Name    string `json:"name"`
		Unit    string `json:"unit"`
		Min     int64  `json:"fair_min_uzs"`
		Max     int64  `json:"fair_max_uzs"`
		Tourist *int64 `json:"tourists_often_asked_uzs,omitempty"`
		Updated string `json:"updated"`
	}
	out := []price{}
	for i, p := range list {
		if i == 12 {
			break
		}
		lo, hi, _, upd := p.Range()
		x := price{Name: i18n.Pick(p.Name, t.Lang), Unit: i18n.Pick(p.Unit, t.Lang), Min: lo, Max: hi, Tourist: p.Tourist,
			Updated: upd.Format("2006-01-02")}
		if sr, ok := sourceFromURL(p.SourceURL, p.SourceYear); ok {
			x.Source = sr.Title
			t.addSource(sr, "")
		}
		out = append(out, x)
	}
	return map[string]any{"count": len(out), "prices": out}, nil
}

// NormCity — model "Samarkand", "Samarqand", "Самарканд" yuborishi mumkin → "samarkand".
func NormCity(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	if c == "" {
		return ""
	}
	if _, ok := cityWords[c]; ok {
		return c
	}
	for id, ws := range map[string][]string{"urgench": {"urganch", "ургенч", "urgench"}, "navoi": {"navoiy", "навои", "navoi"},
		"qarshi": {"qarshi", "карши", "karshi"}, "jizzakh": {"jizzax", "джизак", "jizzakh"}} {
		for _, w := range ws {
			if strings.HasPrefix(c, w) {
				return id
			}
		}
	}
	c = strings.NewReplacer("ʻ", "'", "’", "'").Replace(c)
	for id, ws := range cityWords {
		for _, w := range ws {
			if strings.HasPrefix(c, w) {
				return id
			}
		}
	}
	return c
}
