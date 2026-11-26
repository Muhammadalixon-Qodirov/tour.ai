package api

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

var serviceKinds = []string{"hospital", "clinic", "pharmacy", "police", "embassy", "atm", "exchange", "fuel", "ev_charging"}

type serviceDTO struct {
	ID        string                `json:"id"`
	CityID    string                `json:"cityId"`
	Kind      string                `json:"kind"`
	Name      string                `json:"name"`
	Address   string                `json:"address"`
	Location  domain.LatLng         `json:"location"`
	Phone     *string               `json:"phone,omitempty"`
	Open24h   bool                  `json:"open24h"`
	IsOpenNow *bool                 `json:"isOpenNow,omitempty"` // ish vaqti noma'lum bo'lsa yo'q
	Hours     []domain.OpeningHours `json:"hours"`
	Accepts   []string              `json:"accepts"`
	DistanceM *int                  `json:"distanceM,omitempty"`
	IsDemo    bool                  `json:"isDemo"` // demo ma'lumot — ilovada ogohlantirish ko'rsatilsin
}

func (s *Server) serviceDTO(v store.Service, lang string, loc *time.Location) serviceDTO {
	d := serviceDTO{ID: v.ID, CityID: v.CityID, Kind: v.Kind, Name: i18n.Pick(v.Name, lang), Address: i18n.Pick(v.Address, lang),
		Location: v.Loc, Phone: v.Phone, Open24h: v.Open24h, Hours: v.Hours, Accepts: nonNil(v.Accepts), IsDemo: v.Source == "demo"}
	if d.Hours == nil {
		d.Hours = []domain.OpeningHours{}
	}
	if v.Open24h {
		t := true
		d.IsOpenNow = &t
	} else if len(v.Hours) > 0 && loc != nil {
		now := s.now().In(loc)
		o := domain.IsOpenAt(v.Hours, int(now.Weekday()), now.Hour()*60+now.Minute())
		d.IsOpenNow = &o
	}
	if v.DistanceM != nil {
		m := int(*v.DistanceM)
		d.DistanceM = &m
	}
	return d
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fe := fieldErrors{}
	f := store.ServiceFilter{City: q.Get("city"), Limit: 20}
	if f.City == "" {
		fe.add("city", "required")
	}
	if k := q.Get("kind"); k != "" {
		for _, x := range strings.Split(k, ",") {
			if !slices.Contains(serviceKinds, x) {
				fe.add("kind", strings.Join(serviceKinds, " | "))
				break
			}
			f.Kinds = append(f.Kinds, x)
		}
	}
	if v := q.Get("near"); v != "" {
		p, ok := parseLatLng(v)
		if !ok {
			fe.add("near", "lat,lng")
		}
		f.Near = &p
	}
	if v := q.Get("radiusM"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n <= 0 || n > 50000 {
			fe.add("radiusM", "1..50000")
		}
		f.RadiusM = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			fe.add("limit", "1..50")
		}
		f.Limit = n
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	city, err := s.store.City(r.Context(), f.City, []string{"en"})
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "city_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	list, err := s.store.Services(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	loc, lang := tz(city.Timezone), langOf(r)
	out := make([]serviceDTO, 0, len(list))
	for _, v := range list {
		d := s.serviceDTO(v, lang, loc)
		if q.Get("openNow") == "true" && (d.IsOpenNow == nil || !*d.IsOpenNow) {
			continue
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

type emergencyNumber struct {
	Kind   string `json:"kind"`
	Number string `json:"number"`
	Label  string `json:"label"`
}

var emergencyLabels = map[string]map[string]string{
	"unified":            {"uz": "Yagona favqulodda xizmat", "ru": "Единая служба экстренной помощи", "en": "Emergency (all services)"},
	"ambulance":          {"uz": "Tez yordam", "ru": "Скорая помощь", "en": "Ambulance"},
	"police":             {"uz": "Politsiya", "ru": "Полиция", "en": "Police"},
	"fire":               {"uz": "Yong'in xizmati", "ru": "Пожарная служба", "en": "Fire service"},
	"gas":                {"uz": "Gaz avariya xizmati", "ru": "Аварийная газовая служба", "en": "Gas emergency"},
	"emergency_ministry": {"uz": "Favqulodda vaziyatlar vazirligi", "ru": "МЧС", "en": "Ministry of Emergency Situations"},
	"tourist_hotline":    {"uz": "Turistlar uchun call-markaz (24/7, uz/ru/en)", "ru": "Колл-центр для туристов (24/7, uz/ru/en)", "en": "Tourist hotline (24/7, English)"},
}

func emergencyNumbers(c store.City, lang string) []emergencyNumber {
	out := []emergencyNumber{}
	for _, k := range []string{"unified", "tourist_hotline", "ambulance", "police", "fire", "gas", "emergency_ministry"} {
		if n, ok := c.Emergency[k]; ok && n != "" {
			out = append(out, emergencyNumber{Kind: k, Number: n, Label: i18n.Pick(emergencyLabels[k], lang)})
		}
	}
	return out
}

// emergency — SOS ekrani: raqamlar + (joylashuv berilsa) eng yaqin shifoxona, dorixona, politsiya.
// Joylashuv saqlanmaydi va loglarga yozilmaydi.
func (s *Server) emergency(w http.ResponseWriter, r *http.Request) {
	lang := langOf(r)
	city, err := s.store.City(r.Context(), r.PathValue("id"), i18n.Chain(lang))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "city_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp := map[string]any{"cityId": city.ID, "numbers": emergencyNumbers(city, lang), "nearest": map[string]serviceDTO{},
		"guide": i18n.T(lang, "sos.lost_documents")}
	if v := r.URL.Query().Get("near"); v != "" {
		p, ok := parseLatLng(v)
		if !ok {
			writeValidation(w, r, fieldErrors{"near": "lat,lng"})
			return
		}
		nearest := map[string]serviceDTO{}
		loc := tz(city.Timezone)
		for _, kind := range []string{"hospital", "pharmacy", "police", "embassy"} {
			sv, err := s.store.NearestService(r.Context(), city.ID, kind, p)
			if isNotFound(err) {
				continue
			}
			if err != nil {
				s.internalError(w, r, err)
				return
			}
			nearest[kind] = s.serviceDTO(sv, lang, loc)
		}
		resp["nearest"] = nearest
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, resp)
}

// offlinePack — shahar bo'yicha bitta JSON: joylar (to'liq), qoidalar, geofence, xizmatlar, favqulodda raqamlar.
// ETag versiya + til bo'yicha; If-None-Match → 304; gzip.
func (s *Server) offlinePack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lang := langOf(r)
	city, err := s.store.City(ctx, r.PathValue("id"), i18n.Chain(lang))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "city_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	version, err := s.store.OfflineVersion(ctx, city.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	sum := sha256.Sum256([]byte(version + "|" + lang + "|v1"))
	etag := `"` + hex.EncodeToString(sum[:12]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Vary", "Accept-Language, Accept-Encoding")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	chain := i18n.Chain(lang)
	summaries, err := s.store.ListPlaces(ctx, store.PlaceFilter{City: city.ID, Limit: 2000}, chain)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	ids := make([]string, len(summaries))
	for i, p := range summaries {
		ids[i] = p.ID
	}
	rules, err := s.store.Rules(ctx, ids)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	places := make([]placeDTO, 0, len(ids))
	geofences := []geofenceDTO{}
	var images []string
	for _, id := range ids {
		p, err := s.store.Place(ctx, id, chain)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		d := s.placeDetailDTO(r, p, rules[id], lang)
		d.IsOpenNow = nil // oflaynda vaqt o'tadi — ilova o'zi hisoblaydi (openingHours bor)
		places = append(places, d)
		for _, img := range d.Images {
			images = append(images, img.URL)
		}
		if top := rules[id]; len(top) > 0 {
			if len(top) > 3 {
				top = top[:3]
			}
			geofences = append(geofences, geofenceDTO{PlaceID: p.ID, Name: p.Name, Center: p.Loc, RadiusM: p.GeofenceRadiusM,
				Headline: i18n.T(lang, "geofence_headline", p.Name), TopRules: rulesDTO(top, lang), Version: p.RulesVersion})
		}
	}
	svcs, err := s.store.Services(ctx, store.ServiceFilter{City: city.ID, Limit: 1000})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	services := make([]serviceDTO, 0, len(svcs))
	for _, v := range svcs {
		d := s.serviceDTO(v, lang, nil)
		services = append(services, d)
	}
	priceItems, err := s.store.Prices(ctx, store.PriceFilter{City: city.ID})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	prices := make([]priceItemDTO, 0, len(priceItems))
	for _, x := range priceItems {
		prices = append(prices, priceDTO(x, lang))
	}
	pack := map[string]any{
		"prices":  prices,
		"version": etag[1 : len(etag)-1], "generatedAt": s.now().UTC().Format(time.RFC3339), "lang": lang,
		"city":   map[string]any{"id": city.ID, "name": city.Name, "center": city.Center, "timezone": city.Timezone},
		"places": places, "geofences": geofences, "services": services,
		"emergency": map[string]any{"numbers": emergencyNumbers(city, lang), "guide": i18n.T(lang, "sos.lost_documents")},
		"imageUrls": nonNil(images),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		_ = json.NewEncoder(gz).Encode(pack)
		return
	}
	_ = json.NewEncoder(w).Encode(pack)
}

// ---------- Admin: xizmatlar ----------

type serviceInput struct {
	ID       *string                `json:"id"`
	CityID   *string                `json:"cityId"`
	Kind     *string                `json:"kind"`
	Name     *map[string]string     `json:"name"`
	Address  *map[string]string     `json:"address"`
	Location *domain.LatLng         `json:"location"`
	Phone    *string                `json:"phone"`
	Hours    *[]domain.OpeningHours `json:"hours"`
	Open24h  *bool                  `json:"open24h"`
	Accepts  *[]string              `json:"accepts"`
}

func (in serviceInput) apply(v *store.Service) {
	set(&v.CityID, in.CityID)
	set(&v.Kind, in.Kind)
	set(&v.Name, in.Name)
	set(&v.Address, in.Address)
	set(&v.Loc, in.Location)
	if in.Phone != nil {
		if p := strings.TrimSpace(*in.Phone); p == "" {
			v.Phone = nil
		} else {
			v.Phone = &p
		}
	}
	set(&v.Hours, in.Hours)
	set(&v.Open24h, in.Open24h)
	set(&v.Accepts, in.Accepts)
}

func (s *Server) validateService(r *http.Request, v store.Service) fieldErrors {
	fe := fieldErrors{}
	if _, err := s.store.City(r.Context(), v.CityID, []string{"en", "uz"}); err != nil {
		fe.add("cityId", "unknown city")
	}
	if !slices.Contains(serviceKinds, v.Kind) {
		fe.add("kind", strings.Join(serviceKinds, " | "))
	}
	if !runeLen(strings.TrimSpace(v.Name["uz"]), 2, 120) {
		fe.add("name.uz", "2..120 chars")
	}
	for lang := range v.Name {
		if !i18n.Valid(lang) {
			fe.add("name", "uz | ru | en")
		}
	}
	if !v.Loc.Valid() || v.Loc.Lat == 0 && v.Loc.Lng == 0 {
		fe.add("location", "valid lat/lng required")
	}
	if v.Phone != nil && !runeLen(*v.Phone, 3, 30) {
		fe.add("phone", "3..30 chars")
	}
	for _, h := range v.Hours {
		o, e1 := domain.ParseHHMM(h.Open)
		c, e2 := domain.ParseHHMM(h.Close)
		if h.Day < 0 || h.Day > 6 || e1 != nil || e2 != nil || c <= o {
			fe.add("hours", "day 0..6, open < close")
			break
		}
	}
	for _, a := range v.Accepts {
		if !slices.Contains([]string{"uzcard", "humo", "visa", "mastercard", "unionpay", "mir"}, a) {
			fe.add("accepts", "uzcard | humo | visa | mastercard | unionpay | mir")
			break
		}
	}
	return fe
}

func (s *Server) adminCreateService(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v := store.Service{Name: map[string]string{}, Address: map[string]string{}, Hours: []domain.OpeningHours{}, Accepts: []string{}}
	in.apply(&v)
	fe := s.validateService(r, v)
	if in.ID != nil {
		if !slugRe.MatchString(*in.ID) {
			fe.add("id", "lowercase slug")
		}
		v.ID = *in.ID
	} else {
		v.ID = slugify(v.CityID + " " + v.Kind)
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SaveService(r.Context(), adminOf(r).ID, v, true, nil)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, http.StatusConflict, "place_id_taken", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	saved, err := s.store.Service(r.Context(), v.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) adminPatchService(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.ID != nil {
		writeValidation(w, r, fieldErrors{"id": "can't change id"})
		return
	}
	v, err := s.store.Service(r.Context(), r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	before := v
	in.apply(&v)
	if fe := s.validateService(r, v); len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	if err := s.store.SaveService(r.Context(), adminOf(r).ID, v, false, before); err != nil {
		s.internalError(w, r, err)
		return
	}
	saved, err := s.store.Service(r.Context(), v.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) adminDeleteService(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteService(r.Context(), adminOf(r).ID, r.PathValue("id")); isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	} else if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
