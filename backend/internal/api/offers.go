package api

// Hamkor takliflari va shaharlararo yo'nalishlar (docs/MONETIZATION.md §1–3).
//
// Qoidalar: taklif faqat tanlovga mos kelganda (slot, shahar, yo'nalish, transport turi, sana, kunlar, rezidentlik)
// va har slotga bittadan; hamkor doim partner + sample bilan qaytadi (UI "Hamkor" / "Namuna hamkor" deb yozadi).
// Bosish havolasi /r/{clickId}: clickId imzolangan (HMAC), ichida taklif, foydalanuvchi, safar va vaqt.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

var (
	offerSlots = []string{"transport.cheapest", "transport.fastest", "transport.comfort", "transport.bus",
		"stay", "food", "event", "prep.esim", "prep.insurance", "prep.sim"}
	transportModes = []string{"train", "afrosiyob", "plane", "car", "bus"}
	arrivalChoices = []string{"cheapest", "fastest", "comfort", "bus"}
)

// clickTTL — taklif ko'rsatilgandan keyin havola shuncha vaqt ishlaydi.
const clickTTL = 30 * 24 * time.Hour

// decodeOptionalJSON — bo'sh tana ham qabul qilinadi (ilova `{}` yoki hech narsa yubormasligi mumkin).
func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return false
	}
	if strings.TrimSpace(string(raw)) == "" {
		return true
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return false
	}
	return true
}

// ---------- shaharlararo yo'nalishlar ----------

type intercityDTO struct {
	Mode        string        `json:"mode"`
	DurationMin int           `json:"durationMin"`
	PriceFrom   *domain.Money `json:"priceFrom,omitempty"` // kishi boshiga; car — mashina uchun
	Source      string        `json:"source"`
	SourceURL   *string       `json:"sourceUrl,omitempty"`
	UpdatedAt   string        `json:"updatedAt"`
	Estimate    bool          `json:"estimate,omitempty"` // Safar hisobi (masofa bo'yicha), e'lon qilingan raqam emas
	Via         *string       `json:"via,omitempty"`      // shu shaharda almashadi
}

func intercityOf(r store.IntercityRoute) intercityDTO {
	d := intercityDTO{Mode: r.Mode, DurationMin: r.DurationMin, Source: r.Source, SourceURL: r.SourceURL, UpdatedAt: r.UpdatedAt,
		Estimate: r.Estimate, Via: r.Via}
	if r.PriceFrom != nil {
		m := domain.UZS(*r.PriceFrom)
		d.PriceFrom = &m
	}
	return d
}

func (s *Server) listIntercity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	fe := fieldErrors{}
	if from == "" {
		fe.add("from", "required")
	}
	if to == "" {
		fe.add("to", "required")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	routes, err := s.store.IntercityRoutes(r.Context(), from, to)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]intercityDTO, 0, len(routes))
	for _, rt := range routes {
		out = append(out, intercityOf(rt))
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, out)
}

// choiceRoute — "Qanday borasiz?" varianti: cheapest = poyezd, fastest = samolyot yoki Afrosiyob (qaysi tezroq),
// comfort = mashina, bus = avtobus. Sayt va ilovadagi arrivalOptions bilan bir xil.
func choiceRoute(routes []store.IntercityRoute, choice string) (store.IntercityRoute, bool) {
	by := func(m string) (store.IntercityRoute, bool) {
		i := slices.IndexFunc(routes, func(r store.IntercityRoute) bool { return r.Mode == m })
		if i < 0 {
			return store.IntercityRoute{}, false
		}
		return routes[i], true
	}
	switch choice {
	case "cheapest":
		return by("train")
	case "fastest":
		p, okP := by("plane")
		a, okA := by("afrosiyob")
		switch {
		case okP && okA:
			if a.DurationMin < p.DurationMin {
				return a, true
			}
			return p, true
		case okP:
			return p, true
		default:
			return a, okA
		}
	case "comfort":
		return by("car")
	case "bus":
		return by("bus")
	}
	return store.IntercityRoute{}, false
}

