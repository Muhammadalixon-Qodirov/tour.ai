// Package landmark — Lens "bu qanday joy?": vision modeli bergan nom taxminlarini bazadagi joylar bilan moslash.
// Model faqat nom va tur taxmin qiladi; natijadagi har bir ma'lumot (nom, tarix, manba) bazadagi joydan olinadi.
// Joylashuv berilsa faqat 1.5 km ichidagi joylar ko'riladi va 300 m ichidagilar kuchli ustun turadi.
package landmark

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	RadiusM = 1500 // joylashuv berilsa shu radiusdan tashqaridagi joylar ko'rilmaydi
	NearM   = 300  // shu masofa ichidagi joy kuchli ustun
	// MinConfidence — shundan past bo'lsa natija "Aniq emas, qaysi biri?" (placeId: null, candidates).
	MinConfidence = 0.5
	nameMatch     = 0.72 // nom o'xshashligi shundan past bo'lsa mos kelmagan hisoblanadi
)

// Guess — modelning bitta taxmini: nomlari (turli yozilishda), turi va ishonchi.
type Guess struct {
	Names      []string
	Type       string
	Confidence float64
}

// Input — modelning xulosasi.
type Input struct {
	Guesses []Guess
	Type    string // umumiy joy turi
	City    string // model taxmin qilgan shahar (bo'sh bo'lishi mumkin)
	HasLoc  bool   // so'rovda lat/lng bor
}

// Place — bazadagi nomzod joy.
type Place struct {
	ID        string
	CityID    string
	Category  string
	Names     []string // barcha tillardagi nomlar + taxalluslar
	Text      string   // tavsif va tarix (barcha tillar): "Registon" rasmida "Ulug'bek madrasasi" taxmini uchun
	DistanceM *float64 // HasLoc bo'lsa
	Bulk      bool     // OSM/Wikidata ommaviy importi (jamoa tekshirmagan)
}

// Scored — baholangan joy.
type Scored struct {
	Place      Place
	NameSim    float64 // 0..1, eng yaxshi nom o'xshashligi
	Mention    bool    // nom joy nomida emas, tavsif yoki tarixida uchradi
	TypeFit    bool
	Confidence float64
	score      float64
}

// Result — moslash natijasi.
type Result struct {
	Best       *Scored  // Confidence >= MinConfidence bo'lsa
	Confidence float64  // eng yaxshi nomzod ishonchi (mos kelmasa 0..0.5)
	Ranked     []Scored // nom bo'yicha mos kelganlar, kuchlisi birinchi
}

// distFactor — 300 m gacha 1, 1500 m da 0.5.
func distFactor(d float64) float64 {
	if d <= NearM {
		return 1
	}
	if d >= RadiusM {
		return 0.5
	}
	return 1 - 0.5*(d-NearM)/(RadiusM-NearM)
}

// Match — taxminlarni joylar bilan moslaydi va eng ishonchli joyni tanlaydi.
func Match(in Input, places []Place) Result {
	var ranked []Scored
	for _, p := range places {
		if in.HasLoc && (p.DistanceM == nil || *p.DistanceM > RadiusM) {
			continue
		}
		best := Scored{Place: p, TypeFit: CategoryFits(in.Type, p.Category)}
		for _, g := range in.Guesses {
			sim, mention := 0.0, false
			for _, n := range g.Names {
				for _, pn := range p.Names {
					sim = math.Max(sim, Similarity(n, pn))
				}
			}
			// nom joy nomida yo'q, lekin yaqin (500 m) joyning tavsifi yoki tarixida bor: majmua qismi
			if sim < nameMatch && in.HasLoc && *p.DistanceM <= 500 && p.Text != "" {
				for _, n := range g.Names {
					if Mentions(p.Text, n) {
						sim, mention = 0.75, true
						break
					}
				}
			}
			if sim < nameMatch {
				continue
			}
			typeFit := CategoryFits(g.Type, p.Category) || best.TypeFit
			conf := sim * (0.4 + 0.6*g.Confidence)
			score := conf
			if in.HasLoc {
				f := distFactor(*p.DistanceM)
				conf *= f
				score *= f
				if *p.DistanceM <= NearM {
					conf += 0.05
					score += 0.15 // 300 m ichidagi joy kuchli ustun
				}
			} else {
				conf *= 0.85 // joylashuvsiz: bir xil nomli joylar boshqa shaharda ham bo'lishi mumkin
				if in.City != "" && SameCity(in.City, p.CityID) {
					conf += 0.05
					score += 0.1
				}
			}
			if typeFit {
				score += 0.05
			} else if g.Type != "" && in.Type != "" {
				conf *= 0.9
			}
			if p.Bulk {
				score -= 0.02
			}
			if score > best.score {
				best.NameSim, best.Mention, best.TypeFit, best.Confidence, best.score = sim, mention, typeFit, math.Min(conf, 0.99), score
			}
		}
		if best.NameSim >= nameMatch {
			ranked = append(ranked, best)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].Place.ID < ranked[j].Place.ID
	})
	res := Result{Ranked: ranked}
	if len(ranked) == 0 {
		return res
	}
	top := ranked[0]
	// ikki joy deyarli teng (masalan, ikki shaharda bir xil nomli yodgorlik): aniq emas
	if len(ranked) > 1 && ranked[0].score-ranked[1].score < 0.05 {
		top.Confidence *= 0.6
	}
	res.Confidence = math.Round(top.Confidence*100) / 100
	if res.Confidence >= MinConfidence {
		res.Best = &top
	}
	return res
}

