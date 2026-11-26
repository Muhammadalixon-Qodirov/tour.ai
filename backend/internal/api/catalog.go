package api

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// ---------- DTO lar (API_CONTRACT.md §2) ----------

type placeSummaryDTO struct {
	ID               string         `json:"id"`
	CityID           string         `json:"cityId"`
	Name             string         `json:"name"`
	Tagline          string         `json:"tagline"`
	Category         string         `json:"category"`
	Novelty          domain.Novelty `json:"novelty"`
	OpenedAt         *string        `json:"openedAt,omitempty"`
	Cover            domain.Image   `json:"cover"`
	Location         domain.LatLng  `json:"location"`
	Rating           float64        `json:"rating"`
	ReviewsCount     int            `json:"reviewsCount"`
	PriceLevel       int            `json:"priceLevel"`
	VisitDurationMin int            `json:"visitDurationMin"`
	IsOpenNow        *bool          `json:"isOpenNow,omitempty"`
	Crowd            *crowdDTO      `json:"crowd,omitempty"`
	IsPartner        bool           `json:"isPartner"`
	Trust            string         `json:"trust"` // official | public | community | sample (sample — demo, UI da belgilansin)
	DistanceM        *int           `json:"distanceM,omitempty"`
}

type crowdDTO struct {
	Level    string  `json:"level"`
	BestTime *string `json:"bestTime,omitempty"`
}

type ruleDTO struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"`
	Text       string  `json:"text"`
	Trust      string  `json:"trust"`
	VerifiedBy *string `json:"verifiedBy,omitempty"`
	VerifiedAt *string `json:"verifiedAt,omitempty"`
	Source     *string `json:"source,omitempty"` // URL yoki "general etiquette ..."
}

type placeDTO struct {
	placeSummaryDTO
	Description     string                `json:"description"`
	Aliases         []string              `json:"aliases"`
	VerifiedAt      *string               `json:"verifiedAt,omitempty"`
	Images          []domain.Image        `json:"images"`
	Address         string                `json:"address"`
	OpeningHours    []domain.OpeningHours `json:"openingHours"`
	HoursTrust      string                `json:"hoursTrust"` // ish vaqti ishonch darajasi (sample — namuna)
	HoursNote       *string               `json:"hoursNote,omitempty"`
	EntryFee        *entryFeeDTO          `json:"entryFee,omitempty"`
	FeeNote         *string               `json:"feeNote,omitempty"`
	Rules           []ruleDTO             `json:"rules"`
	History         *historyDTO           `json:"history,omitempty"`
	Accessibility   accessibilityDTO      `json:"accessibility"`
	Payment         paymentDTO            `json:"payment"`
	GeofenceRadiusM int                   `json:"geofenceRadiusM"`
	NearbyPrices    []priceItemDTO        `json:"nearbyPrices"`
}

// entryFeeDTO — kirish narxi: mahalliy (rezident) va chet ellik tarifi; noma'lum tarif yo'q (0 = bepul).
type entryFeeDTO struct {
	Local     *domain.Money `json:"local,omitempty"`
	Foreigner *domain.Money `json:"foreigner,omitempty"`
	Note      *string       `json:"note,omitempty"`
	Source    *sourceDTO    `json:"source,omitempty"`
	Trust     string        `json:"trust"` // official | public | community | sample
}

type sourceDTO struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// sourceOf — URL dan manba: sarlavha = sayt nomi (+ yil).
func sourceOf(url *string, year *int) *sourceDTO {
	if url == nil || *url == "" {
		return nil
	}
	title := *url
	if after, ok := strings.CutPrefix(title, "https://"); ok {
		title = after
	} else if after, ok := strings.CutPrefix(title, "http://"); ok {
		title = after
	}
	title, _, _ = strings.Cut(title, "/")
	title = strings.TrimPrefix(title, "www.")
	if year != nil && *year > 0 {
		title += ", " + strconv.Itoa(*year)
	}
	if !strings.HasPrefix(*url, "http") {
		return &sourceDTO{Title: *url} // "general etiquette ..." kabi matnli manba
	}
	return &sourceDTO{Title: title, URL: *url}
}

type historyDTO struct {
	Trust   string   `json:"trust"`
	Summary string   `json:"summary"`
	Facts   []string `json:"facts"`
	Sources any      `json:"sources"`
}

type accessibilityDTO struct {
	Wheelchair     bool `json:"wheelchair"`
	FamilyFriendly bool `json:"familyFriendly"`
	EasyWalk       bool `json:"easyWalk"`
}

type paymentDTO struct {
	Card     bool `json:"card"`
	CashOnly bool `json:"cashOnly"`
}