// arrivalCostPerPerson — bir kishi yo'lga qancha to'laydi (mashina guruhga bo'linadi, 1000 so'mgacha yaxlitlanadi).
func arrivalCostPerPerson(r store.IntercityRoute, people int) int64 {
	if r.PriceFrom == nil {
		return 0
	}
	if r.Mode == "car" {
		return int64(jsRound(float64(*r.PriceFrom)/float64(max(1, people))/1000)) * 1000
	}
	return *r.PriceFrom
}

func jsRound(x float64) float64 {
	f := float64(int64(x))
	if x-f >= 0.5 {
		return f + 1
	}
	return f
}

// ---------- takliflar ----------

type offerPartnerDTO struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	Logo *domain.Image `json:"logo,omitempty"`
}

type offerDTO struct {
	ID        string          `json:"id"`
	Partner   offerPartnerDTO `json:"partner"`
	Slot      string          `json:"slot"`
	Title     string          `json:"title"`
	Body      string          `json:"body,omitempty"`
	CTA       string          `json:"cta"`
	URL       string          `json:"url"` // kuzatuv havolasi: {PUBLIC_BASE_URL}/r/{clickId}
	PriceFrom *domain.Money   `json:"priceFrom,omitempty"`
	Sample    bool            `json:"sample"`
}

type clickClaims struct {
	Offer  string `json:"o"`
	User   string `json:"u,omitempty"`
	Trip   string `json:"t,omitempty"`
	Issued int64  `json:"i"`
}

func (s *Server) clickKey() []byte {
	m := hmac.New(sha256.New, s.cfg.TokenSecret)
	m.Write([]byte("safar-click-v1"))
	return m.Sum(nil)
}

func (s *Server) signClick(c clickClaims) string {
	raw, _ := json.Marshal(c)
	p := base64.RawURLEncoding.EncodeToString(raw)
	m := hmac.New(sha256.New, s.clickKey())
	m.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}

func (s *Server) verifyClick(tok string) (clickClaims, bool) {
	var c clickClaims
	p, sig, ok := strings.Cut(tok, ".")
	if !ok || len(tok) > 512 {
		return c, false
	}
	m := hmac.New(sha256.New, s.clickKey())
	m.Write([]byte(p))
	want := base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return c, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Offer == "" {
		return c, false
	}
	return c, true
}

func offerText(m map[string]string, lang string, days int) string {
	return strings.ReplaceAll(i18n.Pick(m, lang), "{d}", strconv.Itoa(days))
}

func (s *Server) offerDTO(o store.Offer, lang string, days int, userID, tripID string) offerDTO {
	d := offerDTO{ID: o.ID, Partner: offerPartnerDTO{ID: o.PartnerID, Name: o.PartnerName}, Slot: o.Slot,
		Title: offerText(o.Title, lang, days), Body: offerText(o.Body, lang, days), CTA: offerText(o.CTA, lang, days),
		Sample: o.Sample || o.PartnerSample}
	if o.PriceFrom != nil {
		p := *o.PriceFrom
		if o.PriceUnit == "day" {
			p *= int64(days)
		}
		m := domain.UZS(p)
		d.PriceFrom = &m
	}
	d.URL = s.cfg.PublicBaseURL + "/r/" + s.signClick(clickClaims{Offer: o.ID, User: userID, Trip: tripID, Issued: s.now().Unix()})
	return d
}

