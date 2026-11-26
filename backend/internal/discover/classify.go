// Package discover — yangi joylarni ochiq manbalardan (Wikidata, OpenStreetMap, foydalanuvchi takliflari,
// tadqiqot nomzodlari) topadi, to'g'ri kategoriyaga ajratadi, bazadagi joylar bilan takrorini tekshiradi va
// ishonch yuqori bo'lsa avtomatik chop etadi, aks holda admin navbatiga qo'yadi.
package discover

import (
	"slices"
	"sort"
	"strings"
	"unicode"

	"yoldosh/internal/domain"
)

// Signals — bitta joy haqida ma'lum bo'lgan hamma narsa (qaysi manbadan bo'lsa ham).
type Signals struct {
	Names     map[string]string // uz/ru/en/boshqa — nom
	OSM       map[string]string // OSM teglari (tourism, amenity, historic, leisure, craft, building, religion ...)
	WDClasses []string          // Wikidata P31 klasslari (Q33506 ...)
	Text      string            // tavsif yoki foydalanuvchi izohi
	Hint      string            // foydalanuvchi/tadqiqotchi aytgan kategoriya (ixtiyoriy)
	Sitelinks int               // Wikipedia maqolalari soni (mashhurlik)
	Heritage  bool              // UNESCO / milliy meros ro'yxatida
	NewSince  bool              // yaqinda ochilgan (inception / start_date oxirgi 3 yil)
}

// Result — kategoriya va ishonch (0..1). Reasons — nega shu qaror qilingani (admin uchun).
type Result struct {
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence"`
	Novelty    string   `json:"novelty"`
	Interests  []string `json:"interests"`
	Reasons    []string `json:"reasons"`
}

// AutoPublish — shu ishonchdan yuqori bo'lsa (va manba ishonchli bo'lsa) admin tekshiruvisiz chop etiladi.
const AutoPublish = 0.8

type vote struct {
	cat    string
	score  float64
	reason string
}

// osmRules — teg=qiymat → kategoriya. "*" — har qanday qiymat.
var osmRules = []struct {
	key, val, cat string
	score         float64
}{
	{"tourism", "museum", "museum", 0.95},
	{"tourism", "gallery", "gallery", 0.9},
	{"tourism", "viewpoint", "viewpoint", 0.95},
	{"tourism", "zoo", "park", 0.85},
	{"tourism", "theme_park", "park", 0.85},
	{"tourism", "artwork", "monument", 0.6},
	{"tourism", "attraction", "monument", 0.5},
	{"amenity", "restaurant", "restaurant", 0.95},
	{"amenity", "cafe", "cafe", 0.95},
	{"amenity", "fast_food", "cafe", 0.7},
	{"amenity", "theatre", "theatre", 0.95},
	{"amenity", "concert_hall", "theatre", 0.85},
	{"amenity", "arts_centre", "gallery", 0.7},
	{"amenity", "marketplace", "bazaar", 0.9},
	{"building", "mosque", "mosque", 0.85},
	{"historic", "mosque", "mosque", 0.85},
	{"historic", "madrasa", "monument", 0.9},
	{"historic", "mausoleum", "monument", 0.9},
	{"historic", "tomb", "monument", 0.85},
	{"historic", "archaeological_site", "monument", 0.85},
	{"historic", "castle", "monument", 0.85},
	{"historic", "fort", "monument", 0.85},
	{"historic", "city_gate", "monument", 0.85},
	{"historic", "citywalls", "monument", 0.8},
	{"historic", "monument", "monument", 0.85},
	{"historic", "memorial", "monument", 0.75},
	{"historic", "ruins", "monument", 0.8},
	{"historic", "*", "monument", 0.65},
	{"leisure", "park", "park", 0.85},
	{"leisure", "garden", "park", 0.8},
	{"leisure", "nature_reserve", "park", 0.6},
	{"craft", "*", "workshop", 0.85},
	{"shop", "craft", "workshop", 0.6},
	{"shop", "art", "gallery", 0.55},
}

