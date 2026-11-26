package api

// Lens Story va bir martalik chegirma kodlari (docs/MONETIZATION.md §4).
//
// Kod faqat joyda olinadi: joylashuv joydan 300 m ichida yoki shu joy bugungi marshrutda "current"/"visited".
// Yaqinda (1.5 km) story dasturidagi hamkor bo'lishi kerak. Bir foydalanuvchiga bir hamkor 7 kunda 1 ta kod,
// kod 7 kun amal qiladi. Haqiqatan joylangani tekshirilmaydi (ulashish tugmasi bosilgani yetarli).

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/store"
)

const (
	storyOnSiteM    = 300.0
	storyPartnerM   = 1500.0
	codeWindow      = 7 * 24 * time.Hour
	discountCodeTTL = 7 * 24 * time.Hour
	storyShareBase  = "/s/p/"
)

type storyReq struct {
	PlaceID  string         `json:"placeId"`
	Location *domain.LatLng `json:"location"`
}

func (s *Server) shareURL(placeID, shareID string) string {
	return s.cfg.WebBaseURL + storyShareBase + url.PathEscape(placeID) + "?ref=" + url.QueryEscape(shareID)
}

// createStory — POST /stories {placeId, location?} → {shareId, shareUrl}.
func (s *Server) createStory(w http.ResponseWriter, r *http.Request) {
	var req storyReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if req.PlaceID == "" {
		fe.add("placeId", "required")
	}
	if req.Location != nil && !req.Location.Valid() {
		fe.add("location", "invalid coordinates")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	p, err := s.store.Place(r.Context(), req.PlaceID, []string{"en"})
	if isNotFound(err) || err == nil && p.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	id, err := s.store.CreateStory(r.Context(), userOf(r), p.ID, req.Location)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"shareId": id, "shareUrl": s.shareURL(p.ID, id)})
}

type codePartnerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type codeDTO struct {
	ID          string         `json:"id"`
	Value       string         `json:"value"`
	Partner     codePartnerDTO `json:"partner"`
	DiscountPct int            `json:"discountPct"`
	ExpiresAt   string         `json:"expiresAt"`
	QRPayload   string         `json:"qrPayload"`
	PlaceID     *string        `json:"placeId,omitempty"`
	IssuedAt    string         `json:"issuedAt"`
	UsedAt      *string        `json:"usedAt,omitempty"`
	Sample      bool           `json:"sample"`
}

func (s *Server) codeDTO(c store.DiscountCode) codeDTO {
	d := codeDTO{ID: c.ID, Value: c.Value, Partner: codePartnerDTO{c.PartnerID, c.PartnerName}, DiscountPct: c.DiscountPct,
		ExpiresAt: c.ExpiresAt.UTC().Format(time.RFC3339), QRPayload: s.cfg.WebBaseURL + "/c/" + c.Value, PlaceID: c.PlaceID,
		IssuedAt: c.IssuedAt.UTC().Format(time.RFC3339), Sample: c.Sample}
	if c.UsedAt != nil {
		u := c.UsedAt.UTC().Format(time.RFC3339)
		d.UsedAt = &u
	}
	return d
}

type sharedResp struct {
	Code   *codeDTO `json:"code,omitempty"`
	Reason string   `json:"reason,omitempty"` // not_on_site | no_partner | recent
}