func cityToday(now time.Time) time.Time {
	t := now.In(tz("Asia/Tashkent"))
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// listOffers — GET /offers: kontekstga mos 0 yoki 1 ta taklif. Bir nechtasi mos kelsa kontekst (slot, shahar,
// yo'nalish, sana) bo'yicha barqaror aylantiriladi: bir xil so'rov bir xil hamkorni ko'radi, boshqa kunda boshqasini.
func (s *Server) listOffers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	oq := store.OfferQuery{Slot: q.Get("slot"), City: q.Get("city"), From: q.Get("from"), To: q.Get("to"), Days: 1}
	fe := fieldErrors{}
	if !slices.Contains(offerSlots, oq.Slot) {
		fe.add("slot", strings.Join(offerSlots, " | "))
	}
	oq.Date = cityToday(s.now())
	if v := q.Get("date"); v != "" {
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			fe.add("date", "YYYY-MM-DD")
		}
		oq.Date = d
	}
	if v := q.Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 60 {
			fe.add("days", "1..60")
		}
		oq.Days = n
	}
	if v := q.Get("resident"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			fe.add("resident", "true | false")
		}
		oq.Resident = b
	}
	tripID := ""
	if v := q.Get("trip"); v != "" {
		if _, err := uuid.Parse(v); err != nil {
			fe.add("trip", "uuid")
		}
		tripID = v
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	ctx := r.Context()
	if choice, ok := strings.CutPrefix(oq.Slot, "transport."); ok {
		// Transport: hamkor tanlangan variantning transport turiga mos bo'lishi kerak (samolyot hamkori Afrosiyob
		// eng tez bo'lgan yo'nalishda chiqmaydi). Yo'nalish noma'lum bo'lsa taklif yo'q.
		if oq.To == "" {
			oq.To = oq.City
		}
		if oq.City == "" {
			oq.City = oq.To
		}
		if oq.From == "" || oq.To == "" || oq.From == oq.To {
			writeJSON(w, http.StatusOK, []offerDTO{})
			return
		}
		routes, err := s.store.IntercityRoutes(ctx, oq.From, oq.To)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		rt, ok := choiceRoute(routes, choice)
		if !ok {
			writeJSON(w, http.StatusOK, []offerDTO{})
			return
		}
		oq.Mode = rt.Mode
	}
	offers, err := s.store.MatchingOffers(ctx, oq)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := []offerDTO{}
	if len(offers) > 0 {
		h := fnv.New32a()
		_, _ = h.Write([]byte(strings.Join([]string{oq.Slot, oq.City, oq.From, oq.To, oq.Date.Format(time.DateOnly)}, "|")))
		o := offers[int(h.Sum32()%uint32(len(offers)))]
		out = append(out, s.offerDTO(o, langOf(r), oq.Days, userOf(r), tripID))
	}
	writeJSON(w, http.StatusOK, out)
}

// offerImpressions — POST /offers/{id}/impressions {count}: anonim, kunlik yig'ma; kim ko'rgani saqlanmaydi.
func (s *Server) offerImpressions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count *int `json:"count"`
	}
	if !decodeOptionalJSON(w, r, &req) {
		return
	}
	n := 1
	if req.Count != nil {
		n = *req.Count
	}
	if n < 1 || n > 50 {
		writeValidation(w, r, fieldErrors{"count": "1..50"})
		return
	}
	err := s.store.AddImpressions(r.Context(), r.PathValue("id"), cityToday(s.now()), n)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "offer_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// expandLink — hamkor havola shabloni. {subId} bo'lmasa sub_id qo'shiladi; utm_source doim safar.
func expandLink(tmpl, subID, offerID string) string {
	link := strings.NewReplacer("{subId}", url.QueryEscape(subID), "{offerId}", url.QueryEscape(offerID)).Replace(tmpl)
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	v := u.Query()
	if !strings.Contains(tmpl, "{subId}") {
		v.Set("sub_id", subID)
	}
	if v.Get("utm_source") == "" {
		v.Set("utm_source", "safar")
	}
	u.RawQuery = v.Encode()
	return u.String()
}

func (s *Server) webLang(r *http.Request) string {
	// Sayt bilan bir xil: Accept-Language da qo'llab-quvvatlanadigan til bo'lmasa o'zbekcha.
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if tag = strings.SplitN(tag, "-", 2)[0]; i18n.Valid(tag) {
			return tag
		}
	}
	return "uz"
}

