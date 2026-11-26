package api

// Safar Insights (docs/MONETIZATION.md §5): hududiy turizm bo'limlari, tur firmalar, mehmonxonalar uchun
// anonim agregat. Hisoblash sayt (web/src/lib/monetization/insights.ts) bilan bir xil; K = 50 dan kichik har
// qanday kesim qiymati null (sababi "below_k"), kichik son hech qachon qaytmaydi. Javoblarda shaxsiy maydon yo'q.

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"yoldosh/internal/i18n"
	"yoldosh/internal/planner"
	"yoldosh/internal/store"
)

const (
	kAnon = 50
	// stayShare — kunlik kishi boshi byudjetining tunash uchun ketadigan ulushi ("Talab bor, taklif yo'q").
	stayShare = 0.4
	belowK    = "below_k"
)

// kValue — K dan kichik bo'lsa null.
type kValue = *float64

type shareDTO struct {
	Key    string `json:"key"`
	Count  kValue `json:"count"`
	Share  kValue `json:"share"`
	Reason string `json:"reason,omitempty"` // below_k
	Label  string `json:"label,omitempty"`  // joy nomi (topPlaces)
}

type gapDTO struct {
	Plans        kValue `json:"plans"`
	Share        kValue `json:"share"`
	CheapestStay *int64 `json:"cheapestStay"`
	Reason       string `json:"reason,omitempty"`
}

type insightsDTO struct {
	Region             string            `json:"region"`
	From               string            `json:"from"`
	To                 string            `json:"to"`
	K                  int               `json:"k"`
	Sample             bool              `json:"sample"`
	Trips              kValue            `json:"trips"`
	TripsPrev          kValue            `json:"tripsPrev"`
	Change             *float64          `json:"change"`
	AvgBudgetPerPerson kValue            `json:"avgBudgetPerPerson"`
	AvgDays            kValue            `json:"avgDays"`
	TopPlaces          []shareDTO        `json:"topPlaces"`
	Transport          []shareDTO        `json:"transport"`
	Companion          []shareDTO        `json:"companion"`
	Interests          []shareDTO        `json:"interests"`
	Residency          []shareDTO        `json:"residency"`
	GapNoStay          gapDTO            `json:"gapNoStay"`
	Withheld           map[string]string `json:"withheld"` // null qilingan maydon → sabab
}

// insightsRecord — bitta safarning ruxsat etilgan anonim maydonlari.
type insightsRecord struct {
	days      int
	people    int
	budget    *int64
	interests []string
	companion string
	resident  *bool
	transport string
	places    []string
}

func recordOf(t store.InsightsTrip) insightsRecord {
	var p tripReq
	_ = json.Unmarshal(t.Prefs, &p)
	var a struct {
		Mode string `json:"mode"`
	}
	if len(t.Arrival) > 0 {
		_ = json.Unmarshal(t.Arrival, &a)
	}
	group := 0
	if p.GroupSize != nil {
		group = *p.GroupSize
	}
	rec := insightsRecord{days: t.Days, people: planner.GroupSizeFor(group, p.Companion), interests: p.Interests,
		companion: p.Companion, resident: p.IsResident, transport: a.Mode, places: t.Places}
	if p.BudgetPerDay != nil && *p.BudgetPerDay > 0 {
		rec.budget = p.BudgetPerDay
	}
	return rec
}

func fp(v float64) *float64 { return &v }

func kv(n int) kValue {
	if n >= kAnon {
		return fp(float64(n))
	}
	return nil
}

type tally struct {
	counts map[string]int
	order  []string // birinchi uchragan tartibi (sayt Map bilan bir xil)
}

func newTally() *tally { return &tally{counts: map[string]int{}} }

func (t *tally) add(k string) {
	if k == "" {
		return
	}
	if _, ok := t.counts[k]; !ok {
		t.order = append(t.order, k)
	}
	t.counts[k]++
}

// shares — fixed tartib berilsa shu tartibda (faqat uchraganlari), aks holda soni bo'yicha kamayish tartibida.
func shares(total int, t *tally, fixed []string) []shareDTO {
	var keys []string
	if fixed != nil {
		for _, k := range fixed {
			if _, ok := t.counts[k]; ok {
				keys = append(keys, k)
			}
		}
	} else {
		keys = slices.Clone(t.order)
		sort.SliceStable(keys, func(i, j int) bool { return t.counts[keys[i]] > t.counts[keys[j]] })
	}
	out := make([]shareDTO, 0, len(keys))
	for _, k := range keys {
		c := t.counts[k]
		d := shareDTO{Key: k}
		if total >= kAnon && c >= kAnon {
			d.Count, d.Share = fp(float64(c)), fp(float64(c)/float64(total))
		} else {
			d.Reason = belowK
		}
		out = append(out, d)
	}
	return out
}

