package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/auth"
	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/media"
	"yoldosh/internal/store"
)

const keyAdmin ctxKey = 100

func adminOf(r *http.Request) store.AdminUser {
	a, _ := r.Context().Value(keyAdmin).(store.AdminUser)
	return a
}

// requireAdmin — faqat moderator/admin tokeni va faol admin hisobi.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || tok == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		c, err := s.signer.Verify(tok)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		if !auth.IsAdminRole(c.Role) {
			writeError(w, r, http.StatusForbidden, "forbidden", false)
			return
		}
		a, err := s.store.AdminByID(r.Context(), c.Subject)
		if err != nil || a.Disabled {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		ctx := context.WithValue(r.Context(), keyAdmin, a)
		ctx = context.WithValue(ctx, keyUser, a.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ---------- Kirish ----------

type adminDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	a, err := s.store.AdminByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil && !isNotFound(err) {
		s.internalError(w, r, err)
		return
	}
	// Topilmasa ham parol tekshiriladi — javob vaqti bir xil bo'lsin.
	if !auth.VerifyPassword(a.PasswordHash, req.Password) || a.ID == "" || a.Disabled {
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", false)
		return
	}
	tok, exp, err := s.signer.Issue(a.ID, a.Role, s.now())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	_ = s.store.TouchAdminLogin(r.Context(), a.ID)
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "expiresAt": exp.UTC().Format(time.RFC3339),
		"admin": adminDTO{a.ID, a.Email, a.Name, a.Role}})
}

func (s *Server) adminMe(w http.ResponseWriter, r *http.Request) {
	a := adminOf(r)
	writeJSON(w, http.StatusOK, adminDTO{a.ID, a.Email, a.Name, a.Role})
}

// ---------- Joylar ----------

func (s *Server) adminListPlaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AdminPlaceFilter{Status: q.Get("status"), City: q.Get("city"), Q: strings.TrimSpace(q.Get("q")), Limit: 50}
	fe := fieldErrors{}
	if f.Status != "" && !slices.Contains(placeStatuses, f.Status) {
		fe.add("status", strings.Join(placeStatuses, " | "))
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			fe.add("limit", "1..200")
		}
		f.Limit = n
	}
	off, ok := decodeCursor(q.Get("cursor"))
	if !ok {
		fe.add("cursor", "invalid")
	}
	f.Offset = off
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	rows, err := s.store.AdminPlaces(r.Context(), f)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	var next *string
	if len(rows) == f.Limit {
		next = encodeCursor(f.Offset + f.Limit)
	}
	writeJSON(w, http.StatusOK, page[store.AdminPlaceRow]{Items: rows, NextCursor: next})
}

var placeStatuses = []string{"draft", "pending", "published", "rejected"}

type adminRuleDTO struct {
	Code       string            `json:"code"`
	Severity   string            `json:"severity"`
	Priority   int               `json:"priority"`
	Text       map[string]string `json:"text"`
	VerifiedBy *string           `json:"verifiedBy"`
	VerifiedAt *time.Time        `json:"verifiedAt"`
}

func (s *Server) writeAdminPlace(w http.ResponseWriter, r *http.Request, status int, id string) {
	p, texts, rules, err := s.store.AdminPlace(r.Context(), id)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	rs := make([]adminRuleDTO, 0, len(rules))
	for _, x := range rules {
		rs = append(rs, adminRuleDTO{x.Code, x.Severity, x.Priority, x.Text, x.VerifiedBy, x.VerifiedAt})
	}
	if p.Images == nil {
		p.Images = []domain.Image{}
	}
	writeJSON(w, status, map[string]any{"place": p, "texts": texts, "rules": rs})
}

func (s *Server) adminGetPlace(w http.ResponseWriter, r *http.Request) {
	s.writeAdminPlace(w, r, http.StatusOK, r.PathValue("id"))
}

