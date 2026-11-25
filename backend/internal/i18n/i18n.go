// Package i18n: til tanlash, fallback zanjiri (so'ralgan → en → uz) va foydalanuvchiga ko'rinadigan matnlar.
package i18n

import (
	"strings"
)

var Supported = []string{"uz", "ru", "en"}

func Valid(lang string) bool {
	for _, l := range Supported {
		if l == lang {
			return true
		}
	}
	return false
}

// FromHeader — Accept-Language dan birinchi qo'llab-quvvatlanadigan til; topilmasa "en".
func FromHeader(h string) string {
	for _, part := range strings.Split(h, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		tag = strings.SplitN(tag, "-", 2)[0]
		if Valid(tag) {
			return tag
		}
	}
	return "en"
}

// Chain — matn qidirish tartibi.
func Chain(lang string) []string {
	out := []string{lang}
	for _, l := range []string{"en", "uz"} {
		if l != lang {
			out = append(out, l)
		}
	}
	return out
}

// Pick — {uz,ru,en} xaritasidan zanjir bo'yicha birinchi bo'sh bo'lmagan matn.
func Pick(m map[string]string, lang string) string {
	for _, l := range Chain(lang) {
		if v := strings.TrimSpace(m[l]); v != "" {
			return v
		}
	}
	return ""
}

// T — kalit bo'yicha matn, {0} o'rniga arg qo'yiladi.
func T(lang, key string, args ...string) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	s := Pick(m, lang)
	for i, a := range args {
		s = strings.ReplaceAll(s, "{"+string(rune('0'+i))+"}", a)
	}
	return s
}

var messages = map[string]map[string]string{
	// xatolar
	"validation_failed":         {"uz": "So'rov ma'lumotlari noto'g'ri", "ru": "Некорректные данные запроса", "en": "Invalid request data"},
	"unauthorized":              {"uz": "Sessiya tugagan, qayta kiring", "ru": "Сессия истекла, войдите снова", "en": "Session expired, please sign in again"},
	"forbidden":                 {"uz": "Bu amal uchun ruxsat yo'q", "ru": "Нет доступа к этому действию", "en": "You don't have access to this action"},
	"not_found":                 {"uz": "Topilmadi", "ru": "Не найдено", "en": "Not found"},
	"place_not_found":           {"uz": "Joy topilmadi", "ru": "Место не найдено", "en": "Place not found"},
	"city_not_found":            {"uz": "Shahar topilmadi", "ru": "Город не найден", "en": "City not found"},
	"trip_not_found":            {"uz": "Marshrut topilmadi", "ru": "Маршрут не найден", "en": "Trip not found"},
	"stop_not_found":            {"uz": "To'xtash joyi topilmadi", "ru": "Остановка не найдена", "en": "Stop not found"},
	"route_impossible":          {"uz": "Bu sozlamalar bilan marshrut tuzib bo'lmadi. Qiziqishlar yoki cheklovlarni o'zgartirib ko'ring", "ru": "Не удалось построить маршрут с этими настройками. Измените интересы или ограничения", "en": "We couldn't build a route with these settings. Try changing interests or constraints"},
	"place_already_in_trip":     {"uz": "Bu joy marshrutda allaqachon bor", "ru": "Это место уже есть в маршруте", "en": "This place is already in the trip"},
	"invalid_credentials":       {"uz": "Email yoki parol noto'g'ri", "ru": "Неверный email или пароль", "en": "Wrong email or password"},
	"place_id_taken":            {"uz": "Bu ID band", "ru": "Этот ID уже занят", "en": "This ID is already taken"},
	"invalid_status_transition": {"uz": "Joyning hozirgi holatida bu amalni bajarib bo'lmaydi", "ru": "Это действие недоступно в текущем статусе места", "en": "This action isn't allowed in the place's current status"},
	"image_unsupported":         {"uz": "Faqat JPEG yoki PNG rasm yuklang", "ru": "Загрузите изображение JPEG или PNG", "en": "Upload a JPEG or PNG image"},
	"image_too_large":           {"uz": "Rasm juda katta (8 MB gacha, 8000 px gacha)", "ru": "Изображение слишком большое (до 8 МБ, до 8000 px)", "en": "Image is too large (up to 8 MB and 8000 px)"},
	"rate_limited":              {"uz": "So'rovlar juda ko'p. Birozdan keyin urinib ko'ring", "ru": "Слишком много запросов. Попробуйте чуть позже", "en": "Too many requests. Try again shortly"},
	"internal":                  {"uz": "Serverda xatolik. Qayta urinib ko'ring", "ru": "Ошибка сервера. Попробуйте ещё раз", "en": "Server error. Please try again"},

	// marshrut
	"trip_title":   {"uz": "{0} · {1} kun", "ru": "{0} · {1} дн.", "en": "{0} · {1} days"},
	"trip_title_1": {"uz": "{0} · 1 kun", "ru": "{0} · 1 день", "en": "{0} · 1 day"},
	"trip_summary": {"uz": "{0} ta joy, shundan {1} tasi yangi yoki kam tanilgan.", "ru": "{0} мест, из них {1} — новые или малоизвестные.", "en": "{0} places, {1} of them new or little-known."},
	"day_title":    {"uz": "{0}-kun", "ru": "День {0}", "en": "Day {0}"},

	"reason.new":        {"uz": "Yaqinda ochilgan — hali kam odam biladi", "ru": "Недавно открылось — о нём пока мало кто знает", "en": "Recently opened — few people know it yet"},
	"reason.hidden":     {"uz": "Kam tanilgan, lekin e'tiborga loyiq joy", "ru": "Малоизвестное, но стоящее внимания место", "en": "Little-known but worth your time"},
	"reason.top_rated":  {"uz": "Shaharning eng mashhur joylaridan biri", "ru": "Одно из самых известных мест города", "en": "One of the city's best-known sights"},
	"reason.meal":       {"uz": "Yo'lingiz ustida tushlik uchun qulay joy", "ru": "Удобное место для обеда по пути", "en": "A convenient lunch stop on your way"},
	"reason.user_added": {"uz": "Siz qo'shdingiz", "ru": "Вы добавили", "en": "Added by you"},
	"reason.weather":    {"uz": "Ob-havoga mos: yopiq joy", "ru": "По погоде: место в помещении", "en": "Weather-friendly: indoor place"},
	"reason.interest":   {"uz": "{0} qiziqishingiz uchun", "ru": "Под ваш интерес: {0}", "en": "Matches your interest: {0}"},

	"interest.history":      {"uz": "Tarixga", "ru": "история", "en": "history"},
	"interest.food":         {"uz": "Milliy taomlarga", "ru": "еда", "en": "food"},
	"interest.nature":       {"uz": "Tabiatga", "ru": "природа", "en": "nature"},
	"interest.craft":        {"uz": "Hunarmandchilikka", "ru": "ремёсла", "en": "crafts"},
	"interest.art":          {"uz": "San'atga", "ru": "искусство", "en": "art"},
	"interest.architecture": {"uz": "Me'morchilikka", "ru": "архитектура", "en": "architecture"},
	"interest.shopping":     {"uz": "Xaridga", "ru": "покупки", "en": "shopping"},
	"interest.nightlife":    {"uz": "Kechki hordiqqa", "ru": "вечерний отдых", "en": "evenings out"},

	// geofence
	"geofence_headline": {"uz": "Siz {0} hududidasiz", "ru": "Вы находитесь: {0}", "en": "You are at {0}"},
}
