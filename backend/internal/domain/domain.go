// Package domain: sof tiplar va qoidalar. Hech qanday infratuzilma paketini import qilmaydi.
package domain

import (
	"fmt"
	"math"
	"slices"
)

type Novelty string

const (
	NoveltyIconic Novelty = "iconic"
	NoveltyNew    Novelty = "new"
	NoveltyHidden Novelty = "hidden"
)

func (n Novelty) Valid() bool { return n == NoveltyIconic || n == NoveltyNew || n == NoveltyHidden }

// IsNovel — yangi yoki kam tanilgan joy.
func (n Novelty) IsNovel() bool { return n == NoveltyNew || n == NoveltyHidden }

var Interests = []string{"history", "food", "nature", "craft", "art", "architecture", "shopping", "nightlife"}

var Categories = []string{"monument", "museum", "mosque", "bazaar", "workshop", "gallery",
	"restaurant", "cafe", "park", "viewpoint", "ecovillage", "theatre", "hotel"}

func ValidInterest(s string) bool { return slices.Contains(Interests, s) }
func ValidCategory(s string) bool { return slices.Contains(Categories, s) }

// IsMealCategory — ovqatlanish joyi (marshrutda `meal` to'xtashi bo'ladi).
func IsMealCategory(c string) bool { return c == "restaurant" || c == "cafe" }

// IsOutdoorCategory — ob-havoga sezgir joylar.
func IsOutdoorCategory(c string) bool { return c == "park" || c == "viewpoint" || c == "ecovillage" }

// IsLodging — tunab qolish joyi (mehmonxona). Marshrutda diqqatga sazovor joy emas, joylashish.
func IsLodging(c string) bool { return c == "hotel" }

// CategoryInterests — kategoriya qaysi qiziqishlarga tabiiy mos keladi.
var CategoryInterests = map[string][]string{
	"monument":   {"history", "architecture"},
	"museum":     {"history", "art"},
	"mosque":     {"history", "architecture"},
	"bazaar":     {"food", "shopping", "craft"},
	"workshop":   {"craft", "art", "shopping"},
	"gallery":    {"art"},
	"restaurant": {"food"},
	"cafe":       {"food", "nightlife"},
	"park":       {"nature"},
	"viewpoint":  {"nature", "architecture"},
	"ecovillage": {"nature", "craft", "food"},
	"theatre":    {"art", "nightlife"},
	"hotel":      {}, // mehmonxona qiziqishga bog'lanmaydi (joylashish turi)
}

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (p LatLng) Valid() bool { return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180 }

type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func UZS(amount int64) Money { return Money{Amount: amount, Currency: "UZS"} }

// Image — rasm. URL "/static/..." bilan boshlansa API uni PUBLIC_BASE_URL bilan to'liq manzilga aylantiradi.
type Image struct {
	URL       string `json:"url"`
	Blurhash  string `json:"blurhash,omitempty"`
	Credit    string `json:"credit,omitempty"`    // muallif yoki sayt
	License   string `json:"license,omitempty"`   // "CC BY-SA 4.0"; noma'lum bo'lsa bo'sh
	SourceURL string `json:"sourceUrl,omitempty"` // rasm olingan sahifa
	Source    string `json:"source,omitempty"`    // "Wikimedia Commons", "afisha.uz"
}

// DistanceM — ikki nuqta orasidagi to'g'ri chiziq masofasi (haversine), metrda.
func DistanceM(a, b LatLng) float64 {
	const r = 6371000.0
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLng := (b.Lng - a.Lng) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(h)))
}

// OpeningHours — bitta kun oynasi. Day: 0 = yakshanba ... 6 = shanba (Go time.Weekday bilan bir xil).
// Breaks — shu kun ichidagi tanaffuslar (tushlik, juma namozi), joy bu vaqtda yopiq.
type OpeningHours struct {
	Day    int     `json:"day"`
	Open   string  `json:"open"`
	Close  string  `json:"close"`
	Breaks []Break `json:"breaks,omitempty"`
}

type Break struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"` // lunch | prayer | other
}