// offerClick — GET /r/{clickId}: bosishni yozadi va hamkor sahifasiga 302 (sub-id bilan). Namuna hamkor yoki
// havolasi yo'q taklif saytdagi xavfsiz sahifaga yo'naltiriladi. Imzo buzilgan yoki eskirgan havola — sayt bosh sahifasi.
func (s *Server) offerClick(w http.ResponseWriter, r *http.Request) {
	home := s.cfg.WebBaseURL + "/" + s.webLang(r)
	c, ok := s.verifyClick(r.PathValue("clickId"))
	issued := time.Unix(c.Issued, 0)
	if !ok || s.now().Sub(issued) > clickTTL || issued.After(s.now().Add(time.Hour)) {
		http.Redirect(w, r, home, http.StatusFound)
		return
	}
	ctx := r.Context()
	o, err := s.store.Offer(ctx, c.Offer)
	if isNotFound(err) {
		http.Redirect(w, r, home, http.StatusFound)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	click := store.Click{ID: "ck_" + strings.ToLower(store.RandomCode(16)), OfferID: o.ID, PartnerID: o.PartnerID, IssuedAt: issued,
		Sample: o.Sample || o.PartnerSample}
	if c.User != "" {
		click.UserID = &c.User
	}
	if id, err := uuid.Parse(c.Trip); err == nil {
		click.TripID = &id
	}
	if err := s.store.CreateClick(ctx, click); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	target := ""
	if !click.Sample && o.Active && o.LinkTemplate != nil {
		target = expandLink(*o.LinkTemplate, click.ID, o.ID)
	}
	if target == "" {
		// Namuna: hamkor hali ulanmagan. Sayt hamkor sahifasida buni tushuntiradi.
		target = s.cfg.WebBaseURL + "/" + s.webLang(r) + "/partner?sample=1&offer=" + url.QueryEscape(o.ID)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// ---------- hamkor autentifikatsiyasi ----------

const keyPartner ctxKey = 200

func partnerOf(r *http.Request) store.Partner {
	p, _ := r.Context().Value(keyPartner).(store.Partner)
	return p
}

// PartnerKeyPrefix — hamkor API kalitlari shu bilan boshlanadi (foydalanuvchi tokenidan ajratish uchun).
const PartnerKeyPrefix = "spk_"

func hashPartnerKey(k string) string {
	h := sha256.Sum256([]byte(k))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func partnerKeyFrom(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-Partner-Key")); k != "" {
		return k
	}
	if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && strings.HasPrefix(tok, PartnerKeyPrefix) {
		return tok
	}
	return ""
}

func (s *Server) partnerByKey(ctx context.Context, key string) (store.Partner, error) {
	if !strings.HasPrefix(key, PartnerKeyPrefix) || len(key) > 128 {
		return store.Partner{}, store.ErrNotFound
	}
	return s.store.PartnerByKeyHash(ctx, hashPartnerKey(key))
}

// requirePartner — hamkor kaliti (X-Partner-Key yoki Bearer spk_...) yoki partner rolidagi foydalanuvchi tokeni.
// keyOnly: faqat kalit (server-server: sotuv xabari).
func (s *Server) requirePartner(keyOnly bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var p store.Partner
		var err error
		if key := partnerKeyFrom(r); key != "" {
			p, err = s.partnerByKey(ctx, key)
		} else if keyOnly {
			err = store.ErrNotFound
		} else {
			tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			c, verr := s.signer.Verify(tok)
			if !ok || verr != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
				return
			}
			u, uerr := s.store.User(ctx, c.Subject)
			if uerr != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
				return
			}
			if u.Role != "partner" || u.PartnerID == nil {
				writeError(w, r, http.StatusForbidden, "forbidden", false)
				return
			}
			p, err = s.store.Partner(ctx, *u.PartnerID)
			if err == nil && !p.Active {
				err = store.ErrNotFound
			}
			ctx = context.WithValue(ctx, keyUser, u.ID)
		}
		if isNotFound(err) {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, keyPartner, p)))
	})
}