// wdRules — Wikidata "instance of" klasslari.
var wdRules = map[string]struct {
	cat   string
	score float64
}{
	"Q33506":   {"museum", 0.95},    // museum
	"Q207694":  {"museum", 0.95},    // art museum
	"Q1007870": {"gallery", 0.9},    // art gallery
	"Q32815":   {"mosque", 0.9},     // mosque
	"Q132834":  {"monument", 0.9},   // madrasa
	"Q162875":  {"monument", 0.9},   // mausoleum
	"Q839954":  {"monument", 0.75},  // archaeological site — ko'pi oddiy tepalik, admin ko'radi
	"Q4989906": {"monument", 0.75},  // monument (haykal, byust)
	"Q23413":   {"monument", 0.85},  // castle
	"Q57821":   {"monument", 0.85},  // fortification
	"Q16970":   {"monument", 0.7},   // church building
	"Q12280":   {"monument", 0.6},   // bridge
	"Q570116":  {"monument", 0.5},   // tourist attraction
	"Q22698":   {"park", 0.9},       // park
	"Q24354":   {"theatre", 0.95},   // theatre (building)
	"Q1060829": {"theatre", 0.8},    // concert hall
	"Q330284":  {"bazaar", 0.9},     // marketplace
	"Q35509":   {"viewpoint", 0.6},  // cave
	"Q11303":   {"viewpoint", 0.5},  // skyscraper
	"Q8502":    {"viewpoint", 0.6},  // mountain
	"Q23397":   {"park", 0.6},       // lake
	"Q46169":   {"ecovillage", 0.5}, // national park — tabiat
}

// WikidataClasses — Wikidata so'rovida ishlatiladigan klasslar ro'yxati.
func WikidataClasses() []string {
	out := make([]string, 0, len(wdRules))
	for q := range wdRules {
		out = append(out, q)
	}
	sort.Strings(out)
	return out
}

// keywords — nomdagi yoki matndagi so'z boshlari (uz lotin/kirill, ru, en). Kichik harf, apostrofsiz.
var keywords = []struct {
	cat   string
	words []string
}{
	{"museum", []string{"muzey", "музей", "museum", "muzeyi"}},
	{"mosque", []string{"masjid", "мечеть", "mosque", "jome", "жоме", "джума"}},
	{"monument", []string{"madrasa", "медресе", "madrasah", "maqbara", "мавзолей", "mausoleum", "minora", "минарет", "minaret",
		"qala", "крепост", "fortress", "citadel", "darvoza", "ворота", "gate", "xonaqoh", "ханака", "maqbarasi", "ark", "мемориал",
		"memorial", "haykal", "памятник", "obida", "majmua", "комплекс", "ziyoratgoh", "qadamjo"}},
	{"bazaar", []string{"bozor", "базар", "bazaar", "bazar", "rasta", "toqi", "тоқи", "market", "рынок", "rynok"}},
	{"cafe", []string{"choyxona", "чайхана", "чойхона", "teahouse", "kafe", "кафе", "cafe", "café", "coffee", "кофе", "qahvaxona", "bakery", "nonvoyxona"}},
	{"restaurant", []string{"restoran", "ресторан", "restaurant", "oshxona", "ошхона", "osh markazi", "плов", "palov", "osh", "kabob", "шашлык", "milliy taomlar"}},
	{"theatre", []string{"teatr", "театр", "theatre", "theater", "filarmoniya", "филармон", "opera", "опер", "konsert", "концерт"}},
	{"gallery", []string{"galereya", "галерея", "gallery", "ko'rgazma", "выставк", "art center", "art-center", "san'at markazi"}},
	{"workshop", []string{"ustaxona", "мастерская", "workshop", "kulolchilik", "керамик", "ceramic", "kulol", "ipak", "silk", "шелк",
		"gilam", "ковр", "carpet", "atelier", "hunarmand", "ремесл", "craft", "suzana", "сюзане", "zardo'zi", "pichoq", "ножи"}},
	{"park", []string{"bog'", "bogi", "park", "парк", "сад", "garden", "xiyobon", "хиёбон", "bulvar", "бульвар", "сквер"}},
	{"viewpoint", []string{"tepalik", "смотров", "viewpoint", "manzara", "обзор", "panorama", "панорам", "tower"}},
	{"ecovillage", []string{"eko qishloq", "ekoqishloq", "eco village", "ecovillage", "eco lodge", "эко дерев", "экодерев", "agroturizm", "агротур", "yurt", "юрт", "o'tov", "glamping", "кемпинг", "camping", "dacha", "qishloq turizm"}},
}

