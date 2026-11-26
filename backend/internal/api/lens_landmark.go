package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"yoldosh/internal/assistant"
	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/landmark"
	"yoldosh/internal/pricing"
	"yoldosh/internal/store"
)

// mode=landmark — "Bu qanday joy?": vision modeli joy nomini taxmin qiladi, taxmin bazadagi joylar bilan moslanadi
// (nomlar uz/ru/en, taxalluslar, kategoriya; lat/lng berilsa 1.5 km ichida, 300 m ichidagisi kuchli ustun).
// Javobdagi hamma ma'lumot BAZADAN: nom, tarix, faktlar, manbalar. Model matni foydalanuvchiga chiqmaydi.
//
// Qaror (hujjatda ham): joy topildi, lekin tekshirilgan tarixi yo'q bo'lsa 422 emas, joyning o'zi qaytadi:
// summary = "Bu joy haqida tekshirilgan tarix yo'q", facts = [], sources = joyning o'z ma'lumot manbasi
// (tadqiqotdagi joylashuv manbasi, OSM yoki Wikidata), historyVerified = false. Bunday manba ham bo'lmasa
// 422 no_verified_info. Hech narsa mos kelmasa 200: placeId = null, confidence < 0.5, candidates = yaqin joylar.

type lensLandmarkDTO struct {
	Kind            string             `json:"kind"` // "landmark"
	Confidence      float64            `json:"confidence"`
	PlaceID         *string            `json:"placeId"` // null: aniq emas, candidates dan tanlanadi
	Title           string             `json:"title"`
	Subtitle        string             `json:"subtitle"`
	Summary         string             `json:"summary"`
	Facts           []string           `json:"facts"`
	Sources         []sourceDTO        `json:"sources"`
	HistoryVerified bool               `json:"historyVerified"`
	AudioURL        *string            `json:"audioUrl,omitempty"`
	FollowUps       []string           `json:"followUps"`
	ConversationID  *string            `json:"conversationId"` // placeId bo'lsa doim bor
	Place           *placeSummaryDTO   `json:"place,omitempty"`
	Candidates      []lensCandidateDTO `json:"candidates"`
}

