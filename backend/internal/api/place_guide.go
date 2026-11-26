package api

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"yoldosh/internal/assistant"
	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// GET /places/{id}/guide?from=lat,lng — joy kartochkasi uchun "hammasi bir joyda": qanday boriladi (piyoda, taksi narxi,
// boshqa shahardan poyezd/avtobus), kirish narxi va ish vaqti, yaqin ovqat (o'rtacha narxlar bilan), turar joy, xizmatlar
// va hammasining sodda tushuntirishi (summary). `from` saqlanmaydi va logga yozilmaydi.

// ---------- navigatsiya havolalari (NAVIGATION_PLAN.md) ----------

type navOption struct {
	Mode     string  `json:"mode"`     // transit | car | walk | taxi
	Provider string  `json:"provider"` // yandex_maps | yandex_go | yandex_navi | 2gis | google_maps
	App      *string `json:"app"`      // ilova sxemasi (o'rnatilgan bo'lsa)
	Web      *string `json:"web"`      // ilova yo'q bo'lsa — brauzer
}

type navigationDTO struct {
	Destination map[string]any `json:"destination"`
	Options     []navOption    `json:"options"`
}

func strp(s string) *string { return &s }

// navigationFor — "qayerdan" bo'sh: Yandex/2GIS ilovasining o'zi turistning joylashuvini oladi (ruxsatsiz ham ishlaydi).
func navigationFor(name string, p domain.LatLng) navigationDTO {
	ll := fmt.Sprintf("%.6f,%.6f", p.Lat, p.Lng)
	lonlat := fmt.Sprintf("%.6f,%.6f", p.Lng, p.Lat)
	ym := func(rtt string) navOption {
		return navOption{Mode: map[string]string{"mt": "transit", "auto": "car", "pd": "walk"}[rtt], Provider: "yandex_maps",
			App: strp("yandexmaps://maps.yandex.ru/?rtext=~" + ll + "&rtt=" + rtt), Web: strp("https://yandex.uz/maps/?rtext=~" + ll + "&rtt=" + rtt)}
	}
	return navigationDTO{
		Destination: map[string]any{"lat": p.Lat, "lng": p.Lng, "name": name},
		Options: []navOption{
			ym("mt"), ym("auto"), ym("pd"),
			{Mode: "taxi", Provider: "yandex_go", Web: strp(fmt.Sprintf("https://3.redirect.appmetrica.yandex.com/route?end-lat=%.6f&end-lon=%.6f&tariffClass=econom&ref=safar&appmetrica_tracking_id=25395763362139037", p.Lat, p.Lng))},
			{Mode: "car", Provider: "yandex_navi", App: strp(fmt.Sprintf("yandexnavi://build_route_on_map?lat_to=%.6f&lon_to=%.6f", p.Lat, p.Lng))},
			{Mode: "transit", Provider: "2gis", App: strp("dgis://2gis.ru/routeSearch/rsType/bus/to/" + lonlat), Web: strp("https://2gis.uz/routeSearch/rsType/bus/to/" + lonlat)},
			{Mode: "transit", Provider: "google_maps", Web: strp("https://www.google.com/maps/dir/?api=1&destination=" + ll + "&travelmode=transit")},
		},
	}
}

// ---------- javob ----------

type moneyRange struct {
	Min domain.Money `json:"min"`
	Max domain.Money `json:"max"`
}

type travelEstimate struct {
	Mode        string      `json:"mode"` // walk | taxi
	DurationMin int         `json:"durationMin"`
	Fare        *moneyRange `json:"fare,omitempty"`
	Note        string      `json:"note,omitempty"`
}

type guideIntercityDTO struct {
	Mode        string        `json:"mode"` // train | bus | shared_taxi | flight ...
	DurationMin int           `json:"durationMin"`
	PriceFrom   *domain.Money `json:"priceFrom,omitempty"`
	Estimate    bool          `json:"estimate"`
	Via         *string       `json:"via,omitempty"`
	UpdatedAt   string        `json:"updatedAt"`
	Source      string        `json:"source"`
}

type getThereDTO struct {
	FromCityID *string             `json:"fromCityId,omitempty"` // boshqa shahardan bo'lsa
	DistanceM  *int                `json:"distanceM,omitempty"`
	Estimates  []travelEstimate    `json:"estimates"`
	Intercity  []guideIntercityDTO `json:"intercity"`
	Navigation navigationDTO       `json:"navigation"`
}