// norm — kichik harf, apostroflar (' ʻ ʼ `) olib tashlanadi, ortiqcha bo'shliq bitta.
func normText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case r == '\'' || r == 'ʻ' || r == 'ʼ' || r == '`' || r == '’' || r == '‘':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteRune(' ')
				space = true
			}
		}
	}
	return " " + strings.TrimSpace(b.String()) + " "
}

// hasWord — matnda so'z (so'z boshidan) bormi. "osh" "oshxona" ichida ham topiladi, lekin "qoshiq" ichida emas.
func hasWord(text, w string) bool {
	w = strings.TrimSpace(normText(w))
	return w != "" && strings.Contains(text, " "+w)
}

// exclude — "supermarket", "avtosalon" kabi turistik bo'lmagan narsalar kalit so'zga tushib qolmasin.
var exclude = []string{"supermarket", "супермаркет", "minimarket", "hypermarket", "гипермаркет", "avto", "авто", "apteka", "аптека",
	"bank", "банк", "maktab", "школ", "school", "kasalxona", "больниц", "hospital", "zavod", "завод", "ofis", "офис"}

// Classify — signallar bo'yicha kategoriya. Kuchli manbalar (OSM teg, Wikidata klass) nomdan ustun,
// bir nechta manba bir kategoriyani aytsa ishonch oshadi, qarama-qarshi bo'lsa kamayadi.
func Classify(s Signals) Result {
	var votes []vote
	for _, r := range osmRules {
		v, ok := s.OSM[r.key]
		if !ok || v == "" || v == "no" || (r.val != "*" && v != r.val) {
			continue
		}
		// diniy bino: faqat musulmon masjidi "mosque"; boshqalar tarixiy obida sifatida
		votes = append(votes, vote{r.cat, r.score, "osm:" + r.key + "=" + v})
		if r.val == "*" || r.key == "historic" {
			break // bitta historic qoidasi yetadi
		}
	}
	if s.OSM["amenity"] == "place_of_worship" {
		if s.OSM["religion"] == "muslim" {
			votes = append(votes, vote{"mosque", 0.9, "osm:place_of_worship+muslim"})
		} else {
			votes = append(votes, vote{"monument", 0.6, "osm:place_of_worship"})
		}
	}
	for _, q := range s.WDClasses {
		if r, ok := wdRules[q]; ok {
			votes = append(votes, vote{r.cat, r.score, "wikidata:" + q})
		}
	}
	if s.Hint != "" && domain.ValidCategory(s.Hint) {
		votes = append(votes, vote{s.Hint, 0.6, "hint:" + s.Hint})
	}
	var nameText string
	for _, l := range []string{"uz", "ru", "en"} {
		nameText += normText(s.Names[l])
	}
	for l, n := range s.Names {
		if l != "uz" && l != "ru" && l != "en" {
			nameText += normText(n)
		}
	}
	excluded := slices.ContainsFunc(exclude, func(w string) bool { return hasWord(nameText, w) })
	if !excluded {
		for _, k := range keywords {
			for _, w := range k.words {
				if hasWord(nameText, w) {
					votes = append(votes, vote{k.cat, 0.7, "name:" + w})
					break
				}
			}
		}
		if s.Text != "" {
			text := normText(s.Text)
			for _, k := range keywords {
				for _, w := range k.words {
					if hasWord(text, w) {
						votes = append(votes, vote{k.cat, 0.5, "text:" + w})
						break
					}
				}
			}
		}
	}

	// Kategoriya bo'yicha jamlash: eng kuchli ovoz + har bir qo'shimcha mos ovoz uchun +0.05 (max 0.99).
	type agg struct {
		best       float64
		n          int
		reasons    []string
		structured bool // OSM teg yoki Wikidata klass (nom/matn emas)
	}
	by := map[string]*agg{}
	for _, v := range votes {
		a := by[v.cat]
		if a == nil {
			a = &agg{}
			by[v.cat] = a
		}
		a.best = max(a.best, v.score)
		a.n++
		a.structured = a.structured || strings.HasPrefix(v.reason, "osm:") || strings.HasPrefix(v.reason, "wikidata:")
		a.reasons = append(a.reasons, v.reason)
	}
	cats := make([]string, 0, len(by))
	score := func(c string) float64 { return min(0.99, by[c].best+0.05*float64(by[c].n-1)) }
	for c := range by {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool {
		if score(cats[i]) != score(cats[j]) {
			return score(cats[i]) > score(cats[j])
		}
		return cats[i] < cats[j]
	})
	res := Result{Novelty: string(domain.NoveltyHidden)}
	if len(cats) == 0 {
		res.Reasons = []string{"signal yo'q"}
		return res
	}
	top := cats[0]
	res.Category = top
	res.Confidence = score(top)
	res.Reasons = by[top].reasons
	if len(cats) > 1 {
		second := cats[1]
		// "madrasa ichidagi muzey" — monument va museum ikkalasi to'g'ri bo'lishi mumkin, jarima kichik
		penalty := 0.3
		// mos toifalar yoki kuchli manba (teg/klass) faqat nomdagi so'zga qarshi — jarima kichik
		if compatible(top, second) || by[top].structured && !by[second].structured {
			penalty = 0.1
		}
		res.Confidence -= penalty * score(second)
		res.Reasons = append(res.Reasons, "raqobat:"+second)
	}
	res.Confidence = float64(int(res.Confidence*100+0.5)) / 100
	res.Interests = interestsFor(top, nameText+normText(s.Text))
	switch {
	case s.Heritage || s.Sitelinks >= 12:
		res.Novelty = string(domain.NoveltyIconic)
	case s.NewSince:
		res.Novelty = string(domain.NoveltyNew)
	}
	return res
}

