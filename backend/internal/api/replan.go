package api

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/planner"
	"yoldosh/internal/store"
	"yoldosh/internal/weather"
)

type replanReq struct {
	Reason   string         `json:"reason"` // late | tired | weather | free_time
	Minutes  *int           `json:"minutes"`
	Location *domain.LatLng `json:"location"`
	DayIndex *int           `json:"dayIndex"` // bo'lmasa — bugungi kun
}

type replanResp struct {
	tripDTO
	Changes []string `json:"changes"`
}

func (s *Server) replanTrip(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	var req replanReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	switch req.Reason {
	case "late":
		if req.Minutes == nil || *req.Minutes < 5 || *req.Minutes > 240 {
			fe.add("minutes", "5..240")
		}
	case "free_time":
		if req.Minutes == nil || *req.Minutes < 30 || *req.Minutes > 480 {
			fe.add("minutes", "30..480")
		}
		if req.Location == nil {
			fe.add("location", "required")
		}
	case "tired", "weather":
	default:
		fe.add("reason", "late | tired | weather | free_time")
	}
	if req.Location != nil && !req.Location.Valid() {
		fe.add("location", "invalid coordinates")
	}
	if req.DayIndex != nil && (*req.DayIndex < 0 || *req.DayIndex >= len(t.Days)) {
		fe.add("dayIndex", "0.."+strconv.Itoa(len(t.Days)-1))
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}

	ctx := r.Context()
	city, err := s.store.City(ctx, t.CityID, []string{"en"})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	loc := tz(city.Timezone)
	now := s.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	day := -1
	if req.DayIndex != nil {
		day = *req.DayIndex
	} else {
		for i, d := range t.Days {
			if d.Date.Equal(today) {
				day = i
			}
		}
		if day < 0 && t.Days[0].Date.After(today) {
			day = 0 // sayohat hali boshlanmagan — birinchi kun
		}
	}
	if day < 0 || t.Days[day].Date.Before(today) {
		writeError(w, r, http.StatusUnprocessableEntity, "day_in_past", false)
		return
	}
	nowMin := 0
	isToday := t.Days[day].Date.Equal(today)
	if isToday {
		nowMin = now.Hour()*60 + now.Minute()
	}

	stops := t.Days[day].Stops
	fixed := 0
	for fixed < len(stops) {
		st := stops[fixed]
		if st.Status == "visited" || st.Status == "skipped" || isToday && st.EndMin() <= nowMin {
			fixed++
			continue
		}
		break
	}

	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	rp := planner.NewReplanner(tc.prefs.prefs(t.StartDate), tc.city, tc.cands)
	plan := make([]domain.PlanStop, len(stops))
	for i, st := range stops {
		plan[i] = st.PlanStop
	}
	exclude := map[string]bool{}
	for _, d := range t.Days {
		for _, st := range d.Stops {
			exclude[st.PlaceID] = true
		}
	}
	wd := int(t.Days[day].Date.Weekday())
	lang := langOf(r)

	var out []domain.PlanStop
	var changes []planner.Change
	var extra []string
	switch req.Reason {
	case "late":
		out, changes = rp.Late(plan, fixed, wd, nowMin, *req.Minutes, req.Location)
	case "tired":
		out, changes = rp.Tired(plan, fixed, wd, nowMin, req.Location)
	case "free_time":
		out, changes = rp.FreeTime(plan, fixed, wd, nowMin, *req.Minutes, *req.Location, exclude)
		if len(changes) == 0 {
			extra = append(extra, i18n.T(lang, "change.none_nearby"))
		}
	case "weather":
		f, err := s.weather.Daily(ctx, city.Center, t.Days[day].Date, city.Timezone)
		if errors.Is(err, weather.ErrUnavailable) {
			writeError(w, r, http.StatusServiceUnavailable, "weather_unavailable", true)
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if !f.Bad() {
			out = plan
			extra = append(extra, i18n.T(lang, "change.weather_ok", strconv.Itoa(int(f.MaxTempC+0.5))))
		} else {
			out, changes = rp.Weather(plan, fixed, wd, nowMin, exclude, req.Location)
			if len(changes) == 0 {
				extra = append(extra, i18n.T(lang, "change.weather_indoor_already"))
			} else {
				extra = append(extra, i18n.T(lang, "change.weather_"+f.Reason(), strconv.Itoa(int(f.MaxTempC+0.5))))
			}
		}
	}
	planner.SortChanges(changes)

	// Saqlanib qolgan to'xtashlar ID va holatini saqlaydi; yangilariga yangi ID.
	prev := map[string]store.Stop{}
	for _, st := range stops {
		prev[st.PlaceID] = st
	}
	newStops := make([]store.Stop, len(out))
	for i, ps := range out {
		if old, ok := prev[ps.PlaceID]; ok {
			newStops[i] = store.Stop{ID: old.ID, Status: old.Status, PlanStop: ps}
		} else {
			newStops[i] = store.Stop{ID: uuid.Must(uuid.NewV7()), Status: "upcoming", PlanStop: ps}
		}
	}
	if len(changes) > 0 {
		t.Days[day].Stops = newStops
		share, _ := domain.NoveltyShare(t.PlanDays())
		if err := s.store.ReplaceDayStops(ctx, t.ID, day, newStops, share); err != nil {
			s.internalError(w, r, err)
			return
		}
	}

	texts, err := s.changeTexts(r, changes)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	full, err := s.store.Trip(ctx, t.ID, t.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	dto, err := s.buildTrip(r, full)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, replanResp{tripDTO: dto, Changes: append(extra, texts...)})
}

// changeTexts — o'zgarishlarni foydalanuvchi tilidagi jumlalarga aylantiradi.
func (s *Server) changeTexts(r *http.Request, changes []planner.Change) ([]string, error) {
	lang := langOf(r)
	var ids []string
	for _, c := range changes {
		ids = append(ids, c.PlaceID, c.NewPlaceID)
	}
	places, err := s.store.PlacesByIDs(r.Context(), ids, i18n.Chain(lang))
	if err != nil {
		return nil, err
	}
	name := func(id string) string {
		if p, ok := places[id]; ok {
			return p.Name
		}
		return id
	}
	out := []string{}
	for _, c := range changes {
		switch c.Kind {
		case "shifted":
			out = append(out, i18n.T(lang, "change.shifted", strconv.Itoa(c.Minutes)))
		case "replaced_weather":
			out = append(out, i18n.T(lang, "change.replaced_weather", name(c.PlaceID), name(c.NewPlaceID)))
		default:
			out = append(out, i18n.T(lang, "change."+c.Kind, name(c.PlaceID)))
		}
	}
	return out, nil
}

var stopStatuses = []string{"upcoming", "current", "visited", "skipped"}

type patchStopReq struct {
	Status  *string `json:"status"`  // upcoming | current | visited | skipped
	Rating  *int    `json:"rating"`  // 1-5: tashrifdan keyingi baho (visits ga ham yoziladi)
	PlaceID *string `json:"placeId"` // joyni almashtirish (to'xtash o'rni va id si saqlanadi)
}

// patchStop — to'xtash holati (bordim / o'tkazib yubordim), bahosi yoki joyini almashtirish.
// Kundalik va qayta rejalash holatga tayanadi. Javob — butun Trip.
func (s *Server) patchStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	var req patchStopReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	switch {
	case req.Status == nil && req.Rating == nil && req.PlaceID == nil:
		fe.add("body", "status, rating or placeId required")
	case req.PlaceID != nil && (req.Status != nil || req.Rating != nil):
		fe.add("placeId", "cannot be combined with status or rating")
	}
	if req.Status != nil && !slices.Contains(stopStatuses, *req.Status) {
		fe.add("status", "upcoming | current | visited | skipped")
	}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		fe.add("rating", "1..5")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	d, i, ok := findStop(t, r.PathValue("stopId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, "stop_not_found", false)
		return
	}
	st := t.Days[d].Stops[i]
	if req.PlaceID != nil {
		s.replaceStopPlace(w, r, t, d, i, *req.PlaceID)
		return
	}
	ctx := r.Context()
	if req.Status != nil {
		if err := s.store.SetStopStatus(ctx, t.ID, st.ID, *req.Status); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	if req.Rating != nil {
		if err := s.store.SetStopRating(ctx, t.ID, st.ID, *req.Rating); err != nil {
			s.internalError(w, r, err)
			return
		}
		// Baho joy reytingini ham shakllantiradi (yangi joylarniki ayniqsa).
		if err := s.store.AddVisit(ctx, t.UserID, st.PlaceID, &t.ID, req.Rating, nil); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	full, err := s.store.Trip(ctx, t.ID, t.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeTrip(w, r, http.StatusOK, full)
}

// replaceStopPlace — to'xtashdagi joyni boshqasiga almashtiradi (masalan /swap takliflaridan biri).
func (s *Server) replaceStopPlace(w http.ResponseWriter, r *http.Request, t store.Trip, d, i int, placeID string) {
	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	c, ok := tc.byID[placeID]
	if !ok {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	stops := slices.Clone(t.Days[d].Stops)
	if c.ID != stops[i].PlaceID && tripHasPlace(t, c.ID) {
		writeError(w, r, http.StatusConflict, "place_already_in_trip", false)
		return
	}
	kind := domain.KindPlace
	if c.IsMeal() {
		kind = domain.KindMeal
	}
	code, arg := planner.ReasonFor(c, kind, tc.prefs.prefs(t.StartDate))
	stops[i] = store.Stop{ID: stops[i].ID, Status: "upcoming", PlanStop: domain.PlanStop{Kind: kind, PlaceID: c.ID,
		Novelty: c.Novelty, IsPartner: c.IsPartner, DurationMin: c.VisitMin, ReasonCode: code, ReasonArg: arg}}
	s.saveDay(w, r, t, tc, d, stops)
}

type moveStopReq struct {
	DayIndex *int `json:"dayIndex"` // qaysi kunga (bo'lmasa — shu kun ichida)
	Position *int `json:"position"` // kun ichidagi o'rin, 0 dan; bo'lmasa — oxiriga
	Step     *int `json:"step"`     // -1 | 1: bir pog'ona yuqoriga / pastga
}

// moveStop — to'xtashni boshqa kunga ko'chirish yoki kun ichida tartibini o'zgartirish. Javob — butun Trip.
func (s *Server) moveStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	var req moveStopReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	switch {
	case req.Step != nil && (req.DayIndex != nil || req.Position != nil):
		fe.add("step", "cannot be combined with dayIndex or position")
	case req.Step != nil && *req.Step != -1 && *req.Step != 1:
		fe.add("step", "-1 | 1")
	case req.Step == nil && req.DayIndex == nil && req.Position == nil:
		fe.add("body", "dayIndex, position or step required")
	}
	if req.DayIndex != nil && (*req.DayIndex < 0 || *req.DayIndex >= len(t.Days)) {
		fe.add("dayIndex", "0.."+strconv.Itoa(len(t.Days)-1))
	}
	if req.Position != nil && *req.Position < 0 {
		fe.add("position", ">= 0")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	from, i, ok := findStop(t, r.PathValue("stopId"))
	if !ok {
		writeError(w, r, http.StatusNotFound, "stop_not_found", false)
		return
	}
	src := slices.Clone(t.Days[from].Stops)
	st := src[i]
	to := from
	if req.DayIndex != nil {
		to = *req.DayIndex
	}
	var pos int
	if req.Step != nil {
		pos = i + *req.Step
		if pos < 0 || pos >= len(src) {
			writeValidation(w, r, fieldErrors{"step": "stop is already at the edge of the day"})
			return
		}
	}
	src = slices.Delete(src, i, i+1)
	dst := src
	if to != from {
		dst = slices.Clone(t.Days[to].Stops)
		st.Status = "upcoming" // boshqa kunga ko'chgan joy hali borilmagan
	}
	if req.Step == nil {
		pos = len(dst)
		if req.Position != nil {
			pos = min(*req.Position, len(dst))
		}
	}
	dst = slices.Insert(dst, pos, st)

	tc, err := s.tripCtx(r, t)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	changed := map[int][]store.Stop{to: dst}
	if to != from {
		changed[from] = src
	}
	for day, stops := range changed {
		plan := make([]domain.PlanStop, len(stops))
		for k, x := range stops {
			plan[k] = x.PlanStop
		}
		plan = planner.RetimeAt(plan, int(t.Days[day].Date.Weekday()), tc.prefs.dayStartMin(day), tc.prefs.StartLocation, tc.city, tc.byID)
		for k := range stops {
			stops[k].PlanStop = plan[k]
		}
		t.Days[day].Stops = stops
	}
	share, _ := domain.NoveltyShare(t.PlanDays())
	if err := s.store.ReplaceDays(r.Context(), t.ID, changed, share); err != nil {
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
