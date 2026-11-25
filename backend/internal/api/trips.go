package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/planner"
	"yoldosh/internal/store"
)

// tripReq — POST /trips tanasi (API_CONTRACT.md §5). trips.prefs ga aynan shu shaklda saqlanadi.
type tripReq struct {
	CityID        string         `json:"cityId"`
	StartDate     string         `json:"startDate"`
	Days          int            `json:"days"`
	Interests     []string       `json:"interests"`
	Companion     string         `json:"companion,omitempty"`
	Pace          string         `json:"pace,omitempty"`
	Budget        int            `json:"budget,omitempty"`
	StartLocation *domain.LatLng `json:"startLocation,omitempty"`
	Constraints   struct {
		Wheelchair bool `json:"wheelchair"`
		WithKids   bool `json:"withKids"`
		Vegetarian bool `json:"vegetarian"`
	} `json:"constraints"`
}

const maxTripDays = 7

func (req *tripReq) validate(now time.Time) (time.Time, fieldErrors) {
	fe := fieldErrors{}
	if req.CityID == "" {
		fe.add("cityId", "required")
	}
	start, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		fe.add("startDate", "YYYY-MM-DD")
	} else if start.Before(now.AddDate(0, 0, -1).Truncate(24*time.Hour)) || start.After(now.AddDate(1, 0, 0)) {
		fe.add("startDate", "today .. +1 year")
	}
	if req.Days < 1 || req.Days > maxTripDays {
		fe.add("days", "1.."+strconv.Itoa(maxTripDays))
	}
	if !validInterests(req.Interests) {
		fe.add("interests", "unknown interest")
	}
	if req.Companion != "" && !slices.Contains(companions, req.Companion) {
		fe.add("companion", strings.Join(companions, " | "))
	}
	if req.Pace == "" {
		req.Pace = "balanced"
	}
	if !slices.Contains(paces, req.Pace) {
		fe.add("pace", strings.Join(paces, " | "))
	}
	if req.Budget < 0 || req.Budget > 3 {
		fe.add("budget", "1..3")
	}
	if req.StartLocation != nil && !req.StartLocation.Valid() {
		fe.add("startLocation", "invalid coordinates")
	}
	return start, fe
}

func (req tripReq) prefs(start time.Time) planner.Prefs {
	return planner.Prefs{Days: req.Days, StartDate: start, Interests: req.Interests, Pace: req.Pace, Budget: req.Budget,
		Start: req.StartLocation, Wheelchair: req.Constraints.Wheelchair, WithKids: req.Constraints.WithKids || req.Companion == "family"}
}

func plannerCity(c store.City, avg float64) planner.City {
	return planner.City{Center: c.Center, TaxiBase: c.TaxiBase, TaxiPerKm: c.TaxiPerKm, AvgRating: avg}
}