// typeCats — model joy turi qaysi kategoriyalarga mos keladi.
var typeCats = map[string][]string{
	"madrasa":   {"monument", "mosque", "museum", "workshop"},
	"mosque":    {"mosque", "monument"},
	"mausoleum": {"monument", "mosque"},
	"minaret":   {"monument", "mosque"},
	"square":    {"monument", "park", "viewpoint"},
	"monument":  {"monument", "park"},
	"statue":    {"monument", "park"},
	"museum":    {"museum", "monument", "gallery"},
	"bazaar":    {"bazaar"},
	"park":      {"park", "viewpoint", "ecovillage"},
	"theatre":   {"theatre"},
	"fortress":  {"monument", "museum", "viewpoint"},
	"palace":    {"monument", "museum"},
	"gate":      {"monument"},
	"bridge":    {"monument", "viewpoint"},
}

// CategoryFits — joy turi kategoriyaga mosmi.
func CategoryFits(typ, category string) bool {
	for _, c := range typeCats[strings.ToLower(typ)] {
		if c == category {
			return true
		}
	}
	return false
}

var cityKeys = map[string][]string{
	"samarkand": {"samar"}, "bukhara": {"bukh", "bux"}, "tashkent": {"tash", "tosh"}, "namangan": {"naman"},
	"khiva": {"khiv", "xiv"}, "fergana": {"ferg", "farg"},
}

// SameCity — model aytgan shahar nomi shahar id siga mosmi ("Samarqand" = samarkand).
func SameCity(name, cityID string) bool {
	n := Normalize(name)
	for _, k := range cityKeys[cityID] {
		if strings.HasPrefix(n, Normalize(k)) {
			return true
		}
	}
	return n != "" && strings.HasPrefix(n, Normalize(cityID))
}

// generic — joy turini bildiruvchi umumiy so'zlar (nom solishtirishda e'tiborga olinmaydi). Normalize dan keyingi shaklda.
var generic = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`madrasa madrasah madrasasi medrese medresse madrassa mosque masjid masjidi mechet
		mausoleum maqbara maqbarasi mavzoley mavzolei mausoleum square maydon maydoni ploshad complex kompleks majmua majmuasi
		ansambl ansambli ensemble minaret minora minorasi monument haykal haykali pamyatnik memorial yodgorlik yodgorligi
		museum muzey muzeyi muzeysi muzeji park bogi bog istirohat bazaar bozor bozori bazar rynok market theatre theater teatr teatri
		the of and va i im imeni nomidagi palace saroy saroyi dvorets fortress qala qalasi krepost gate darvoza darvozasi vorota
		khanaka xonaqoh xonaqohi mosque jome juma juma statue`) {
		generic[Normalize(w)] = true
	}
}

var cyr = map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "j", 'з': "z", 'и': "i",
	'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "x", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sh", 'ъ': "", 'ы': "i", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'ў': "o", 'қ': "q", 'ғ': "g", 'ҳ': "h"}

