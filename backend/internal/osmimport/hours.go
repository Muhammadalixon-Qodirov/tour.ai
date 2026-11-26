// Package osmimport — OpenStreetMap obyektlarini (osm-extract chiqishi) bazaga yuklaydi.
package osmimport

import (
	"regexp"
	"strings"

	"yoldosh/internal/domain"
)

var osmDays = map[string]int{"Su": 0, "Mo": 1, "Tu": 2, "We": 3, "Th": 4, "Fr": 5, "Sa": 6}

var timeRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})-(\d{1,2}):(\d{2})$`)

// ParseHours — OSM opening_hours ning keng tarqalgan oddiy shakllari:
// "24/7", "09:00-18:00", "Mo-Fr 09:00-18:00; Sa 10:00-16:00", "Mo-Su 08:00-13:00,14:00-20:00" (tanaffus),
// "Mo-Sa 09:00-18:00; Su off". Tushunilmasa — nil (ish vaqti noma'lum).
func ParseHours(s string) (hours []domain.OpeningHours, open24 bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false, false
	}
	if s == "24/7" {
		for d := range 7 {
			hours = append(hours, domain.OpeningHours{Day: d, Open: "00:00", Close: "24:00"})
		}
		return hours, true, true
	}
	byDay := map[int]domain.OpeningHours{}
	for _, rule := range strings.Split(s, ";") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		days := []int{0, 1, 2, 3, 4, 5, 6}
		times := rule
		if sp := strings.IndexByte(rule, ' '); sp > 0 && !strings.ContainsAny(rule[:sp], ":") {
			var okDays bool
			days, okDays = parseDays(rule[:sp])
			if !okDays {
				return nil, false, false
			}
			times = strings.TrimSpace(rule[sp+1:])
		}
		if times == "off" || times == "closed" {
			for _, d := range days {
				delete(byDay, d)
			}
			continue
		}
		var spans [][2]string
		for _, part := range strings.Split(times, ",") {
			m := timeRe.FindStringSubmatch(strings.TrimSpace(part))
			if m == nil {
				return nil, false, false
			}
			spans = append(spans, [2]string{pad(m[1]) + ":" + m[2], pad(m[3]) + ":" + m[4]})
		}
		h := domain.OpeningHours{Open: spans[0][0], Close: spans[len(spans)-1][1]}
		for i := 1; i < len(spans); i++ { // oraliqlar orasidagi bo'shliq — tanaffus
			h.Breaks = append(h.Breaks, domain.Break{From: spans[i-1][1], To: spans[i][0], Reason: "lunch"})
		}
		o, err1 := domain.ParseHHMM(h.Open)
		c, err2 := domain.ParseHHMM(h.Close)
		if err1 != nil || err2 != nil || c <= o {
			return nil, false, false // tun bo'yi ishlaydigan (22:00-02:00) shakllar hozircha qo'llanmaydi
		}
		for _, d := range days {
			x := h
			x.Day = d
			byDay[d] = x
		}
	}
	for d := range 7 {
		if h, ok := byDay[d]; ok {
			hours = append(hours, h)
		}
	}
	return hours, false, len(hours) > 0
}

func pad(h string) string {
	if len(h) == 1 {
		return "0" + h
	}
	return h
}

func parseDays(spec string) ([]int, bool) {
	var out []int
	for _, part := range strings.Split(spec, ",") {
		if a, b, isRange := strings.Cut(part, "-"); isRange {
			da, ok1 := osmDays[a]
			db, ok2 := osmDays[b]
			if !ok1 || !ok2 {
				return nil, false
			}
			for d := da; ; d = (d + 1) % 7 {
				out = append(out, d)
				if d == db {
					break
				}
			}
		} else {
			d, ok := osmDays[part]
			if !ok {
				return nil, false
			}
			out = append(out, d)
		}
	}
	return out, len(out) > 0
}
