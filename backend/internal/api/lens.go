package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/pricing"
)

// POST /lens/identify (multipart: image, mode=price, quotedPrice?) — "Adolatli narx":
// rasmdagi buyum AI bilan aniqlanadi → O'zbekiston onlayn marketlarida (OLX, Texnomart, Asaxiy, Olcha, Idea) qidiriladi
// → mos kelmagan takliflar (aksessuar, boshqa model) ajratiladi → o'rtacha narx va oraliq.
// mode=landmark | translate hozircha qo'llab-quvvatlanmaydi (501).

type lensListingDTO struct {
	Title string       `json:"title"`
	Price domain.Money `json:"price"`
	URL   string       `json:"url"`
	Used  bool         `json:"used,omitempty"`
}

type lensMarketDTO struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	SearchURL string           `json:"searchUrl"`
	Found     int              `json:"found"` // mos kelgan takliflar soni
	Median    *domain.Money    `json:"median,omitempty"`
	Listings  []lensListingDTO `json:"listings"` // eng arzon 3 tasi
	Error     bool             `json:"error,omitempty"`
}

type lensOnlineDTO struct {
	Query        string          `json:"query"`
	Average      *domain.Money   `json:"average,omitempty"`
	Lowest       *domain.Money   `json:"lowest,omitempty"`
	Highest      *domain.Money   `json:"highest,omitempty"`
	MarketsCount int             `json:"marketsCount"` // narx topilgan marketlar soni
	Markets      []lensMarketDTO `json:"markets"`
}

type fairRangeMoneyDTO struct {
	Min  domain.Money `json:"min"`
	Max  domain.Money `json:"max"`
	Unit string       `json:"unit"`
}

type phraseDTO struct {
	Text            string `json:"text"`
	Transliteration string `json:"transliteration,omitempty"`
	Meaning         string `json:"meaning"`
}

type lensPriceDTO struct {
	Kind         string             `json:"kind"` // "price"
	Found        bool               `json:"found"`
	Confidence   float64            `json:"confidence"`
	Item         string             `json:"item"`
	ItemNames    map[string]string  `json:"itemNames"`
	Category     string             `json:"category"`     // PriceCategory: souvenir | market | food
	ItemCategory string             `json:"itemCategory"` // electronics, clothing, handicraft ...
	FairRange    *fairRangeMoneyDTO `json:"fairRange,omitempty"`
	Quoted       *domain.Money      `json:"quoted,omitempty"`
	Verdict      string             `json:"verdict,omitempty"` // fair | high | very_high
	OverpayRatio *float64           `json:"overpayRatio,omitempty"`
	Tip          string             `json:"tip"`
	Phrases      []phraseDTO        `json:"phrases"`
	UpdatedAt    string             `json:"updatedAt"`
	SampleSize   int                `json:"sampleSize"`
	Online       lensOnlineDTO      `json:"online"`
}

var lensText = map[string]map[string]string{
	"avg": {"uz": "Onlayn do'konlarda o'rtacha narx %s so'm (odatda %s–%s).", "ru": "Средняя цена в онлайн-магазинах %s сум (обычно %s–%s).",
		"en": "Average online price is %s UZS (usually %s–%s)."},
	"fair": {"uz": "Aytilgan narx adolatli.", "ru": "Названная цена справедливая.", "en": "The quoted price is fair."},
	"high": {"uz": "Aytilgan narx biroz baland, %s atrofida savdolashing.", "ru": "Цена немного завышена, торгуйтесь до %s.",
		"en": "The quoted price is a bit high, bargain towards %s."},
	"veryHigh": {"uz": "Aytilgan narx juda baland (%.1f barobar). %s dan boshlab savdolashing yoki boshqa joydan qarang.",
		"ru": "Цена сильно завышена (в %.1f раза). Начните торг с %s или поищите в другом месте.",
		"en": "The quoted price is very high (%.1fx). Start bargaining at %s or look elsewhere."},
	"handmade": {"uz": "Qo'lda ishlangan buyum narxi sifat va ustaga qarab ancha farq qiladi.",
		"ru": "Цена ручной работы сильно зависит от качества и мастера.", "en": "Handmade items vary a lot by quality and maker."},
	"notFound": {"uz": "Bu buyumni onlayn do'konlarda topa olmadik. Bir nechta sotuvchidan narx so'rab solishtiring.",
		"ru": "Не нашли этот товар в онлайн-магазинах. Сравните цены у нескольких продавцов.",
		"en": "We couldn't find this item in online stores. Compare prices from a few sellers."},
}