// lensCandidateDTO — "Aniq emas, qaysi biri?" ro'yxati: to'liq PlaceSummary + kontraktdagi qisqa maydonlar.
type lensCandidateDTO struct {
	placeSummaryDTO
	PlaceID  string `json:"placeId"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

var landmarkText = map[string]map[string]string{
	"noHistory": {"uz": "Bu joy haqida tekshirilgan tarix yo'q.", "ru": "Проверенной истории об этом месте пока нет.",
		"en": "There is no verified history for this place yet."},
	"unsureTitle": {"uz": "Aniq emas, qaysi biri?", "ru": "Не уверены. Какое из этих мест?", "en": "Not sure. Which one is it?"},
	"unsurePick": {"uz": "Rasmdagi joy katalogdagi joy bilan aniq mos kelmadi. Ro'yxatdan to'g'risini tanlang.",
		"ru": "Место на фото не удалось точно сопоставить с каталогом. Выберите подходящее из списка.",
		"en": "The photo did not clearly match a place in the catalog. Pick the right one from the list."},
	"unsureNoLoc": {"uz": "Rasmdagi joy katalogdagi joylar bilan mos kelmadi. Joylashuvni yoqib, qayta urinib ko'ring.",
		"ru": "Место на фото не найдено в каталоге. Включите геолокацию и попробуйте ещё раз.",
		"en": "The photo did not match a place in the catalog. Turn on location and try again."},
	"unsureNone": {"uz": "Rasmdagi joy katalogdagi joylar bilan mos kelmadi, yaqin atrofda ham katalogdagi joy yo'q.",
		"ru": "Место на фото не найдено в каталоге, и рядом нет мест из каталога.",
		"en": "The photo did not match a place in the catalog, and there are no catalog places nearby."},
	"qBuilt":   {"uz": "Kim va qachon qurdirgan?", "ru": "Кто и когда это построил?", "en": "Who built it and when?"},
	"qMuseum":  {"uz": "Ichkarida nimalarni ko'rsa bo'ladi?", "ru": "Что можно посмотреть внутри?", "en": "What can I see inside?"},
	"qDo":      {"uz": "Bu yerda nima qilsa bo'ladi?", "ru": "Чем здесь заняться?", "en": "What is there to do here?"},
	"qFee":     {"uz": "Kirish narxi qancha?", "ru": "Сколько стоит вход?", "en": "How much is the entry fee?"},
	"qOpen":    {"uz": "Hozir ochiqmi?", "ru": "Сейчас открыто?", "en": "Is it open now?"},
	"qNearby":  {"uz": "Yaqin atrofda yana nima bor?", "ru": "Что ещё есть рядом?", "en": "What else is nearby?"},
	"qHistory": {"uz": "Bu joyning tarixi haqida batafsil", "ru": "Подробнее об истории этого места", "en": "Tell me more about its history"},
}

func lt(k, lang string) string { return i18n.Pick(landmarkText[k], lang) }

// ---------- kesh: rasm xeshi (+ yaxlitlangan joylashuv) bo'yicha, 6 soat ----------

type lensMemo struct {
	mu sync.Mutex
	m  map[string]lensMemoEntry
}

type lensMemoEntry struct {
	at time.Time
	v  any
}

const lensMemoTTL = 6 * time.Hour

func newLensMemo() *lensMemo { return &lensMemo{m: map[string]lensMemoEntry{}} }

func (c *lensMemo) get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Since(e.at) > lensMemoTTL {
		return nil, false
	}
	return e.v, true
}

func (c *lensMemo) set(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 2000 { // oddiy chegara (bozor qidiruvi keshi kabi)
		c.m = map[string]lensMemoEntry{}
	}
	c.m[k] = lensMemoEntry{time.Now(), v}
}

func imageHash(img []byte) string {
	h := sha256.Sum256(img)
	return hex.EncodeToString(h[:])
}

// lensVision — vision natijasi keshdan yoki modeldan (faqat muvaffaqiyatli javob keshlanadi).
func (s *Server) lensVision(_ context.Context, key string, fn func() (any, error)) (any, error) {
	if v, ok := s.lensCache.get(key); ok {
		return v, nil
	}
	v, err := fn()
	if err == nil {
		s.lensCache.set(key, v)
	}
	return v, err
}

// ---------- landmark ----------

type lensCandRef struct {
	ID   string
	Dist *float64
}

// landmarkOutcome — moslash natijasi (tilga bog'liq emas), rasm xeshi + joylashuv bo'yicha keshlanadi.
type landmarkOutcome struct {
	PlaceID    string
	Dist       *float64
	Confidence float64
	Candidates []lensCandRef
}

func locKey(loc *domain.LatLng) string {
	if loc == nil {
		return "noloc"
	}
	return fmt.Sprintf("%.3f,%.3f", loc.Lat, loc.Lng) // taxminan 100 m
}

func (s *Server) lensLandmark(ctx context.Context, w http.ResponseWriter, r *http.Request, hash string, v pricing.LandmarkVision,
	loc *domain.LatLng, lang string) {
	if !v.IsPlace && len(v.Guesses) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "lens_not_recognized", false)
		return
	}
	key := "match|" + hash + "|" + locKey(loc)
	var out landmarkOutcome
	if c, ok := s.lensCache.get(key); ok {
		out = c.(landmarkOutcome)
	} else {
		var err error
		if out, err = s.matchLandmark(ctx, v, loc); err != nil {
			s.internalError(w, r, err)
			return
		}
		s.lensCache.set(key, out)
	}
	if out.PlaceID != "" {
		s.lensLandmarkPlace(w, r, out.PlaceID, loc, out.Confidence, lang)
		return
	}
	s.lensLandmarkUnsure(w, r, out, loc, lang)
}

// matchLandmark — bazadan nomzodlar (joylashuv bo'yicha yoki nom bo'yicha) va moslash.
func (s *Server) matchLandmark(ctx context.Context, v pricing.LandmarkVision, loc *domain.LatLng) (landmarkOutcome, error) {
	in := landmark.Input{Type: v.Type, City: v.City, HasLoc: loc != nil}
	for _, g := range v.Guesses {
		in.Guesses = append(in.Guesses, landmark.Guess{Names: g.Names, Type: g.Type, Confidence: g.Confidence})
	}
	rows, err := s.store.LandmarkPlaces(ctx, loc, landmark.RadiusM, landmark.SearchWords(in.Guesses), 300)
	if err != nil {
		return landmarkOutcome{}, err
	}
	places := make([]landmark.Place, 0, len(rows))
	for _, p := range rows {
		places = append(places, landmark.Place{ID: p.ID, CityID: p.CityID, Category: p.Category, Names: p.Names, Text: p.Text,
			DistanceM: p.DistanceM, Bulk: p.Source == "osm" || p.Source == "wikidata"})
	}
	res := landmark.Match(in, places)
	out := landmarkOutcome{Confidence: res.Confidence}
	if res.Best != nil {
		out.PlaceID, out.Dist = res.Best.Place.ID, res.Best.Place.DistanceM
		return out, nil
	}
	// "Aniq emas": avval nomi mos kelganlar, keyin (joylashuv bo'lsa) yaqin joylar: turi mos keladigani va
	// jamoa tekshirgani birinchi
	seen := map[string]bool{}
	add := func(id string, d *float64) {
		if !seen[id] && len(out.Candidates) < 5 {
			seen[id] = true
			out.Candidates = append(out.Candidates, lensCandRef{id, d})
		}
	}
	for _, sc := range res.Ranked {
		add(sc.Place.ID, sc.Place.DistanceM)
	}
	if loc != nil {
		near := slices.Clone(places) // masofa bo'yicha tartiblangan
		slices.SortStableFunc(near, func(a, b landmark.Place) int {
			ra, rb := nearRank(a, v.Type), nearRank(b, v.Type)
			if ra != rb {
				return ra - rb
			}
			return 0
		})
		for _, p := range near {
			add(p.ID, p.DistanceM)
		}
	}
	return out, nil
}

// nearRank — yaqin joylar ichida tartib: 300 m ichida turi mos (0), 300 m ichida (1), turi mos (2), jamoa tekshirgan (3), qolgani (4).
func nearRank(p landmark.Place, typ string) int {
	near := p.DistanceM != nil && *p.DistanceM <= landmark.NearM
	fit := landmark.CategoryFits(typ, p.Category)
	switch {
	case near && fit:
		return 0
	case near && !p.Bulk:
		return 1
	case fit:
		return 2
	case !p.Bulk:
		return 3
	}
	return 4
}

func (s *Server) lensLandmarkUnsure(w http.ResponseWriter, r *http.Request, out landmarkOutcome, loc *domain.LatLng, lang string) {
	res := lensLandmarkDTO{Kind: "landmark", Confidence: min(out.Confidence, 0.49), Title: lt("unsureTitle", lang),
		Facts: []string{}, Sources: []sourceDTO{}, FollowUps: []string{}, Candidates: []lensCandidateDTO{}}
	if len(out.Candidates) > 0 {
		ids := make([]string, 0, len(out.Candidates))
		for _, c := range out.Candidates {
			ids = append(ids, c.ID)
		}
		ps, err := s.store.PlacesByIDs(r.Context(), ids, i18n.Chain(lang))
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		for _, c := range out.Candidates {
			p, ok := ps[c.ID]
			if !ok {
				continue
			}
			p.DistanceM = c.Dist
			sub := i18n.Pick(discover.CategoryLabel[p.Category], lang)
			if t := strings.TrimSpace(p.Tagline); t != "" {
				sub = assistant.Sanitize(t, lang)
			}
			res.Candidates = append(res.Candidates, lensCandidateDTO{placeSummaryDTO: s.summaryDTO(p, tashkentLoc()), PlaceID: p.ID,
				Title: p.Name, Subtitle: sub})
		}
	}
	switch {
	case len(res.Candidates) > 0:
		res.Summary = lt("unsurePick", lang)
	case loc == nil:
		res.Summary = lt("unsureNoLoc", lang)
	default:
		res.Summary = lt("unsureNone", lang)
	}
	writeJSON(w, http.StatusOK, res)
}

// lensLandmarkPlace — topilgan (yoki ro'yxatdan tanlangan) joy uchun natija faqat bazadan + joyga bog'langan suhbat.
func (s *Server) lensLandmarkPlace(w http.ResponseWriter, r *http.Request, placeID string, loc *domain.LatLng, conf float64, lang string) {
	ctx := r.Context()
	p, err := s.store.Place(ctx, placeID, i18n.Chain(lang))
	if isNotFound(err) || err == nil && p.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if loc != nil {
		d := domain.DistanceM(*loc, p.Loc)
		p.DistanceM = &d
	}
	res := lensLandmarkDTO{Kind: "landmark", Confidence: conf, PlaceID: &p.ID, Title: p.Name, Facts: []string{},
		Sources: []sourceDTO{}, Candidates: []lensCandidateDTO{}}
	res.Subtitle = i18n.Pick(discover.CategoryLabel[p.Category], lang)
	if t := strings.TrimSpace(p.Tagline); t != "" && !strings.EqualFold(t, res.Subtitle) {
		res.Subtitle += " · " + t
	}
	if p.HistorySummary != nil && strings.TrimSpace(*p.HistorySummary) != "" {
		res.Sources = parseSources(p.Sources)
	}
	if len(res.Sources) > 0 {
		res.HistoryVerified = true
		res.Summary = firstSentences(*p.HistorySummary, 3)
		for _, f := range p.HistoryFacts {
			if f = strings.TrimSpace(f); f != "" {
				res.Facts = append(res.Facts, assistant.Sanitize(f, lang))
			}
		}
	} else {
		// tekshirilgan tarix yo'q: joyning o'zi va uning ma'lumot manbasi (tarix to'qilmaydi)
		src, err := s.store.PlaceDataSource(ctx, p.ID)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if src == "" {
			writeError(w, r, http.StatusUnprocessableEntity, "no_verified_info", false)
			return
		}
		res.Summary = lt("noHistory", lang)
		res.Sources = []sourceDTO{*sourceOf(&src, nil)}
	}
	res.Summary = assistant.Sanitize(res.Summary, lang)
	res.Subtitle = assistant.Sanitize(res.Subtitle, lang)
	res.FollowUps = landmarkFollowUps(p, res.HistoryVerified, lang)
	sum := s.summaryDTO(p, tashkentLoc())
	res.Place = &sum

	conv, err := s.startLensConversation(context.WithoutCancel(ctx), userOf(r), p, res)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	cs := conv.String()
	res.ConversationID = &cs
	writeJSON(w, http.StatusOK, res)
}

// landmarkFollowUps — 3 ta qisqa savol (so'rov tilida); yo'riqchi chatda javob bera oladiganlari.
func landmarkFollowUps(p store.Place, verified bool, lang string) []string {
	var out []string
	if verified {
		switch p.Category {
		case "monument", "mosque":
			out = append(out, lt("qBuilt", lang))
		default:
			out = append(out, lt("qHistory", lang))
		}
	} else if p.Category == "museum" || p.Category == "gallery" {
		out = append(out, lt("qMuseum", lang))
	} else {
		out = append(out, lt("qDo", lang))
	}
	if len(p.EntryFee) > 0 && string(p.EntryFee) != "null" && string(p.EntryFee) != "{}" {
		out = append(out, lt("qFee", lang))
	} else {
		out = append(out, lt("qOpen", lang))
	}
	return append(out, lt("qNearby", lang))
}

// startLensConversation — yo'riqchi suhbati shu joyga bog'lanib ochiladi; birinchi xabar (yordamchi) — Lens natijasi,
// shunda chatdagi keyingi savollar ("Kim qurdirgan?") joy kontekstida davom etadi.
func (s *Server) startLensConversation(ctx context.Context, uid string, p store.Place, res lensLandmarkDTO) (uuid.UUID, error) {
	conv := uuid.Must(uuid.NewV7())
	text := res.Title + ". " + res.Summary
	if len(res.Facts) > 0 {
		text += "\n" + strings.Join(res.Facts, "\n")
	}
	last := []rune(text)
	if len(last) > 160 {
		last = append(last[:157], '…')
	}
	title := []rune(p.Name)
	if len(title) > 80 {
		title = append(title[:77], '…')
	}
	err := pgx.BeginFunc(ctx, s.store.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO guide_conversations (id, user_id, title, city_id, place_id, message_count, last_text)
			VALUES ($1,$2,$3,$4,$5,1,$6)`, conv, uid, string(title), p.CityID, p.ID, string(last)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO guide_messages (id, conversation_id, user_id, role, text, mode, place_ids)
			VALUES ($1,$2,$3,'assistant',$4,'lens',$5)`, uuid.Must(uuid.NewV7()), conv, uid, text, []string{p.ID})
		return err
	})
	return conv, err
}

// parseSources — place_i18n.sources ([{title, url}]) dan bo'sh bo'lmagan manbalar.
func parseSources(raw json.RawMessage) []sourceDTO {
	var in []sourceDTO
	_ = json.Unmarshal(raw, &in)
	out := []sourceDTO{}
	for _, x := range in {
		x.Title, x.URL = strings.TrimSpace(x.Title), strings.TrimSpace(x.URL)
		if x.Title == "" && x.URL != "" {
			if s := sourceOf(&x.URL, nil); s != nil {
				x.Title = s.Title
			}
		}
		if x.Title != "" {
			out = append(out, x)
		}
	}
	return out
}

// firstSentences — matnning dastlabki n ta gapi (qisqa xulosa uchun).
func firstSentences(s string, n int) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	count := 0
	for i, r := range rs {
		if (r == '.' || r == '!' || r == '?') && (i == len(rs)-1 || rs[i+1] == ' ' || rs[i+1] == '\n') {
			count++
			if count == n {
				return string(rs[:i+1])
			}
		}
	}
	return s
}