// ratePartner — limit hamkor bo'yicha (kalit almashtirib aylanib o'tib bo'lmaydi).
func (s *Server) ratePartner(bucket string, perMin int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wait, ok := s.limiter.Allow(bucket+":"+partnerOf(r).ID, perMin); !ok {
			w.Header().Set("Retry-After", wait)
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", true)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- sotuv (CPA) va hamkor statistikasi ----------

type conversionReq struct {
	ClickID    string `json:"clickId"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	ExternalID string `json:"externalId"`
}

func (s *Server) partnerConversion(w http.ResponseWriter, r *http.Request) {
	var req conversionReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	req.ClickID, req.ExternalID = strings.TrimSpace(req.ClickID), strings.TrimSpace(req.ExternalID)
	if req.ClickID == "" || len(req.ClickID) > 64 {
		fe.add("clickId", "required (sub_id from the redirect)")
	}
	if req.Amount < 0 || req.Amount > 1_000_000_000_000 {
		fe.add("amount", ">= 0")
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if req.Currency == "" {
		req.Currency = "UZS"
	}
	if len(req.Currency) != 3 || strings.Trim(req.Currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		fe.add("currency", "ISO 4217, e.g. UZS")
	}
	if req.ExternalID == "" || len(req.ExternalID) > 128 {
		fe.add("externalId", "1..128 chars")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	c, dup, err := s.store.AddConversion(r.Context(), partnerOf(r).ID, store.Conversion{ClickID: req.ClickID, Amount: req.Amount,
		Currency: req.Currency, ExternalID: req.ExternalID})
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "click_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	status := http.StatusCreated
	if dup {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"id": c.ID.String(), "clickId": c.ClickID, "amount": c.Amount, "currency": c.Currency,
		"externalId": c.ExternalID, "createdAt": c.CreatedAt.UTC().Format(time.RFC3339), "duplicate": dup})
}

type statsDTO struct {
	Impressions   int64          `json:"impressions"`
	Clicks        int64          `json:"clicks"`
	Conversions   int64          `json:"conversions"`
	Amount        domain.Money   `json:"amount"`  // so'mdagi sotuvlar
	Amounts       []domain.Money `json:"amounts"` // valyuta bo'yicha (so'mdan boshqasi ham bo'lsa)
	CodesIssued   int64          `json:"codesIssued"`
	CodesRedeemed int64          `json:"codesRedeemed"`
}

func statsOf(r store.StatsRow) statsDTO {
	d := statsDTO{Impressions: r.Impressions, Clicks: r.Clicks, Conversions: r.Conversions, Amount: domain.UZS(0),
		Amounts: []domain.Money{}, CodesIssued: r.CodesIssued, CodesRedeemed: r.CodesUsed}
	for _, m := range r.Amounts {
		if m.Currency == "UZS" {
			d.Amount.Amount = m.Amount
		}
		d.Amounts = append(d.Amounts, domain.Money{Amount: m.Amount, Currency: m.Currency})
	}
	return d
}

// dateRange — ?from=&to= (YYYY-MM-DD); standart: oxirgi 30 kun bugun bilan. Eng ko'pi maxDays kun.
func (s *Server) dateRange(r *http.Request, fe fieldErrors, maxDays int) (time.Time, time.Time) {
	q := r.URL.Query()
	to := cityToday(s.now())
	if v := q.Get("to"); v != "" {
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			fe.add("to", "YYYY-MM-DD")
		}
		to = d
	}
	from := to.AddDate(0, 0, -29)
	if v := q.Get("from"); v != "" {
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			fe.add("from", "YYYY-MM-DD")
		}
		from = d
	}
	if from.After(to) {
		fe.add("from", "<= to")
	} else if to.Sub(from) > time.Duration(maxDays-1)*24*time.Hour {
		fe.add("to", "range up to "+strconv.Itoa(maxDays)+" days")
	}
	return from, to
}

func (s *Server) partnerStats(w http.ResponseWriter, r *http.Request) {
	fe := fieldErrors{}
	from, to := s.dateRange(r, fe, 366)
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	p := partnerOf(r)
	st, err := s.store.PartnerStats(r.Context(), p.ID, from, to)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	type monthDTO struct {
		Month string `json:"month"` // YYYY-MM
		statsDTO
	}
	type offerRow struct {
		OfferID string `json:"offerId"`
		statsDTO
	}
	months := make([]monthDTO, 0, len(st.Months))
	for _, m := range st.Months {
		months = append(months, monthDTO{m.Key, statsOf(m)})
	}
	offers := make([]offerRow, 0, len(st.Offers))
	for _, o := range st.Offers {
		offers = append(offers, offerRow{o.Key, statsOf(o)})
	}
	type partnerRef struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Sample bool   `json:"sample"`
	}
	writeJSON(w, http.StatusOK, struct {
		Partner partnerRef `json:"partner"`
		From    string     `json:"from"`
		To      string     `json:"to"`
		statsDTO
		Months []monthDTO `json:"months"`
		Offers []offerRow `json:"offers"`
	}{partnerRef{p.ID, p.Name, p.Sample}, from.Format(time.DateOnly), to.Format(time.DateOnly), statsOf(st.Total), months, offers})
}
