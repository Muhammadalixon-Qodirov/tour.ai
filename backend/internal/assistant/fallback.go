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
	"offTopic": {"uz": "Safar yordamchisi O'zbekiston bo'ylab sayohatga yordam beradi: joylar, ovqat, mehmonxona, narxlar va safar rejasi. Masalan: \"Samarqandda arzon osh qayerda?\"",
		"ru": "Помощник Safar помогает с поездкой по Узбекистану: места, еда, жильё, цены и план поездки. Например: «Где недорогой плов в Самарканде?»",
		"en": "The Safar assistant helps with travel in Uzbekistan: places, food, stays, prices and trip plans. For example: \"Where is cheap plov in Samarkand?\""},
	"hello": {"uz": "Salom! Joylar, ovqat, mehmonxona, narxlar yoki safar rejasi haqida so'rashingiz mumkin.",
		"ru": "Здравствуйте! Можно спросить о местах, еде, жилье, ценах или плане поездки.",
		"en": "Hello! Ask about places, food, stays, prices or your trip plan."},
	"thanks": {"uz": "Arzimaydi! Yana savol bo'lsa, bemalol yozing.", "ru": "Пожалуйста! Если будут ещё вопросы, пишите.",
		"en": "You're welcome! Ask anytime."},
	"whoAreYou": {"uz": "Safar ilovasining sayohat yordamchisi: joylar, ovqat, mehmonxona, narxlar va safar rejasi bo'yicha yordam beradi. Masalan: \"Buxoroda nima ko'rish mumkin?\"",
		"ru": "Помощник по путешествиям в приложении Safar: места, еда, жильё, цены и план поездки. Например: «Что посмотреть в Бухаре?»",
		"en": "The travel assistant in the Safar app: places, food, stays, prices and trip plans. For example: \"What to see in Bukhara?\""},
	"whoAmI": {"uz": "Ismingiz hali ma'lum emas. Profilda ism, qiziqishlar va byudjetni ko'rsatsangiz, tavsiyalar shaxsiyroq bo'ladi.",
		"ru": "Ваше имя пока неизвестно. Если указать в профиле имя, интересы и бюджет, советы будут точнее.",
		"en": "Your name isn't known yet. Add your name, interests and budget in the profile for more personal suggestions."},
	"whoAmIName": {"uz": "Siz %s. Yana nima bilan yordam kerak?", "ru": "Вы %s. Чем ещё помочь?", "en": "You are %s. How else can the assistant help?"},
	"intro":      {"uz": "Bazada topildi:", "ru": "Найдено в базе:", "en": "Found in the base:"},
	"none":       {"uz": "Bu haqda tekshirilgan ma'lumot yo'q. Shahar yoki joy turini aniqroq yozing (masalan: \"Samarqandda arzon osh\").", "ru": "Проверенной информации об этом нет. Уточните город или тип места (например: «недорогой плов в Самарканде»).", "en": "There is no verified information on this yet. Name a city or a type of place (for example: \"cheap plov in Samarkand\")."},
	"cheap":      {"uz": "arzon", "ru": "недорого", "en": "budget"},
	"family":     {"uz": "bolalar bilan qulay", "ru": "удобно с детьми", "en": "good for kids"},
	"open":       {"uz": "hozir ochiq", "ru": "сейчас открыто", "en": "open now"},
	"hidden":     {"uz": "kam tanilgan", "ru": "малоизвестное", "en": "little-known"},
	"free":       {"uz": "bepul", "ru": "бесплатно", "en": "free"},
	"perNight":   {"uz": "so'm/kecha", "ru": "сум/ночь", "en": "UZS/night"},
	"tripDay":    {"uz": "Reja, %d-kun (%s):", "ru": "План, день %d (%s):", "en": "Plan, day %d (%s):"},
	"lunch":      {"uz": "tushlik", "ru": "обед", "en": "lunch"},
	"approx":     {"uz": "taxminan", "ru": "примерно", "en": "about"},
}

func tr(key, lang string) string {
	if v := fbText[key][lang]; v != "" {
		return v
	}
	return fbText[key]["uz"]
}

