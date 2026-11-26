package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// ---------- manbalar ----------

// Source — javobda ishlatilgan manba (SSE "sources" hodisasi). PlaceID bo'lsa — faqat shu joy javobda tilga olinganda.
type Source struct {
	Title   string `json:"title"`
	URL     string `json:"url,omitempty"`
	PlaceID string `json:"-"`
}

func (t *Tools) addSource(s Source, placeID string) {
	if s.Title == "" && s.URL == "" {
		return
	}
	s.PlaceID = placeID
	for _, x := range t.sources {
		if x.URL == s.URL && x.Title == s.Title {
			return
		}
	}
	t.sources = append(t.sources, s)
}

// SourcesFor — ishlatilgan manbalar; joy manbalari faqat mentioned(placeID) bo'lsa. Ko'pi bilan 6 ta.
func (t *Tools) SourcesFor(mentioned func(placeID string) bool) []Source {
	out := []Source{}
	for _, s := range t.sources {
		if s.PlaceID != "" && !mentioned(s.PlaceID) {
			continue
		}
		out = append(out, s)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func parseSources(raw json.RawMessage) []Source {
	var list []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var out []Source
	for _, x := range list {
		if x.Title != "" || x.URL != "" {
			out = append(out, Source{Title: x.Title, URL: x.URL})
		}
	}
	return out
}

// sourceFromURL — URL dan manba: sarlavha = sayt nomi (+ yil); URL bo'lmasa matnli manba.
func sourceFromURL(url *string, year *int) (Source, bool) {
	if url == nil || strings.TrimSpace(*url) == "" {
		return Source{}, false
	}
	u := strings.TrimSpace(*url)
	if !strings.HasPrefix(u, "http") {
		return Source{Title: u}, true
	}
	title := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	title, _, _ = strings.Cut(title, "/")
	title = strings.TrimPrefix(title, "www.")
	if year != nil && *year > 0 {
		title += ", " + strconv.Itoa(*year)
	}
	return Source{Title: title, URL: u}, true
}

// ---------- marshrut ----------

// TripInfo — foydalanuvchi marshruti (handler tuzadi va egasini tekshiradi). get_trip shu ma'lumotni qaytaradi.
type TripInfo struct {
	ID         string        `json:"trip_id"`
	CityID     string        `json:"city"`
	Title      string        `json:"title"`
	StartDate  string        `json:"start_date"`
	StartLabel string        `json:"start_point,omitempty"` // "Hotel Bibikhanum" (startLabel)
	StartKind  string        `json:"start_kind,omitempty"`
	Days       []TripDay     `json:"days"`
	Extra      any           `json:"extra,omitempty"` // jami xarajat, shaharga yo'l (arrival), tayyorgarlik, sozlamalar
	Places     []store.Place `json:"-"`               // kartochkalar uchun
}

type TripDay struct {
	Day     int        `json:"day"` // 1 dan
	Date    string     `json:"date"`
	Weekday string     `json:"weekday"`
	CostUZS int64      `json:"cost_per_person_uzs"`
	Start   string     `json:"from_start_point,omitempty"` // "walk 12 min" — boshlanish nuqtasidan birinchi joygacha
	Stops   []TripStop `json:"stops"`
}

type TripStop struct {
	Arrive   string `json:"arrive"`
	Name     string `json:"name"`
	PlaceID  string `json:"place_id"`
	Kind     string `json:"kind"` // place | meal
	Minutes  int    `json:"duration_min"`
	Category string `json:"category,omitempty"`
	Getting  string `json:"getting_there,omitempty"` // "taxi 15 min, 18000 UZS"
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	New      bool   `json:"little_known,omitempty"`
}

// Brief — kontekst qatori uchun qisqa xulosa.
func (ti *TripInfo) Brief() string {
	var b strings.Builder
	fmt.Fprintf(&b, "trip_id=%s; trip=%q, city=%s, from %s", ti.ID, ti.Title, ti.CityID, ti.StartDate)
	if ti.StartLabel != "" {
		fmt.Fprintf(&b, ", start point %q", ti.StartLabel)
	}
	for _, d := range ti.Days {
		fmt.Fprintf(&b, "; day %d (%s %s):", d.Day, d.Date, d.Weekday)
		for i, s := range d.Stops {
			if i > 0 {
				b.WriteString(",")
			}
			name := s.Name
			if s.Kind == "meal" {
				name = "lunch at " + name
			}
			fmt.Fprintf(&b, " %s %s", s.Arrive, name)
			if s.Status == "visited" || s.Status == "skipped" {
				b.WriteString(" (" + s.Status + ")")
			}
		}
	}
	out := b.String()
	if r := []rune(out); len(r) > 1500 {
		out = string(r[:1500]) + "…"
	}
	return out
}

// Preload — marshrut joylari: javobda nomi tilga olinsa kartochka chiqadi (fallback rejimida ham faqat tilga olinsa).
func (t *Tools) Preload(places []store.Place) {
	if t.preload == nil {
		t.preload = map[string]bool{}
	}
	for _, p := range places {
		if _, ok := t.Found[p.ID]; !ok {
			t.preload[p.ID] = true
			t.remember(p)
		}
	}
}

// Preloaded — joy faqat marshrutdan qo'shilgan (asbob qaytarmagan).
func (t *Tools) Preloaded(id string) bool { return t.preload[id] }

func (t *Tools) GetTrip(context.Context) (any, error) {
	if t.Trip == nil {
		return map[string]string{"error": "no trip is attached to this chat; ask the user to open the chat from the plan"}, nil
	}
	return t.Trip, nil
}

// ---------- search_events ----------

type SearchEventsInput struct {
	City string `json:"city,omitempty"`
	From string `json:"from,omitempty"` // YYYY-MM-DD
	To   string `json:"to,omitempty"`
}

func (t *Tools) SearchEvents(ctx context.Context, in SearchEventsInput) (any, error) {
	now := t.Now().In(tashkent)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tashkent)
	if d, err := time.ParseInLocation(time.DateOnly, in.From, tashkent); err == nil && d.After(from) {
		from = d
	}
	to := from.AddDate(0, 0, 14)
	if d, err := time.ParseInLocation(time.DateOnly, in.To, tashkent); err == nil && !d.Before(from) {
		to = d.Add(24*time.Hour - time.Second)
	}
	if from.Before(now) {
		from = now
	}
	list, err := t.Store.Events(ctx, store.EventFilter{City: NormCity(in.City), From: from, To: &to})
	if err != nil {
		return nil, err
	}
	type ev struct {
		Title    string `json:"title"`
		Kind     string `json:"kind"`
		City     string `json:"city"`
		Venue    string `json:"venue,omitempty"`
		Starts   string `json:"starts"`
		Ends     string `json:"ends"`
		PriceUZS *int64 `json:"price_uzs,omitempty"`
		Free     bool   `json:"free"`
		Source   string `json:"source,omitempty"`
	}
	out := []ev{}
	for i, e := range list {
		if i == 10 {
			break
		}
		x := ev{Title: i18n.Pick(e.Title, t.Lang), Kind: e.Kind, City: e.CityID, Venue: i18n.Pick(e.Venue, t.Lang),
			Starts: e.StartsAt.In(tashkent).Format("2006-01-02 15:04 Mon"), Ends: e.EndsAt.In(tashkent).Format("2006-01-02 15:04"),
			PriceUZS: e.Price, Free: e.IsFree}
		if sr, ok := sourceFromURL(e.SourceURL, nil); ok {
			x.Source = sr.Title
			t.addSource(sr, "")
		}
		out = append(out, x)
	}
	return map[string]any{"count": len(out), "from": from.Format(time.DateOnly), "to": to.Format(time.DateOnly), "events": out}, nil
}

// ---------- get_intercity ----------

type GetIntercityInput struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (t *Tools) GetIntercity(ctx context.Context, in GetIntercityInput) (any, error) {
	from, to := NormCity(in.From), NormCity(in.To)
	if from == "" || to == "" {
		return map[string]string{"error": "from and to city ids required"}, nil
	}
	routes, err := t.Store.IntercityRoutes(ctx, from, to)
	if err != nil {
		return nil, err
	}
	type route struct {
		Mode        string  `json:"mode"`
		DurationMin int     `json:"duration_min"`
		PriceFrom   *int64  `json:"price_from_uzs,omitempty"`
		Estimate    bool    `json:"estimate"`
		Via         *string `json:"via,omitempty"`
		Updated     string  `json:"updated"`
		Source      string  `json:"source"`
	}
	out := []route{}
	for _, r := range routes {
		x := route{Mode: r.Mode, DurationMin: r.DurationMin, PriceFrom: r.PriceFrom, Estimate: r.Estimate, Via: r.Via,
			Updated: r.UpdatedAt, Source: r.Source}
		if sr, ok := sourceFromURL(r.SourceURL, nil); ok {
			if r.Source != "" {
				sr.Title = r.Source
			}
			t.addSource(sr, "")
		} else if r.Source != "" {
			t.addSource(Source{Title: r.Source}, "")
		}
		out = append(out, x)
	}
	return map[string]any{"from": from, "to": to, "count": len(out), "routes": out}, nil
}

// ---------- get_services ----------

type GetServicesInput struct {
	City  string         `json:"city,omitempty"`
	Near  *domain.LatLng `json:"near,omitempty"`
	Kinds []string       `json:"kinds,omitempty"`
}

var assistantServiceKinds = []string{"atm", "pharmacy", "exchange", "hospital", "clinic", "police", "embassy"}

func (t *Tools) GetServices(ctx context.Context, in GetServicesInput) (any, error) {
	city := NormCity(in.City)
	if city == "" && t.Trip != nil {
		city = t.Trip.CityID
	}
	if city == "" {
		return map[string]string{"error": "city required"}, nil
	}
	c, err := t.Store.City(ctx, city, i18n.Chain(t.Lang))
	if errors.Is(err, store.ErrNotFound) {
		return map[string]string{"error": "unknown city"}, nil
	}
	if err != nil {
		return nil, err
	}
	kinds := []string{}
	for _, k := range in.Kinds {
		if slices.Contains(assistantServiceKinds, k) {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 {
		kinds = []string{"atm", "pharmacy", "exchange"}
	}
	type svc struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Address   string `json:"address,omitempty"`
		Phone     string `json:"phone,omitempty"`
		Open24h   bool   `json:"open_24h"`
		DistanceM *int   `json:"distance_m,omitempty"`
		Demo      bool   `json:"demo_data"`
	}
	out := []svc{}
	for _, k := range kinds {
		f := store.ServiceFilter{City: city, Kinds: []string{k}, Near: in.Near, Limit: 2}
		if in.Near == nil {
			f.Near = &c.Center
		}
		list, err := t.Store.Services(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, v := range list {
			addr := i18n.Pick(v.Address, t.Lang)
			x := svc{Kind: k, Name: i18n.ServiceName(v.Kind, i18n.Pick(v.Name, t.Lang), addr, t.Lang), Address: addr,
				Open24h: v.Open24h, Demo: v.Source == "demo"}
			if v.Phone != nil {
				x.Phone = *v.Phone
			}
			if v.DistanceM != nil && in.Near != nil {
				d := int(*v.DistanceM)
				x.DistanceM = &d
			}
			out = append(out, x)
		}
	}
	nums := []map[string]string{}
	for _, k := range i18n.EmergencyKinds {
		if n := c.Emergency[k]; n != "" {
			nums = append(nums, map[string]string{"service": i18n.EmergencyLabel(k, t.Lang), "number": n})
		}
	}
	res := map[string]any{"city": city, "services": out, "emergency_numbers": nums}
	if in.Near == nil {
		res["note"] = "no user location: services are the ones nearest to the city centre"
	}
	return res, nil
}

// ---------- get_weather ----------

type GetWeatherInput struct {
	City string `json:"city,omitempty"`
	Date string `json:"date,omitempty"` // YYYY-MM-DD, bo'sh — bugun
}

func (t *Tools) GetWeather(ctx context.Context, in GetWeatherInput) (any, error) {
	if t.Weather == nil {
		return map[string]string{"error": "weather unavailable"}, nil
	}
	city := NormCity(in.City)
	if city == "" && t.Trip != nil {
		city = t.Trip.CityID
	}
	c, err := t.Store.City(ctx, city, []string{"en"})
	if errors.Is(err, store.ErrNotFound) || city == "" {
		return map[string]string{"error": "unknown city"}, nil
	}
	if err != nil {
		return nil, err
	}
	now := t.Now().In(tashkent)
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if d, err := time.Parse(time.DateOnly, in.Date); err == nil {
		date = d
	}
	if days := date.Sub(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24; days < 0 || days > 15 {
		return map[string]string{"error": "forecast is available only for today and the next 15 days"}, nil
	}
	f, err := t.Weather.Daily(ctx, c.Center, date, c.Timezone)
	if err != nil {
		return map[string]string{"error": "weather unavailable"}, nil
	}
	t.addSource(Source{Title: "Open-Meteo", URL: "https://open-meteo.com"}, "")
	return map[string]any{"city": city, "date": date.Format(time.DateOnly), "max_temp_c": f.MaxTempC, "precip_mm": f.PrecipMM,
		"precip_probability_pct": f.PrecipProb, "uncomfortable": f.Reason(), "source": "Open-Meteo forecast"}, nil
}
