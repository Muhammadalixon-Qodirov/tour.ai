package assistant

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
)

// Fallback — API kalitsiz (yoki model ishlamay qolganda) javob: savoldan filtrlarni qoidalar bilan ajratib, bazadan qidiradi.

var cityWords = map[string][]string{
	"tashkent": {"toshkent", "ташкент", "tashkent"}, "samarkand": {"samarqand", "самарканд", "samarkand"},
	"bukhara": {"buxoro", "бухар", "bukhara"}, "khiva": {"xiva", "хив", "khiva"}, "fergana": {"farg'ona", "фергана", "fergana"},
	"namangan": {"namangan", "наманган"}, "kokand": {"qo'qon", "коканд", "kokand"}, "margilan": {"marg'ilon", "маргилан", "margilan"},
	"andijan": {"andijon", "андижан", "andijan"}, "nukus": {"nukus", "нукус"}, "shahrisabz": {"shahrisabz", "шахрисабз"},
	"termez": {"termiz", "термез", "termez"}, "bostanliq": {"chorvoq", "chimyon", "чарвак", "чимган", "charvak", "chimgan"},
}

var (
	familyWords = []string{"bola", "farzand", "oila", "дет", "ребен", "семь", "kid", "child", "family"}
	cheapWords  = []string{"arzon", "tejam", "byudjet", "дешев", "недорог", "бюджет", "cheap", "budget", "inexpensive"}
	openWords   = []string{"hozir ochiq", "ochiq", "открыт", "сейчас", "open now"}
	hiddenWords = []string{"kam tanilgan", "yashirin", "noodatiy", "малоизвест", "необычн", "hidden", "off the beaten", "local"}
	dishWords   = []string{"osh", "palov", "plov", "плов", "lag'mon", "лагман", "somsa", "самса", "shashlik", "шашлык", "manti", "манты",
		"non", "лепешк", "norin", "нарын", "chuchvara", "kabob"}
	tripWords = []string{"reja", "kun", "ertaga", "bugun", "tushlik", "план", "маршрут", "день", "завтра", "сегодня", "обед",
		"plan", "trip", "day", "tomorrow", "today", "lunch"}
	stayWords = []string{"mehmonxona", "hostel", "хостел", "гостиниц", "отел", "hotel", "yotoq", "tunash", "guesthouse", "mehmon uyi"}
)

func hasAny(text string, ws []string) []string {
	var out []string
	for _, w := range ws {
		if strings.Contains(text, " "+strings.ToLower(w)) {
			out = append(out, w)
		}
	}
	return out
}

var fbText = map[string]map[string]string{
	"intro":    {"uz": "Bazada topildi:", "ru": "Найдено в базе:", "en": "Found in the base:"},
	"none":     {"uz": "Bu haqda tekshirilgan ma'lumot yo'q. Shahar yoki joy turini aniqroq yozing (masalan: \"Samarqandda arzon osh\").", "ru": "Проверенной информации об этом нет. Уточните город или тип места (например: «недорогой плов в Самарканде»).", "en": "There is no verified information on this yet. Name a city or a type of place (for example: \"cheap plov in Samarkand\")."},
	"cheap":    {"uz": "arzon", "ru": "недорого", "en": "budget"},
	"family":   {"uz": "bolalar bilan qulay", "ru": "удобно с детьми", "en": "good for kids"},
	"open":     {"uz": "hozir ochiq", "ru": "сейчас открыто", "en": "open now"},
	"hidden":   {"uz": "kam tanilgan", "ru": "малоизвестное", "en": "little-known"},
	"free":     {"uz": "bepul", "ru": "бесплатно", "en": "free"},
	"perNight": {"uz": "so'm/kecha", "ru": "сум/ночь", "en": "UZS/night"},
	"tripDay":  {"uz": "Reja, %d-kun (%s):", "ru": "План, день %d (%s):", "en": "Plan, day %d (%s):"},
	"lunch":    {"uz": "tushlik", "ru": "обед", "en": "lunch"},
	"approx":   {"uz": "taxminan", "ru": "примерно", "en": "about"},
}

func tr(key, lang string) string {
	if v := fbText[key][lang]; v != "" {
		return v
	}
	return fbText[key]["uz"]
}