type todayHoursDTO struct {
	Known     bool    `json:"known"`
	Open      *string `json:"open,omitempty"`
	Close     *string `json:"close,omitempty"`
	ClosedDay bool    `json:"closedToday"`
	IsOpenNow *bool   `json:"isOpenNow,omitempty"`
}

type visitDTO struct {
	EntryFee         *entryFeeDTO          `json:"entryFee,omitempty"`
	FeeNote          *string               `json:"feeNote,omitempty"`
	Today            todayHoursDTO         `json:"today"`
	OpeningHours     []domain.OpeningHours `json:"openingHours"`
	HoursNote        *string               `json:"hoursNote,omitempty"`
	VisitDurationMin int                   `json:"visitDurationMin"`
	BestTime         *string               `json:"bestTime,omitempty"`
	Rules            []ruleDTO             `json:"rules"`
	Payment          paymentDTO            `json:"payment"`
}

type placeGuideDTO struct {
	Place       placeSummaryDTO          `json:"place"`
	Summary     []string                 `json:"summary"` // sodda tushuntirish: har qator — bitta fikr
	Visit       visitDTO                 `json:"visit"`
	GetThere    getThereDTO              `json:"getThere"`
	Food        []nearbyPlace            `json:"food"`
	FoodPrices  []priceItemDTO           `json:"foodPrices"` // shahardagi odatiy ovqat narxlari (osh, somsa, choy ...)
	Stays       []nearbyStay             `json:"stays"`
	Services    map[string]nearbyService `json:"services"`
	Attribution string                   `json:"attribution"`
}

// taxiFare — shahar tarifi bo'yicha taxmin: yo'l to'g'ri chiziqdan ~1.35 baravar uzun; oraliq +30%, 1000 ga yaxlitlangan.
func taxiFare(c store.City, straightM float64) (min, max int64, durMin int) {
	km := straightM * 1.35 / 1000
	base, perKm := c.TaxiBase, c.TaxiPerKm
	if base == 0 {
		base = 4600
	}
	if perKm == 0 {
		perKm = 2400
	}
	f := float64(base) + float64(perKm)*km
	round := func(v float64) int64 { return int64(math.Ceil(v/1000)) * 1000 }
	return round(f), round(f * 1.3), int(math.Ceil(km/22*60)) + 3 // shahar ichida ~22 km/soat + kutish
}

