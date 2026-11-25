package planner

import (
	"math"
	"slices"
	"sort"

	"yoldosh/internal/domain"
)

// Qayta rejalash: kechikdim / charchadim / ob-havo / bo'sh vaqt.
// Kunning boshidagi `fixed` ta to'xtash (o'tib ketgan yoki belgilangan) o'zgarmaydi; qolganlari qayta hisoblanadi.

const (
	ReplanDayEndMin = 21 * 60 // qayta rejalashda kun biroz uzayishi mumkin
	freeTimeRadiusM = 1500.0
)

type Change struct {
	Kind       string // removed | removed_tired | replaced_weather | added | shifted
	PlaceID    string
	NewPlaceID string
	Minutes    int
}

type Replanner struct {
	p     Prefs
	city  City
	byID  map[string]Candidate
	cands []Candidate
	score map[string]float64
}

func NewReplanner(p Prefs, city City, cands []Candidate) *Replanner {
	if city.AvgRating <= 0 {
		city.AvgRating = 4.3
	}
	r := &Replanner{p: p, city: city, byID: map[string]Candidate{}, cands: cands, score: map[string]float64{}}
	for _, c := range cands {
		r.byID[c.ID] = c
		r.score[c.ID] = Score(c, p, city.AvgRating)
	}
	return r
}

func (r *Replanner) ByID() map[string]Candidate { return r.byID }

// RetimeFrom — stops ni `from` daqiqada `pos` nuqtadan jo'nab, ketma-ket qayta hisoblaydi.
func (r *Replanner) RetimeFrom(stops []domain.PlanStop, wd, from int, pos *domain.LatLng) []domain.PlanStop {
	out := slices.Clone(stops)
	t := from
	cur := pos
	for i := range out {
		c, ok := r.byID[out[i].PlaceID]
		if !ok {
			continue
		}
		arrive := t
		out[i].Transit = nil
		if cur != nil {
			tr := TransitBetween(*cur, c.Loc, r.city)
			out[i].Transit = &tr
			arrive += tr.DurationMin
		}
		if o, _, ok := domain.DayWindow(c.Hours, wd); ok {
			arrive = max(arrive, o)
		}
		out[i].ArriveMin = arrive
		t = arrive + out[i].DurationMin
		loc := c.Loc
		cur = &loc
	}
	return out
}

// departure — `fixed` dan keyingi qism qayerdan va qachon boshlanadi (reja bo'yicha).
func (r *Replanner) departure(stops []domain.PlanStop, fixed int, now int, loc *domain.LatLng) (int, *domain.LatLng) {
	t := max(now, DayStartMin)
	pos := r.p.Start
	if fixed > 0 {
		prev := stops[fixed-1]
		t = max(t, prev.EndMin())
		if c, ok := r.byID[prev.PlaceID]; ok {
			l := c.Loc
			pos = &l
		}
	}
	if loc != nil {
		pos = loc
	}
	return t, pos
}

// fits — to'xtash ish vaqti va kun oxiriga sig'adimi.
func (r *Replanner) fits(s domain.PlanStop, wd int) bool {
	c, ok := r.byID[s.PlaceID]
	if !ok {
		return true
	}
	_, cl, ok := domain.DayWindow(c.Hours, wd)
	return ok && s.EndMin() <= min(cl, ReplanDayEndMin)
}

// dropOne — olib tashlash uchun eng mos to'xtash: avval mashhur joylar, eng past ball; ovqat va yagona yangi joy oxirida.
func (r *Replanner) dropOne(rest []domain.PlanStop) int {
	novel := 0
	for _, s := range rest {
		if s.CountsForNovelty() {
			novel++
		}
	}
	best, bestKey := -1, math.MaxFloat64
	for i, s := range rest {
		key := r.score[s.PlaceID]
		switch {
		case s.Kind == domain.KindMeal:
			key += 100
		case s.CountsForNovelty() && novel <= 1:
			key += 50 // kunning yagona yangi joyi — iloji boricha saqlanadi
		case s.CountsForNovelty():
			key += 1
		}
		if key < bestKey {
			best, bestKey = i, key
		}
	}
	return best
}