func compatible(a, b string) bool {
	pair := map[[2]string]bool{{"monument", "museum"}: true, {"monument", "mosque"}: true, {"cafe", "restaurant"}: true,
		{"gallery", "museum"}: true, {"gallery", "workshop"}: true, {"park", "viewpoint"}: true, {"bazaar", "workshop"}: true,
		{"ecovillage", "park"}: true, {"theatre", "gallery"}: true}
	return pair[[2]string{a, b}] || pair[[2]string{b, a}]
}

func interestsFor(cat, text string) []string {
	out := slices.Clone(domain.CategoryInterests[cat])
	add := func(i string) {
		if !slices.Contains(out, i) {
			out = append(out, i)
		}
	}
	for _, w := range []string{"osh", "palov", "плов", "non", "kabob", "taom", "food"} {
		if hasWord(text, w) {
			add("food")
		}
	}
	for _, w := range []string{"hunarmand", "kulol", "ipak", "gilam", "ремесл", "craft", "ceramic"} {
		if hasWord(text, w) {
			add("craft")
		}
	}
	return out
}

// DefaultVisitMin — kategoriya bo'yicha odatiy tashrif davomiyligi (daqiqa).
func DefaultVisitMin(cat string) int {
	switch cat {
	case "restaurant", "museum":
		return 60
	case "viewpoint", "mosque":
		return 30
	case "park", "ecovillage":
		return 90
	case "theatre":
		return 120
	case "bazaar":
		return 60
	}
	return 45
}

// CategoryLabel — kategoriya nomi (tagline yo'q bo'lsa).
var CategoryLabel = map[string]map[string]string{
	"monument":   {"uz": "Tarixiy obida", "ru": "Исторический памятник", "en": "Historic site"},
	"museum":     {"uz": "Muzey", "ru": "Музей", "en": "Museum"},
	"mosque":     {"uz": "Masjid", "ru": "Мечеть", "en": "Mosque"},
	"bazaar":     {"uz": "Bozor", "ru": "Базар", "en": "Bazaar"},
	"workshop":   {"uz": "Hunarmand ustaxonasi", "ru": "Ремесленная мастерская", "en": "Craft workshop"},
	"gallery":    {"uz": "Galereya", "ru": "Галерея", "en": "Gallery"},
	"restaurant": {"uz": "Restoran", "ru": "Ресторан", "en": "Restaurant"},
	"cafe":       {"uz": "Kafe", "ru": "Кафе", "en": "Café"},
	"park":       {"uz": "Bog' va dam olish maskani", "ru": "Парк", "en": "Park"},
	"viewpoint":  {"uz": "Manzara nuqtasi", "ru": "Смотровая площадка", "en": "Viewpoint"},
	"ecovillage": {"uz": "Eko-qishloq", "ru": "Эко-деревня", "en": "Eco-village"},
	"theatre":    {"uz": "Teatr", "ru": "Театр", "en": "Theatre"},
}