// storyShared — POST /stories/{shareId}/shared {location?}: ulashildi deb belgilaydi; shartlar bajarilsa kod beradi.
// recent: shu hamkor 7 kun ichida kod bergan, o'sha kod qaytadi.
func (s *Server) storyShared(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Location *domain.LatLng `json:"location"`
	}
	if !decodeOptionalJSON(w, r, &req) {
		return
	}
	if req.Location != nil && !req.Location.Valid() {
		writeValidation(w, r, fieldErrors{"location": "invalid coordinates"})
		return
	}
	ctx := r.Context()
	uid := userOf(r)
	st, err := s.store.Story(ctx, r.PathValue("shareId"), uid)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "story_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.store.MarkStoryShared(ctx, st.ShareID, req.Location); err != nil {
		s.internalError(w, r, err)
		return
	}
	p, err := s.store.Place(ctx, st.PlaceID, []string{"en"})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	loc := st.Loc
	if req.Location != nil {
		loc = req.Location
	}
	onSite := loc != nil && domain.DistanceM(*loc, p.Loc) <= storyOnSiteM
	if !onSite {
		city, err := s.store.City(ctx, p.CityID, []string{"en"})
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		n := s.now().In(tz(city.Timezone))
		if onSite, err = s.store.AtPlaceToday(ctx, uid, p.ID, time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	if !onSite {
		writeJSON(w, http.StatusOK, sharedResp{Reason: "not_on_site"})
		return
	}
	partner, _, err := s.store.NearestStoryPartner(ctx, p.Loc, storyPartnerM)
	if isNotFound(err) {
		writeJSON(w, http.StatusOK, sharedResp{Reason: "no_partner"})
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	code, recent, err := s.store.IssueCode(ctx, uid, partner, p.ID, st.ShareID, s.now(), codeWindow, discountCodeTTL)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	d := s.codeDTO(code)
	res := sharedResp{Code: &d}
	if recent {
		res.Reason = "recent"
	}
	writeJSON(w, http.StatusOK, res)
}

// myCodes — GET /me/codes: amaldagi kodlar; ?include=all — oxirgi 30 kundagi ishlatilgan va muddati o'tganlar ham.
func (s *Server) myCodes(w http.ResponseWriter, r *http.Request) {
	inc := r.URL.Query().Get("include")
	if inc != "" && inc != "all" {
		writeValidation(w, r, fieldErrors{"include": "all"})
		return
	}
	codes, err := s.store.UserCodes(r.Context(), userOf(r), inc == "all", s.now())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]codeDTO, 0, len(codes))
	for _, c := range codes {
		out = append(out, s.codeDTO(c))
	}
	writeJSON(w, http.StatusOK, out)
}

var discountCodeRe = regexp.MustCompile(`SF-?([A-Z0-9]{4})-?([A-Z0-9]{4})$`)

// normaliseCode — qo'lda yozilgan kod, skanerlangan QR (…/c/SF-XXXX-XXXX), bo'sh joy va kichik harf bilan ham.
// Sayt normaliseCode bilan bir xil.
func normaliseCode(raw string) string {
	t := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	if m := discountCodeRe.FindStringSubmatch(t); m != nil {
		return "SF-" + m[1] + "-" + m[2]
	}
	return t
}

// redeemCode — POST /partner/codes/redeem {value}: hamkor faqat o'z kodini tekshiradi.
func (s *Server) redeemCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	v := normaliseCode(req.Value)
	if v == "" || len(v) > 200 {
		writeValidation(w, r, fieldErrors{"value": "required"})
		return
	}
	p := partnerOf(r)
	status, c, err := s.store.RedeemCode(r.Context(), p.ID, v, s.now())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := map[string]any{"status": status}
	if status != "not_found" {
		out["discountPct"] = c.DiscountPct
		out["partner"] = p.Name
		out["sample"] = c.Sample
	}
	writeJSON(w, http.StatusOK, out)
}

// storyLink — GET /s/p/{placeId}?ref=: story havolasi. ref sanaladi, keyin saytdagi joy sahifasiga 302.
func (s *Server) storyLink(w http.ResponseWriter, r *http.Request) {
	placeID := r.PathValue("placeId")
	ref := r.URL.Query().Get("ref")
	if len(ref) > 64 {
		ref = ref[:64]
	}
	known := false
	if p, err := s.store.Place(r.Context(), placeID, []string{"en"}); err == nil && p.Status == "published" {
		known = true
		if ref != "" {
			if _, err := s.store.CountStoryRef(r.Context(), ref, p.ID); err != nil {
				s.internalError(w, r, err)
				return
			}
		}
	}
	target := s.cfg.WebBaseURL + "/" + s.webLang(r) + "/places"
	if known {
		target += "/" + url.PathEscape(placeID)
	}
	if ref != "" {
		target += "?ref=" + url.QueryEscape(ref)
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}