// placeInput — yaratish va qisman tahrirlash uchun. Yo'q maydon = o'zgarmaydi.
// texts: {"ru": {...}} — shu tilni qisman yangilaydi; {"ru": null} — tilni o'chiradi.
type placeInput struct {
	ID               *string                    `json:"id"`
	CityID           *string                    `json:"cityId"`
	Category         *string                    `json:"category"`
	Novelty          *string                    `json:"novelty"`
	OpenedAt         *string                    `json:"openedAt"` // "" = tozalash
	Status           *string                    `json:"status"`   // faqat yaratishda: draft | pending
	Location         *domain.LatLng             `json:"location"`
	GeofenceRadiusM  *int                       `json:"geofenceRadiusM"`
	Interests        *[]string                  `json:"interests"`
	PriceLevel       *int                       `json:"priceLevel"`
	VisitDurationMin *int                       `json:"visitDurationMin"`
	OpeningHours     *[]domain.OpeningHours     `json:"openingHours"`
	EntryFee         *json.RawMessage           `json:"entryFee"`
	Wheelchair       *bool                      `json:"wheelchair"`
	FamilyFriendly   *bool                      `json:"familyFriendly"`
	EasyWalk         *bool                      `json:"easyWalk"`
	PayCard          *bool                      `json:"payCard"`
	CashOnly         *bool                      `json:"cashOnly"`
	IsPartner        *bool                      `json:"isPartner"`
	Images           *[]domain.Image            `json:"images"`
	Contacts         *map[string]string         `json:"contacts"`
	Aliases          *[]string                  `json:"aliases"`
	Trust            *string                    `json:"trust"`
	VerifiedAt       *time.Time                 `json:"verifiedAt"`
	Texts            map[string]*placeTextInput `json:"texts"`
}

type placeTextInput struct {
	Name         *string          `json:"name"`
	Tagline      *string          `json:"tagline"`
	Description  *string          `json:"description"`
	Address      *string          `json:"address"`
	History      *string          `json:"history"` // "" = tozalash
	HistoryTrust *string          `json:"historyTrust"`
	Facts        *[]string        `json:"facts"`
	Sources      *json.RawMessage `json:"sources"`
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}

func (in placeInput) apply(p *store.PlaceRecord, texts map[string]store.PlaceText) {
	set(&p.CityID, in.CityID)
	set(&p.Category, in.Category)
	if in.Novelty != nil {
		p.Novelty = domain.Novelty(*in.Novelty)
	}
	if in.OpenedAt != nil {
		if *in.OpenedAt == "" {
			p.OpenedAt = nil
		} else {
			v := *in.OpenedAt
			p.OpenedAt = &v
		}
	}
	set(&p.Location, in.Location)
	set(&p.GeofenceRadiusM, in.GeofenceRadiusM)
	set(&p.Interests, in.Interests)
	set(&p.PriceLevel, in.PriceLevel)
	set(&p.VisitMin, in.VisitDurationMin)
	set(&p.OpeningHours, in.OpeningHours)
	set(&p.EntryFee, in.EntryFee)
	set(&p.Wheelchair, in.Wheelchair)
	set(&p.FamilyFriendly, in.FamilyFriendly)
	set(&p.EasyWalk, in.EasyWalk)
	set(&p.PayCard, in.PayCard)
	set(&p.CashOnly, in.CashOnly)
	set(&p.IsPartner, in.IsPartner)
	set(&p.Images, in.Images)
	set(&p.Contacts, in.Contacts)
	set(&p.Aliases, in.Aliases)
	set(&p.Trust, in.Trust)
	if in.VerifiedAt != nil {
		v := *in.VerifiedAt
		p.VerifiedAt = &v
	}
	for lang, ti := range in.Texts {
		if ti == nil {
			delete(texts, lang)
			continue
		}
		t := texts[lang]
		set(&t.Name, trimPtr(ti.Name))
		set(&t.Tagline, trimPtr(ti.Tagline))
		set(&t.Description, trimPtr(ti.Description))
		set(&t.Address, trimPtr(ti.Address))
		if ti.History != nil {
			if h := strings.TrimSpace(*ti.History); h == "" {
				t.History = nil
			} else {
				t.History = &h
			}
		}
		set(&t.Facts, ti.Facts)
		if ti.HistoryTrust != nil {
			v := *ti.HistoryTrust
			t.HistoryTrust = &v
		}
		set(&t.Sources, ti.Sources)
		texts[lang] = t
	}
}