func (s *Server) placeGuide(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lang := langOf(r)
	p, err := s.store.Place(ctx, r.PathValue("id"), i18n.Chain(lang))
	if isNotFound(err) || err == nil && p.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	var from *domain.LatLng
	if v := r.URL.Query().Get("from"); v != "" {
		f, ok := parseLatLng(v)
		if !ok {
			writeValidation(w, r, fieldErrors{"from": "lat,lng"})
			return
		}
		from = &f
	}
	rules, err := s.store.Rules(ctx, []string{p.ID})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	detail := s.placeDetailDTO(r, p, rules[p.ID], lang)
	cities, err := s.store.Cities(ctx, i18n.Chain(lang))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	var city store.City
	cityName := map[string]string{}
	for _, c := range cities {
		cityName[c.ID] = c.Name
		if c.ID == p.CityID {
			city = c
		}
	}
	loc := tz(city.Timezone)
	now := s.now().In(loc)

	g := placeGuideDTO{Place: detail.placeSummaryDTO, Attribution: "© OpenStreetMap contributors"}
	// ---- tashrif ----
	g.Visit = visitDTO{EntryFee: detail.EntryFee, FeeNote: detail.FeeNote, OpeningHours: detail.OpeningHours, HoursNote: detail.HoursNote,
		VisitDurationMin: p.VisitMin, BestTime: p.BestTime, Rules: detail.Rules, Payment: detail.Payment}
	if len(g.Visit.Rules) > 3 {
		g.Visit.Rules = g.Visit.Rules[:3]
	}
	if len(p.Hours) > 0 {
		t := todayHoursDTO{Known: true, ClosedDay: true}
		for _, h := range p.Hours {
			if h.Day == int(now.Weekday()) {
				o, c := h.Open, h.Close
				t.Open, t.Close, t.ClosedDay = &o, &c, false
			}
		}
		open := domain.IsOpenAt(p.Hours, int(now.Weekday()), now.Hour()*60+now.Minute())
		t.IsOpenNow = &open
		g.Visit.Today = t
	}
	// ---- borish ----
	g.GetThere = getThereDTO{Estimates: []travelEstimate{}, Intercity: []guideIntercityDTO{}, Navigation: navigationFor(p.Name, p.Loc)}
	if from != nil {
		d := domain.DistanceM(*from, p.Loc)
		di := int(d)
		g.GetThere.DistanceM = &di
		// boshqa shahardan: eng yaqin shahar markazi 40 km dan uzoq bo'lsa
		fromCity := ""
		best := math.MaxFloat64
		for _, c := range cities {
			if cd := domain.DistanceM(*from, c.Center); cd < best {
				best, fromCity = cd, c.ID
			}
		}
		if d > 40000 && fromCity != "" && fromCity != p.CityID {
			g.GetThere.FromCityID = &fromCity
			routes, err := s.store.IntercityRoutes(ctx, fromCity, p.CityID)
			if err != nil {
				s.internalError(w, r, err)
				return
			}
			for _, rt := range routes {
				x := guideIntercityDTO{Mode: rt.Mode, DurationMin: rt.DurationMin, Estimate: rt.Estimate, Via: rt.Via, UpdatedAt: rt.UpdatedAt, Source: rt.Source}
				if rt.PriceFrom != nil {
					m := domain.UZS(*rt.PriceFrom)
					x.PriceFrom = &m
				}
				g.GetThere.Intercity = append(g.GetThere.Intercity, x)
			}
		} else {
			if d <= 3000 {
				g.GetThere.Estimates = append(g.GetThere.Estimates, travelEstimate{Mode: "walk", DurationMin: walkMin(d)})
			}
			if d >= 300 {
				lo, hi, dur := taxiFare(city, d)
				g.GetThere.Estimates = append(g.GetThere.Estimates, travelEstimate{Mode: "taxi", DurationMin: dur,
					Fare: &moneyRange{domain.UZS(lo), domain.UZS(hi)}, Note: i18n.Pick(guideText["taxiNote"], lang)})
			}
		}
	}
	// ---- atrof: ovqat, turar joy, xizmatlar ----
	near, err := s.nearbyData(ctx, p, s.userBudget(r), 3, lang)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	g.Food, g.Stays, g.Services = near.Food, near.Stays, near.Services
	prices, err := s.store.Prices(ctx, store.PriceFilter{City: p.CityID, Category: "food"})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	g.FoodPrices = []priceItemDTO{}
	for i, x := range prices {
		if i == 4 {
			break
		}
		g.FoodPrices = append(g.FoodPrices, priceDTO(x, lang))
	}
	g.Summary = guideSummary(g, cityName, lang)
	for i, line := range g.Summary { // bazadagi erkin matnlar (qoida, izoh) ham "≈", "~", "—" siz
		g.Summary[i] = assistant.Sanitize(line, langOr(lang))
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, g)
}

// ---------- sodda tushuntirish ----------