func (s *Server) createTrip(w http.ResponseWriter, r *http.Request) {
	var req tripReq
	if !decodeJSON(w, r, &req) {
		return
	}
	start, fe := req.validate(s.now())
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	ctx := r.Context()
	city, err := s.store.City(ctx, req.CityID, i18n.Chain(langOf(r)))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "city_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	cands, avg, err := s.store.Candidates(ctx, city.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	days, err := planner.Plan(req.prefs(start), plannerCity(city, avg), cands)
	if errors.Is(err, planner.ErrImpossible) {
		writeError(w, r, http.StatusUnprocessableEntity, "route_impossible", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if v := domain.Validate(days); len(v) > 0 {
		// Shahar bazasida yangi joy yetarli bo'lmasa bo'lishi mumkin — marshrut baribir beriladi, lekin kuzatiladi.
		slog.Warn("trip invariants", "city", city.ID, "violations", v)
	}

	prefsJSON, _ := json.Marshal(req)
	share, _ := domain.NoveltyShare(days)
	t := &store.Trip{ID: uuid.Must(uuid.NewV7()), UserID: userOf(r), CityID: city.ID, Status: "ready",
		StartDate: start, Prefs: prefsJSON, NoveltyShare: share, Planner: "fallback"}
	for _, d := range days {
		td := store.TripDay{Index: d.Index, Date: start.AddDate(0, 0, d.Index)}
		for _, ps := range d.Stops {
			td.Stops = append(td.Stops, store.Stop{ID: uuid.Must(uuid.NewV7()), Status: "upcoming", PlanStop: ps})
		}
		t.Days = append(t.Days, td)
	}
	if err := s.store.CreateTrip(ctx, t); err != nil {
		s.internalError(w, r, err)
		return
	}
	full, err := s.store.Trip(ctx, t.ID, t.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeTrip(w, r, http.StatusCreated, full)
}

// ---------- javob ----------

type tripDTO struct {
	ID           string       `json:"id"`
	CityID       string       `json:"cityId"`
	Title        string       `json:"title"`
	Status       string       `json:"status"`
	StartDate    string       `json:"startDate"`
	Summary      string       `json:"summary"`
	NoveltyShare float64      `json:"noveltyShare"`
	Days         []tripDayDTO `json:"days"`
	Totals       tripTotals   `json:"totals"`
}

type tripDayDTO struct {
	Index int       `json:"index"`
	Date  string    `json:"date"`
	Title string    `json:"title"`
	Stops []stopDTO `json:"stops"`
}

type stopDTO struct {
	ID              string           `json:"id"`
	Kind            domain.StopKind  `json:"kind"`
	Place           *placeSummaryDTO `json:"place,omitempty"`
	ArriveAt        string           `json:"arriveAt"`
	DurationMin     int              `json:"durationMin"`
	Reason          string           `json:"reason"`
	TransitFromPrev *domain.Transit  `json:"transitFromPrev,omitempty"`
	Status          string           `json:"status"`
}

type tripTotals struct {
	Places        int          `json:"places"`
	NewPlaces     int          `json:"newPlaces"`
	WalkingM      int          `json:"walkingM"`
	EstimatedCost domain.Money `json:"estimatedCost"` // hozircha faqat taksi; kirish chiptalari narx moduli bilan qo'shiladi
}

func reasonText(st store.Stop, lang string) string {
	if st.ReasonCode == "ai" && st.ReasonText != "" {
		return st.ReasonText
	}
	if st.ReasonCode == "interest" {
		return i18n.T(lang, "reason.interest", i18n.T(lang, "interest."+st.ReasonArg))
	}
	return i18n.T(lang, "reason."+st.ReasonCode)
}

func (s *Server) writeTrip(w http.ResponseWriter, r *http.Request, status int, t store.Trip) {
	dto, err := s.buildTrip(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, status, dto)
}

func (s *Server) buildTrip(r *http.Request, t store.Trip) (tripDTO, error) {
	lang := langOf(r)
	chain := i18n.Chain(lang)
	city, err := s.store.City(r.Context(), t.CityID, chain)
	if err != nil {
		return tripDTO{}, err
	}
	var ids []string
	for _, d := range t.Days {
		for _, st := range d.Stops {
			ids = append(ids, st.PlaceID)
		}
	}
	places, err := s.store.PlacesByIDs(r.Context(), ids, chain)
	if err != nil {
		return tripDTO{}, err
	}
	out := tripDTO{ID: t.ID.String(), CityID: t.CityID, Status: t.Status, StartDate: t.StartDate.Format(time.DateOnly),
		NoveltyShare: round2(t.NoveltyShare), Days: []tripDayDTO{}, Totals: tripTotals{EstimatedCost: domain.UZS(0)}}
	if len(t.Days) == 1 {
		out.Title = i18n.T(lang, "trip_title_1", city.Name)
	} else {
		out.Title = i18n.T(lang, "trip_title", city.Name, strconv.Itoa(len(t.Days)))
	}
	for _, d := range t.Days {
		dd := tripDayDTO{Index: d.Index, Date: d.Date.Format(time.DateOnly), Title: i18n.T(lang, "day_title", strconv.Itoa(d.Index+1)), Stops: []stopDTO{}}
		for _, st := range d.Stops {
			sd := stopDTO{ID: st.ID.String(), Kind: st.Kind, ArriveAt: domain.FormatHHMM(st.ArriveMin), DurationMin: st.DurationMin,
				Reason: reasonText(st, lang), TransitFromPrev: st.Transit, Status: st.Status}
			if p, ok := places[st.PlaceID]; ok {
				ps := s.summaryDTO(p, nil)
				sd.Place = &ps
			}
			if st.Kind == domain.KindPlace {
				out.Totals.Places++
				if st.CountsForNovelty() {
					out.Totals.NewPlaces++
				}
			}
			if st.Transit != nil {
				if st.Transit.Mode == "walk" {
					out.Totals.WalkingM += st.Transit.DistanceM
				}
				if st.Transit.Fare != nil {
					out.Totals.EstimatedCost.Amount += st.Transit.Fare.Amount
				}
			}
			dd.Stops = append(dd.Stops, sd)
		}
		out.Days = append(out.Days, dd)
	}
	if t.Summary != nil && *t.Summary != "" {
		out.Summary = *t.Summary
	} else {
		out.Summary = i18n.T(lang, "trip_summary", strconv.Itoa(out.Totals.Places), strconv.Itoa(out.Totals.NewPlaces))
	}
	return out, nil
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// ---------- o'qish ----------

func (s *Server) loadTrip(w http.ResponseWriter, r *http.Request) (store.Trip, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "trip_not_found", false)
		return store.Trip{}, false
	}
	t, err := s.store.Trip(r.Context(), id, userOf(r))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "trip_not_found", false)
		return t, false
	}
	if err != nil {
		s.internalError(w, r, err)
		return t, false
	}
	return t, true
}