var trustLevels = []string{"official", "public", "community", "sample"}

var ruleCodes = []string{"no_flash", "no_photo", "photo_fee", "dress_code", "shoes_off", "quiet",
	"no_drone", "no_food", "no_smoking", "no_pets", "ticket_required", "prayer_time_limit"}

func runeLen(s string, lo, hi int) bool { n := utf8.RuneCountInString(s); return n >= lo && n <= hi }

// validatePlace — saqlashdan oldin to'liq tekshiruv.
func (s *Server) validatePlace(r *http.Request, p store.PlaceRecord, texts map[string]store.PlaceText) fieldErrors {
	fe := fieldErrors{}
	if _, err := s.store.City(r.Context(), p.CityID, []string{"en", "uz"}); err != nil {
		fe.add("cityId", "unknown city")
	}
	if !domain.ValidCategory(p.Category) {
		fe.add("category", strings.Join(domain.Categories, " | "))
	}
	if !p.Novelty.Valid() {
		fe.add("novelty", "iconic | new | hidden")
	}
	if p.OpenedAt != nil {
		if d, err := time.Parse(time.DateOnly, *p.OpenedAt); err != nil || d.After(s.now().AddDate(0, 1, 0)) {
			fe.add("openedAt", "YYYY-MM-DD, not in the future")
		}
	}
	if !p.Location.Valid() || p.Location.Lat == 0 && p.Location.Lng == 0 {
		fe.add("location", "valid lat/lng required")
	}
	if p.GeofenceRadiusM < 20 || p.GeofenceRadiusM > 1000 {
		fe.add("geofenceRadiusM", "20..1000")
	}
	if !validInterests(p.Interests) {
		fe.add("interests", strings.Join(domain.Interests, " | "))
	}
	if p.PriceLevel < 0 || p.PriceLevel > 3 {
		fe.add("priceLevel", "0..3")
	}
	if p.VisitMin < 10 || p.VisitMin > 480 {
		fe.add("visitDurationMin", "10..480")
	}
	days := map[int]bool{}
	for _, h := range p.OpeningHours {
		o, err1 := domain.ParseHHMM(h.Open)
		c, err2 := domain.ParseHHMM(h.Close)
		if h.Day < 0 || h.Day > 6 || days[h.Day] || err1 != nil || err2 != nil || c <= o {
			fe.add("openingHours", "one entry per day 0..6, open < close, HH:MM")
			break
		}
		days[h.Day] = true
		for _, b := range h.Breaks {
			f, e1 := domain.ParseHHMM(b.From)
			t, e2 := domain.ParseHHMM(b.To)
			if e1 != nil || e2 != nil || f < o || t > c || t <= f ||
				b.Reason != "" && !slices.Contains([]string{"lunch", "prayer", "other"}, b.Reason) {
				fe.add("openingHours.breaks", "within open..close, from < to, reason lunch | prayer | other")
				break
			}
		}
	}
	if !slices.Contains(trustLevels, p.Trust) {
		fe.add("trust", strings.Join(trustLevels, " | "))
	}
	if len(p.Aliases) > 10 {
		fe.add("aliases", "max 10")
	}
	for _, a := range p.Aliases {
		if !runeLen(strings.TrimSpace(a), 2, 80) {
			fe.add("aliases", "2..80 chars each")
			break
		}
	}
	if len(p.EntryFee) > 0 && string(p.EntryFee) != "null" {
		var obj map[string]any
		if json.Unmarshal(p.EntryFee, &obj) != nil || len(p.EntryFee) > 1024 {
			fe.add("entryFee", "object or null")
		}
	}
	for k, v := range p.Contacts {
		if !slices.Contains([]string{"instagram", "telegram", "website", "phone"}, k) || !runeLen(v, 0, 200) {
			fe.add("contacts", "instagram | telegram | website | phone, max 200 chars")
			break
		}
	}
	if len(p.Images) > 20 {
		fe.add("images", "max 20")
	}
	for _, img := range p.Images {
		if !strings.HasPrefix(img.URL, "https://") && !strings.HasPrefix(img.URL, s.cfg.PublicBaseURL+"/") &&
			!strings.HasPrefix(img.URL, "/static/places/") { // katalog seed rasmlari nisbiy saqlanadi
			fe.add("images", "https URLs only")
			break
		}
	}
	if _, ok := texts["uz"]; !ok {
		fe.add("texts.uz", "required")
	}
	for lang, t := range texts {
		k := "texts." + lang
		switch {
		case !i18n.Valid(lang):
			fe.add(k, "uz | ru | en")
		case !runeLen(t.Name, 2, 120):
			fe.add(k+".name", "2..120 chars")
		case !runeLen(t.Tagline, 2, 160):
			fe.add(k+".tagline", "2..160 chars")
		case !runeLen(t.Description, 0, 3000) || !runeLen(t.Address, 0, 300):
			fe.add(k+".description", "max 3000 chars")
		case t.History != nil && !runeLen(*t.History, 0, 3000):
			fe.add(k+".history", "max 3000 chars")
		case len(t.Facts) > 10:
			fe.add(k+".facts", "max 10")
		case t.HistoryTrust != nil && !slices.Contains(trustLevels, *t.HistoryTrust):
			fe.add(k+".historyTrust", strings.Join(trustLevels, " | "))
		}
	}
	return fe
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// slugify — "Kulolchilik ustaxonasi" → "kulolchilik-ustaxonasi-3f9a".
func slugify(name string) string {
	r := strings.NewReplacer("o'", "o", "g'", "g", "oʻ", "o", "gʻ", "g", "ʼ", "", "'", "", "’", "")
	name = r.Replace(strings.ToLower(name))
	var b strings.Builder
	dash := false
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 50 {
		slug = strings.Trim(slug[:50], "-")
	}
	if slug == "" {
		slug = "place"
	}
	suffix := make([]byte, 2)
	_, _ = rand.Read(suffix)
	return slug + "-" + hex.EncodeToString(suffix)
}

func (s *Server) adminCreatePlace(w http.ResponseWriter, r *http.Request) {
	var in placeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := store.PlaceRecord{Status: "draft", Novelty: domain.NoveltyNew, GeofenceRadiusM: 80, VisitMin: 45,
		FamilyFriendly: true, EasyWalk: true, Interests: []string{}, Images: []domain.Image{}, Source: "manual",
		Contacts: map[string]string{}, Aliases: []string{}, Trust: "public"}
	texts := map[string]store.PlaceText{}
	in.apply(&p, texts)
	fe := s.validatePlace(r, p, texts)
	if in.Status != nil {
		if *in.Status != "draft" && *in.Status != "pending" {
			fe.add("status", "draft | pending (publish separately)")
		}
		p.Status = *in.Status
	}
	if in.ID != nil {
		if !slugRe.MatchString(*in.ID) {
			fe.add("id", "lowercase slug: a-z 0-9 -")
		}
		p.ID = *in.ID
	} else if t, ok := texts["uz"]; ok {
		p.ID = slugify(t.Name)
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SavePlace(r.Context(), adminOf(r).ID, p, texts, true, nil)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, http.StatusConflict, "place_id_taken", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeAdminPlace(w, r, http.StatusCreated, p.ID)
}

func (s *Server) adminPatchPlace(w http.ResponseWriter, r *http.Request) {
	var in placeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.ID != nil || in.Status != nil {
		writeValidation(w, r, fieldErrors{"id/status": "use publish / reject / submit endpoints"})
		return
	}
	p, texts, _, err := s.store.AdminPlace(r.Context(), r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	before := map[string]any{"place": p, "texts": texts}
	beforeJSON, _ := json.Marshal(before) // apply texts xaritasini o'zgartiradi — avvalgi holat nusxasi
	in.apply(&p, texts)
	if fe := s.validatePlace(r, p, texts); len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	if err := s.store.SavePlace(r.Context(), adminOf(r).ID, p, texts, false, json.RawMessage(beforeJSON)); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeAdminPlace(w, r, http.StatusOK, p.ID)
}

// changeStatus — holat o'tishlari: submit (→pending), publish, reject (sabab bilan), unpublish (→draft).
func (s *Server) changeStatus(to string, from ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var reason *string
		if to == "rejected" {
			var req struct {
				Reason string `json:"reason"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			req.Reason = strings.TrimSpace(req.Reason)
			if !runeLen(req.Reason, 3, 500) {
				writeValidation(w, r, fieldErrors{"reason": "3..500 chars"})
				return
			}
			reason = &req.Reason
		}
		p, texts, _, err := s.store.AdminPlace(r.Context(), id)
		if isNotFound(err) {
			writeError(w, r, http.StatusNotFound, "place_not_found", false)
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if !slices.Contains(from, p.Status) {
			writeError(w, r, http.StatusConflict, "invalid_status_transition", false)
			return
		}
		if to == "published" {
			// Chop etishdan oldin to'liq tekshiruv: yarim to'ldirilgan joy turistga chiqmasin.
			if fe := s.validatePlace(r, p, texts); len(fe) > 0 {
				writeValidation(w, r, fe)
				return
			}
		}
		if err := s.store.SetPlaceStatus(r.Context(), adminOf(r).ID, id, to, reason); err != nil {
			s.internalError(w, r, err)
			return
		}
		// AI xizmati ulanganda (7-bosqich) shu yerda qayta indekslash signali yuboriladi.
		s.writeAdminPlace(w, r, http.StatusOK, id)
	}
}

func (s *Server) adminPutRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rules []store.RuleInput `json:"rules"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if len(req.Rules) > 15 {
		fe.add("rules", "max 15")
	}
	for i, x := range req.Rules {
		k := "rules[" + strconv.Itoa(i) + "]"
		switch {
		case !slices.Contains(ruleCodes, x.Code):
			fe.add(k+".code", strings.Join(ruleCodes, " | "))
		case x.Severity != "info" && x.Severity != "warn" && x.Severity != "forbidden":
			fe.add(k+".severity", "info | warn | forbidden")
		case x.Trust != "" && !slices.Contains(trustLevels, x.Trust):
			fe.add(k+".trust", strings.Join(trustLevels, " | "))
		case x.Priority < 0 || x.Priority > 100:
			fe.add(k+".priority", "0..100")
		case !runeLen(strings.TrimSpace(x.Text["uz"]), 3, 200):
			fe.add(k+".text.uz", "3..200 chars")
		}
		for lang, t := range x.Text {
			if !i18n.Valid(lang) || !runeLen(t, 0, 200) {
				fe.add(k+".text."+lang, "uz | ru | en, max 200 chars")
			}
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	version, err := s.store.ReplaceRules(r.Context(), adminOf(r).ID, r.PathValue("id"), req.Rules)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"version": version})
}

func (s *Server) adminUploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", false)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("image")
	if err != nil {
		writeValidation(w, r, fieldErrors{"image": "multipart file required"})
		return
	}
	defer f.Close()
	credit := strings.TrimSpace(r.FormValue("credit"))
	if !runeLen(credit, 0, 200) {
		writeValidation(w, r, fieldErrors{"credit": "max 200 chars"})
		return
	}
	data, _, _, err := media.Process(f)
	switch {
	case errors.Is(err, media.ErrUnsupported):
		writeError(w, r, http.StatusUnsupportedMediaType, "image_unsupported", false)
		return
	case errors.Is(err, media.ErrTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", false)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	if _, _, _, err := s.store.AdminPlace(r.Context(), r.PathValue("id")); isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	url, err := s.media.Save(media.NewName(), data)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	images, err := s.store.AppendPlaceImage(r.Context(), adminOf(r).ID, r.PathValue("id"), domain.Image{URL: url, Credit: credit})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"image": domain.Image{URL: url, Credit: credit}, "images": images})
}

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !media.SafeName(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, s.cfg.UploadDir+"/"+name)
}

func (s *Server) adminNovelty(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			writeValidation(w, r, fieldErrors{"days": "1..365"})
			return
		}
		days = n
	}
	k, err := s.store.NoveltyAnalytics(r.Context(), s.now().AddDate(0, 0, -days))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": days, "cities": k})
}

// newAdminID — CLI uchun.
func NewAdminID() string { return "a_" + uuid.Must(uuid.NewV7()).String() }