// Fallback — matnli javob va asboblar orqali topilgan joylar (t.Found).
func Fallback(ctx context.Context, req Request, t *Tools) (string, error) {
	lang := req.Lang
	if lang != "ru" && lang != "en" {
		lang = "uz"
	}
	if req.LangGuessed {
		lang = DetectLang(req.Message, lang)
	}
	text := " " + strings.ToLower(strings.NewReplacer("ʻ", "'", "’", "'", "‘", "'", ",", " ", ".", " ", "?", " ", "!", " ").Replace(req.Message)) + " "
	city := req.CityID
	for id, ws := range cityWords {
		if len(hasAny(text, ws)) > 0 {
			city = id
			break
		}
	}
	// joy turini umumiy klassifikator bilan aniqlaymiz (choyxona → cafe, muzey → museum ...)
	res := discover.Classify(discover.Signals{Names: map[string]string{"uz": req.Message}})
	in := SearchPlacesInput{City: city, Limit: 5, Family: len(hasAny(text, familyWords)) > 0, OpenNow: len(hasAny(text, openWords)) > 0}
	if res.Category != "" {
		in.Categories = []string{res.Category}
	}
	dishes := hasAny(text, dishWords)
	if len(dishes) > 0 {
		in.Words = dishes
		if res.Category == "" || res.Category == "cafe" {
			in.Categories = []string{"restaurant", "cafe"}
		}
	}
	if len(hasAny(text, cheapWords)) > 0 {
		one := 1
		in.MaxPriceLevel = &one
	}
	if len(hasAny(text, hiddenWords)) > 0 {
		in.Novelty = "novel"
	}
	if req.Location != nil {
		in.Near = &domain.LatLng{Lat: req.Location[0], Lng: req.Location[1]}
	}
	var b strings.Builder
	if req.Trip != nil && (len(hasAny(text, tripWords)) > 0 || strings.Contains(text, "-kun")) {
		if ans := tripAnswer(req.Trip, text, lang, t.Now()); ans != "" {
			return ans, nil
		}
	}
	if len(hasAny(text, stayWords)) > 0 && (city != "" || in.Near != nil) {
		var max int64
		if in.MaxPriceLevel != nil {
			max = 400000
		}
		r, err := t.SearchStays(ctx, SearchStaysInput{City: city, Near: in.Near, MaxPriceUZS: max, Limit: 5})
		if err != nil {
			return "", err
		}
		if list, ok := r.(map[string]any)["stays"].([]stayBrief); ok && len(list) > 0 {
			b.WriteString(tr("intro", lang) + "\n" + strings.Join(stayLines(list, lang), "\n"))
			return b.String(), nil
		}
	}
	if city == "" && in.Near == nil && len(in.Categories) == 0 && len(in.Words) == 0 {
		return tr("none", lang), nil
	}
	// topilmasa filtrlarni bosqichma-bosqich yumshatamiz: avval taom so'zlari, keyin "hozir ochiq"
	var places []placeBrief
	for attempt := 0; attempt < 3; attempt++ {
		r, err := t.SearchPlaces(ctx, in)
		if err != nil {
			return "", err
		}
		places = r.(map[string]any)["places"].([]placeBrief)
		if len(places) > 0 {
			break
		}
		switch {
		case len(in.Words) > 0:
			in.Words = nil
		case in.OpenNow:
			in.OpenNow = false
		default:
			attempt = 3
		}
	}
	if len(places) == 0 {
		return tr("none", lang), nil
	}
	b.WriteString(tr("intro", lang))
	for _, p := range places {
		// faqat foydalanuvchi so'ragan mezonlar (+ hozir ochiq) — har joyga bir xil belgilar yopishmasin
		var why []string
		if in.MaxPriceLevel != nil {
			if p.PriceLevel != nil && *p.PriceLevel == 0 {
				why = append(why, tr("free", lang))
			} else {
				why = append(why, tr("cheap", lang))
			}
		}
		if in.Family && p.Family {
			why = append(why, tr("family", lang))
		}
		if p.OpenNow != nil && *p.OpenNow {
			why = append(why, tr("open", lang))
		}
		if in.Novelty != "" && p.Novelty != "iconic" {
			why = append(why, tr("hidden", lang))
		}
		line := "\n• " + p.Name
		if p.Tagline != "" {
			line += ": " + p.Tagline
		}
		if len(why) > 0 {
			line += " (" + strings.Join(why, ", ") + ")"
		}
		b.WriteString(line)
	}
	return b.String(), nil
}