// fitDay — sig'maydigan to'xtashlarni olib tashlab, hammasi sig'guncha qayta hisoblaydi.
func (r *Replanner) fitDay(fixedPart, rest []domain.PlanStop, wd, from int, pos *domain.LatLng, kind string) ([]domain.PlanStop, []Change) {
	var changes []Change
	for {
		rest = r.RetimeFrom(rest, wd, from, pos)
		bad := slices.IndexFunc(rest, func(s domain.PlanStop) bool { return !r.fits(s, wd) })
		if bad < 0 || len(rest) == 0 {
			break
		}
		i := r.dropOne(rest)
		changes = append(changes, Change{Kind: kind, PlaceID: rest[i].PlaceID})
		rest = slices.Delete(rest, i, i+1)
	}
	return append(slices.Clone(fixedPart), rest...), changes
}

// Late — `minutes` ga kechikish: qolgan to'xtashlar suriladi, sig'maganlari olib tashlanadi.
func (r *Replanner) Late(stops []domain.PlanStop, fixed, wd, now, minutes int, loc *domain.LatLng) ([]domain.PlanStop, []Change) {
	if fixed >= len(stops) {
		return stops, nil
	}
	from, pos := r.departure(stops, fixed, now, loc)
	// rejadagi jo'nash vaqti + kechikish
	next := stops[fixed]
	planned := next.ArriveMin
	if next.Transit != nil {
		planned -= next.Transit.DurationMin
	}
	from = max(from, planned+minutes)
	out, changes := r.fitDay(stops[:fixed], slices.Clone(stops[fixed:]), wd, from, pos, "removed")
	if fixed < len(out) {
		if shift := out[fixed].ArriveMin - next.ArriveMin; shift > 0 {
			changes = append([]Change{{Kind: "shifted", Minutes: shift}}, changes...)
		}
	}
	return out, changes
}

// Tired — qolgan diqqatga sazovor joylarning yarmi qoladi (eng yaxshilari, yangi joy saqlanadi).
func (r *Replanner) Tired(stops []domain.PlanStop, fixed, wd, now int, loc *domain.LatLng) ([]domain.PlanStop, []Change) {
	if fixed >= len(stops) {
		return stops, nil
	}
	rest := slices.Clone(stops[fixed:])
	places := 0
	for _, s := range rest {
		if s.Kind == domain.KindPlace {
			places++
		}
	}
	keep := (places + 1) / 2
	var changes []Change
	for places > keep {
		i := r.dropOne(rest)
		if rest[i].Kind == domain.KindMeal {
			break
		}
		changes = append(changes, Change{Kind: "removed_tired", PlaceID: rest[i].PlaceID})
		rest = slices.Delete(rest, i, i+1)
		places--
	}
	from, pos := r.departure(stops, fixed, now, loc)
	out, more := r.fitDay(stops[:fixed], rest, wd, from, pos, "removed")
	return out, append(changes, more...)
}

var indoorCategories = []string{"museum", "gallery", "workshop", "mosque", "bazaar"}

// Weather — yomon ob-havoda ochiq havodagi joylar yaqin yopiq joylarga almashtiriladi.
// Yangi joyning o'rniga iloji boricha yangi joy qo'yiladi (kvota saqlansin).
func (r *Replanner) Weather(stops []domain.PlanStop, fixed, wd, now int, exclude map[string]bool, loc *domain.LatLng) ([]domain.PlanStop, []Change) {
	if fixed >= len(stops) {
		return stops, nil
	}
	used := map[string]bool{}
	for k := range exclude {
		used[k] = true
	}
	rest := slices.Clone(stops[fixed:])
	var changes []Change
	for i, s := range rest {
		c, ok := r.byID[s.PlaceID]
		if !ok || s.Kind != domain.KindPlace || !domain.IsOutdoorCategory(c.Category) {
			continue
		}
		repl, ok := r.nearestIndoor(c, wd, used, s.CountsForNovelty())
		if !ok {
			continue
		}
		used[repl.ID] = true
		rest[i] = domain.PlanStop{Kind: domain.KindPlace, PlaceID: repl.ID, Novelty: repl.Novelty, IsPartner: repl.IsPartner,
			DurationMin: repl.VisitMin, ReasonCode: "weather"}
		changes = append(changes, Change{Kind: "replaced_weather", PlaceID: c.ID, NewPlaceID: repl.ID})
	}
	from, pos := r.departure(stops, fixed, now, loc)
	out, more := r.fitDay(stops[:fixed], rest, wd, from, pos, "removed")
	return out, append(changes, more...)
}

