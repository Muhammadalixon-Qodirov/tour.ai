// Package planner — deterministik marshrut rejalashtirgichi.
//
// Ilova va saytdagi rejalashtirgich (mobile/src/mock/planner.ts) bilan bir xil qoidalar:
// shahar markazidan 18 km ichidagi joylar → ball (qiziqish, reyting, yangilik, olomon) − 0.35/km →
// yangilik kvotasi (kuniga kamida 1, jami kamida 1/3; Hamkor kvotaga kirmaydi) → qolgani mashhur joylar (+2.5) →
// eng yaqin qo'shni zanjiri bo'yicha kunlarga bo'lish → ikkinchi joydan keyin tushlik → 09:00 dan 20:00 gacha
// jadval (ish vaqti va tanaffuslar hisobga olinadi, vaqtlar 5 daqiqaga yaxlitlanadi) → kishi boshiga kunlik byudjetga
// sig'dirish (eng qimmat mashhur joy birinchi tushadi). Jamoa tanlagan joylar OSM importidan ustun; guruh ovozlari
// (Boost/Exclude) ballga qo'shiladi yoki joyni chiqaradi.
// AI ga bog'liq emas: AI ishlamasa ham shu natija beriladi.
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
	DayStartMin = 9 * 60  // 09:00
	DayEndMin   = 20 * 60 // 20:00 — oxirgi joy shu vaqtgacha tugashi kerak

	ReachM             = 18000.0   // kunlik marshrut shahar markazidan shu radiusda
	KmPenalty          = 0.35      // markazdan har km uchun ball kamayadi
	LandmarkBonus      = 2.5       // kvotadan keyingi joylarda mashhur joy ustun
	BulkPenalty        = 2.0       // OSM importi (jamoa tanlamagan): bitta qiziqish mosligicha past
	BoostWeight        = 4.0       // guruh ovozi (0..1) ball qo'shadi
	TargetNoveltyShare = 1.0 / 3.0 // generatsiyada maqsad (kontrakt minimumi domain.MinNoveltyShare = 0.25)
	MealCost           = 60000     // tushlik, kishi boshiga, so'm (byudjet hisobi)

	WalkMaxM    = 1300.0 // ko'cha masofasi; undan uzoq — taksi
	roadFactor  = 1.35   // ko'cha yo'li to'g'ri chiziqdan uzunroq
	walkMPerMin = 75.0
	bayesM      = 20.0

	defaultTaxiBase  = 6000
	defaultTaxiPerKm = 2500
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
	Bulk           bool   // ommaviy importdan (OSM), jamoa tanlamagan — reytingda pastroq
	Crowd          string // low | medium | high
	FeeLocal       *int64 // kirish narxi, so'm (nil = noma'lum)
	FeeForeigner   *int64
}

func (c Candidate) IsMeal() bool  { return domain.IsMealCategory(c.Category) }
func (c Candidate) isNovel() bool { return c.Novelty.IsNovel() && !c.IsPartner }

// isFood — rejalashtirishda tushlik joyi (ilovadagidek bozor ham shu yerda).
func (c Candidate) isFood() bool { return c.IsMeal() || c.Category == "bazaar" }

// Fee — kirish narxi: rezident uchun mahalliy tarif, aks holda chet ellik tarifi; noma'lum bo'lsa 0.
func (c Candidate) Fee(resident bool) int64 {
	f := c.FeeForeigner
	if resident {
		f = c.FeeLocal
	}
	if f == nil {
		return 0
	}
	return *f
}

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

	BudgetPerDay int64 // kishi boshiga kuniga, so'm; 0 = cheklovsiz
	IsResident   bool  // kirish narxi mahalliy tarifda
	GroupSize    int   // taksi shu songa bo'linadi; 0 = 1
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