var lensPhrases = []struct {
	text, tr string
	meaning  map[string]string
}{
	{"Bu qancha turadi?", "bu qancha turadi", map[string]string{"uz": "Bu qancha turadi?", "ru": "Сколько это стоит?", "en": "How much is this?"}},
	{"Qimmat ekan, arzonroq bo'ladimi?", "qimmat ekan, arzonroq bo'ladimi", map[string]string{"uz": "Qimmat ekan, arzonroq bo'ladimi?", "ru": "Дороговато, можно дешевле?", "en": "That's expensive, can it be cheaper?"}},
	{"Oxirgi narxi qancha?", "oxirgi narxi qancha", map[string]string{"uz": "Oxirgi narxi qancha?", "ru": "Какая последняя цена?", "en": "What's your final price?"}},
}

func (s *Server) lensIdentify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	if err := r.ParseMultipartForm(3 << 20); err != nil {
		writeValidation(w, r, fieldErrors{"image": "multipart, ≤ 2 MB"})
		return
	}
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "price"
	}
	if mode != "price" {
		writeError(w, r, http.StatusNotImplemented, "lens_mode_not_supported", false)
		return
	}
	if s.lens == nil {
		writeError(w, r, http.StatusServiceUnavailable, "lens_unavailable", true)
		return
	}
	f, _, err := r.FormFile("image")
	if err != nil {
		writeValidation(w, r, fieldErrors{"image": "required"})
		return
	}
	img, err := io.ReadAll(io.LimitReader(f, 3<<20))
	f.Close()
	if err != nil || len(img) == 0 {
		writeValidation(w, r, fieldErrors{"image": "required"})
		return
	}
	mime := http.DetectContentType(img)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		writeValidation(w, r, fieldErrors{"image": "jpeg | png | webp"})
		return
	}
	var quoted *int64
	if v := strings.TrimSpace(r.FormValue("quotedPrice")); v != "" {
		n, err := strconv.ParseInt(strings.ReplaceAll(v, " ", ""), 10, 64)
		if err != nil || n <= 0 {
			writeValidation(w, r, fieldErrors{"quotedPrice": "so'm, > 0"})
			return
		}
		quoted = &n
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(90 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 80*time.Second)
	defer cancel()
	lang := langOf(r)

	id, err := s.lens.Identify(ctx, img, mime)
	if errors.Is(err, pricing.ErrBusy) {
		writeError(w, r, http.StatusServiceUnavailable, "lens_busy", true)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if id.Confidence < 0.3 || id.Query == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "lens_not_recognized", false)
		return
	}
	res := s.lensPrices(ctx, id, lang)
	res.Confidence = id.Confidence
	if quoted != nil {
		q := domain.UZS(*quoted)
		res.Quoted = &q
	}
	t := func(k string, a ...any) string { return fmt.Sprintf(i18n.Pick(lensText[k], lang), a...) }
	if res.Found {
		avg, lo, hi := res.Online.Average.Amount, res.FairRange.Min.Amount, res.FairRange.Max.Amount
		res.Tip = t("avg", sum(avg), sum(lo), sum(hi))
		if quoted != nil && avg > 0 {
			ratio := float64(*quoted) / float64(avg)
			ratio = float64(int(ratio*10+0.5)) / 10
			res.OverpayRatio = &ratio
			switch {
			case float64(*quoted) <= float64(hi)*1.1:
				res.Verdict = "fair"
				res.Tip += " " + t("fair")
			case float64(*quoted) <= float64(hi)*1.6:
				res.Verdict = "high"
				res.Tip += " " + t("high", sum(avg)+" so'm")
			default:
				res.Verdict = "very_high"
				res.Tip += " " + t("veryHigh", ratio, sum(lo)+" so'm")
			}
		}
	} else {
		res.Tip = t("notFound")
	}
	if id.Handmade {
		res.Tip += " " + t("handmade")
	}
	for _, p := range lensPhrases {
		res.Phrases = append(res.Phrases, phraseDTO{Text: p.text, Transliteration: p.tr, Meaning: i18n.Pick(p.meaning, lang)})
	}
	writeJSON(w, http.StatusOK, res)
}

