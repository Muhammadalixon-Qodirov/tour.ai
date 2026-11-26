package api

import (
	"context"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// Joy sahifasi pastidagi bo'limlar: "Yaqin turar joylar", "Ovqatlanish", "Yana nima ko'rish mumkin", "Kerakli xizmatlar".
// Bitta so'rov — frontend sahifani pastga surganda ko'rsatadi. Saralash: masofa × sifat × holat (ochiq/yopiq) × byudjet.

// nearbyItem — masofa (distanceM) ichki DTO da; bu yerda piyoda vaqt va sabab.
type nearbyItem struct {
	WalkMin int    `json:"walkMin"`
	Reason  string `json:"reason"` // nega tanlandi: "Eng yaqin", "Arzon variant", "Hozir ochiq"...
}

type nearbyStay struct {
	stayDTO
	nearbyItem
}

type nearbyPlace struct {
	placeSummaryDTO
	nearbyItem
}

type nearbyService struct {
	serviceDTO
	nearbyItem
}

type nearbyResp struct {
	Origin      map[string]any           `json:"origin"`
	Stays       []nearbyStay             `json:"stays"`
	Food        []nearbyPlace            `json:"food"`
	Sights      []nearbyPlace            `json:"sights"`
	Services    map[string]nearbyService `json:"services"`
	Attribution string                   `json:"attribution"`
}

func walkMin(d float64) int { return int(math.Ceil(d * 1.3 / 80)) }

// decay — masofa oshgan sari ball kamayadi (halfM metrda ikki baravar).
func decay(d, halfM float64) float64 { return 1 / (1 + d/halfM) }

var nearbyReason = map[string]map[string]string{
	"closest":  {"uz": "Eng yaqin", "ru": "Ближе всего", "en": "Closest"},
	"budget":   {"uz": "Arzon variant", "ru": "Бюджетный вариант", "en": "Budget option"},
	"comfort":  {"uz": "Qulay mehmonxona", "ru": "Комфортный отель", "en": "Comfortable hotel"},
	"verified": {"uz": "Jamoa tekshirgan", "ru": "Проверено командой", "en": "Checked by our team"},
	"fit":      {"uz": "Byudjetingizga mos", "ru": "Подходит под бюджет", "en": "Fits your budget"},
	"open":     {"uz": "Hozir ochiq", "ru": "Сейчас открыто", "en": "Open now"},
	"novel":    {"uz": "Kam tanilgan joy", "ru": "Малоизвестное место", "en": "Little-known spot"},
	"near":     {"uz": "Yaqin atrofda", "ru": "Поблизости", "en": "Nearby"},
}

func nearbyWhy(key, lang string) string { return i18n.Pick(nearbyReason[key], lang) }

func (s *Server) placeNearby(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lang := langOf(r)
	origin, err := s.store.Place(ctx, r.PathValue("id"), i18n.Chain(lang))
	if isNotFound(err) || err == nil && origin.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	limit := 5
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 15 {
			limit = n
		}
	}
	budget := 0 // kirgan foydalanuvchining byudjeti (1 tejamkor .. 3 qulay)
	if uid := userOf(r); uid != "" {
		if u, err := s.store.User(ctx, uid); err == nil && u.Budget != nil {
			budget = *u.Budget
		}
	}
	city, err := s.store.City(ctx, origin.CityID, []string{"en"})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	loc := tz(city.Timezone)
	now := s.now().In(loc)
	wd, minute := int(now.Weekday()), now.Hour()*60+now.Minute()

	resp := nearbyResp{Origin: map[string]any{"placeId": origin.ID, "name": origin.Name, "location": origin.Loc},
		Stays: []nearbyStay{}, Food: []nearbyPlace{}, Sights: []nearbyPlace{}, Services: map[string]nearbyService{},
		Attribution: "© OpenStreetMap contributors"}

	// ---- Turar joylar: 2 km (kam bo'lsa 5 km), turli narxdagi variantlar ----
	stays, err := s.store.Stays(ctx, store.StayFilter{Near: &origin.Loc, RadiusM: 2000, Limit: 60})
	if err == nil && len(stays) < limit {
		stays, err = s.store.Stays(ctx, store.StayFilter{Near: &origin.Loc, RadiusM: 5000, Limit: 60})
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp.Stays = rankStays(s, stays, budget, limit, lang)

	// ---- Ovqatlanish: 1.5 km, ochiq va tanlangan joylar ustun ----
	food, err := s.nearPlaces(ctx, origin, []string{"restaurant", "cafe"}, 1500, limit, lang)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp.Food = rankPlaces(s, food, origin.ID, limit, lang, func(p store.Place) (float64, string) {
		score, why := decay(*p.DistanceM, 500), "closest"
		switch {
		case len(p.Hours) == 0:
			score *= 0.8 // ish vaqti noma'lum
		case domain.IsOpenAt(p.Hours, wd, minute):
			score *= 1.2
			why = "open"
		default:
			score *= 0.4 // hozir yopiq
		}
		if p.Trust != "public" || p.Novelty != domain.NoveltyIconic { // jamoa yoki tadqiqot tanlagan
			score *= 1.3
		}
		if budget > 0 && p.PriceLevel > budget {
			score *= 0.7
		}
		return score, why
	})

	// ---- Yana nima ko'rish mumkin: 2 km, yangi / kam tanilgan joylar birinchi ----
	sights, err := s.nearPlaces(ctx, origin, []string{"monument", "museum", "mosque", "gallery", "workshop", "park", "viewpoint",
		"bazaar", "theatre", "ecovillage"}, 2000, limit, lang)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	resp.Sights = rankPlaces(s, sights, origin.ID, limit, lang, func(p store.Place) (float64, string) {
		score, why := decay(*p.DistanceM, 700), "near"
		if p.Novelty.IsNovel() && !p.IsPartner {
			score *= 1.5
			why = "novel"
		}
		if p.Trust == "public" && p.Novelty == domain.NoveltyHidden && len(p.Images) == 0 {
			score *= 0.6 // OSM dagi oddiy yodgorlik — jamoa tanlaganlardan keyin
		}
		return score, why
	})

	// ---- Kerakli xizmatlar: eng yaqini ----
	for _, kind := range []string{"atm", "pharmacy", "exchange"} {
		sv, err := s.store.NearestService(ctx, origin.CityID, kind, origin.Loc)
		if isNotFound(err) || err == nil && sv.DistanceM != nil && *sv.DistanceM > 3000 {
			continue
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		d := *sv.DistanceM
		resp.Services[kind] = nearbyService{s.serviceDTO(sv, lang, loc), nearbyItem{WalkMin: walkMin(d), Reason: nearbyWhy("closest", lang)}}
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, resp)
}

// nearPlaces — radius ichida kam bo'lsa (limit dan kam) radiusni 3 baravar kengaytiradi.
func (s *Server) nearPlaces(ctx context.Context, origin store.Place, cats []string, radius float64, limit int, lang string) ([]store.Place, error) {
	f := store.PlaceFilter{City: origin.CityID, Categories: cats, Near: &origin.Loc, RadiusM: radius, Limit: 80}
	list, err := s.store.ListPlaces(ctx, f, i18n.Chain(lang))
	if err == nil && len(list) <= limit { // origin o'zi ham ro'yxatda bo'lishi mumkin
		f.RadiusM = radius * 3
		list, err = s.store.ListPlaces(ctx, f, i18n.Chain(lang))
	}
	return list, err
}

// rankStays — ball bo'yicha, lekin turli narx toifalari aralash: tejamkor (hostel/mehmon uyi) va mehmonxona ikkalasi ham chiqadi.
func rankStays(s *Server, stays []store.Stay, budget, limit int, lang string) []nearbyStay {
	type scored struct {
		v     store.Stay
		score float64
		why   string
	}
	budgetKind := func(v store.Stay) bool { return v.Kind == "hostel" || v.Kind == "guesthouse" || v.Kind == "motel" }
	var list []scored
	for _, v := range stays {
		d := *v.DistanceM
		sc, why := decay(d, 600), "closest"
		if v.Verified || v.FairPrice {
			sc *= 1.4
			why = "verified"
		}
		if v.PriceTrust != nil && *v.PriceTrust != "estimate" {
			sc *= 1.15 // aniq narx bor
		}
		if v.Rating != nil {
			sc *= 0.8 + *v.Rating/25
		}
		switch {
		case budget == 1 && budgetKind(v), budget == 3 && v.Stars != nil && *v.Stars >= 4:
			sc *= 1.3
			why = "fit"
		case budget == 1 && v.Stars != nil && *v.Stars >= 4:
			sc *= 0.6
		}
		list = append(list, scored{v, sc, why})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].score > list[j].score })
	// xilma-xillik: birinchi limit ichida kamida bitta tejamkor va bitta mehmonxona (bo'lsa)
	pick := []scored{}
	has := func(f func(store.Stay) bool) bool {
		return slices.ContainsFunc(pick, func(x scored) bool { return f(x.v) })
	}
	isHotel := func(v store.Stay) bool { return v.Kind == "hotel" || v.Kind == "boutique" }
	for _, x := range list {
		if len(pick) == limit-1 {
			if !has(budgetKind) && !budgetKind(x.v) || !has(isHotel) && !isHotel(x.v) {
				continue
			}
		}
		pick = append(pick, x)
		if len(pick) == limit {
			break
		}
	}
	if len(pick) < limit { // xilma-xillik sharti sabab yetmay qolsa — qolganlari bilan to'ldirish
		for _, x := range list {
			if len(pick) == limit {
				break
			}
			if !slices.ContainsFunc(pick, func(p scored) bool { return p.v.ID == x.v.ID }) {
				pick = append(pick, x)
			}
		}
	}
	out := make([]nearbyStay, 0, len(pick))
	for _, x := range pick {
		why := x.why
		switch {
		case why == "closest" && budgetKind(x.v):
			why = "budget"
		case why == "closest" && isHotel(x.v) && x.v.Stars != nil && *x.v.Stars >= 4:
			why = "comfort"
		case why == "closest":
			why = "near" // "Eng yaqin" faqat haqiqatan eng yaqiniga (pastda)
		}
		d := *x.v.DistanceM
		out = append(out, nearbyStay{s.stayDTO(x.v, lang), nearbyItem{WalkMin: walkMin(d), Reason: nearbyWhy(why, lang)}})
	}
	if len(out) > 0 { // eng yaqini har doim "Eng yaqin" deb belgilanadi
		minI := 0
		for i := range out {
			if *out[i].stayDTO.DistanceM < *out[minI].stayDTO.DistanceM {
				minI = i
			}
		}
		out[minI].Reason = nearbyWhy("closest", lang)
	}
	return out
}

func rankPlaces(s *Server, places []store.Place, exclude string, limit int, lang string, score func(store.Place) (float64, string)) []nearbyPlace {
	type scored struct {
		p   store.Place
		sc  float64
		why string
	}
	var list []scored
	for _, p := range places {
		if p.ID == exclude || p.DistanceM == nil {
			continue
		}
		sc, why := score(p)
		list = append(list, scored{p, sc, why})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].sc > list[j].sc })
	out := []nearbyPlace{}
	for i := 0; i < len(list) && i < limit; i++ {
		d := *list[i].p.DistanceM
		out = append(out, nearbyPlace{s.summaryDTO(list[i].p, nil), nearbyItem{WalkMin: walkMin(d), Reason: nearbyWhy(list[i].why, lang)}})
	}
	return out
}
