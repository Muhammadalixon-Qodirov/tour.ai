// Package planner — deterministik marshrut rejalashtirgichi.
//
// Nomzodlar → ball → kunlarga bo'lish (yangi joy "langar"lari bilan) → tartib (eng yaqin qo'shni + 2-opt)
// → vaqt jadvali va transport → yangilik kvotasini tuzatish. AI ga bog'liq emas: AI ishlamasa ham shu natija beriladi.
package planner

import (
	"errors"
	"math"
	"slices"
	"sort"
	"time"

	"yoldosh/internal/domain"
)

var ErrImpossible = errors.New("route_impossible")

const (
	DayStartMin   = 9 * 60     // 09:00
	DayEndMin     = 20 * 60    // 20:00 — oxirgi joy shu vaqtgacha tugashi kerak
	LunchFromMin  = 12*60 + 30 // 12:30
	LunchUntilMin = 15 * 60    // tushlik shu vaqtdan kech boshlanmaydi
	WalkMaxM      = 1200.0     // to'g'ri chiziq bo'yicha; undan uzoq — taksi
	walkMPerMin   = 80.0
	taxiKmPerHour = 22.0
	taxiPickupMin = 4
	detourFactor  = 1.3 // ko'cha yo'li to'g'ri chiziqdan uzunroq
	bayesM        = 20.0
)

type Candidate struct {
	ID             string
	Category       string
	Novelty        domain.Novelty
	IsPartner      bool
	Interests      []string
	Loc            domain.LatLng
	Rating         float64
	Reviews        int
	VisitMin       int
	Hours          []domain.OpeningHours
	PriceLevel     int
	Wheelchair     bool
	FamilyFriendly bool
	Bulk           bool // ommaviy importdan (OSM), jamoa tanlamagan — reytingda pastroq
}

func (c Candidate) IsMeal() bool  { return domain.IsMealCategory(c.Category) }
func (c Candidate) isNovel() bool { return c.Novelty.IsNovel() && !c.IsPartner }

type Prefs struct {
	Days       int
	StartDate  time.Time
	Interests  []string
	Pace       string // relaxed | balanced | active
	Budget     int    // 1..3, 0 = ahamiyatsiz
	Start      *domain.LatLng
	Wheelchair bool
	WithKids   bool
	Boost      map[string]float64 // guruh ovozlari: 0..1 (ko'pchilik yoqtirgan joy ustun)
	Exclude    map[string]bool    // guruh ko'pchiligi rad etgan joylar
}

type City struct {
	Center    domain.LatLng
	TaxiBase  int64
	TaxiPerKm int64
	AvgRating float64
}

// SightsPerDay — kuniga nechta diqqatga sazovor joy (tushlikdan tashqari).
func SightsPerDay(pace string) int {
	switch pace {
	case "relaxed":
		return 3
	case "active":
		return 5
	default:
		return 4
	}
}

// Plan — to'liq marshrut tuzadi.
func Plan(p Prefs, city City, cands []Candidate) ([]domain.PlanDay, error) {
	if p.Days < 1 {
		return nil, ErrImpossible
	}
	pl := newPlanner(p, city, cands)
	if len(pl.sights) == 0 || len(pl.sights) < p.Days {
		return nil, ErrImpossible
	}
	days := make([]domain.PlanDay, p.Days)
	for d := range p.Days {
		days[d] = pl.planDay(d)
	}
	pl.enforceShare(days)
	total := 0
	for _, d := range days {
		total += len(d.Stops)
	}
	if total == 0 {
		return nil, ErrImpossible
	}
	return days, nil
}

type planner struct {
	p      Prefs
	city   City
	sights []Candidate
	meals  []Candidate
	byID   map[string]Candidate
	score  map[string]float64
	used   map[string]bool
}

