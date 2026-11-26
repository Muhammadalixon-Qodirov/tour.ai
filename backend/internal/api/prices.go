package api

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// ---------- umumiy: ETag bilan kesh ----------

// writeCached — katalog javobi: tana xeshi ETag bo'ladi, If-None-Match mos kelsa 304 (tana yuborilmaydi).
// Accept-Language bo'yicha farq qiladi (Vary withLang da qo'yiladi).
func writeCached(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(buf.Bytes())
	etag := `"` + hex.EncodeToString(sum[:12]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	if match := r.Header.Get("If-None-Match"); match != "" && slices.Contains(strings.Split(strings.ReplaceAll(match, " ", ""), ","), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// ---------- katalog rasmlari ----------

var imageName = regexp.MustCompile(`^[a-z0-9_]+\.jpg$`)

// placeImage — binar ichidagi katalog rasmlari (/static/places/{file}). Fayl nomi o'zgarmaydi → uzoq kesh.
func (s *Server) placeImage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if s.images == nil || !imageName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFileFS(w, r, s.images, path.Clean(name))
}

// ---------- narxlar ----------

var priceCategories = []string{"taxi", "food", "souvenir", "market", "ticket", "transport"}

type fairRangeDTO struct {
	Min  domain.Money `json:"min"`
	Max  domain.Money `json:"max"`
	Unit string       `json:"unit"`
}

type priceItemDTO struct {
	ID             string        `json:"id"`
	CityID         string        `json:"cityId"`
	Category       string        `json:"category"`
	Name           string        `json:"name"`
	FairRange      fairRangeDTO  `json:"fairRange"`
	TouristTypical *domain.Money `json:"touristTypical,omitempty"`
	UpdatedAt      string        `json:"updatedAt"`
	SampleSize     int           `json:"sampleSize"` // tasdiqlangan foydalanuvchi kuzatuvlari (tadqiqot oralig'i kirmaydi)
	Trust          string        `json:"trust"`      // oraliq ishonchi: kuzatuvlardan bo'lsa community, aks holda bazaniki (sample)
	Source         *sourceDTO    `json:"source,omitempty"`
	PlaceIDs       []string      `json:"placeIds,omitempty"`
}

func priceDTO(p store.PriceItem, lang string) priceItemDTO {
	lo, hi, n, updated := p.Range()
	d := priceItemDTO{ID: p.ID, CityID: p.CityID, Category: p.Category, Name: i18n.Pick(p.Name, lang),
		FairRange: fairRangeDTO{Min: domain.UZS(lo), Max: domain.UZS(hi), Unit: i18n.Pick(p.Unit, lang)},
		UpdatedAt: updated.UTC().Format(time.RFC3339), SampleSize: n, Source: sourceOf(p.SourceURL, p.SourceYear)}
	d.Trust = cmp.Or(p.Trust, "sample")
	if lo != p.BaseMin || hi != p.BaseMax {
		d.Trust = "community"
	}
	if p.Tourist != nil {
		m := domain.UZS(*p.Tourist)
		d.TouristTypical = &m
	}
	if len(p.PlaceIDs) > 0 {
		d.PlaceIDs = p.PlaceIDs
	}
	return d
}

// nearPriceCats — joy turiga qarab turistga kerak bo'ladigan narx turlari (ilovadagi NEAR_CATS bilan bir xil).
var nearPriceCats = map[string][]string{
	"bazaar":     {"market", "souvenir", "food"},
	"workshop":   {"souvenir"},
	"gallery":    {"souvenir"},
	"restaurant": {"food"},
	"cafe":       {"food"},
}

// nearbyPrices — joy sahifasidagi narxlar (0-5 ta): shu joyning o'z kirish chiptasi birinchi, keyin joy turiga mos
// narxlar (har turdan 2 tagacha). Boshqa obidalarning chiptalari ko'rsatilmaydi.
func nearbyPrices(p store.Place, city []store.PriceItem) []store.PriceItem {
	var out []store.PriceItem
	for _, x := range city {
		if x.Category == "ticket" && slices.Contains(x.PlaceIDs, p.ID) {
			out = append(out, x)
		}
	}
	cats, ok := nearPriceCats[p.Category]
	if !ok {
		cats = []string{"taxi"}
	}
	for _, c := range cats {
		n := 0
		for _, x := range city {
			if x.Category == c && n < 2 && !slices.ContainsFunc(out, func(y store.PriceItem) bool { return y.ID == x.ID }) {
				out = append(out, x)
				n++
			}
		}
	}
	return out[:min(len(out), 5)]
}

func (s *Server) listPrices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.PriceFilter{City: q.Get("city"), Category: q.Get("category"), Q: strings.TrimSpace(q.Get("q"))}
	fe := fieldErrors{}
	if f.Category != "" && !slices.Contains(priceCategories, f.Category) {
		fe.add("category", strings.Join(priceCategories, " | "))
	}
	if len(f.Q) > 100 {
		fe.add("q", "max 100 chars")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	items, err := s.store.Prices(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	lang := langOf(r)
	out := make([]priceItemDTO, 0, len(items))
	for _, p := range items {
		out = append(out, priceDTO(p, lang))
	}
	writeCached(w, r, out)
}

type priceReportReq struct {
	PriceItemID *string        `json:"priceItemId"`
	CityID      *string        `json:"cityId"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Paid        domain.Money   `json:"paid"`
	Buyer       *string        `json:"buyer"` // local | tourist
	Location    *domain.LatLng `json:"location"`
}

type priceReportResp struct {
	ID        string        `json:"id"`
	Status    string        `json:"status"` // approved — oraliqqa darhol qo'shildi; pending — moderator ko'radi
	PriceItem *priceItemDTO `json:"priceItem,omitempty"`
}

// reportPrice — foydalanuvchi kuzatuvi (anonim yoki hisob bilan). Tadqiqot oralig'iga yaqin narx avtomatik
// tasdiqlanadi va oraliqni qayta hisoblashga kiradi; g'ayrioddiy narx moderatsiyaga tushadi.
func (s *Server) reportPrice(w http.ResponseWriter, r *http.Request) {
	var req priceReportReq
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	fe := fieldErrors{}
	var item *store.PriceItem
	if req.PriceItemID != nil {
		it, err := s.store.PriceItem(r.Context(), *req.PriceItemID)
		if isNotFound(err) {
			writeError(w, r, http.StatusNotFound, "price_item_not_found", false)
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		item = &it
		if req.Category == "" {
			req.Category = it.Category
		}
		if req.Name == "" {
			req.Name = i18n.Pick(it.Name, langOf(r))
		}
		req.CityID = &it.CityID
	}
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 120 {
		fe.add("name", "1..120 chars")
	}
	if !slices.Contains(priceCategories, req.Category) || item != nil && item.Category != req.Category {
		fe.add("category", strings.Join(priceCategories, " | "))
	}
	if req.Paid.Currency != "" && req.Paid.Currency != "UZS" {
		fe.add("paid.currency", "UZS")
	}
	if req.Paid.Amount < 100 || req.Paid.Amount > 100_000_000 {
		fe.add("paid.amount", "100..100000000")
	}
	if req.Buyer != nil && *req.Buyer != "local" && *req.Buyer != "tourist" {
		fe.add("buyer", "local | tourist")
	}
	if req.Location != nil && !req.Location.Valid() {
		fe.add("location", "invalid coordinates")
	}
	if req.CityID != nil && item == nil {
		if _, err := s.store.City(r.Context(), *req.CityID, []string{"en"}); err != nil {
			fe.add("cityId", "unknown city")
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	status := "pending"
	if item != nil && domain.PlausiblePrice(item.BaseMin, item.BaseMax, req.Paid.Amount) {
		status = "approved"
	}
	rep := store.PriceReport{ID: uuid.Must(uuid.NewV7()), PriceItemID: req.PriceItemID, CityID: req.CityID, UserID: userOf(r),
		Name: req.Name, Category: req.Category, Paid: req.Paid.Amount, Buyer: req.Buyer, Loc: req.Location, Status: status}
	if err := s.store.AddPriceReport(r.Context(), rep); err != nil {
		s.internalError(w, r, err)
		return
	}
	resp := priceReportResp{ID: rep.ID.String(), Status: status}
	if item != nil {
		it, err := s.store.PriceItem(r.Context(), item.ID)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		d := priceDTO(it, langOf(r))
		resp.PriceItem = &d
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ---------- tadbirlar ----------

type eventDTO struct {
	ID        string        `json:"id"`
	CityID    string        `json:"cityId"`
	Title     string        `json:"title"`
	Kind      string        `json:"kind"`
	Cover     domain.Image  `json:"cover"`
	StartsAt  string        `json:"startsAt"`
	EndsAt    string        `json:"endsAt"`
	Venue     string        `json:"venue"`
	Location  domain.LatLng `json:"location"`
	Price     *domain.Money `json:"price,omitempty"`
	IsFree    bool          `json:"isFree"`
	TicketURL *string       `json:"ticketUrl,omitempty"`
	Source    *sourceDTO    `json:"source,omitempty"`
	Recurring *string       `json:"recurring,omitempty"` // "annual": sana o'tgan yilgidan ko'chirilgan
}

// parseDay — "YYYY-MM-DD" (shahar vaqti, kun boshi) yoki RFC 3339.
func parseDay(v string, loc *time.Location) (time.Time, bool) {
	if t, err := time.ParseInLocation(time.DateOnly, v, loc); err == nil {
		return t, true
	}
	t, err := time.Parse(time.RFC3339, v)
	return t, err == nil
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	loc := tz("Asia/Tashkent")
	f := store.EventFilter{City: q.Get("city"), From: s.now()}
	fe := fieldErrors{}
	if v := q.Get("from"); v != "" {
		t, ok := parseDay(v, loc)
		if !ok {
			fe.add("from", "YYYY-MM-DD or RFC 3339")
		}
		// o'tib ketgan tadbirlar ko'rsatilmaydi: from hozirdan oldin bo'lsa ham hozirdan
		if t.After(f.From) {
			f.From = t
		}
	}
	if v := q.Get("to"); v != "" {
		t, ok := parseDay(v, loc)
		if !ok {
			fe.add("to", "YYYY-MM-DD or RFC 3339")
		}
		if len(v) == len(time.DateOnly) {
			t = t.AddDate(0, 0, 1).Add(-time.Second) // kun oxirigacha
		}
		f.To = &t
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	events, err := s.store.Events(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	lang := langOf(r)
	out := make([]eventDTO, 0, len(events))
	for _, e := range events {
		d := eventDTO{ID: e.ID, CityID: e.CityID, Title: i18n.Pick(e.Title, lang), Kind: e.Kind,
			Cover:    domain.Image{URL: s.cfg.PublicBaseURL + "/static/ph/event.svg"},
			StartsAt: e.StartsAt.UTC().Format(time.RFC3339), EndsAt: e.EndsAt.UTC().Format(time.RFC3339),
			Venue: i18n.Pick(e.Venue, lang), Location: e.Loc, IsFree: e.IsFree, TicketURL: e.TicketURL,
			Source: sourceOf(e.SourceURL, nil), Recurring: e.Recurring}
		if e.Image != nil && e.Image.URL != "" {
			d.Cover = s.absImage(*e.Image)
		}
		if e.Price != nil && *e.Price > 0 {
			m := domain.UZS(*e.Price)
			d.Price = &m
		}
		out = append(out, d)
	}
	writeCached(w, r, out)
}