// lensPrices — qidiruv (kerak bo'lsa kengroq so'rov bilan), saralash va statistika.
func (s *Server) lensPrices(ctx context.Context, id pricing.Identified, lang string) lensPriceDTO {
	item := id.Item[lang]
	if item == "" {
		item = firstNonEmptyStr(id.Item["uz"], id.Item["en"], id.Item["ru"])
	}
	cat := "market"
	switch id.Category {
	case "souvenir", "handicraft", "jewelry":
		cat = "souvenir"
	case "food":
		cat = "food"
	}
	if id.Handmade {
		cat = "souvenir"
	}
	out := lensPriceDTO{Kind: "price", Item: item, ItemNames: id.Item, Category: cat, ItemCategory: id.Category,
		UpdatedAt: s.now().UTC().Format(time.RFC3339), Phrases: []phraseDTO{}}

	query := id.Query
	results := s.markets.Search(ctx, query)
	matched := s.matchListings(ctx, id, results)
	// kam topilsa — kengroq so'rov; qaysi biri ko'proq mos taklif bersa, o'sha olinadi
	if countMatched(matched) < 5 && id.AltQuery != "" && !strings.EqualFold(id.AltQuery, query) {
		r2 := s.markets.Search(ctx, id.AltQuery)
		if m2 := s.matchListings(ctx, id, r2); countMatched(m2) > countMatched(matched) {
			query, results, matched = id.AltQuery, r2, m2
		}
	}
	out.Online = lensOnlineDTO{Query: query, Markets: []lensMarketDTO{}}
	var all []int64
	for i, r := range results {
		m := lensMarketDTO{ID: r.Market.ID, Name: r.Market.Name, SearchURL: r.Market.SearchURL(query), Error: r.Err != nil,
			Listings: []lensListingDTO{}}
		ls := matched[i]
		var prices []int64
		for _, l := range ls {
			prices = append(prices, l.Price)
		}
		if len(prices) > 0 {
			st, _ := pricing.Compute(prices)
			med := domain.UZS(st.Median)
			m.Median = &med
			m.Found = st.Count
			all = append(all, prices...)
			out.Online.MarketsCount++
			sort.Slice(ls, func(a, b int) bool { return ls[a].Price < ls[b].Price })
			for _, l := range ls {
				if len(m.Listings) == 3 {
					break
				}
				if l.Price < st.Lowest || l.Price > st.Highest { // chetga chiqqan (hisobga kirmagan) — ko'rsatilmaydi
					continue
				}
				m.Listings = append(m.Listings, lensListingDTO{Title: l.Title, Price: domain.UZS(l.Price), URL: l.URL, Used: l.Used})
			}
		}
		out.Online.Markets = append(out.Online.Markets, m)
	}
	st, _ := pricing.Compute(all)
	if st.Count > 0 {
		avg, lo, hi := domain.UZS(st.Average), domain.UZS(st.Lowest), domain.UZS(st.Highest)
		out.Online.Average, out.Online.Lowest, out.Online.Highest = &avg, &lo, &hi
		out.FairRange = &fairRangeMoneyDTO{Min: domain.UZS(st.Min), Max: domain.UZS(st.Max), Unit: "dona"}
		out.SampleSize = st.Count
		out.Found = true
	}
	return out
}

// matchListings — har market uchun faqat aynan shu buyumga mos takliflar (bitta AI chaqiruvida).
func (s *Server) matchListings(ctx context.Context, id pricing.Identified, results []pricing.MarketResult) [][]pricing.Listing {
	var flat []pricing.Listing
	var owner []int
	for i, r := range results {
		for j, l := range r.Listings {
			if j == 15 { // har marketdan 15 tagacha (birinchilari eng mos)
				break
			}
			flat = append(flat, l)
			owner = append(owner, i)
		}
	}
	keep := s.lens.Relevant(ctx, id, flat)
	out := make([][]pricing.Listing, len(results))
	for k, ok := range keep {
		if ok {
			out[owner[k]] = append(out[owner[k]], flat[k])
		}
	}
	return out
}

func countMatched(m [][]pricing.Listing) int {
	n := 0
	for _, x := range m {
		n += len(x)
	}
	return n
}

func firstNonEmptyStr(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}