func newPlanner(p Prefs, city City, cands []Candidate) *planner {
	if city.AvgRating <= 0 {
		city.AvgRating = 4.3
	}
	pl := &planner{p: p, city: city, byID: map[string]Candidate{}, score: map[string]float64{}, used: map[string]bool{}}
	for _, c := range cands {
		if p.Wheelchair && !c.Wheelchair || p.WithKids && !c.FamilyFriendly || p.Exclude[c.ID] {
			continue
		}
		if c.VisitMin <= 0 {
			c.VisitMin = 45
		}
		pl.byID[c.ID] = c
		pl.score[c.ID] = Score(c, p, city.AvgRating)
		if c.IsMeal() {
			pl.meals = append(pl.meals, c)
		} else {
			pl.sights = append(pl.sights, c)
		}
	}
	byScore := func(s []Candidate) {
		sort.SliceStable(s, func(i, j int) bool {
			si, sj := pl.score[s[i].ID], pl.score[s[j].ID]
			if si != sj {
				return si > sj
			}
			return s[i].ID < s[j].ID
		})
	}
	byScore(pl.sights)
	byScore(pl.meals)
	return pl
}

// Score — qiziqish mosligi × sifat (Bayes reyting) × yangilik bonusi × byudjet mosligi.
// Hamkorlik ball QO'SHMAYDI.
func Score(c Candidate, p Prefs, cityAvg float64) float64 {
	interest := 0.7
	if len(p.Interests) > 0 {
		overlap := 0
		for _, i := range p.Interests {
			if matchesInterest(c, i) {
				overlap++
			}
		}
		need := min(2, len(p.Interests))
		interest = 0.35 + 0.65*math.Min(1, float64(overlap)/float64(need))
	}
	v := float64(c.Reviews)
	bayes := (v*c.Rating + bayesM*cityAvg) / (v + bayesM)
	quality := bayes / 5
	novelty := 1.0
	switch c.Novelty {
	case domain.NoveltyNew:
		novelty = 1.25
	case domain.NoveltyHidden:
		novelty = 1.15
	}
	budget := 1.0
	if p.Budget > 0 && c.PriceLevel > p.Budget {
		budget = 0.6
	}
	curated := 1.0
	if c.Bulk {
		curated = 0.75 // qo'lda tanlangan va tekshirilgan joylar ustun; OSM joylari bo'shliqni to'ldiradi
	}
	return interest * quality * novelty * budget * curated * (1 + p.Boost[c.ID])
}

func matchesInterest(c Candidate, interest string) bool {
	return slices.Contains(c.Interests, interest) || slices.Contains(domain.CategoryInterests[c.Category], interest)
}

func (pl *planner) start() domain.LatLng {
	if pl.p.Start != nil {
		return *pl.p.Start
	}
	return pl.city.Center
}

func (pl *planner) weekday(day int) int {
	return int(pl.p.StartDate.AddDate(0, 0, day).Weekday())
}

// openLongEnough — joy shu kuni ochiq va tashrif uchun yetarli vaqt bor.
func openLongEnough(c Candidate, weekday int) bool {
	o, cl, ok := domain.DayWindow(c.Hours, weekday)
	return ok && cl-max(o, DayStartMin) >= c.VisitMin && o < DayEndMin
}

func (pl *planner) planDay(d int) domain.PlanDay {
	wd := pl.weekday(d)
	target := SightsPerDay(pl.p.Pace)
	anchors := 1
	if target >= 4 {
		anchors = 2
	}

	// Muvozanat: "mashhur + yangi". Yangi joylar kunning yarmidan oshmaydi (kamida 1 ta), qolgani mashhur joylar.
	maxNovel := max(1, target/2)

	var chosen []Candidate
	var centroid domain.LatLng
	novel := 0
	const (
		anyKind = iota
		onlyNovel
		onlyIconic
	)
	pick := func(mode int) bool {
		best, bestVal := -1, -1.0
		for i, c := range pl.sights {
			if pl.used[c.ID] || containsID(chosen, c.ID) || !openLongEnough(c, wd) {
				continue
			}
			switch {
			case mode == onlyNovel && !c.isNovel(),
				mode == onlyIconic && c.isNovel(),
				mode == anyKind && c.isNovel() && novel >= maxNovel:
				continue
			}
			val := pl.score[c.ID]
			if len(chosen) > 0 {
				val /= 1 + domain.DistanceM(centroid, c.Loc)/2500
			}
			if val > bestVal {
				best, bestVal = i, val
			}
		}
		if best < 0 {
			return false
		}
		c := pl.sights[best]
		chosen = append(chosen, c)
		if c.isNovel() {
			novel++
		}
		centroid = mean(chosen)
		return true
	}
	// 1) eng yaxshi yangi joy — kun "langari", klaster markazi shu atrofda
	pick(onlyNovel)
	// 2) kamida bitta mashhur joy (bo'lsa)
	if target > 1 {
		pick(onlyIconic)
	}
	for len(chosen) < target {
		if novel < min(anchors, maxNovel) && pick(onlyNovel) {
			continue
		}
		if !pick(anyKind) {
			// mashhur joylar tugasa, cheklovni yumshatib yangi joy bilan to'ldiramiz
			if !pick(onlyNovel) {
				break
			}
		}
	}

	order := pl.orderRoute(chosen)
	stops := pl.scheduleNew(order, wd)
	day := domain.PlanDay{Index: d, Stops: stops}
	if dayNovel(day) == 0 {
		pl.replaceWithNovel(&day, wd)
	}
	for _, s := range day.Stops {
		pl.used[s.PlaceID] = true
	}
	return day
}

