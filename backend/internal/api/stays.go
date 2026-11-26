package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

var stayKinds = []string{"hotel", "boutique", "guesthouse", "hostel", "motel", "apartment"}

type stayPriceDTO struct {
	Min       *domain.Money `json:"min"`
	Max       *domain.Money `json:"max"`
	Estimated bool          `json:"estimated"` // true — taxminiy (turi/yulduzi bo'yicha), UI da "≈" bilan
	Trust     *string       `json:"trust"`
	Note      *string       `json:"note,omitempty"`
	UpdatedAt *string       `json:"updatedAt,omitempty"`
	Source    *sourceDTO    `json:"source,omitempty"` // narx qayerdan olingan (tadqiqot yozuvlari)
}

type stayDTO struct {
	ID            string            `json:"id"`
	CityID        string            `json:"cityId"`
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	Cover         domain.Image      `json:"cover"`
	Images        []domain.Image    `json:"images"`
	Location      domain.LatLng     `json:"location"`
	Address       string            `json:"address"`
	Stars         *int              `json:"stars"`
	Rating        *float64          `json:"rating"`                 // 5 ballik
	RatingSource  *sourceDTO        `json:"ratingSource,omitempty"` // reyting manbasi
	ReviewsCount  int               `json:"reviewsCount"`
	PricePerNight stayPriceDTO      `json:"pricePerNight"`
	FairPrice     bool              `json:"fairPrice"`
	Verified      bool              `json:"verified"`
	IsPartner     bool              `json:"isPartner"`
	Amenities     []string          `json:"amenities"`
	Contacts      map[string]string `json:"contacts"`
	CheckIn       *string           `json:"checkIn,omitempty"`
	CheckOut      *string           `json:"checkOut,omitempty"`
	Wheelchair    *bool             `json:"wheelchair"`
	EMehmon       *bool             `json:"emehmon"` // chet ellikni E-Mehmon orqali ro'yxatga oladimi (null — noma'lum)
	// Hamkor bron havolalari faqat WEB saytda ko'rsatiladi (Booking shartlari: mobil ilovada taqiqlangan).
	BookingURL *string `json:"bookingUrl,omitempty"`
	AgodaURL   *string `json:"agodaUrl,omitempty"`
	Trust      string  `json:"trust"`
	DistanceM  *int    `json:"distanceM,omitempty"`
}

func (s *Server) stayDTO(v store.Stay, lang string) stayDTO {
	d := stayDTO{ID: v.ID, CityID: v.CityID, Name: i18n.Pick(v.Name, lang), Kind: v.Kind, Images: s.absImages(v.Images), Location: v.Loc,
		Address: v.Address, Stars: v.Stars, Rating: v.Rating, ReviewsCount: v.ReviewsCount, FairPrice: v.FairPrice,
		Verified: v.Verified, IsPartner: v.IsPartner, Amenities: nonNil(v.Amenities), Contacts: v.Contacts, CheckIn: v.CheckIn,
		CheckOut: v.CheckOut, Wheelchair: v.Wheelchair, EMehmon: v.EMehmon, BookingURL: v.BookingURL, AgodaURL: v.AgodaURL,
		Trust: v.Trust, RatingSource: sourceOf(v.RatingSource, nil)}
	if d.Contacts == nil {
		d.Contacts = map[string]string{}
	}
	if len(d.Images) > 0 {
		d.Cover = d.Images[0]
	} else {
		d.Cover = domain.Image{URL: s.cfg.PublicBaseURL + "/static/ph/" + v.Kind + ".svg"}
	}
	p := stayPriceDTO{Trust: v.PriceTrust, Note: v.PriceNote, Estimated: v.PriceTrust != nil && *v.PriceTrust == "estimate",
		Source: sourceOf(v.PriceSource, nil)}
	if v.PriceMin != nil {
		m := domain.UZS(*v.PriceMin)
		p.Min = &m
	}
	if v.PriceMax != nil {
		m := domain.UZS(*v.PriceMax)
		p.Max = &m
	}
	if v.PriceUpdatedAt != nil {
		t := v.PriceUpdatedAt.UTC().Format(time.RFC3339)
		p.UpdatedAt = &t
	}
	d.PricePerNight = p
	if v.DistanceM != nil {
		m := int(*v.DistanceM)
		d.DistanceM = &m
	}
	return d
}