func (s *Server) coverOf(p store.Place) domain.Image {
	if len(p.Images) > 0 && p.Images[0].URL != "" {
		return s.absImage(p.Images[0])
	}
	return domain.Image{URL: s.cfg.PublicBaseURL + "/static/ph/" + p.Category + ".svg"}
}

// absImage — bazadagi nisbiy "/static/..." manzilni PUBLIC_BASE_URL bilan to'liq manzilga aylantiradi
// (domen almashsa bazani o'zgartirish shart emas).
func (s *Server) absImage(im domain.Image) domain.Image {
	if strings.HasPrefix(im.URL, "/") {
		im.URL = s.cfg.PublicBaseURL + im.URL
	}
	return im
}

func (s *Server) absImages(in []domain.Image) []domain.Image {
	out := make([]domain.Image, 0, len(in))
	for _, im := range in {
		out = append(out, s.absImage(im))
	}
	return out
}

// openNow — shahar vaqt zonasida hozir ochiqmi.
func openNow(p store.Place, loc *time.Location, now time.Time) bool {
	t := now.In(loc)
	return domain.IsOpenAt(p.Hours, int(t.Weekday()), t.Hour()*60+t.Minute())
}

func (s *Server) summaryDTO(p store.Place, loc *time.Location) placeSummaryDTO {
	d := placeSummaryDTO{
		ID: p.ID, CityID: p.CityID, Name: p.Name, Tagline: p.Tagline, Category: p.Category, Novelty: p.Novelty,
		Cover: s.coverOf(p), Location: p.Loc, Rating: p.Rating, ReviewsCount: p.Reviews, PriceLevel: p.PriceLevel,
		VisitDurationMin: p.VisitMin, IsPartner: p.IsPartner, Trust: p.Trust,
	}
	if p.OpenedAt != nil {
		v := p.OpenedAt.Format(time.DateOnly)
		d.OpenedAt = &v
	}
	if loc != nil {
		v := openNow(p, loc, s.now())
		d.IsOpenNow = &v
	}
	if p.DistanceM != nil {
		v := int(*p.DistanceM)
		d.DistanceM = &v
	}
	if p.Crowd != nil {
		d.Crowd = &crowdDTO{Level: *p.Crowd, BestTime: p.BestTime}
	}
	return d
}

func rulesDTO(rules []store.Rule, lang string) []ruleDTO {
	out := make([]ruleDTO, 0, len(rules))
	for _, r := range rules {
		d := ruleDTO{Code: r.Code, Severity: r.Severity, Text: i18n.Pick(r.Text, lang), Trust: r.Trust, VerifiedBy: r.VerifiedBy, Source: r.Source}
		if r.VerifiedAt != nil {
			v := r.VerifiedAt.UTC().Format(time.RFC3339)
			d.VerifiedAt = &v
		}
		out = append(out, d)
	}
	return out
}

// tz — shahar vaqt zonasi (xato bo'lsa Toshkent).
func tz(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	loc, _ := time.LoadLocation("Asia/Tashkent")
	return loc
}

// ---------- handlerlar ----------

type cityDTO struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Country        string        `json:"country"`
	Cover          domain.Image  `json:"cover"`
	Center         domain.LatLng `json:"center"`
	PlacesCount    int           `json:"placesCount"`
	NewPlacesCount int           `json:"newPlacesCount"`
}

func (s *Server) listCities(w http.ResponseWriter, r *http.Request) {
	cities, err := s.store.Cities(r.Context(), i18n.Chain(langOf(r)))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]cityDTO, 0, len(cities))
	for _, c := range cities {
		if c.PlacesCount == 0 {
			continue // hali tasdiqlangan joyi yo'q shahar turistga ko'rinmaydi
		}
		cover := domain.Image{URL: s.cfg.PublicBaseURL + "/static/ph/city.svg"}
		if c.CoverURL != nil && *c.CoverURL != "" {
			cover = s.absImage(domain.Image{URL: *c.CoverURL})
		}
		out = append(out, cityDTO{ID: c.ID, Name: c.Name, Country: c.Country, Cover: cover, Center: c.Center,
			PlacesCount: c.PlacesCount, NewPlacesCount: c.NewPlacesCount})
	}
	writeCached(w, r, out)
}

type page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func encodeCursor(offset int) *string {
	v := base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(offset)))
	return &v
}

func decodeCursor(c string) (int, bool) {
	if c == "" {
		return 0, true
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(string(b), "o:"))
	return n, err == nil && n >= 0 && n <= 100000
}

