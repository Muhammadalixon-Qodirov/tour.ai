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
	"price_item_not_found":      {"uz": "Narx bandi topilmadi", "ru": "Позиция цены не найдена", "en": "Price item not found"},
	"place_already_in_trip":     {"uz": "Bu joy marshrutda allaqachon bor", "ru": "Это место уже есть в маршруте", "en": "This place is already in the trip"},
	"invalid_credentials":       {"uz": "Email yoki parol noto'g'ri", "ru": "Неверный email или пароль", "en": "Wrong email or password"},
	"place_id_taken":            {"uz": "Bu ID band", "ru": "Этот ID уже занят", "en": "This ID is already taken"},
	"invalid_status_transition": {"uz": "Joyning hozirgi holatida bu amalni bajarib bo'lmaydi", "ru": "Это действие недоступно в текущем статусе места", "en": "This action isn't allowed in the place's current status"},
	"image_unsupported":         {"uz": "Faqat JPEG yoki PNG rasm yuklang", "ru": "Загрузите изображение JPEG или PNG", "en": "Upload a JPEG or PNG image"},
	"image_too_large":           {"uz": "Rasm juda katta (8 MB gacha, 8000 px gacha)", "ru": "Изображение слишком большое (до 8 МБ, до 8000 px)", "en": "Image is too large (up to 8 MB and 8000 px)"},
	"code_invalid":              {"uz": "Kod noto'g'ri yoki muddati o'tgan", "ru": "Неверный или просроченный код", "en": "The code is wrong or has expired"},
	"code_locked":               {"uz": "Juda ko'p noto'g'ri urinish. Yangi kod so'rang", "ru": "Слишком много попыток. Запросите новый код", "en": "Too many attempts. Request a new code"},
	"email_unavailable":         {"uz": "Hozir email yuborib bo'lmadi. Keyinroq urinib ko'ring yoki Google orqali kiring", "ru": "Не удалось отправить письмо. Попробуйте позже или войдите через Google", "en": "We couldn't send the email. Try again later or sign in with Google"},
	"google_unavailable":        {"uz": "Google orqali kirish hozircha sozlanmagan", "ru": "Вход через Google пока не настроен", "en": "Google sign-in isn't set up yet"},
	"google_token_invalid":      {"uz": "Google orqali kirib bo'lmadi. Qayta urinib ko'ring", "ru": "Не удалось войти через Google. Попробуйте ещё раз", "en": "Google sign-in failed. Please try again"},
	"refresh_invalid":           {"uz": "Sessiya tugagan, qayta kiring", "ru": "Сессия истекла, войдите снова", "en": "Your session has ended, please sign in again"},
	"group_not_found":           {"uz": "Guruh topilmadi", "ru": "Группа не найдена", "en": "Group not found"},
	"invite_invalid":            {"uz": "Taklif kodi noto'g'ri", "ru": "Неверный код приглашения", "en": "Invalid invite code"},
	"stay_not_found":            {"uz": "Turar joy topilmadi", "ru": "Жильё не найдено", "en": "Accommodation not found"},
	"offer_not_found":           {"uz": "Taklif topilmadi", "ru": "Предложение не найдено", "en": "Offer not found"},
	"offer_id_taken":            {"uz": "Bu taklif ID si band", "ru": "Этот ID предложения уже занят", "en": "This offer ID is already taken"},
	"partner_not_found":         {"uz": "Hamkor topilmadi", "ru": "Партнёр не найден", "en": "Partner not found"},
	"partner_id_taken":          {"uz": "Bu hamkor ID si band", "ru": "Этот ID партнёра уже занят", "en": "This partner ID is already taken"},
	"click_not_found":           {"uz": "Bosish topilmadi. clickId sifatida yo'naltirishdagi sub_id ni yuboring", "ru": "Переход не найден. Передайте sub_id из ссылки как clickId", "en": "Click not found. Send the sub_id from the redirect as clickId"},
	"story_not_found":           {"uz": "Story topilmadi", "ru": "История не найдена", "en": "Story not found"},
	"user_not_found":            {"uz": "Foydalanuvchi topilmadi", "ru": "Пользователь не найден", "en": "User not found"},
	"insights_region_forbidden": {"uz": "Bu hudud statistikasi uchun ruxsat yo'q", "ru": "Нет доступа к статистике этого региона", "en": "You don't have access to this region's statistics"},
	"rate_limited":              {"uz": "So'rovlar juda ko'p. Birozdan keyin urinib ko'ring", "ru": "Слишком много запросов. Попробуйте чуть позже", "en": "Too many requests. Try again shortly"},
	"internal":                  {"uz": "Serverda xatolik. Qayta urinib ko'ring", "ru": "Ошибка сервера. Попробуйте ещё раз", "en": "Server error. Please try again"},

	// Lens (POST /lens/identify)
	"lens_unavailable":        {"uz": "Rasm orqali tanish hozir ishlamayapti. Keyinroq urinib ko'ring", "ru": "Распознавание по фото сейчас недоступно. Попробуйте позже", "en": "Photo recognition is unavailable right now. Try again later"},
	"lens_busy":               {"uz": "So'rovlar ko'p. Bir necha soniyadan keyin qayta urinib ko'ring", "ru": "Слишком много запросов. Повторите через несколько секунд", "en": "Too many requests. Try again in a few seconds"},
	"lens_not_recognized":     {"uz": "Rasmdagi narsani tanib bo'lmadi. Yaqinroqdan va yorug'roq joyda suratga oling", "ru": "Не удалось распознать объект на фото. Снимите ближе и при хорошем свете", "en": "Couldn't recognize what's in the photo. Take it closer and in better light"},
	"lens_mode_not_supported": {"uz": "Bu rejim qo'llab-quvvatlanmaydi. auto, landmark yoki price tanlang", "ru": "Этот режим не поддерживается. Выберите auto, landmark или price", "en": "This mode isn't supported. Use auto, landmark or price"},
	"no_verified_info":        {"uz": "Bu joy haqida tekshirilgan ma'lumot yo'q", "ru": "Проверенной информации об этом месте нет", "en": "There is no verified information about this place"},
	"conversation_not_found":  {"uz": "Suhbat topilmadi", "ru": "Разговор не найден", "en": "Conversation not found"},

	// marshrut
	"trip_title":   {"uz": "{0} · {1} kun", "ru": "{0} · {1} дн.", "en": "{0} · {1} days"},
	"trip_title_1": {"uz": "{0} · 1 kun", "ru": "{0} · 1 день", "en": "{0} · 1 day"},
	"trip_summary": {"uz": "{0} ta joy, shundan {1} tasi yangi yoki kam tanilgan.", "ru": "{0} мест, из них {1} новые или малоизвестные.", "en": "{0} places, {1} of them new or little-known."},
	"day_title":    {"uz": "{0}-kun", "ru": "День {0}", "en": "Day {0}"},

	"reason.new":        {"uz": "Yaqinda ochilgan, hali kam odam biladi", "ru": "Недавно открылось, о нём пока мало кто знает", "en": "Recently opened, few people know it yet"},
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

	"reason.free_time": {"uz": "Bo'sh vaqtingiz uchun, yaqin atrofda", "ru": "На свободное время, совсем рядом", "en": "For your free time, close by"},

	// qayta rejalash
	"change.shifted":                {"uz": "Qolgan joylar {0} daqiqaga surildi", "ru": "Остальные места сдвинуты на {0} мин", "en": "Remaining stops moved by {0} min"},
	"change.removed":                {"uz": "{0} olib tashlandi: vaqtga sig'madi", "ru": "{0} убрано: не хватает времени", "en": "{0} removed: not enough time"},
	"change.removed_tired":          {"uz": "{0} olib tashlandi, dam olishingiz uchun", "ru": "{0} убрано, чтобы вы отдохнули", "en": "{0} removed so you can rest"},
	"change.replaced_weather":       {"uz": "{0} o'rniga {1}, ob-havo sababli", "ru": "{1} вместо {0} из-за погоды", "en": "{1} instead of {0} because of the weather"},
	"change.added":                  {"uz": "{0} qo'shildi, yaqin atrofda", "ru": "Добавлено рядом с вами: {0}", "en": "{0} added, close by"},
	"change.none_nearby":            {"uz": "Yaqin atrofda vaqtga sig'adigan joy topilmadi", "ru": "Поблизости нет мест, которые успеете посетить", "en": "Nothing nearby fits into that time"},
	"change.weather_ok":             {"uz": "Ob-havo yaxshi (+{0}°C), o'zgarish kerak emas", "ru": "Погода хорошая (+{0}°C), менять ничего не нужно", "en": "The weather is fine (+{0}°C), no changes needed"},
	"change.weather_hot":            {"uz": "Jazirama (+{0}°C): ochiq joylar yopiq joylarga almashtirildi", "ru": "Жара (+{0}°C): открытые места заменены на крытые", "en": "Heat (+{0}°C): outdoor stops swapped for indoor ones"},
	"change.weather_rain":           {"uz": "Yomg'ir kutilmoqda: ochiq joylar yopiq joylarga almashtirildi", "ru": "Ожидается дождь: открытые места заменены на крытые", "en": "Rain expected: outdoor stops swapped for indoor ones"},
	"change.weather_indoor_already": {"uz": "Ob-havo noqulay, lekin marshrutingizdagi joylar yopiq. O'zgarish kerak emas", "ru": "Погода неважная, но все места в маршруте крытые. Менять не нужно", "en": "The weather is poor, but your stops are indoors. No changes needed"},
	"weather_unavailable":           {"uz": "Ob-havo ma'lumoti hozir mavjud emas", "ru": "Данные о погоде сейчас недоступны", "en": "Weather data is unavailable right now"},
	"day_in_past":                   {"uz": "O'tib ketgan kunni qayta rejalab bo'lmaydi", "ru": "Нельзя перепланировать прошедший день", "en": "A past day can't be replanned"},

	// kundalik va lenta
	"diary_not_found":        {"uz": "Hikoya topilmadi", "ru": "История не найдена", "en": "Story not found"},
	"diary_day_not_started":  {"uz": "Bu kun hali boshlanmagan. Hikoya kun o'tgandan keyin yoziladi", "ru": "Этот день ещё не начался. История пишется после него", "en": "This day hasn't started yet. The story is written afterwards"},
	"diary_nothing_visited":  {"uz": "Hikoya uchun bu kunda borilgan joy yo'q", "ru": "В этот день нет посещённых мест для истории", "en": "No visited places that day to write a story about"},
	"diary_regenerate_limit": {"uz": "Qayta yozish limiti tugadi. Matnni o'zingiz tahrirlashingiz mumkin", "ru": "Лимит перегенерации исчерпан. Можно отредактировать текст вручную", "en": "Regeneration limit reached. You can still edit the text yourself"},
	"too_many_photos":        {"uz": "Bir kunga 30 tagacha rasm yuklash mumkin", "ru": "Можно загрузить до 30 фото за день", "en": "Up to 30 photos per day"},
	"content_rejected":       {"uz": "Postda havolalar bo'lmasligi kerak", "ru": "В публикации не должно быть ссылок", "en": "Posts can't contain links"},
	"post_not_found":         {"uz": "Post topilmadi", "ru": "Публикация не найдена", "en": "Post not found"},
	"post_hidden":            {"uz": "Post shikoyatlar sababli yashirilgan va moderator tekshiruvida", "ru": "Публикация скрыта из-за жалоб и проверяется модератором", "en": "The post is hidden after reports and is under review"},

	"sos.lost_documents": {
		"uz": "Hujjat yo'qolsa: 1) eng yaqin politsiyaga ariza bering va ma'lumotnoma oling; 2) mamlakatingiz elchixonasi yoki konsulligiga murojaat qiling; 3) mehmonxonaga xabar bering, ro'yxatdan o'tish ma'lumotlari ularda bor.",
		"ru": "Если потеряли документы: 1) подайте заявление в ближайшую полицию и получите справку; 2) обратитесь в посольство или консульство своей страны; 3) сообщите в гостиницу, у неё есть данные о вашей регистрации.",
		"en": "If you lose your documents: 1) report it at the nearest police station and get a certificate; 2) contact your country's embassy or consulate; 3) tell your hotel, they hold your registration details.",
	},

	// Safar Insights eksport ustunlari
	"insights.kpi.trips":              {"uz": "Rejalashtirilgan safarlar", "ru": "Запланированные поездки", "en": "Planned trips"},
	"insights.kpi.trips_prev":         {"uz": "O'tgan davrda", "ru": "В прошлом периоде", "en": "Previous period"},
	"insights.kpi.change":             {"uz": "O'zgarish", "ru": "Изменение", "en": "Change"},
	"insights.kpi.budget":             {"uz": "O'rtacha byudjet, kishi boshiga kuniga", "ru": "Средний бюджет на человека в день", "en": "Average budget per person per day"},
	"insights.kpi.days":               {"uz": "O'rtacha muddat, kun", "ru": "Средняя длительность, дней", "en": "Average length, days"},
	"insights.gap.no_stay":            {"uz": "Byudjetga mos turar joy topilmagan rejalar", "ru": "Планы без жилья в рамках бюджета", "en": "Plans with no stay within budget"},
	"insights.transport.train":        {"uz": "Poyezd", "ru": "Поезд", "en": "Train"},
	"insights.transport.afrosiyob":    {"uz": "Afrosiyob", "ru": "Афросиаб", "en": "Afrosiyob"},
	"insights.transport.plane":        {"uz": "Samolyot", "ru": "Самолёт", "en": "Plane"},
	"insights.transport.car":          {"uz": "Mashina", "ru": "Машина", "en": "Car"},
	"insights.transport.bus":          {"uz": "Avtobus", "ru": "Автобус", "en": "Bus"},
	"insights.companion.solo":         {"uz": "Yolg'iz", "ru": "Один", "en": "Solo"},
	"insights.companion.couple":       {"uz": "Juftlik", "ru": "Пара", "en": "Couple"},
	"insights.companion.family":       {"uz": "Oila", "ru": "Семья", "en": "Family"},
	"insights.companion.friends":      {"uz": "Do'stlar", "ru": "Друзья", "en": "Friends"},
	"insights.residency.resident":     {"uz": "Rezident", "ru": "Резидент", "en": "Resident"},
	"insights.residency.foreign":      {"uz": "Chet ellik", "ru": "Иностранец", "en": "Foreign"},
	"insights.interests.history":      {"uz": "Tarix", "ru": "История", "en": "History"},
	"insights.interests.food":         {"uz": "Milliy taomlar", "ru": "Еда", "en": "Food"},
	"insights.interests.nature":       {"uz": "Tabiat", "ru": "Природа", "en": "Nature"},
	"insights.interests.craft":        {"uz": "Hunarmandchilik", "ru": "Ремёсла", "en": "Crafts"},
	"insights.interests.art":          {"uz": "San'at", "ru": "Искусство", "en": "Art"},
	"insights.interests.architecture": {"uz": "Me'morchilik", "ru": "Архитектура", "en": "Architecture"},
	"insights.interests.shopping":     {"uz": "Xarid", "ru": "Покупки", "en": "Shopping"},
	"insights.interests.nightlife":    {"uz": "Kechki hordiq", "ru": "Вечерний отдых", "en": "Evenings out"},
	// geofence
	"geofence_headline": {"uz": "Siz {0} hududidasiz", "ru": "Вы находитесь: {0}", "en": "You are at {0}"},
}