var folds = strings.NewReplacer("dzh", "j", "zh", "j", "kh", "x", "gh", "g", "q", "k", "ya", "a", "yo", "o", "yu", "u",
	"ye", "e", "w", "v", "ou", "u")

// Normalize — kichik harf, kirill lotinga, apostrof olib tashlanadi, yaqin tovushlar birlashtiriladi
// (kh = x, q = k, ya = a), takror harflar bittaga. Natija — bo'shliq bilan ajratilgan so'zlar.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case cyr[r] != "" || r == 'ъ' || r == 'ь':
			b.WriteString(cyr[r])
		case r == '\'' || r == 'ʻ' || r == '‘' || r == '’' || r == '`' || r == 'ʼ':
		case r == 'ö' || r == 'ő':
			b.WriteByte('o')
		case r == 'ü':
			b.WriteByte('u')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	words := strings.Fields(folds.Replace(b.String()))
	for i, w := range words {
		rs := []rune(w)
		out := rs[:0]
		for j, r := range rs {
			if j > 0 && r == out[len(out)-1] {
				continue
			}
			// "y" undoshdan oldin yoki oxirda = "i" (Khanym = Xonim)
			if r == 'y' && j > 0 && (j == len(rs)-1 || !isVowel(rs[j+1])) {
				r = 'i'
			}
			out = append(out, r)
		}
		words[i] = string(out)
	}
	return strings.Join(words, " ")
}

func isVowel(r rune) bool { return strings.ContainsRune("aeiou", r) }

// core — umumiy so'zlarsiz so'zlar (hammasi umumiy bo'lsa — barchasi).
func core(s string) []string {
	all := strings.Fields(Normalize(s))
	var out []string
	for _, w := range all {
		if !generic[w] {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

// Similarity — ikki nom o'xshashligi 0..1: qo'shilgan asosiy so'zlar (Levenshtein) va so'zlar ustma-ustligi (Dice).
func Similarity(a, b string) float64 {
	ta, tb := core(a), core(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	ja, jb := strings.Join(ta, ""), strings.Join(tb, "")
	if len([]rune(ja)) < 4 || len([]rune(jb)) < 4 { // juda qisqa: faqat aynan
		if ja == jb {
			return 1
		}
		return 0
	}
	s1 := ratio(ja, jb)
	used := make([]bool, len(tb))
	m := 0
	for _, x := range ta {
		for j, y := range tb {
			if !used[j] && tokenEq(x, y) {
				used[j] = true
				m++
				break
			}
		}
	}
	s2 := 2 * float64(m) / float64(len(ta)+len(tb))
	return math.Max(s1, s2)
}

func tokenEq(x, y string) bool {
	if x == y {
		return true
	}
	if len([]rune(x)) < 4 || len([]rune(y)) < 4 {
		return false
	}
	return ratio(x, y) >= 0.8
}

// Mentions — nomning barcha asosiy so'zlari matnda bormi (har biri o'xshash so'z sifatida).
func Mentions(text, name string) bool {
	ws := core(name)
	if len(ws) == 0 || len(strings.Join(ws, "")) < 5 {
		return false
	}
	words := strings.Fields(Normalize(text))
	for _, w := range ws {
		ok := false
		for _, t := range words {
			if tokenEq(w, t) || len(w) >= 5 && strings.HasPrefix(t, w) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func ratio(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	n := max(len(ra), len(rb))
	if n == 0 {
		return 1
	}
	return 1 - float64(lev(ra, rb))/float64(n)
}

func lev(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// SearchWords — bazadan nom bo'yicha qidirish uchun so'z boshlari (har nomning asosiy so'zlaridan 4 harf).
func SearchWords(gs []Guess) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range gs {
		for _, n := range g.Names {
			clean := strings.NewReplacer("'", "", "ʻ", "", "’", "", "‘", "", "`", "", "ʼ", "").Replace(strings.ToLower(n))
			for _, w := range strings.FieldsFunc(clean, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
				if generic[Normalize(w)] || len([]rune(w)) < 3 {
					continue
				}
				if rs := []rune(w); len(rs) > 4 {
					w = string(rs[:4])
				}
				if !seen[w] {
					seen[w] = true
					out = append(out, w)
				}
			}
		}
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}