func (r *Replanner) nearestIndoor(near Candidate, wd int, used map[string]bool, wantNovel bool) (Candidate, bool) {
	var best Candidate
	bestVal := -1.0
	for _, c := range r.cands {
		if used[c.ID] || c.IsMeal() || !slices.Contains(indoorCategories, c.Category) || !openLongEnough(c, wd) {
			continue
		}
		val := r.score[c.ID] / (1 + domain.DistanceM(near.Loc, c.Loc)/1500)
		if wantNovel && c.isNovel() {
			val += 10
		}
		if val > bestVal {
			best, bestVal = c, val
		}
	}
	return best, bestVal >= 0
}

// FreeTime — "bo'sh N daqiqam bor": joriy nuqtadan 1.5 km ichida vaqtga sig'adigan qisqa marshrut,
// kunning qolgan qismidan oldin qo'yiladi (qolgan to'xtashlar keyinga suriladi).
func (r *Replanner) FreeTime(stops []domain.PlanStop, fixed, wd, now, minutes int, loc domain.LatLng, exclude map[string]bool) ([]domain.PlanStop, []Change) {
	used := map[string]bool{}
	for k := range exclude {
		used[k] = true
	}
	t := max(now, DayStartMin)
	end := t + minutes
	pos := loc
	var added []domain.PlanStop
	var changes []Change
	for {
		var best Candidate
		bestVal := -1.0
		for _, c := range r.cands {
			if used[c.ID] || c.IsMeal() || domain.DistanceM(loc, c.Loc) > freeTimeRadiusM {
				continue
			}
			tr := TransitBetween(pos, c.Loc, r.city)
			arrive := t + tr.DurationMin
			dur := min(c.VisitMin, 60)
			if arrive+dur > end || !domain.IsOpenAt(c.Hours, wd, arrive) || !domain.IsOpenAt(c.Hours, wd, arrive+dur-1) {
				continue
			}
			val := r.score[c.ID] / (1 + domain.DistanceM(pos, c.Loc)/700)
			if c.isNovel() {
				val *= 1.2
			}
			if val > bestVal {
				best, bestVal = c, val
			}
		}
		if bestVal < 0 {
			break
		}
		tr := TransitBetween(pos, best.Loc, r.city)
		dur := min(best.VisitMin, 60)
		s := domain.PlanStop{Kind: domain.KindPlace, PlaceID: best.ID, Novelty: best.Novelty, IsPartner: best.IsPartner,
			ArriveMin: t + tr.DurationMin, DurationMin: dur, Transit: &tr, ReasonCode: "free_time"}
		added = append(added, s)
		changes = append(changes, Change{Kind: "added", PlaceID: best.ID})
		used[best.ID] = true
		t, pos = s.EndMin(), best.Loc
	}
	if len(added) == 0 {
		return stops, nil
	}
	rest := slices.Clone(stops[fixed:])
	out := append(slices.Clone(stops[:fixed]), added...)
	fitted, more := r.fitDay(out, rest, wd, t, &pos, "removed")
	return fitted, append(changes, more...)
}

// SortChanges — barqaror tartib (testlar va UI uchun).
func SortChanges(c []Change) {
	order := map[string]int{"shifted": 0, "replaced_weather": 1, "added": 2, "removed_tired": 3, "removed": 4}
	sort.SliceStable(c, func(i, j int) bool { return order[c[i].Kind] < order[c[j].Kind] })
}