func stayLines(list []stayBrief, lang string) []string {
	var out []string
	for _, v := range list {
		line := "• " + v.Name + " (" + kindName(v.Kind, lang) + ")"
		if v.PriceMin != nil {
			pre := ""
			if v.Estimated {
				pre = tr("approx", lang) + " "
			}
			line += fmt.Sprintf(", %s%s %s", pre, groupDigits(*v.PriceMin), tr("perNight", lang))
		}
		out = append(out, line)
	}
	return out
}

// DetectLang — savol tili (ilova tilni yubormagan bo'lsa): kirill → ru (o'zbek kirill so'zlari bo'lsa uz),
// o'zbekcha belgilar/qo'shimchalar → uz, aks holda def.
func DetectLang(msg, def string) string {
	low := " " + strings.ToLower(msg) + " "
	cyr := 0
	for _, r := range low {
		if r >= 'а' && r <= 'я' || r == 'ё' || r == 'ў' || r == 'қ' || r == 'ғ' || r == 'ҳ' {
			cyr++
		}
	}
	if cyr > 3 {
		if strings.ContainsAny(low, "ўқғҳ") {
			return "uz"
		}
		return "ru"
	}
	for _, w := range []string{"o'", "g'", "oʻ", "gʻ", "da ", "dagi ", "lar", "qayer", "qanday", "bilan", "uchun", "mumkin", "yaxshi", "arzon", "joy"} {
		if strings.Contains(low, w) {
			return "uz"
		}
	}
	for _, w := range []string{" the ", " where ", " what ", " with ", " cheap ", " best ", " near ", " kids "} {
		if strings.Contains(low, w) {
			return "en"
		}
	}
	return def
}

// groupDigits — 450000 → "450 000"
func groupDigits(v int64) string {
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

var stayKindNames = map[string]map[string]string{
	"hotel": {"uz": "mehmonxona", "ru": "отель", "en": "hotel"}, "boutique": {"uz": "butik-mehmonxona", "ru": "бутик-отель", "en": "boutique hotel"},
	"guesthouse": {"uz": "mehmon uyi", "ru": "гостевой дом", "en": "guesthouse"}, "hostel": {"uz": "hostel", "ru": "хостел", "en": "hostel"},
	"motel": {"uz": "motel", "ru": "мотель", "en": "motel"}, "apartment": {"uz": "kvartira", "ru": "апартаменты", "en": "apartment"},
}

func kindName(kind, lang string) string {
	if m, ok := stayKindNames[kind]; ok {
		return i18n.Pick(m, lang)
	}
	return kind
}

// tripAnswer — kalitsiz rejimda marshrut haqidagi savol: tanlangan kun rejasi ("ertaga", "2-kun", "bugun").
func tripAnswer(ti *TripInfo, text, lang string, now time.Time) string {
	if len(ti.Days) == 0 {
		return ""
	}
	today := now.In(tashkent).Format(time.DateOnly)
	tomorrow := now.In(tashkent).AddDate(0, 0, 1).Format(time.DateOnly)
	pick := -1
	for i, d := range ti.Days {
		switch {
		case len(hasAny(text, []string{"ertaga", "завтра", "tomorrow"})) > 0 && d.Date == tomorrow,
			len(hasAny(text, []string{"bugun", "сегодня", "today"})) > 0 && d.Date == today:
			pick = i
		}
	}
	if pick < 0 {
		for i := range ti.Days {
			n := strconv.Itoa(i + 1)
			if strings.Contains(text, " "+n+"-kun") || strings.Contains(text, "день "+n) || strings.Contains(text, " "+n+"-й день") ||
				strings.Contains(text, "day "+n) {
				pick = i
			}
		}
	}
	if pick < 0 {
		pick = 0
		for i, d := range ti.Days {
			if d.Date == today {
				pick = i
			}
		}
	}
	d := ti.Days[pick]
	var b strings.Builder
	b.WriteString(fmt.Sprintf(tr("tripDay", lang), d.Day, d.Date))
	for _, st := range d.Stops {
		name := st.Name
		if st.Kind == "meal" {
			name = tr("lunch", lang) + ": " + name
		}
		b.WriteString("\n• " + st.Arrive + " " + name)
	}
	return b.String()
}
