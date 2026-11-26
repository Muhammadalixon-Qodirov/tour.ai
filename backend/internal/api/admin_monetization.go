package api

// Admin: hamkorlar, takliflar, hamkor API kalitlari va foydalanuvchi rollari (partner, insights).
// Pul bilan bog'liq amallar faqat admin rolida (moderator emas). Har o'zgarish admin_audit ga yoziladi.

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/store"
)

var monetIDRe = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

// adminOnly — moderator emas, faqat admin.
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if adminOf(r).Role != "admin" {
			writeError(w, r, http.StatusForbidden, "forbidden", false)
			return
		}
		next(w, r)
	}
}

// mergePatch — joriy yozuv ustiga PATCH tanasidagi maydonlar (null = tozalash). Noma'lum maydon rad etiladi.
func mergePatch(w http.ResponseWriter, r *http.Request, dst any) bool {
	var patch map[string]json.RawMessage
	if !decodeJSONLoose(w, r, &patch) {
		return false
	}
	cur, _ := json.Marshal(dst)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(cur, &m)
	for k, v := range patch {
		if k == "id" {
			continue
		}
		m[k] = v
	}
	merged, _ := json.Marshal(m)
	dec := json.NewDecoder(bytes.NewReader(merged))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return false
	}
	return true
}

func decodeJSONLoose(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return false
	}
	return true
}

func validText(m map[string]string, required bool, max int) bool {
	if required && strings.TrimSpace(m["uz"]) == "" {
		return false
	}
	for k, v := range m {
		if k != "uz" && k != "ru" && k != "en" || utf8.RuneCountInString(v) > max {
			return false
		}
	}
	return true
}

func validURL(v *string) bool {
	if v == nil {
		return true
	}
	u, err := url.Parse(strings.NewReplacer("{subId}", "x", "{offerId}", "x").Replace(*v))
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// ---------- hamkorlar ----------

func validatePartner(p store.Partner) fieldErrors {
	fe := fieldErrors{}
	if !monetIDRe.MatchString(p.ID) {
		fe.add("id", "a-z 0-9 _ , 2..40")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Name)); n < 2 || n > 80 {
		fe.add("name", "2..80 chars")
	}
	if p.Location != nil && !p.Location.Valid() {
		fe.add("location", "invalid coordinates")
	}
	if p.DiscountPct != nil && (*p.DiscountPct < 1 || *p.DiscountPct > 50) {
		fe.add("discountPct", "1..50")
	}
	if p.DiscountPct != nil && p.Location == nil {
		fe.add("location", "required for a story discount (discountPct)")
	}
	if !validURL(p.Website) {
		fe.add("website", "http(s) URL")
	}
	return fe
}

func (s *Server) savePartnerResp(w http.ResponseWriter, r *http.Request, p store.Partner, create bool) {
	if fe := validatePartner(p); len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SavePartner(r.Context(), adminOf(r).ID, p, create)
	switch {
	case err == store.ErrConflict:
		writeError(w, r, http.StatusConflict, "partner_id_taken", false)
		return
	case err == store.ErrInvalid:
		writeValidation(w, r, fieldErrors{"cityId": "unknown city"})
		return
	case isNotFound(err):
		writeError(w, r, http.StatusNotFound, "partner_not_found", false)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	out, err := s.store.Partner(r.Context(), p.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
	}
	writeJSON(w, status, out)
}

func (s *Server) adminListPartners(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Partners(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) adminCreatePartner(w http.ResponseWriter, r *http.Request) {
	p := store.Partner{Active: true}
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.ID == "" {
		p.ID = "p_" + strings.ToLower(store.RandomCode(10))
	}
	p.Name = strings.TrimSpace(p.Name)
	s.savePartnerResp(w, r, p, true)
}

func (s *Server) adminPatchPartner(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.Partner(r.Context(), r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "partner_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	created := p.CreatedAt
	if !mergePatch(w, r, &p) {
		return
	}
	p.CreatedAt = created
	s.savePartnerResp(w, r, p, false)
}

// adminCreatePartnerKey — yangi API kaliti. Kalit faqat shu javobda ko'rinadi (bazada xeshi).
func (s *Server) adminCreatePartnerKey(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		s.internalError(w, r, err)
		return
	}
	key := PartnerKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	k, err := s.store.CreatePartnerKey(r.Context(), adminOf(r).ID, r.PathValue("id"), hashPartnerKey(key), key[:8])
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "partner_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": k.ID, "key": key, "prefix": k.Prefix, "createdAt": k.CreatedAt.UTC().Format(time.RFC3339)})
}

func (s *Server) adminListPartnerKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.PartnerKeys(r.Context(), r.PathValue("id"))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) adminRevokePartnerKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("keyId"))
	if err == nil {
		err = s.store.RevokePartnerKey(r.Context(), adminOf(r).ID, r.PathValue("id"), id)
	} else {
		err = store.ErrNotFound
	}
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- takliflar ----------

func validDate(v *string) bool {
	if v == nil {
		return true
	}
	_, err := time.Parse(time.DateOnly, *v)
	return err == nil
}