func parseLatLng(s string) (domain.LatLng, bool) {
	a, b, ok := strings.Cut(s, ",")
	if !ok {
		return domain.LatLng{}, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	p := domain.LatLng{Lat: lat, Lng: lng}
	return p, err1 == nil && err2 == nil && p.Valid()
}

func (s *Server) listPlaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fe := fieldErrors{}
	f := store.PlaceFilter{City: q.Get("city"), Category: q.Get("category"), Novelty: q.Get("novelty"),
		Interest: q.Get("interest"), Q: strings.TrimSpace(q.Get("q")), Limit: 20}
	if f.Category != "" && !domain.ValidCategory(f.Category) {
		fe.add("category", "unknown category")
	}
	if f.Novelty != "" && f.Novelty != "novel" && !domain.Novelty(f.Novelty).Valid() {
		fe.add("novelty", "iconic | new | hidden | novel")
	}
	if f.Interest != "" && !domain.ValidInterest(f.Interest) {
		fe.add("interest", "unknown interest")
	}
	if len(f.Q) > 100 {
		fe.add("q", "max 100 chars")
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			fe.add("limit", "1..50")
		}
		f.Limit = n
	}
	off, ok := decodeCursor(q.Get("cursor"))
	if !ok {
		fe.add("cursor", "invalid")
	}
	f.Offset = off
	if v := q.Get("near"); v != "" {
		p, ok := parseLatLng(v)
		if !ok {
			fe.add("near", "lat,lng")
		}
		f.Near = &p
		f.RadiusM = 5000
	}
	if v := q.Get("radiusM"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n <= 0 || n > 50000 {
			fe.add("radiusM", "1..50000")
		}
		f.RadiusM = n
	}
	f.Wheelchair = q.Get("wheelchair") == "true"
	openOnly := q.Get("openNow") == "true"
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}

	places, err := s.store.ListPlaces(r.Context(), f, i18n.Chain(langOf(r)))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	tzs := s.cityZones(r)
	items := make([]placeSummaryDTO, 0, len(places))
	for _, p := range places {
		d := s.summaryDTO(p, tzs(p.CityID))
		if openOnly && (d.IsOpenNow == nil || !*d.IsOpenNow) {
			continue
		}
		items = append(items, d)
	}
	var next *string
	if len(places) == f.Limit {
		next = encodeCursor(f.Offset + f.Limit)
	}
	writeCached(w, r, page[placeSummaryDTO]{Items: items, NextCursor: next})
}

// cityZones — shahar id → vaqt zonasi (so'rov davomida keshlanadi).
func (s *Server) cityZones(r *http.Request) func(string) *time.Location {
	cache := map[string]*time.Location{}
	return func(cityID string) *time.Location {
		if loc, ok := cache[cityID]; ok {
			return loc
		}
		name := "Asia/Tashkent"
		if c, err := s.store.City(r.Context(), cityID, []string{"en"}); err == nil {
			name = c.Timezone
		}
		cache[cityID] = tz(name)
		return cache[cityID]
	}
}

func (s *Server) getPlace(w http.ResponseWriter, r *http.Request) {
	lang := langOf(r)
	p, err := s.store.Place(r.Context(), r.PathValue("id"), i18n.Chain(lang))
	if isNotFound(err) || err == nil && p.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	rules, err := s.store.Rules(r.Context(), []string{p.ID})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	d := s.placeDetailDTO(r, p, rules[p.ID], lang)
	prices, err := s.store.Prices(r.Context(), store.PriceFilter{City: p.CityID})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for _, x := range nearbyPrices(p, prices) {
		d.NearbyPrices = append(d.NearbyPrices, priceDTO(x, lang))
	}
	writeCached(w, r, d)
}