// GroupSizeFor — taksi ulushi uchun odamlar soni (ilovadagi groupSizeFor bilan bir xil).
func GroupSizeFor(groupSize int, companion string) int {
	if groupSize > 0 {
		return groupSize
	}
	switch companion {
	case "couple":
		return 2
	case "family":
		return 3
	case "friends":
		return 4
	}
	return 1
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
	groups := pl.split()
	days := make([]domain.PlanDay, p.Days)
	for d := range p.Days {
		days[d] = pl.planDay(d, groups[d])
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
	score  map[string]float64 // tanlash bali (ilovadagi score)
	used   map[string]bool
}

func newPlanner(p Prefs, city City, cands []Candidate) *planner {
	if city.AvgRating <= 0 {
		city.AvgRating = 4.3
	}
	pl := &planner{p: p, city: city, byID: map[string]Candidate{}, score: map[string]float64{}, used: map[string]bool{}}
	for _, c := range cands {
		if c.VisitMin <= 0 {
			c.VisitMin = 45
		}
		pl.byID[c.ID] = c // foydalanuvchi qo'shgan uzoq joylar ham Retime da topilsin
		if p.Wheelchair && !c.Wheelchair || p.WithKids && !c.FamilyFriendly || p.Exclude[c.ID] {
			continue
		}
		// Uzoq viloyat joylari (Namangan tog'lari, Imom Buxoriy) kunlik rejaga kirmaydi, qo'shimcha sifatida taklif qilinadi.
		if domain.DistanceM(city.Center, c.Loc) > ReachM || !pl.openOnSomeDay(c) {
			continue
		}
		pl.score[c.ID] = PickScore(c, p.Interests) + p.Boost[c.ID]*BoostWeight
		if c.isFood() {
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

func (pl *planner) openOnSomeDay(c Candidate) bool {
	for d := range min(pl.p.Days, 7) {
		if openLongEnough(c, pl.weekday(d)) {
			return true
		}
	}
	return false
}

// PickScore — ilovadagi score(): qiziqish mosligi ×2, reyting (noma'lum = 4.5), yangilik +0.6 (Hamkor emas),
// olomon (kam +0.3, ko'p −0.3). Jamoa tanlagan joylar OSM importidan ustun (−BulkPenalty).
// Hamkorlik ball QO'SHMAYDI.
func PickScore(c Candidate, interests []string) float64 {
	match := 0
	for _, i := range c.Interests {
		if slices.Contains(interests, i) {
			match++
		}
	}
	rating := c.Rating
	if rating <= 0 {
		rating = 4.5
	}
	v := float64(match)*2 + (rating-4)*2
	if c.isNovel() {
		v += 0.6
	}
	switch c.Crowd {
	case "low":
		v += 0.3
	case "high":
		v -= 0.3
	}
	if c.Bulk {
		v -= BulkPenalty
	}
	return v
}

// Score — qiziqish mosligi × sifat (Bayes reyting) × yangilik bonusi × byudjet mosligi (0..~1.25).
// Qayta rejalash va muqobil joylar shu shkalada ishlaydi. Hamkorlik ball QO'SHMAYDI.
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
	return ok && min(cl, DayEndMin)-max(o, DayStartMin) >= c.VisitMin
}

// near — markazdan uzoqlik jarimasi bilan ball.
func (pl *planner) near(c Candidate) float64 {
	return pl.score[c.ID] - domain.DistanceM(pl.city.Center, c.Loc)/1000*KmPenalty
}

// split — joylarni tanlaydi va kunlarga bo'ladi (ilovadagi planTrip bilan bir xil tartib).
func (pl *planner) split() [][]Candidate {
	days := pl.p.Days
	want := min(len(pl.sights), days*SightsPerDay(pl.p.Pace))

	// 1) Yangilik kvotasi: kuniga kamida bitta va jami kamida 1/3.
	quota := max(days, int(math.Ceil(float64(want)/3)))
	var novel, rest []Candidate
	for _, c := range pl.sights {
		if c.isNovel() {
			novel = append(novel, c)
		} else {
			rest = append(rest, c)
		}
	}
	byKey := func(s []Candidate, key func(Candidate) float64) {
		sort.SliceStable(s, func(i, j int) bool {
			ki, kj := key(s[i]), key(s[j])
			if ki != kj {
				return ki > kj
			}
			return s[i].ID < s[j].ID
		})
	}
	byKey(novel, pl.near)
	novel = novel[:min(len(novel), quota)]
	// 2) Qolgani: avval shaharning mashhur joylari (Registonsiz Samarqand — shaxsiy reja emas, buzilgan reja).
	for _, c := range pl.sights {
		if c.isNovel() && !containsID(novel, c.ID) {
			rest = append(rest, c)
		}
	}
	byKey(rest, func(c Candidate) float64 {
		v := pl.near(c)
		if c.Novelty == domain.NoveltyIconic {
			v += LandmarkBonus
		}
		return v
	})
	chosen := append(slices.Clone(novel), rest[:min(len(rest), want-len(novel))]...)

	// 3) Geografik bo'lish: eng yaqin qo'shni zanjiri, keyin teng bo'laklarga.
	chain := nearestOrder(chosen, pl.start())
	groups := make([][]Candidate, days)
	for i, c := range chain {
		d := min(days-1, i*days/len(chain))
		groups[d] = append(groups[d], c)
	}
	// 4) Tuzatish: har kunda kamida bitta yangi joy; ≥ 2 joyli kunda kamida bitta mashhur joy.
	swapInto := func(want func(Candidate) bool, need func([]Candidate) bool) {
		for di, day := range groups {
			if len(day) == 0 || !need(day) {
				continue
			}
			for dj, donor := range groups {
				if dj == di {
					continue
				}
				var give []int
				for k, c := range donor {
					if want(c) {
						give = append(give, k)
					}
				}
				if len(give) < 2 {
					continue
				}
				g := give[1]
				last := len(day) - 1
				groups[dj][g], groups[di][last] = groups[di][last], groups[dj][g]
				break
			}
		}
	}
	countIf := func(day []Candidate, f func(Candidate) bool) int {
		n := 0
		for _, c := range day {
			if f(c) {
				n++
			}
		}
		return n
	}
	isNovel := func(c Candidate) bool { return c.isNovel() }
	isIconic := func(c Candidate) bool { return !c.isNovel() }
	swapInto(isNovel, func(d []Candidate) bool { return countIf(d, isNovel) == 0 })
	swapInto(isIconic, func(d []Candidate) bool { return len(d) >= 2 && countIf(d, isIconic) == 0 && countIf(d, isNovel) >= 2 })
	return groups
}

func containsID(cs []Candidate, id string) bool {
	return slices.ContainsFunc(cs, func(c Candidate) bool { return c.ID == id })
}

// nearestOrder — `from` nuqtadan eng yaqin qo'shni tartibi.
func nearestOrder(cs []Candidate, from domain.LatLng) []Candidate {
	rest := slices.Clone(cs)
	out := make([]Candidate, 0, len(cs))
	pos := from
	for len(rest) > 0 {
		bi := 0
		for i := 1; i < len(rest); i++ {
			if domain.DistanceM(pos, rest[i].Loc) < domain.DistanceM(pos, rest[bi].Loc) {
				bi = i
			}
		}
		out = append(out, rest[bi])
		pos = rest[bi].Loc
		rest = slices.Delete(rest, bi, bi+1)
	}
	return out
}

func roundUp5(m int) int { return (m + 4) / 5 * 5 }

func (pl *planner) planDay(d int, group []Candidate) domain.PlanDay {
	wd := pl.weekday(d)
	day := domain.PlanDay{Index: d}
	if len(group) == 0 {
		return day
	}
	ordered := nearestOrder(group, group[0].Loc)
	day.Stops = pl.schedule(ordered, wd)
	day.Stops = pl.fitBudget(day.Stops, wd)
	if dayNovel(day) == 0 {
		pl.replaceWith(&day, wd, true)
	}
	if places, iconic := dayCounts(day); places >= 2 && iconic == 0 {
		pl.replaceWith(&day, wd, false)
	}
	for _, s := range day.Stops {
		pl.used[s.PlaceID] = true
	}
	return day
}

// schedule — vaqt jadvali: ish vaqti yoki 20:00 ga sig'maydigan joy tashlab ketiladi; ikkinchi joydan keyin
// (bitta joy bo'lsa undan keyin) eng yaqin ochiq ovqatlanish joyida tushlik.
func (pl *planner) schedule(order []Candidate, wd int) []domain.PlanStop {
	var stops []domain.PlanStop
	t := DayStartMin
	pos := pl.start()
	first := pl.p.Start == nil
	sights, lunch := 0, false
	addMeal := func() {
		lunch = true
		meals := slices.Clone(pl.meals)
		sort.SliceStable(meals, func(i, j int) bool { return domain.DistanceM(pos, meals[i].Loc) < domain.DistanceM(pos, meals[j].Loc) })
		for _, m := range meals {
			if pl.used[m.ID] {
				continue
			}
			tr := pl.Transit(pos, m.Loc)
			arrive := roundUp5(t + tr.DurationMin)
			if !domain.IsOpenAt(m.Hours, wd, arrive) {
				continue
			}
			start, ok := domain.FitVisit(m.Hours, wd, arrive, m.VisitMin)
			if !ok || start+m.VisitMin > DayEndMin {
				continue
			}
			arrive = start
			stops = append(stops, pl.stopFor(m, domain.KindMeal, arrive, m.VisitMin, &tr))
			pl.used[m.ID] = true
			t, pos = arrive+m.VisitMin, m.Loc
			return
		}
	}
	for _, c := range order {
		var tr *domain.Transit
		arrive := t
		if !first {
			x := pl.Transit(pos, c.Loc)
			tr = &x
			arrive += x.DurationMin
		}
		start, ok := domain.FitVisit(c.Hours, wd, roundUp5(arrive), c.VisitMin)
		if !ok || start+c.VisitMin > DayEndMin {
			continue
		}
		arrive = start
		stops = append(stops, pl.stopFor(c, domain.KindPlace, arrive, c.VisitMin, tr))
		t, pos, first = arrive+c.VisitMin, c.Loc, false
		if sights++; sights == min(2, len(order)) && !lunch {
			addMeal()
		}
	}
	return stops
}

// StopCost — bir kishining to'xtashdagi xarajati: kirish + tushlik + taksi ulushi (1 000 so'mgacha yaxlitlangan).
func StopCost(s domain.PlanStop, c Candidate, p Prefs) int64 {
	cost := c.Fee(p.IsResident)
	if s.Kind == domain.KindMeal {
		cost += MealCost
	}
	if s.Transit != nil && s.Transit.Fare != nil {
		g := int64(max(1, p.GroupSize))
		cost += int64(math.Round(float64(s.Transit.Fare.Amount)/float64(g)/1000)) * 1000
	}
	return cost
}

// DayCost — kunning kishi boshiga xarajati.
func DayCost(stops []domain.PlanStop, byID map[string]Candidate, p Prefs) int64 {
	var total int64
	for _, s := range stops {
		total += StopCost(s, byID[s.PlaceID], p)
	}
	return total
}

// fitBudget — kun byudjetga sig'guncha eng qimmat MASHHUR joy olib tashlanadi. Yangi va kam tanilgan joylar
// pul uchun olib tashlanmaydi (kvota saqlanadi).
func (pl *planner) fitBudget(stops []domain.PlanStop, wd int) []domain.PlanStop {
	if pl.p.BudgetPerDay <= 0 {
		return stops
	}
	for DayCost(stops, pl.byID, pl.p) > pl.p.BudgetPerDay && len(stops) > 2 {
		victim, cost := -1, int64(0)
		for i, s := range stops {
			if s.Kind != domain.KindPlace || s.Novelty != domain.NoveltyIconic {
				continue
			}
			if c := StopCost(s, pl.byID[s.PlaceID], pl.p); c > cost {
				victim, cost = i, c
			}
		}
		if victim < 0 {
			break
		}
		stops = Retime(slices.Delete(slices.Clone(stops), victim, victim+1), wd, pl.p.Start, pl.city, pl.byID)
	}
	return stops
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

// TransitBetween — ilovadagi transitBetween: ko'cha masofasi = to'g'ri chiziq × 1.35; 1.3 km gacha piyoda
// (75 m/daqiqa), undan uzoq — shahar taksi tarifi (yo'lga chiqish + km), 1 000 so'mgacha yaxlitlanadi.
// Shahar ichida ~21 km/soat, 12 km dan keyin ~55 km/soat, + 4 daqiqa kutish.
func TransitBetween(a, b domain.LatLng, city City) domain.Transit {
	road := math.Round(domain.DistanceM(a, b)*roadFactor/10) * 10
	if road < WalkMaxM {
		return domain.Transit{Mode: "walk", DistanceM: int(road), DurationMin: max(2, int(math.Round(road/walkMPerMin)))}
	}
	km := road / 1000
	base, perKm := city.TaxiBase, city.TaxiPerKm
	if base <= 0 && perKm <= 0 {
		base, perKm = defaultTaxiBase, defaultTaxiPerKm
	}
	fare := int64(math.Round((float64(base)+km*float64(perKm))/1000)) * 1000
	minutes := km/0.35 + 4
	if km > 12 {
		minutes = 12/0.35 + (km-12)/0.92 + 4
	}
	m := domain.UZS(max(fare, 1000))
	return domain.Transit{Mode: "taxi", DistanceM: int(road), DurationMin: int(math.Round(minutes)), Fare: &m}
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

func dayCounts(d domain.PlanDay) (places, iconic int) {
	for _, s := range d.Stops {
		if s.Kind == domain.KindPlace {
			places++
			if !s.CountsForNovelty() {
				iconic++
			}
		}
	}
	return
}

// replaceWithNovel — kundagi eng past balli mashhur joyni eng yaqin mos yangi joyga almashtiradi.
func (pl *planner) replaceWithNovel(day *domain.PlanDay, wd int) bool {
	return pl.replaceWith(day, wd, true)
}

// replaceWith — wantNovel: eng past balli mashhur joy → eng yaqin yangi joy; aks holda eng past balli
// yangi joy (kunning yagona yangi joyi emas) → eng yaqin mashhur joy. Vaqtga sig'magan almashtirish qabul qilinmaydi.
func (pl *planner) replaceWith(day *domain.PlanDay, wd int, wantNovel bool) bool {
	novel := dayNovel(*day)
	worst, worstScore := -1, math.MaxFloat64
	for i, s := range day.Stops {
		if s.Kind != domain.KindPlace || s.CountsForNovelty() == wantNovel || (!wantNovel && novel <= 1) {
			continue
		}
		if sc := pl.score[s.PlaceID]; sc < worstScore {
			worst, worstScore = i, sc
		}
	}
	if worst < 0 {
		return false
	}
	ref := pl.byID[day.Stops[worst].PlaceID].Loc
	alts := slices.Clone(pl.sights)
	sort.SliceStable(alts, func(i, j int) bool { return domain.DistanceM(ref, alts[i].Loc) < domain.DistanceM(ref, alts[j].Loc) })
	for _, c := range alts {
		if c.isNovel() != wantNovel || pl.used[c.ID] || inDay(*day, c.ID) || !openLongEnough(c, wd) {
			continue
		}
		stops := pl.Reschedule(day.Stops, worst, c, wd)
		if fitsDay(stops, pl.byID, wd) {
			day.Stops = stops
			return true
		}
	}
	return false
}

// fitsDay — hamma to'xtash ish vaqti va 20:00 ichida.
func fitsDay(stops []domain.PlanStop, byID map[string]Candidate, wd int) bool {
	for _, s := range stops {
		start, ok := domain.FitVisit(byID[s.PlaceID].Hours, wd, s.ArriveMin, s.DurationMin)
		if !ok || start != s.ArriveMin || s.EndMin() > DayEndMin {
			return false
		}
	}
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

// enforceShare — umumiy ulush 1/3 dan past bo'lsa, eng kam yangi joyli kundan boshlab almashtiradi.
// Kunning yagona mashhur joyiga tegilmaydi (muvozanat: "mashhur + yangi").
func (pl *planner) enforceShare(days []domain.PlanDay) {
	for range 20 {
		share, total := domain.NoveltyShare(days)
		if total == 0 || share >= TargetNoveltyShare-1e-9 {
			return
		}
		bi, bestRatio := -1, math.MaxFloat64
		for i, d := range days {
			places, iconic := dayCounts(d)
			if iconic < 2 {
				continue
			}
			if r := float64(dayNovel(d)) / float64(places); r < bestRatio {
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
		arrive = roundUp5(arrive)
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