func containsID(cs []Candidate, id string) bool {
	return slices.ContainsFunc(cs, func(c Candidate) bool { return c.ID == id })
}

func mean(cs []Candidate) domain.LatLng {
	var m domain.LatLng
	for _, c := range cs {
		m.Lat += c.Loc.Lat
		m.Lng += c.Loc.Lng
	}
	n := float64(len(cs))
	return domain.LatLng{Lat: m.Lat / n, Lng: m.Lng / n}
}

// orderRoute — boshlanish nuqtasidan eng yaqin qo'shni, keyin 2-opt bilan yo'lni qisqartirish.
func (pl *planner) orderRoute(cs []Candidate) []Candidate {
	if len(cs) <= 1 {
		return cs
	}
	rest := slices.Clone(cs)
	var out []Candidate
	pos := pl.start()
	for len(rest) > 0 {
		bi := 0
		for i := range rest {
			if domain.DistanceM(pos, rest[i].Loc) < domain.DistanceM(pos, rest[bi].Loc) {
				bi = i
			}
		}
		out = append(out, rest[bi])
		pos = rest[bi].Loc
		rest = slices.Delete(rest, bi, bi+1)
	}
	start := pl.start()
	pathLen := func(r []Candidate) float64 {
		total, p := 0.0, start
		for _, c := range r {
			total += domain.DistanceM(p, c.Loc)
			p = c.Loc
		}
		return total
	}
	for improved := true; improved; {
		improved = false
		for i := 0; i < len(out)-1; i++ {
			for j := i + 1; j < len(out); j++ {
				cand := slices.Clone(out)
				slices.Reverse(cand[i : j+1])
				if pathLen(cand) < pathLen(out)-1 {
					out, improved = cand, true
				}
			}
		}
	}
	return out
}

// scheduleNew — vaqt jadvali: ish vaqtiga sig'maydigan joy tashlab ketiladi, 12:30 dan keyin tushlik qo'yiladi.
func (pl *planner) scheduleNew(order []Candidate, wd int) []domain.PlanStop {
	var stops []domain.PlanStop
	t := DayStartMin
	pos := pl.start()
	first := true
	lunch := false
	usedMeal := map[string]bool{}

	addMeal := func() {
		m, ok := pl.nearestMeal(pos, wd, t, usedMeal)
		if !ok {
			return
		}
		tr := pl.Transit(pos, m.Loc)
		arrive := t + tr.DurationMin
		dur := max(45, m.VisitMin)
		if !domain.IsOpenAt(m.Hours, wd, arrive) || arrive+dur > DayEndMin {
			return
		}
		stops = append(stops, pl.stopFor(m, domain.KindMeal, arrive, dur, &tr))
		usedMeal[m.ID] = true
		lunch = true
		t, pos, first = arrive+dur, m.Loc, false
	}

	for _, c := range order {
		if !lunch && t >= LunchFromMin && t <= LunchUntilMin {
			addMeal()
		}
		var tr *domain.Transit
		arrive := t
		if !first || pl.p.Start != nil {
			x := pl.Transit(pos, c.Loc)
			tr = &x
			arrive += x.DurationMin
		}
		start, ok := domain.FitVisit(c.Hours, wd, arrive, c.VisitMin)
		if !ok || start+c.VisitMin > DayEndMin {
			continue
		}
		arrive = start
		stops = append(stops, pl.stopFor(c, domain.KindPlace, arrive, c.VisitMin, tr))
		t, pos, first = arrive+c.VisitMin, c.Loc, false
	}
	if !lunch && t >= LunchFromMin-60 && t <= LunchUntilMin {
		addMeal()
	}
	return stops
}