var guideText = map[string]map[string]string{
	"taxiNote":    {"uz": "taxminiy, Yandex Go tarifi bo'yicha", "ru": "примерно, по тарифу Yandex Go", "en": "approx., Yandex Go fares"},
	"free":        {"uz": "Kirish bepul.", "ru": "Вход бесплатный.", "en": "Entry is free."},
	"fee":         {"uz": "Kirish: %s.", "ru": "Вход: %s.", "en": "Entry: %s."},
	"foreigner":   {"uz": "chet elliklar %s", "ru": "иностранцам %s", "en": "foreigners %s"},
	"local":       {"uz": "O'zbekiston fuqarolari %s", "ru": "гражданам Узбекистана %s", "en": "Uzbek citizens %s"},
	"feeUnknown":  {"uz": "Kirish narxi haqida tekshirilgan ma'lumot yo'q.", "ru": "Нет проверенных данных о цене входа.", "en": "No verified entry price yet."},
	"openNow":     {"uz": "Hozir ochiq, bugun %s gacha.", "ru": "Сейчас открыто, сегодня до %s.", "en": "Open now, today until %s."},
	"opensToday":  {"uz": "Bugun %s–%s ochiq (hozir yopiq).", "ru": "Сегодня открыто %s–%s (сейчас закрыто).", "en": "Open today %s–%s (closed now)."},
	"closedToday": {"uz": "Bugun dam olish kuni.", "ru": "Сегодня выходной.", "en": "Closed today."},
	"hoursUnknown": {"uz": "Ish vaqti haqida tekshirilgan ma'lumot yo'q.", "ru": "Нет проверенных данных о часах работы.",
		"en": "Opening hours not verified yet."},
	"duration": {"uz": "Ko'rish uchun taxminan %d daqiqa kerak.", "ru": "На осмотр нужно примерно %d мин.", "en": "Plan about %d min for the visit."},
	"walk":     {"uz": "Piyoda %d daqiqa", "ru": "Пешком %d мин", "en": "%d min on foot"},
	"taxi":     {"uz": "taksi taxminan %s–%s so'm (%d daq)", "ru": "такси примерно %s–%s сум (%d мин)", "en": "taxi about %s–%s UZS (%d min)"},
	"taxiOnly": {"uz": "Taksi taxminan %s–%s so'm (%d daq).", "ru": "Такси примерно %s–%s сум (%d мин).", "en": "Taxi about %s–%s UZS (%d min)."},
	"noFrom": {"uz": "Borish: \"Qanday boraman?\" tugmasi Yandex xaritada yo'lni ochadi (avtobus, piyoda, taksi).",
		"ru": "Как добраться: кнопка «Как доехать?» откроет маршрут в Яндекс Картах (автобус, пешком, такси).",
		"en": "Getting there: the \"Directions\" button opens the route in Yandex Maps (bus, walk, taxi)."},
	"intercity": {"uz": "%sdan: %s.", "ru": "Из %s: %s.", "en": "From %s: %s."},
	"food":      {"uz": "Yaqinda ovqatlanish: %s (%d daq piyoda).", "ru": "Поесть рядом: %s (%d мин пешком).", "en": "Eat nearby: %s (%d min walk)."},
	"meal":      {"uz": "Odatiy narx: %s, %s so'm.", "ru": "Обычная цена: %s, %s сум.", "en": "Typical price: %s, %s UZS."},
	"stay":      {"uz": "Tunash: %s, %s.", "ru": "Ночлег: %s, %s.", "en": "Stay: %s, %s."},
	"perNight":  {"uz": "%s so'm/kecha", "ru": "%s сум/ночь", "en": "%s UZS/night"},
	"approx":    {"uz": "taxminan", "ru": "примерно", "en": "about"},
	"hm":        {"uz": "%d soat %02d daq", "ru": "%d ч %02d мин", "en": "%dh %02dm"},
	"from":      {"uz": "%s so'mdan", "ru": "от %s сум", "en": "from %s UZS"},
	"rule":      {"uz": "Eslatma: %s", "ru": "Важно: %s", "en": "Note: %s"},
}

var modeName = map[string]map[string]string{
	"train": {"uz": "poyezd", "ru": "поезд", "en": "train"}, "bus": {"uz": "avtobus", "ru": "автобус", "en": "bus"},
	"shared_taxi": {"uz": "shahar-aro taksi", "ru": "междугороднее такси", "en": "shared taxi"},
	"flight":      {"uz": "samolyot", "ru": "самолёт", "en": "flight"}, "plane": {"uz": "samolyot", "ru": "самолёт", "en": "flight"},
	"afrosiyob": {"uz": "Afrosiyob tezyurar poyezdi", "ru": "скоростной поезд Afrosiyob", "en": "Afrosiyob high-speed train"}, "car": {"uz": "mashinada", "ru": "на машине", "en": "by car"},
}