func (s *Server) listStays(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fe := fieldErrors{}
	f := store.StayFilter{City: q.Get("city"), Q: strings.TrimSpace(q.Get("q")), Limit: 20}
	if k := q.Get("kind"); k != "" {
		for _, x := range strings.Split(k, ",") {
			if !slices.Contains(stayKinds, x) {
				fe.add("kind", strings.Join(stayKinds, " | "))
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
		f.Near, f.RadiusM = &p, 3000
	}
	if v := q.Get("radiusM"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n <= 0 || n > 50000 {
			fe.add("radiusM", "1..50000")
		}
		f.RadiusM = n
	}
	if v := q.Get("maxPrice"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			fe.add("maxPrice", "so'm, > 0")
		}
		f.MaxPrice = n
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
	if f.City == "" && f.Near == nil {
		fe.add("city", "city or near required")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	list, err := s.store.Stays(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	lang := langOf(r)
	items := make([]stayDTO, 0, len(list))
	for _, v := range list {
		items = append(items, s.stayDTO(v, lang))
	}
	var next *string
	if len(list) == f.Limit {
		next = encodeCursor(f.Offset + f.Limit)
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, page[stayDTO]{Items: items, NextCursor: next})
}

func (s *Server) getStay(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.Stay(r.Context(), r.PathValue("id"))
	if isNotFound(err) || err == nil && v.Status != "published" {
		writeError(w, r, http.StatusNotFound, "stay_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.stayDTO(v, langOf(r)))
}

// ---------- Admin ----------

func (s *Server) adminListStays(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "published"
	}
	off, _ := decodeCursor(q.Get("cursor"))
	list, err := s.store.Stays(r.Context(), store.StayFilter{City: q.Get("city"), Status: status, Q: q.Get("q"), Limit: 50, Offset: off})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	var next *string
	if len(list) == 50 {
		next = encodeCursor(off + 50)
	}
	writeJSON(w, http.StatusOK, page[store.Stay]{Items: list, NextCursor: next})
}

// adminPatchStay — qisman tahrir: narx (aniq), kontaktlar, E-Mehmon, qulayliklar, holat, bron havolalari.
func (s *Server) adminPatchStay(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.Stay(r.Context(), r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "stay_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	before := v
	raw := map[string]json.RawMessage{}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return
	}
	// faqat ruxsat etilgan maydonlar
	allowed := map[string]any{"name": &v.Name, "kind": &v.Kind, "status": &v.Status, "address": &v.Address, "stars": &v.Stars,
		"priceMin": &v.PriceMin, "priceMax": &v.PriceMax, "priceTrust": &v.PriceTrust, "priceNote": &v.PriceNote,
		"amenities": &v.Amenities, "contacts": &v.Contacts, "checkIn": &v.CheckIn, "checkOut": &v.CheckOut,
		"wheelchair": &v.Wheelchair, "emehmon": &v.EMehmon, "images": &v.Images, "bookingUrl": &v.BookingURL,
		"agodaUrl": &v.AgodaURL, "isPartner": &v.IsPartner, "fairPrice": &v.FairPrice, "verified": &v.Verified,
		"trust": &v.Trust, "location": &v.Loc}
	fe := fieldErrors{}
	for k, val := range raw {
		dst, ok := allowed[k]
		if !ok {
			fe.add(k, "unknown field")
			continue
		}
		if err := json.Unmarshal(val, dst); err != nil {
			fe.add(k, err.Error())
		}
	}
	switch {
	case !slices.Contains(stayKinds, v.Kind):
		fe.add("kind", strings.Join(stayKinds, " | "))
	case !slices.Contains(placeStatuses, v.Status):
		fe.add("status", strings.Join(placeStatuses, " | "))
	case strings.TrimSpace(v.Name["uz"]) == "":
		fe.add("name.uz", "required")
	case v.Stars != nil && (*v.Stars < 1 || *v.Stars > 5):
		fe.add("stars", "1..5")
	case v.PriceMin != nil && *v.PriceMin <= 0, v.PriceMax != nil && v.PriceMin != nil && *v.PriceMax < *v.PriceMin:
		fe.add("price", "priceMin > 0, priceMax >= priceMin (so'm)")
	case v.PriceTrust != nil && !slices.Contains(append(slices.Clone(trustLevels), "estimate"), *v.PriceTrust):
		fe.add("priceTrust", "official | public | community | sample | estimate")
	case !slices.Contains(trustLevels, v.Trust):
		fe.add("trust", strings.Join(trustLevels, " | "))
	case !v.Loc.Valid():
		fe.add("location", "invalid")
	}
	if _, ok := raw["priceMin"]; ok && v.PriceTrust != nil && *v.PriceTrust == "estimate" {
		t := "official" // narx qo'lda kiritildi — endi taxmin emas (import ustiga yozmaydi)
		v.PriceTrust = &t
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	if err := s.store.SaveStay(r.Context(), adminOf(r).ID, v, before); err != nil {
		s.internalError(w, r, err)
		return
	}
	saved, err := s.store.Stay(r.Context(), v.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