func (pl *planner) nearestMeal(pos domain.LatLng, wd, t int, exclude map[string]bool) (Candidate, bool) {
	var best Candidate
	bestVal := -1.0
	for _, m := range pl.meals {
		if exclude[m.ID] || pl.used[m.ID] || !domain.IsOpenAt(m.Hours, wd, t+15) {
			continue
		}
		val := pl.score[m.ID] / (1 + domain.DistanceM(pos, m.Loc)/1500)
		if val > bestVal {
			best, bestVal = m, val
		}
	}
	return best, bestVal >= 0
}

func (pl *planner) stopFor(c Candidate, kind domain.StopKind, arrive, dur int, tr *domain.Transit) domain.PlanStop {
	code, arg := ReasonFor(c, kind, pl.p)
	return domain.PlanStop{Kind: kind, PlaceID: c.ID, Novelty: c.Novelty, IsPartner: c.IsPartner,
		ArriveMin: arrive, DurationMin: dur, Transit: tr, ReasonCode: code, ReasonArg: arg}
}

// ReasonFor — nega shu joy tanlangani (i18n shablon kaliti + argument).
func ReasonFor(c Candidate, kind domain.StopKind, p Prefs) (code, arg string) {
	if kind == domain.KindMeal {
		return "meal", ""
	}
	if c.Novelty == domain.NoveltyNew && !c.IsPartner {
		return "new", ""
	}
	if c.Novelty == domain.NoveltyHidden && !c.IsPartner {
		return "hidden", ""
	}
	for _, i := range p.Interests {
		if matchesInterest(c, i) {
			return "interest", i
		}
	}
	return "top_rated", ""
}

// Transit — ikki nuqta orasidagi yo'l: piyoda yoki taksi (taxminiy narx bilan).
func (pl *planner) Transit(a, b domain.LatLng) domain.Transit {
	return TransitBetween(a, b, pl.city)
}

func TransitBetween(a, b domain.LatLng, city City) domain.Transit {
	straight := domain.DistanceM(a, b)
	road := straight * detourFactor
	if straight < WalkMaxM {
		return domain.Transit{Mode: "walk", DistanceM: int(math.Round(road)), DurationMin: int(math.Ceil(road / walkMPerMin))}
	}
	km := road / 1000
	fare := city.TaxiBase + int64(math.Ceil(km))*city.TaxiPerKm
	fare = (fare + 999) / 1000 * 1000
	m := domain.UZS(fare)
	return domain.Transit{Mode: "taxi", DistanceM: int(math.Round(road)),
		DurationMin: int(math.Ceil(km/taxiKmPerHour*60)) + taxiPickupMin, Fare: &m}
}

func dayNovel(d domain.PlanDay) int {
	n := 0
	for _, s := range d.Stops {
		if s.CountsForNovelty() {
			n++
		}
	}
	return n
}

// replaceWithNovel — kundagi eng past balli mashhur joyni eng yaqin mos yangi joyga almashtiradi.
func (pl *planner) replaceWithNovel(day *domain.PlanDay, wd int) bool {
	worst, worstScore := -1, math.MaxFloat64
	for i, s := range day.Stops {
		if s.Kind == domain.KindPlace && !s.CountsForNovelty() && pl.score[s.PlaceID] < worstScore {
			worst, worstScore = i, pl.score[s.PlaceID]
		}
	}
	if worst < 0 {
		return false
	}
	ref := pl.byID[day.Stops[worst].PlaceID].Loc
	var best Candidate
	bestD := math.MaxFloat64
	for _, c := range pl.sights {
		if !c.isNovel() || pl.used[c.ID] || inDay(*day, c.ID) || !openLongEnough(c, wd) {
			continue
		}
		if d := domain.DistanceM(ref, c.Loc); d < bestD {
			best, bestD = c, d
		}
	}
	if best.ID == "" {
		return false
	}
	day.Stops = pl.Reschedule(day.Stops, worst, best, wd)
	return true
}