// sum — 45000 → "45 000"
func sum(v int64) string {
	s := fmt.Sprint(v)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func guideSummary(g placeGuideDTO, cityName map[string]string, lang string) []string {
	t := func(k string, a ...any) string { return fmt.Sprintf(i18n.Pick(guideText[k], lang), a...) }
	var out []string
	// kirish narxi
	switch fee := g.Visit.EntryFee; {
	case fee == nil:
		out = append(out, t("feeUnknown"))
	case (fee.Foreigner == nil || fee.Foreigner.Amount == 0) && (fee.Local == nil || fee.Local.Amount == 0):
		out = append(out, t("free"))
	default:
		var parts []string
		if fee.Foreigner != nil {
			parts = append(parts, t("foreigner", sum(fee.Foreigner.Amount)+" "+map[string]string{"uz": "so'm", "ru": "сум", "en": "UZS"}[langOr(lang)]))
		}
		if fee.Local != nil && (fee.Foreigner == nil || fee.Local.Amount != fee.Foreigner.Amount) {
			parts = append(parts, t("local", sum(fee.Local.Amount)+" "+map[string]string{"uz": "so'm", "ru": "сум", "en": "UZS"}[langOr(lang)]))
		}
		out = append(out, t("fee", strings.Join(parts, ", ")))
	}
	// ish vaqti
	switch d := g.Visit.Today; {
	case !d.Known:
		out = append(out, t("hoursUnknown"))
	case d.ClosedDay:
		out = append(out, t("closedToday"))
	case d.IsOpenNow != nil && *d.IsOpenNow:
		out = append(out, t("openNow", *d.Close))
	default:
		out = append(out, t("opensToday", *d.Open, *d.Close))
	}
	if g.Visit.VisitDurationMin > 0 && g.Place.Category != "restaurant" && g.Place.Category != "cafe" {
		out = append(out, t("duration", g.Visit.VisitDurationMin))
	}
	// borish
	switch gt := g.GetThere; {
	case gt.FromCityID != nil && len(gt.Intercity) > 0:
		var opts []string
		for i, x := range gt.Intercity {
			if i == 3 {
				break
			}
			o := i18n.Pick(modeName[x.Mode], lang)
			if o == "" {
				o = x.Mode
			}
			o += " " + t("hm", x.DurationMin/60, x.DurationMin%60)
			if x.PriceFrom != nil {
				o += ", " + t("from", sum(x.PriceFrom.Amount))
			}
			opts = append(opts, o)
		}
		out = append(out, t("intercity", cityName[*gt.FromCityID], strings.Join(opts, "; ")))
	case len(gt.Estimates) > 0:
		var walk, taxi *travelEstimate
		for i := range gt.Estimates {
			switch gt.Estimates[i].Mode {
			case "walk":
				walk = &gt.Estimates[i]
			case "taxi":
				taxi = &gt.Estimates[i]
			}
		}
		switch {
		case walk != nil && taxi != nil:
			out = append(out, t("walk", walk.DurationMin)+", "+t("taxi", sum(taxi.Fare.Min.Amount), sum(taxi.Fare.Max.Amount), taxi.DurationMin)+".")
		case walk != nil:
			out = append(out, t("walk", walk.DurationMin)+".")
		case taxi != nil:
			out = append(out, t("taxiOnly", sum(taxi.Fare.Min.Amount), sum(taxi.Fare.Max.Amount), taxi.DurationMin))
		}
	default:
		out = append(out, t("noFrom"))
	}
	// ovqat
	if len(g.Food) > 0 && g.Place.Category != "restaurant" && g.Place.Category != "cafe" {
		out = append(out, t("food", g.Food[0].Name, g.Food[0].WalkMin))
	}
	if len(g.FoodPrices) > 0 {
		fp := g.FoodPrices[0]
		out = append(out, t("meal", strings.ToLower(fp.Name), sum(fp.FairRange.Min.Amount)+"–"+sum(fp.FairRange.Max.Amount)))
	}
	// turar joy (eng arzon ko'rsatilgani)
	var cheap *nearbyStay
	for i := range g.Stays {
		st := &g.Stays[i]
		if st.PricePerNight.Min != nil && (cheap == nil || st.PricePerNight.Min.Amount < cheap.PricePerNight.Min.Amount) {
			cheap = st
		}
	}
	if cheap != nil {
		price := t("perNight", sum(cheap.PricePerNight.Min.Amount))
		if cheap.PricePerNight.Estimated {
			price = t("approx") + " " + price
		}
		out = append(out, t("stay", cheap.Name, price))
	}
	// eng muhim qoida
	for _, r := range g.Visit.Rules {
		if r.Severity == "forbidden" || r.Severity == "warn" {
			out = append(out, t("rule", r.Text))
			break
		}
	}
	return out
}

func langOr(l string) string {
	if l == "ru" || l == "en" {
		return l
	}
	return "uz"
}
