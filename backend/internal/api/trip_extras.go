package api

// Safarga qo'shimchalar: "Qanday borasiz?" (trip.arrival) va "Safarga tayyorgarlik" (trip.prep), PATCH /trips/{id}.
// Yo'l narxi kishi boshiga estimatedCost ga qo'shiladi (sayt va ilova bilan bir xil: mashina guruhga bo'linadi).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"yoldosh/internal/domain"
	"yoldosh/internal/planner"
	"yoldosh/internal/store"
)

var (
	prepItems    = []string{"route", "transport", "esim", "sim", "insurance", "currency"}
	prepStatuses = []string{"done", "skipped"}
)

type arrivalDTO struct {
	From      string  `json:"from"`
	Mode      string  `json:"mode"`
	Choice    *string `json:"choice,omitempty"`    // cheapest | fastest | comfort | bus
	PriceFrom *int64  `json:"priceFrom,omitempty"` // mijozdagi narx nusxasi (so'm; car — mashina uchun), oflayn hisob uchun
}

type prepState map[string]string

func (s *Server) addArrival(r *http.Request, t store.Trip, req tripReq, out *tripDTO) error {
	out.Prep = prepState{}
	if len(t.Prep) > 0 {
		_ = json.Unmarshal(t.Prep, &out.Prep)
	}
	if len(t.Arrival) == 0 {
		return nil
	}
	var a arrivalDTO
	if err := json.Unmarshal(t.Arrival, &a); err != nil || a.From == "" {
		return nil
	}
	out.Arrival = &a
	routes, err := s.store.IntercityRoutes(r.Context(), a.From, t.CityID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(routes, func(x store.IntercityRoute) bool { return x.Mode == a.Mode })
	if i < 0 {
		return nil
	}
	group := 0
	if req.GroupSize != nil {
		group = *req.GroupSize
	}
	cost := arrivalCostPerPerson(routes[i], planner.GroupSizeFor(group, req.Companion))
	m := domain.UZS(cost)
	out.Totals.ArrivalCost = &m
	out.Totals.EstimatedCost.Amount += cost
	return nil
}

// patchTrip — PATCH /trips/{id} {arrival?, prep?}. null — tozalash; prep to'liq almashtiriladi.
func (s *Server) patchTrip(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return
	}
	var body map[string]json.RawMessage
	if !decodeJSON(w, r, &body) {
		return
	}
	fe := fieldErrors{}
	var p store.TripExtrasPatch
	for k := range body {
		if k != "arrival" && k != "prep" {
			fe.add(k, "unknown field (arrival | prep)")
		}
	}
	ctx := r.Context()
	if raw, ok := body["arrival"]; ok {
		p.SetArrival = true
		if !isNull(raw) {
			var a arrivalDTO
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&a); err != nil {
				fe.add("arrival", err.Error())
			} else {
				switch {
				case a.From == "" || a.From == t.CityID:
					fe.add("arrival.from", "a city other than the trip city")
				case !slices.Contains(transportModes, a.Mode):
					fe.add("arrival.mode", strings.Join(transportModes, " | "))
				case a.Choice != nil && !slices.Contains(arrivalChoices, *a.Choice):
					fe.add("arrival.choice", strings.Join(arrivalChoices, " | "))
				case a.PriceFrom != nil && *a.PriceFrom < 0:
					fe.add("arrival.priceFrom", ">= 0")
				default:
					if _, err := s.store.City(ctx, a.From, []string{"en"}); isNotFound(err) {
						fe.add("arrival.from", "unknown city")
					} else if err != nil {
						s.internalError(w, r, err)
						return
					} else if routes, err := s.store.IntercityRoutes(ctx, a.From, t.CityID); err != nil {
						s.internalError(w, r, err)
						return
					} else if len(routes) > 0 && !slices.ContainsFunc(routes, func(x store.IntercityRoute) bool { return x.Mode == a.Mode }) {
						fe.add("arrival.mode", "no such route between these cities")
					}
				}
				p.Arrival, _ = json.Marshal(a)
			}
		}
	}
	if raw, ok := body["prep"]; ok {
		p.SetPrep = true
		if !isNull(raw) {
			var st prepState
			if err := json.Unmarshal(raw, &st); err != nil {
				fe.add("prep", "object {item: done | skipped}")
			} else {
				for k, v := range st {
					if !slices.Contains(prepItems, k) {
						fe.add("prep."+k, "unknown item ("+strings.Join(prepItems, " | ")+")")
					} else if !slices.Contains(prepStatuses, v) {
						fe.add("prep."+k, "done | skipped")
					}
				}
				if len(st) > 0 {
					p.Prep, _ = json.Marshal(st)
				}
			}
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	if err := s.store.UpdateTripExtras(ctx, t.ID, p); err != nil {
		s.internalError(w, r, err)
		return
	}
	full, err := s.store.Trip(ctx, t.ID, userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeTrip(w, r, http.StatusOK, full)
}

func isNull(raw json.RawMessage) bool { return strings.TrimSpace(string(raw)) == "null" }