// Fallback — matnli javob va asboblar orqali topilgan joylar (t.Found).
func Fallback(ctx context.Context, req Request, t *Tools) (string, error) {
	lang := req.ReplyLang
	if lang == "" {
		lang = req.Lang
		if req.LangGuessed {
			lang = DetectLang(req.Message, lang)
		}
	}
	if lang != "ru" && lang != "en" {
		lang = "uz"
	}
	if ans, ok := smallTalk(req, lang); ok {
		return ans, nil
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
		// qisqa xabar joy nomi bo'lishi mumkin ("Registan", "Chor Minor") — nom bo'yicha qidiramiz
		var words []string
		for _, w := range strings.Fields(strings.Trim(text, " ")) {
			if len([]rune(w)) >= 4 {
				words = append(words, w)
			}
		}
		if len(words) > 0 && len(words) <= 3 {
			// avval nomi mos kelgan joylar ("Registon" → Registon), bo'lmasa tavsifida uchraganlar
			r, err := t.SearchPlaces(ctx, SearchPlacesInput{Name: strings.Join(words, " "), Limit: 5})
			if err == nil && r.(map[string]any)["count"].(int) == 0 {
				r, err = t.SearchPlaces(ctx, SearchPlacesInput{Words: words, Limit: 5})
			}
			if err != nil {
				return "", err
			}
			if ps := r.(map[string]any)["places"].([]placeBrief); len(ps) > 0 {
				b.WriteString(tr("intro", lang))
				for _, p := range ps {
					b.WriteString("\n• " + p.Name)
					if p.Tagline != "" {
						b.WriteString(": " + p.Tagline)
					}
				}
				return b.String(), nil
			}
		}
		return tr("offTopic", lang), nil
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

// DetectLang — savol tili: aniqlansa o'sha, aks holda def.
func DetectLang(msg, def string) string {
	if l, ok := DetectLangOK(msg); ok {
		return l
	}
	return def
}

var (
	uzWords = toSet("kim kimman kimsan kimsiz man men san sen siz biz nima nimaga nimalar nega qayer qayerda qayerga qayerdan " +
		"qanday qancha qachon nechta bor bormi yo'q yoq salom assalomu alaykum rahmat raxmat qalay yaxshi yomon kerak kerakmi mumkin " +
		"mumkinmi uchun bilan va ham emas menga senga sizga mening sening sizning osh palov joy joylar shahar ber bering aytchi " +
		"ayting ayt qil qilib bo'ladi boladi bo'lsa bolsa ertaga bugun kecha arzon qimmat mehmonxona ovqat borsa borish ko'rish " +
		"korish tushlik nonushta kechki dam olish sayohat yordam iltimos hop xop ha yo'l yol pul narxi narx")
	enWords = toSet("the a an where what who how is are am i me my you your can could best near cheap kids hotel food please " +
		"thanks thank hello hi to in of for and do does tell show find there this that it")
	uzSuffix = []string{"man", "san", "miz", "siz", "mi", "chi", "dagi", "larni", "ning", "lar", "da", "ga", "dan", "ni"}
	// o'zbekcha yozilishi inglizchadan farq qiladigan shahar o'zaklari (Samarqand ≠ Samarkand) — kuchli belgi
	uzStems = []string{"samarqand", "toshkent", "buxoro", "xiva", "farg'ona", "fargona", "qo'qon", "qoqon", "marg'ilon",
		"andijon", "urganch", "termiz", "navoiy", "qarshi", "jizzax", "shahrisabz", "nukus", "namangan", "chorvoq", "chimyon"}
)

func toSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// DetectLangOK — xabar tilini aniqlaydi; ok=false — aniqlab bo'lmadi (juda qisqa yoki noaniq).
func DetectLangOK(msg string) (string, bool) {
	low := strings.ToLower(strings.NewReplacer("ʻ", "'", "’", "'", "‘", "'", "`", "'").Replace(msg))
	cyr, lat := 0, 0
	for _, r := range low {
		switch {
		case r >= 'а' && r <= 'я' || r == 'ё' || r == 'ў' || r == 'қ' || r == 'ғ' || r == 'ҳ':
			cyr++
		case r >= 'a' && r <= 'z':
			lat++
		}
	}
	if cyr > 0 && cyr >= lat {
		if strings.ContainsAny(low, "ўқғҳ") {
			return "uz", true
		}
		return "ru", true
	}
	if lat == 0 {
		return "", false
	}
	if strings.Contains(low, "o'") || strings.Contains(low, "g'") {
		return "uz", true
	}
	var uz, en float64
	for _, w := range strings.FieldsFunc(low, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r == '\'') }) {
		isStem := false
		for _, st := range uzStems {
			if strings.HasPrefix(w, st) {
				isStem = true
				break
			}
		}
		switch {
		case uzWords[w] || isStem:
			uz++
		case enWords[w]:
			en++
		default:
			for _, suf := range uzSuffix {
				if len(w) > len(suf)+2 && strings.HasSuffix(w, suf) {
					uz += 0.4
					break
				}
			}
		}
	}
	switch {
	case uz > en && uz >= 0.8:
		return "uz", true
	case en > uz && en >= 1:
		return "en", true
	}
	return "", false
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

// SmallTalk — salomlashish, rahmat, "kimsan", "men kimman" ga tayyor, tekshirilgan javob (modelga bormaydi: tez,
// limitni tejaydi va model namuna savolda fakt to'qimaydi).
func SmallTalk(req Request) (string, bool) {
	lang := req.ReplyLang
	if lang != "ru" && lang != "en" {
		lang = "uz"
	}
	return smallTalk(req, lang)
}

func smallTalk(req Request, lang string) (string, bool) {
	low := " " + strings.ToLower(strings.NewReplacer("ʻ", "'", "’", "'", ",", " ", ".", " ", "?", " ", "!", " ").Replace(req.Message)) + " "
	has := func(ws ...string) bool {
		for _, w := range ws {
			if strings.Contains(low, w) {
				return true
			}
		}
		return false
	}
	n := len(strings.Fields(low))
	switch {
	case has("kimman", "who am i", "кто я", "обо мне", "about me", "meni bilasan", "men haqimda"):
		if name := profileValue(req.Profile, "name"); name != "" {
			return fmt.Sprintf(tr("whoAmIName", lang), name), true
		}
		return tr("whoAmI", lang), true
	case has("kimsan", "kimsiz", "who are you", "кто ты", "кто вы", "nima qila olasan", "what can you do", "что ты умеешь", "что умеешь"):
		return tr("whoAreYou", lang), true
	case has("anekdot", "hazil", " joke", "анекдот", "шутк", " she'r", " sher ayt", "стих", " poem", "siyosat", "политик",
		"politic", "uy vazifa", "homework", "домашн", "kod yoz", "write code", "напиши код"):
		return tr("offTopic", lang), true
	case n <= 4 && has(" rahmat", " raxmat", " thank", " thx", "спасибо", " katta rahmat"):
		return tr("thanks", lang), true
	case n <= 3 && has(" salom ", " assalomu", " hello ", " hi ", " hey ", "привет", "здравствуй", " qalay", " как дела"):
		return tr("hello", lang), true
	}
	return "", false
}

func profileValue(profile, key string) string {
	for _, part := range strings.Split(profile, ";") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok && k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