func summarise(region string, from, to time.Time, cur, prev []insightsRecord, cheapest *int64) insightsDTO {
	n := len(cur)
	out := insightsDTO{Region: region, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly), K: kAnon,
		Trips: kv(n), TripsPrev: kv(len(prev)), Withheld: map[string]string{}}
	if n >= kAnon && len(prev) >= kAnon {
		out.Change = fp(float64(n)/float64(len(prev)) - 1)
	}
	var budgets []int64
	days := 0
	places, transport, companion, interests, residency := newTally(), newTally(), newTally(), newTally(), newTally()
	withMode := 0
	for _, t := range cur {
		if t.budget != nil {
			budgets = append(budgets, *t.budget)
		}
		days += t.days
		seen := map[string]bool{}
		for _, p := range t.places {
			if !seen[p] {
				seen[p] = true
				places.add(p)
			}
		}
		if t.transport != "" {
			withMode++
			transport.add(t.transport)
		}
		companion.add(t.companion)
		for _, i := range t.interests {
			interests.add(i)
		}
		if t.resident != nil {
			if *t.resident {
				residency.add("resident")
			} else {
				residency.add("foreign")
			}
		}
	}
	if len(budgets) >= kAnon {
		var sum int64
		for _, b := range budgets {
			sum += b
		}
		out.AvgBudgetPerPerson = fp(math.Round(float64(sum)/float64(len(budgets))/1000) * 1000)
	}
	if n >= kAnon {
		out.AvgDays = fp(math.Round(float64(days)/float64(n)*10) / 10)
	}
	out.TopPlaces = shares(n, places, nil)
	if len(out.TopPlaces) > 8 {
		out.TopPlaces = out.TopPlaces[:8]
	}
	out.Transport = shares(withMode, transport, transportModes)
	out.Companion = shares(n, companion, companions)
	out.Interests = shares(n, interests, nil)
	if len(out.Interests) > 6 {
		out.Interests = out.Interests[:6]
	}
	out.Residency = shares(n, residency, []string{"resident", "foreign"})

	// Talab bor, taklif yo'q: 2+ kunlik, byudjeti bor rejalar; eng arzon turar joy (2 kishilik xona, odam soniga
	// 2 tagacha bo'linadi) kunlik byudjetning 40% iga sig'maydimi.
	out.GapNoStay.CheapestStay = cheapest
	staying, gap := 0, 0
	for _, t := range cur {
		if t.days > 1 && t.budget != nil {
			staying++
			if cheapest != nil && float64(*cheapest)/float64(min(t.people, 2)) > float64(*t.budget)*stayShare {
				gap++
			}
		}
	}
	switch {
	case staying < kAnon:
		out.GapNoStay.Reason = belowK
	case gap > 0 && gap < kAnon:
		// Ulush ham kichik sonni ochib qo'yadi (staying ma'lum): ikkalasi ham yashiriladi.
		out.GapNoStay.Reason = belowK
	default:
		out.GapNoStay.Plans, out.GapNoStay.Share = fp(float64(gap)), fp(float64(gap)/float64(staying))
	}

	for k, v := range map[string]bool{"trips": out.Trips == nil, "tripsPrev": out.TripsPrev == nil, "change": out.Change == nil,
		"avgBudgetPerPerson": out.AvgBudgetPerPerson == nil, "avgDays": out.AvgDays == nil, "gapNoStay": out.GapNoStay.Plans == nil} {
		if v {
			out.Withheld[k] = belowK
		}
	}
	return out
}

// ---------- autentifikatsiya ----------

const keyRegions ctxKey = 300

// requireInsights — insights rolidagi foydalanuvchi (rol bazadan tekshiriladi: admin olib qo'ysa darhol ishlamaydi).
func (s *Server) requireInsights(next http.Handler) http.Handler {
	return s.requireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.store.User(r.Context(), userOf(r))
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		if u.Role != "insights" {
			writeError(w, r, http.StatusForbidden, "forbidden", false)
			return
		}
		regions, err := s.store.InsightsRegions(r.Context(), u.ID)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyRegions, regions)))
	}))
}