func (s *Server) getTrip(w http.ResponseWriter, r *http.Request) {
	if t, ok := s.loadTrip(w, r); ok {
		s.writeTrip(w, r, http.StatusOK, t)
	}
}

type tripListDTO struct {
	ID           string  `json:"id"`
	CityID       string  `json:"cityId"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	StartDate    string  `json:"startDate"`
	Days         int     `json:"days"`
	NoveltyShare float64 `json:"noveltyShare"`
	CreatedAt    string  `json:"createdAt"`
}

func (s *Server) listTrips(w http.ResponseWriter, r *http.Request) {
	lang := langOf(r)
	items, err := s.store.Trips(r.Context(), userOf(r), 50)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	names := map[string]string{}
	if cities, err := s.store.Cities(r.Context(), i18n.Chain(lang)); err == nil {
		for _, c := range cities {
			names[c.ID] = c.Name
		}
	}
	out := make([]tripListDTO, 0, len(items))
	for _, it := range items {
		title := i18n.T(lang, "trip_title", names[it.CityID], strconv.Itoa(it.Days))
		if it.Days == 1 {
			title = i18n.T(lang, "trip_title_1", names[it.CityID])
		}
		out = append(out, tripListDTO{ID: it.ID.String(), CityID: it.CityID, Title: title, Status: it.Status,
			StartDate: it.StartDate.Format(time.DateOnly), Days: it.Days, NoveltyShare: round2(it.NoveltyShare),
			CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- tahrirlash ----------

// tripContext — kunni qayta hisoblash uchun kerakli narsalar.
type tripContext struct {
	prefs tripReq
	city  planner.City
	byID  map[string]planner.Candidate
	cands []planner.Candidate
}

func (s *Server) tripCtx(r *http.Request, t store.Trip) (tripContext, error) {
	var tc tripContext
	if err := json.Unmarshal(t.Prefs, &tc.prefs); err != nil {
		return tc, err
	}
	city, err := s.store.City(r.Context(), t.CityID, []string{"en"})
	if err != nil {
		return tc, err
	}
	cands, avg, err := s.store.Candidates(r.Context(), t.CityID)
	if err != nil {
		return tc, err
	}
	tc.city, tc.cands = plannerCity(city, avg), cands
	tc.byID = make(map[string]planner.Candidate, len(cands))
	for _, c := range cands {
		tc.byID[c.ID] = c
	}
	return tc, nil
}

// saveDay — kun vaqtlarini qayta hisoblaydi, saqlaydi va yangilangan marshrutni qaytaradi.
func (s *Server) saveDay(w http.ResponseWriter, r *http.Request, t store.Trip, tc tripContext, day int, stops []store.Stop) {
	plan := make([]domain.PlanStop, len(stops))
	for i, st := range stops {
		plan[i] = st.PlanStop
	}
	plan = planner.Retime(plan, int(t.Days[day].Date.Weekday()), tc.prefs.StartLocation, tc.city, tc.byID)
	for i := range stops {
		stops[i].PlanStop = plan[i]
	}
	t.Days[day].Stops = stops
	share, _ := domain.NoveltyShare(t.PlanDays())
	if err := s.store.ReplaceDayStops(r.Context(), t.ID, day, stops, share); err != nil {
		s.internalError(w, r, err)
		return
	}
	full, err := s.store.Trip(r.Context(), t.ID, t.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeTrip(w, r, http.StatusOK, full)
}

func findStop(t store.Trip, id string) (day, idx int, ok bool) {
	sid, err := uuid.Parse(id)
	if err != nil {
		return 0, 0, false
	}
	for d, dd := range t.Days {
		for i, st := range dd.Stops {
			if st.ID == sid {
				return d, i, true
			}
		}
	}
	return 0, 0, false
}

func tripHasPlace(t store.Trip, placeID string) bool {
	for _, d := range t.Days {
		for _, st := range d.Stops {
			if st.PlaceID == placeID {
				return true
			}
		}
	}
	return false
}

type addStopReq struct {
	DayIndex int     `json:"dayIndex"`
	PlaceID  string  `json:"placeId"`
	EventID  string  `json:"eventId"`
	After    *string `json:"after"`
}

func (s *Server) addStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	var req addStopReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if req.DayIndex < 0 || req.DayIndex >= len(t.Days) {
		fe.add("dayIndex", "0.."+strconv.Itoa(len(t.Days)-1))
	}
	if req.EventID != "" {
		fe.add("eventId", "events are not available yet")
	}
	if req.PlaceID == "" {
		fe.add("placeId", "required")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	c, ok := tc.byID[req.PlaceID]
	if !ok {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if tripHasPlace(t, c.ID) {
		writeError(w, r, http.StatusConflict, "place_already_in_trip", false)
		return
	}
	kind := domain.KindPlace
	if c.IsMeal() {
		kind = domain.KindMeal
	}
	stops := slices.Clone(t.Days[req.DayIndex].Stops)
	pos := len(stops)
	if req.After != nil {
		d, i, ok := findStop(t, *req.After)
		if !ok || d != req.DayIndex {
			writeError(w, r, http.StatusNotFound, "stop_not_found", false)
			return
		}
		pos = i + 1
	}
	ns := store.Stop{ID: uuid.Must(uuid.NewV7()), Status: "upcoming", PlanStop: domain.PlanStop{
		Kind: kind, PlaceID: c.ID, Novelty: c.Novelty, IsPartner: c.IsPartner, DurationMin: c.VisitMin, ReasonCode: "user_added"}}
	stops = slices.Insert(stops, pos, ns)
	s.saveDay(w, r, t, tc, req.DayIndex, stops)
}

// deleteStop — foydalanuvchi oxirgi yangi joyni ham o'chira oladi (uning erki); kvota faqat generatsiyada majburiy.
func (s *Server) deleteStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	d, i, ok := findStop(t, r.PathValue("stopId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, "stop_not_found", false)
		return
	}
	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	stops := slices.Delete(slices.Clone(t.Days[d].Stops), i, i+1)
	s.saveDay(w, r, t, tc, d, stops)
}

func (s *Server) swapStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	d, i, ok := findStop(t, r.PathValue("stopId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, "stop_not_found", false)
		return
	}
	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	target, ok := tc.byID[t.Days[d].Stops[i].PlaceID]
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"alternatives": []placeSummaryDTO{}})
		return
	}
	exclude := map[string]bool{}
	for _, dd := range t.Days {
		for _, st := range dd.Stops {
			exclude[st.PlaceID] = true
		}
	}
	alts := planner.Alternatives(target, tc.cands, exclude, int(t.Days[d].Date.Weekday()), 5)
	ids := make([]string, len(alts))
	for k, a := range alts {
		ids[k] = a.ID
	}
	places, err := s.store.PlacesByIDs(r.Context(), ids, i18n.Chain(langOf(r)))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]placeSummaryDTO, 0, len(ids))
	for _, id := range ids {
		if p, ok := places[id]; ok {
			out = append(out, s.summaryDTO(p, nil))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"alternatives": out})
}