func (s *Server) placeDetailDTO(r *http.Request, p store.Place, rules []store.Rule, lang string) placeDTO {
	d := placeDTO{
		placeSummaryDTO: s.summaryDTO(p, s.cityZones(r)(p.CityID)),
		Description:     p.Description, Aliases: nonNil(p.Aliases), Images: s.absImages(p.Images), Address: p.Address, OpeningHours: p.Hours,
		HoursNote: p.HoursNote, FeeNote: p.FeeNote,
		Rules:           rulesDTO(rules, lang),
		Accessibility:   accessibilityDTO{p.Wheelchair, p.FamilyFriendly, p.EasyWalk},
		Payment:         paymentDTO{p.PayCard, p.CashOnly},
		GeofenceRadiusM: p.GeofenceRadiusM, NearbyPrices: []priceItemDTO{},
	}
	if d.OpeningHours == nil {
		d.OpeningHours = []domain.OpeningHours{}
	}
	d.HoursTrust = p.Trust
	if p.HoursTrust != nil {
		d.HoursTrust = *p.HoursTrust
	}
	if len(p.EntryFee) > 0 && string(p.EntryFee) != "null" {
		var fee struct {
			Local     *domain.Money `json:"local"`
			Foreigner *domain.Money `json:"foreigner"`
			Note      *string       `json:"note"`
			Source    *string       `json:"source"`
			Year      *int          `json:"year"`
			Trust     string        `json:"trust"`
		}
		if json.Unmarshal(p.EntryFee, &fee) == nil && (fee.Local != nil || fee.Foreigner != nil) {
			d.EntryFee = &entryFeeDTO{Local: fee.Local, Foreigner: fee.Foreigner, Note: p.FeeNote, Source: sourceOf(fee.Source, fee.Year),
				Trust: cmp.Or(fee.Trust, p.Trust)}
			if d.EntryFee.Note == nil {
				d.EntryFee.Note = fee.Note
			}
		}
	}
	if p.VerifiedAt != nil {
		v := p.VerifiedAt.UTC().Format(time.RFC3339)
		d.VerifiedAt = &v
	}
	if p.HistorySummary != nil && *p.HistorySummary != "" {
		trust := p.Trust
		if p.HistoryTrust != nil {
			trust = *p.HistoryTrust
		}
		d.History = &historyDTO{Trust: trust, Summary: *p.HistorySummary, Facts: p.HistoryFacts, Sources: p.Sources}
		if d.History.Facts == nil {
			d.History.Facts = []string{}
		}
	}
	return d
}

type reviewDTO struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

func (s *Server) placeReviews(w http.ResponseWriter, r *http.Request) {
	revs, err := s.store.Reviews(r.Context(), r.PathValue("id"), 50)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]reviewDTO, 0, len(revs))
	for _, v := range revs {
		author := v.Author
		if author == "" {
			author = map[string]string{"uz": "Sayohatchi", "ru": "Путешественник", "en": "Traveller"}[langOf(r)]
		}
		out = append(out, reviewDTO{ID: v.ID.String(), Author: author, Rating: v.Rating, Text: v.Text,
			CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, out)
}

type geofenceDTO struct {
	PlaceID  string        `json:"placeId"`
	Name     string        `json:"name"`
	Center   domain.LatLng `json:"center"`
	RadiusM  int           `json:"radiusM"`
	Headline string        `json:"headline"`
	TopRules []ruleDTO     `json:"topRules"`
	Version  int           `json:"version"`
}

func (s *Server) geofences(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	if city == "" {
		writeValidation(w, r, fieldErrors{"city": "required"})
		return
	}
	lang := langOf(r)
	places, err := s.store.GeofencePlaces(r.Context(), city, i18n.Chain(lang))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	ids := make([]string, len(places))
	for i, p := range places {
		ids[i] = p.ID
	}
	rules, err := s.store.Rules(r.Context(), ids)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]geofenceDTO, 0, len(places))
	for _, p := range places {
		top := rules[p.ID]
		if len(top) > 3 {
			top = top[:3]
		}
		out = append(out, geofenceDTO{PlaceID: p.ID, Name: p.Name, Center: p.Loc, RadiusM: p.GeofenceRadiusM,
			Headline: i18n.T(lang, "geofence_headline", p.Name), TopRules: rulesDTO(top, lang), Version: p.RulesVersion})
	}
	writeCached(w, r, out)
}

// placeholder — rasm hali yo'q joylar uchun kategoriya rangidagi oddiy SVG.
func (s *Server) placeholder(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	if !ok || !(domain.ValidCategory(name) || name == "city" || name == "event" || slices.Contains(stayKinds, name)) {
		http.NotFound(w, r)
		return
	}
	colors := map[string]string{"monument": "#b5651d", "museum": "#6d4c8d", "mosque": "#1f7a8c", "bazaar": "#c27a06",
		"workshop": "#8a5a2b", "gallery": "#c93f7a", "restaurant": "#b03a2e", "cafe": "#7b5e3b", "park": "#3a8f3a",
		"viewpoint": "#2f6fd1", "ecovillage": "#5b8c2a", "theatre": "#8e3b6a", "city": "#0f8f7e", "event": "#a8551f",
		"hotel": "#2b5c8a", "boutique": "#7a4b8c", "guesthouse": "#a0602a", "hostel": "#3a7d5c", "motel": "#6b6b6b", "apartment": "#466b8f"}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 500"><rect width="800" height="500" fill="%s"/>`+
		`<text x="400" y="265" font-family="sans-serif" font-size="44" fill="#fff" fill-opacity="0.85" text-anchor="middle">%s</text></svg>`,
		colors[name], html.EscapeString(name))
}