func (s *Server) validateOffer(r *http.Request, o store.Offer) fieldErrors {
	fe := fieldErrors{}
	if !monetIDRe.MatchString(o.ID) {
		fe.add("id", "a-z 0-9 _ , 2..40")
	}
	if !slices.Contains(offerSlots, o.Slot) {
		fe.add("slot", strings.Join(offerSlots, " | "))
	}
	if o.Mode != nil && (!slices.Contains(transportModes, *o.Mode) || !strings.HasPrefix(o.Slot, "transport.")) {
		fe.add("mode", strings.Join(transportModes, " | ")+" (transport slots only)")
	}
	if (o.RouteFrom == nil) != (o.RouteTo == nil) || o.RouteFrom != nil && *o.RouteFrom == *o.RouteTo {
		fe.add("routeFrom", "routeFrom and routeTo together, different cities")
	}
	if !validDate(o.ValidFrom) || !validDate(o.ValidTo) || o.ValidFrom != nil && o.ValidTo != nil && *o.ValidFrom > *o.ValidTo {
		fe.add("validFrom", "YYYY-MM-DD, validFrom <= validTo")
	}
	if o.DaysMin != nil && (*o.DaysMin < 1 || *o.DaysMin > 365) || o.DaysMax != nil && (*o.DaysMax < 1 || *o.DaysMax > 365) ||
		o.DaysMin != nil && o.DaysMax != nil && *o.DaysMin > *o.DaysMax {
		fe.add("daysMin", "1..365, daysMin <= daysMax")
	}
	if !validText(o.Title, true, 120) {
		fe.add("title", "{uz (required), ru, en}, up to 120 chars")
	}
	if !validText(o.Body, false, 200) {
		fe.add("body", "{uz, ru, en}, up to 200 chars")
	}
	if !validText(o.CTA, true, 40) {
		fe.add("cta", "{uz (required), ru, en}, up to 40 chars")
	}
	if o.PriceFrom != nil && *o.PriceFrom < 0 {
		fe.add("priceFrom", ">= 0")
	}
	if o.PriceUnit != "trip" && o.PriceUnit != "day" {
		fe.add("priceUnit", "trip | day")
	}
	if !validURL(o.LinkTemplate) {
		fe.add("linkTemplate", "http(s) URL; {subId} {offerId} placeholders")
	}
	if o.Active && !o.Sample && o.LinkTemplate == nil {
		fe.add("linkTemplate", "required for a live (non-sample) offer")
	}
	if o.Priority < -100 || o.Priority > 100 {
		fe.add("priority", "-100..100")
	}
	if len(fe) == 0 {
		p, err := s.store.Partner(r.Context(), o.PartnerID)
		if err != nil {
			fe.add("partnerId", "unknown partner")
		} else if p.Sample && !o.Sample {
			fe.add("sample", "a sample partner's offers are samples")
		}
	}
	return fe
}

func (s *Server) saveOfferResp(w http.ResponseWriter, r *http.Request, o store.Offer, create bool) {
	if fe := s.validateOffer(r, o); len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SaveOffer(r.Context(), adminOf(r).ID, o, create)
	switch {
	case err == store.ErrConflict:
		writeError(w, r, http.StatusConflict, "offer_id_taken", false)
		return
	case err == store.ErrInvalid:
		writeValidation(w, r, fieldErrors{"cityId": "unknown city"})
		return
	case isNotFound(err):
		writeError(w, r, http.StatusNotFound, "offer_not_found", false)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	out, err := s.store.Offer(r.Context(), o.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	status := http.StatusOK
	if create {
		status = http.StatusCreated
	}
	writeJSON(w, status, out)
}

func (s *Server) adminListOffers(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Offers(r.Context(), r.URL.Query().Get("partner"))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) adminCreateOffer(w http.ResponseWriter, r *http.Request) {
	o := store.Offer{Active: true, PriceUnit: "trip"}
	if !decodeJSON(w, r, &o) {
		return
	}
	if o.ID == "" {
		o.ID = "o_" + strings.ToLower(store.RandomCode(10))
	}
	s.saveOfferResp(w, r, o, true)
}

func (s *Server) adminPatchOffer(w http.ResponseWriter, r *http.Request) {
	o, err := s.store.Offer(r.Context(), r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "offer_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if !mergePatch(w, r, &o) {
		return
	}
	s.saveOfferResp(w, r, o, false)
}

// ---------- rollar ----------

type roleReq struct {
	Role      string   `json:"role"` // tourist | partner | insights
	PartnerID *string  `json:"partnerId"`
	Regions   []string `json:"regions"`
}

// adminSetUserRole — PUT /admin/users/{id}/role. {id} — foydalanuvchi ID si yoki email.
func (s *Server) adminSetUserRole(w http.ResponseWriter, r *http.Request) {
	var req roleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	switch req.Role {
	case "tourist":
		req.PartnerID, req.Regions = nil, nil
	case "partner":
		if req.PartnerID == nil || *req.PartnerID == "" {
			fe.add("partnerId", "required for role partner")
		}
		req.Regions = nil
	case "insights":
		if len(req.Regions) == 0 || len(req.Regions) > 20 {
			fe.add("regions", "1..20 city ids")
		}
		req.PartnerID = nil
	default:
		fe.add("role", "tourist | partner | insights")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	var u store.User
	var err error
	if strings.Contains(id, "@") {
		u, err = s.store.UserByEmail(ctx, id)
	} else {
		u, err = s.store.User(ctx, id)
	}
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "user_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if req.PartnerID != nil {
		if _, err := s.store.Partner(ctx, *req.PartnerID); err != nil {
			writeValidation(w, r, fieldErrors{"partnerId": "unknown partner"})
			return
		}
	}
	err = s.store.SetUserRole(ctx, adminOf(r).ID, u.ID, req.Role, req.PartnerID, req.Regions)
	if err == store.ErrInvalid {
		writeValidation(w, r, fieldErrors{"regions": "unknown city"})
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	regions, err := s.store.InsightsRegions(ctx, u.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "email": u.Email, "role": req.Role, "partnerId": req.PartnerID, "regions": regions})
}