func inDay(d domain.PlanDay, id string) bool {
	return slices.ContainsFunc(d.Stops, func(s domain.PlanStop) bool { return s.PlaceID == id })
}

// Reschedule — i-chi to'xtashni c ga almashtirib, vaqtlarni qayta hisoblaydi.
func (pl *planner) Reschedule(stops []domain.PlanStop, i int, c Candidate, wd int) []domain.PlanStop {
	out := slices.Clone(stops)
	out[i] = pl.stopFor(c, domain.KindPlace, 0, c.VisitMin, nil)
	return Retime(out, wd, pl.p.Start, pl.city, pl.byID)
}

// enforceShare — umumiy ulush 25% dan past bo'lsa, eng kam yangi joyli kundan boshlab almashtiradi.
func (pl *planner) enforceShare(days []domain.PlanDay) {
	for range 20 {
		share, total := domain.NoveltyShare(days)
		if total == 0 || share >= domain.MinNoveltyShare {
			return
		}
		bi, bestRatio := -1, math.MaxFloat64
		for i, d := range days {
			places := 0
			for _, s := range d.Stops {
				if s.Kind == domain.KindPlace {
					places++
				}
			}
			if places == 0 {
				continue
			}
			if r := float64(dayNovel(d)) / float64(places); r < bestRatio && places > dayNovel(d) {
				bi, bestRatio = i, r
			}
		}
		if bi < 0 || !pl.replaceWithNovel(&days[bi], pl.weekday(bi)) {
			return
		}
		for _, s := range days[bi].Stops {
			pl.used[s.PlaceID] = true
		}
	}
}

// Retime — tartibni saqlagan holda vaqtlar va transportni qayta hisoblaydi (foydalanuvchi tahriridan keyin).
// Joylar tashlab ketilmaydi: foydalanuvchi qarori hurmat qilinadi.
func Retime(stops []domain.PlanStop, wd int, start *domain.LatLng, city City, byID map[string]Candidate) []domain.PlanStop {
	out := slices.Clone(stops)
	t := DayStartMin
	var pos domain.LatLng
	hasPos := false
	if start != nil {
		pos, hasPos = *start, true
	}
	for i := range out {
		c, ok := byID[out[i].PlaceID]
		if !ok {
			continue
		}
		arrive := t
		out[i].Transit = nil
		if hasPos {
			tr := TransitBetween(pos, c.Loc, city)
			out[i].Transit = &tr
			arrive += tr.DurationMin
		}
		if out[i].DurationMin <= 0 {
			out[i].DurationMin = c.VisitMin
		}
		if start, _ := domain.FitVisit(c.Hours, wd, arrive, out[i].DurationMin); start > arrive {
			arrive = start
		}
		out[i].ArriveMin = arrive
		t, pos, hasPos = arrive+out[i].DurationMin, c.Loc, true
	}
	return out
}

// Alternatives — almashtirish uchun o'xshash joylar: yangi joylar birinchi, keyin o'xshashlik va masofa.
func Alternatives(target Candidate, cands []Candidate, exclude map[string]bool, wd, n int) []Candidate {
	type scored struct {
		c   Candidate
		val float64
	}
	var list []scored
	for _, c := range cands {
		if c.ID == target.ID || exclude[c.ID] || c.IsMeal() != target.IsMeal() || !openLongEnough(c, wd) {
			continue
		}
		d := domain.DistanceM(target.Loc, c.Loc)
		if d > 4000 {
			continue
		}
		sim := 0.3
		if c.Category == target.Category {
			sim += 0.4
		}
		for _, i := range target.Interests {
			if slices.Contains(c.Interests, i) {
				sim += 0.1
			}
		}
		val := sim / (1 + d/1500)
		if c.isNovel() {
			val += 10 // yangi joylar ro'yxat boshida
		}
		list = append(list, scored{c, val})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].val != list[j].val {
			return list[i].val > list[j].val
		}
		return list[i].c.ID < list[j].c.ID
	})
	out := make([]Candidate, 0, n)
	for i := 0; i < len(list) && i < n; i++ {
		out = append(out, list[i].c)
	}
	return out
}