// ParseHHMM "09:30" → 570.
func ParseHHMM(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || h < 0 || h > 24 || m < 0 || m > 59 {
		return 0, fmt.Errorf("noto'g'ri vaqt %q", s)
	}
	return h*60 + m, nil
}

// FormatHHMM 570 → "09:30".
func FormatHHMM(min int) string {
	if min < 0 {
		min = 0
	}
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// DayWindow — berilgan hafta kunidagi ish vaqti (daqiqada). Jadval bo'sh bo'lsa joy har doim ochiq deb olinadi.
func DayWindow(hours []OpeningHours, weekday int) (open, close int, ok bool) {
	if len(hours) == 0 {
		return 0, 24 * 60, true
	}
	for _, h := range hours {
		if h.Day != weekday {
			continue
		}
		o, err1 := ParseHHMM(h.Open)
		c, err2 := ParseHHMM(h.Close)
		if err1 != nil || err2 != nil || c <= o {
			continue
		}
		return o, c, true
	}
	return 0, 0, false
}

// dayBreaks — shu kungi tanaffuslar (daqiqada, tartiblangan emas).
func dayBreaks(hours []OpeningHours, weekday int) [][2]int {
	var out [][2]int
	for _, h := range hours {
		if h.Day != weekday {
			continue
		}
		for _, b := range h.Breaks {
			f, err1 := ParseHHMM(b.From)
			t, err2 := ParseHHMM(b.To)
			if err1 == nil && err2 == nil && t > f {
				out = append(out, [2]int{f, t})
			}
		}
	}
	return out
}

// IsOpenAt — joy shu daqiqada ochiqmi (tanaffuslar hisobga olinadi).
func IsOpenAt(hours []OpeningHours, weekday, minute int) bool {
	o, c, ok := DayWindow(hours, weekday)
	if !ok || minute < o || minute >= c {
		return false
	}
	for _, b := range dayBreaks(hours, weekday) {
		if minute >= b[0] && minute < b[1] {
			return false
		}
	}
	return true
}

// FitVisit — tashrifni ish vaqti va tanaffuslarga moslaydi: kelish vaqtini ochilishgacha yoki tanaffus
// oxirigacha suradi. ok=false — shu kuni yopilishgacha sig'maydi.
func FitVisit(hours []OpeningHours, weekday, arrive, dur int) (start int, ok bool) {
	o, c, open := DayWindow(hours, weekday)
	if !open {
		return arrive, false
	}
	start = max(arrive, o)
	breaks := dayBreaks(hours, weekday)
	for moved := true; moved; {
		moved = false
		for _, b := range breaks {
			if start < b[1] && start+dur > b[0] {
				start, moved = b[1], true
			}
		}
	}
	return start, start+dur <= c
}

// FairRange — adolatli narx oralig'i. Kuzatuvlar 5 tadan kam bo'lsa tadqiqot (ochiq manba) oralig'i qoladi;
// 5 ta va ko'p bo'lsa oraliq kuzatuvlar va tadqiqot chegaralari birgalikdagi 20- va 80-protsentil
// (bitta g'alati kuzatuv oraliqni buzmaydi). obs o'sish tartibida bo'lishi shart emas.
func FairRange(baseMin, baseMax int64, obs []int64) (lo, hi int64) {
	if len(obs) < MinPriceSample {
		return baseMin, baseMax
	}
	all := append(slices.Clone(obs), baseMin, baseMax)
	slices.Sort(all)
	at := func(q float64) int64 { return all[int(math.Round(q*float64(len(all)-1)))] }
	return at(0.2), at(0.8)
}

// MinPriceSample — oraliqni kuzatuvlardan hisoblash uchun eng kam tasdiqlangan kuzatuv.
const MinPriceSample = 5

// PlausiblePrice — kuzatuv avtomatik tasdiqlanadimi: tadqiqot oralig'ining yarmidan ikki baravarigacha.
// Undan tashqaridagi (masalan turistga aytilgan 3 baravar narx) moderatsiyaga tushadi.
func PlausiblePrice(baseMin, baseMax, paid int64) bool {
	return paid*2 >= baseMin && paid <= baseMax*2
}
