package i18n

import "strings"

// Xizmatlar (bankomat, dorixona ...) va favqulodda raqamlar uchun nomlar: API (GET /cities/{id}/services, joy yo'riqnomasi)
// va AI yordamchi bir xil matndan foydalanadi.

var serviceLabels = map[string]map[string]string{
	"hospital":    {"uz": "Shifoxona", "ru": "Больница", "en": "Hospital"},
	"clinic":      {"uz": "Klinika", "ru": "Клиника", "en": "Clinic"},
	"pharmacy":    {"uz": "Dorixona", "ru": "Аптека", "en": "Pharmacy"},
	"police":      {"uz": "Politsiya", "ru": "Полиция", "en": "Police"},
	"embassy":     {"uz": "Elchixona", "ru": "Посольство", "en": "Embassy"},
	"atm":         {"uz": "Bankomat", "ru": "Банкомат", "en": "ATM"},
	"exchange":    {"uz": "Valyuta ayirboshlash", "ru": "Обмен валют", "en": "Currency exchange"},
	"fuel":        {"uz": "Yoqilg'i quyish shoxobchasi", "ru": "АЗС", "en": "Fuel station"},
	"ev_charging": {"uz": "Elektromobil quvvatlash", "ru": "Зарядная станция", "en": "EV charging"},
}

// genericServiceNames — OSM da tez-tez uchraydigan, joyni ajratmaydigan nomlar ("Pharmacy", "Аптека", "Bank").
var genericServiceNames = map[string]bool{
	"apteka": true, "аптека": true, "dorixona": true, "pharmacy": true, "drugstore": true, "chemist": true,
	"atm": true, "bankomat": true, "банкомат": true, "cash machine": true, "bank": true, "банк": true,
	"exchange": true, "currency exchange": true, "обмен валют": true, "обменный пункт": true, "valyuta ayirboshlash": true,
	"hospital": true, "больница": true, "shifoxona": true, "kasalxona": true, "clinic": true, "клиника": true, "klinika": true,
	"poliklinika": true, "поликлиника": true, "police": true, "полиция": true, "politsiya": true, "militsiya": true, "милиция": true,
	"embassy": true, "посольство": true, "elchixona": true, "fuel": true, "азс": true, "gas station": true, "zapravka": true,
	"заправка": true, "charging station": true, "зарядная станция": true,
}

// ServiceLabel — xizmat turining nomi ("pharmacy" → "Dorixona").
func ServiceLabel(kind, lang string) string {
	if m, ok := serviceLabels[kind]; ok {
		return Pick(m, lang)
	}
	return kind
}

// IsGenericServiceName — nom bo'sh yoki faqat tur nomi (har qanday tilda).
func IsGenericServiceName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return true
	}
	if genericServiceNames[n] {
		return true
	}
	for _, m := range serviceLabels {
		for _, v := range m {
			if strings.EqualFold(v, n) {
				return true
			}
		}
	}
	return false
}

// ServiceName — ko'rsatiladigan nom: o'z nomi bo'lsa o'sha; umumiy yoki bo'sh bo'lsa "Dorixona · Navoiy ko'chasi, 12"
// (manzil bo'lmasa faqat tur nomi). Shunda o'zbekcha so'rovda "Pharmacy" chiqmaydi.
func ServiceName(kind, name, address, lang string) string {
	if !IsGenericServiceName(name) {
		return strings.TrimSpace(name)
	}
	label := ServiceLabel(kind, lang)
	if a := strings.TrimSpace(address); a != "" {
		return label + " · " + a
	}
	return label
}

var emergencyLabels = map[string]map[string]string{
	"unified":            {"uz": "Yagona favqulodda xizmat", "ru": "Единая служба экстренной помощи", "en": "Emergency (all services)"},
	"ambulance":          {"uz": "Tez yordam", "ru": "Скорая помощь", "en": "Ambulance"},
	"police":             {"uz": "Politsiya", "ru": "Полиция", "en": "Police"},
	"fire":               {"uz": "Yong'in xizmati", "ru": "Пожарная служба", "en": "Fire service"},
	"gas":                {"uz": "Gaz avariya xizmati", "ru": "Аварийная газовая служба", "en": "Gas emergency"},
	"emergency_ministry": {"uz": "Favqulodda vaziyatlar vazirligi", "ru": "МЧС", "en": "Ministry of Emergency Situations"},
	"tourist_hotline":    {"uz": "Turistlar uchun call-markaz (24/7, uz/ru/en)", "ru": "Колл-центр для туристов (24/7, uz/ru/en)", "en": "Tourist hotline (24/7, English)"},
}

// EmergencyKinds — ko'rsatish tartibi.
var EmergencyKinds = []string{"unified", "tourist_hotline", "ambulance", "police", "fire", "gas", "emergency_ministry"}

func EmergencyLabel(kind, lang string) string { return Pick(emergencyLabels[kind], lang) }