func (s *Server) insightsFor(w http.ResponseWriter, r *http.Request) (insightsDTO, bool) {
	fe := fieldErrors{}
	region := r.URL.Query().Get("region")
	regions, _ := r.Context().Value(keyRegions).([]string)
	if region == "" {
		if len(regions) == 1 {
			region = regions[0]
		} else {
			fe.add("region", "required")
		}
	}
	from, to := s.dateRange(r, fe, 366)
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return insightsDTO{}, false
	}
	if !slices.Contains(regions, region) {
		writeError(w, r, http.StatusForbidden, "insights_region_forbidden", false)
		return insightsDTO{}, false
	}
	ctx := r.Context()
	span := int(to.Sub(from).Hours()/24) + 1
	pFrom, pTo := from.AddDate(0, 0, -span), from.AddDate(0, 0, -1)
	load := func(a, b time.Time) ([]insightsRecord, error) {
		trips, err := s.store.InsightsTrips(ctx, region, a, b)
		if err != nil {
			return nil, err
		}
		out := make([]insightsRecord, 0, len(trips))
		for _, t := range trips {
			out = append(out, recordOf(t))
		}
		return out, nil
	}
	cur, err := load(from, to)
	if err != nil {
		s.internalError(w, r, err)
		return insightsDTO{}, false
	}
	prev, err := load(pFrom, pTo)
	if err != nil {
		s.internalError(w, r, err)
		return insightsDTO{}, false
	}
	cheapest, err := s.store.CheapestStay(ctx, region)
	if err != nil {
		s.internalError(w, r, err)
		return insightsDTO{}, false
	}
	out := summarise(region, from, to, cur, prev, cheapest)
	// Joy nomlari (faqat ko'rsatilgan kesimlar uchun).
	var ids []string
	for _, p := range out.TopPlaces {
		ids = append(ids, p.Key)
	}
	if len(ids) > 0 {
		names, err := s.store.PlacesByIDs(ctx, ids, i18n.Chain(langOf(r)))
		if err != nil {
			s.internalError(w, r, err)
			return insightsDTO{}, false
		}
		for i, p := range out.TopPlaces {
			if pl, ok := names[p.Key]; ok {
				out.TopPlaces[i].Label = pl.Name
			}
		}
	}
	return out, true
}

func (s *Server) insightsSummary(w http.ResponseWriter, r *http.Request) {
	if out, ok := s.insightsFor(w, r); ok {
		w.Header().Set("Cache-Control", "private, max-age=300")
		writeJSON(w, http.StatusOK, out)
	}
}

func insightsLabel(lang, section, key string) string {
	switch section {
	case "kpi", "gap", "transport", "companion", "residency", "interests":
		return i18n.T(lang, "insights."+section+"."+key)
	}
	return key
}

// insightsExport — GET /insights/export?...&format=csv: saytdagi toCsv bilan bir xil qatorlar; yashirilgan katak bo'sh.
func (s *Server) insightsExport(w http.ResponseWriter, r *http.Request) {
	if f := r.URL.Query().Get("format"); f != "" && f != "csv" {
		writeValidation(w, r, fieldErrors{"format": "csv"})
		return
	}
	sm, ok := s.insightsFor(w, r)
	if !ok {
		return
	}
	lang := langOf(r)
	num := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	share := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', 4, 64)
	}
	rows := [][]string{
		{"section", "key", "label", "count", "share"},
		{"meta", "region", sm.Region, "", ""},
		{"meta", "period", sm.From + ".." + sm.To, "", ""},
		{"meta", "k_anonymity", strconv.Itoa(sm.K), "", ""},
		{"meta", "sample", strconv.FormatBool(sm.Sample), "", ""},
		{"kpi", "trips", insightsLabel(lang, "kpi", "trips"), num(sm.Trips), ""},
		{"kpi", "trips_prev", insightsLabel(lang, "kpi", "trips_prev"), num(sm.TripsPrev), ""},
		{"kpi", "change", insightsLabel(lang, "kpi", "change"), "", share(sm.Change)},
		{"kpi", "avg_budget_per_person", insightsLabel(lang, "kpi", "budget"), num(sm.AvgBudgetPerPerson), ""},
		{"kpi", "avg_days", insightsLabel(lang, "kpi", "days"), num(sm.AvgDays), ""},
	}
	add := func(section string, list []shareDTO) {
		for _, x := range list {
			label := insightsLabel(lang, section, x.Key)
			if x.Label != "" {
				label = x.Label
			}
			rows = append(rows, []string{section, x.Key, label, num(x.Count), share(x.Share)})
		}
	}
	add("top_places", sm.TopPlaces)
	add("transport", sm.Transport)
	add("companion", sm.Companion)
	add("interests", sm.Interests)
	add("residency", sm.Residency)
	rows = append(rows, []string{"gap", "no_stay_under_budget", insightsLabel(lang, "gap", "no_stay"), num(sm.GapNoStay.Plans), share(sm.GapNoStay.Share)})
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	_ = cw.WriteAll(rows)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="safar-insights-`+sm.Region+`-`+sm.From+`-`+sm.To+`.csv"`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(buf.Bytes())
}
