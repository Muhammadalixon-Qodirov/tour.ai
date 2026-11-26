package api

import (
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// ---------- Saqlangan joylar ----------

func (s *Server) listSaved(w http.ResponseWriter, r *http.Request) {
	ids, err := s.store.SavedPlaceIDs(r.Context(), userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out, err := s.placesFor(r, ids)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) setSaved(saved bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("placeId")
		if saved {
			p, err := s.store.Place(r.Context(), id, []string{"en"})
			if isNotFound(err) || err == nil && p.Status != "published" {
				writeError(w, r, http.StatusNotFound, "place_not_found", false)
				return
			}
			if err != nil {
				s.internalError(w, r, err)
				return
			}
			err = s.store.SavePlaceFor(r.Context(), userOf(r), id)
			if err != nil {
				s.internalError(w, r, err)
				return
			}
		} else if err := s.store.UnsavePlace(r.Context(), userOf(r), id); err != nil {
			s.internalError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---------- Guruh safari ----------

type groupMemberDTO struct {
	UserID      string   `json:"userId"`
	DisplayName string   `json:"displayName"`
	Role        string   `json:"role"`
	IsMe        bool     `json:"isMe"`
	Budget      *int     `json:"budget"`
	Interests   []string `json:"interests"`
	Pace        *string  `json:"pace"`
}

type groupVoteDTO struct {
	Place  *placeSummaryDTO `json:"place"`
	Up     int              `json:"up"`
	Down   int              `json:"down"`
	MyVote int              `json:"myVote"`
}

type groupDTO struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	CityID       *string          `json:"cityId"`
	InviteCode   string           `json:"inviteCode"`
	IsOwner      bool             `json:"isOwner"`
	MembersCount int              `json:"membersCount"`
	Members      []groupMemberDTO `json:"members,omitempty"`
	Votes        []groupVoteDTO   `json:"votes,omitempty"`
	TripIDs      []string         `json:"tripIds,omitempty"`
	CreatedAt    string           `json:"createdAt"`
}

func (s *Server) groupDTO(r *http.Request, g store.Group, full bool) (groupDTO, error) {
	me := userOf(r)
	d := groupDTO{ID: g.ID.String(), Name: g.Name, CityID: g.CityID, InviteCode: g.InviteCode, IsOwner: g.OwnerID == me,
		MembersCount: len(g.Members), CreatedAt: g.CreatedAt.UTC().Format(time.RFC3339)}
	if !full {
		return d, nil
	}
	guest := map[string]string{"uz": "Sayohatchi", "ru": "Путешественник", "en": "Traveller"}[langOf(r)]
	d.Members = []groupMemberDTO{}
	for _, m := range g.Members {
		name := guest
		if m.DisplayName != nil && *m.DisplayName != "" {
			name = *m.DisplayName
		}
		d.Members = append(d.Members, groupMemberDTO{UserID: m.UserID, DisplayName: name, Role: m.Role, IsMe: m.UserID == me,
			Budget: m.Budget, Interests: nonNil(m.Interests), Pace: m.Pace})
	}
	ids := make([]string, len(g.Votes))
	for i, v := range g.Votes {
		ids[i] = v.PlaceID
	}
	places, err := s.store.PlacesByIDs(r.Context(), ids, i18n.Chain(langOf(r)))
	if err != nil {
		return d, err
	}
	d.Votes = []groupVoteDTO{}
	for _, v := range g.Votes {
		vd := groupVoteDTO{Up: v.Up, Down: v.Down, MyVote: v.MyVote}
		if p, ok := places[v.PlaceID]; ok {
			ps := s.summaryDTO(p, nil)
			vd.Place = &ps
		}
		d.Votes = append(d.Votes, vd)
	}
	d.TripIDs = []string{}
	for _, t := range g.TripIDs {
		d.TripIDs = append(d.TripIDs, t.String())
	}
	return d, nil
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string  `json:"name"`
		CityID *string `json:"cityId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !runeLen(req.Name, 2, 60) {
		writeValidation(w, r, fieldErrors{"name": "2..60 chars"})
		return
	}
	if req.CityID != nil {
		if _, err := s.store.City(r.Context(), *req.CityID, []string{"en"}); err != nil {
			writeValidation(w, r, fieldErrors{"cityId": "unknown city"})
			return
		}
	}
	id, err := s.store.CreateGroup(r.Context(), userOf(r), req.Name, req.CityID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.respondGroup(w, r, http.StatusCreated, id)
}

func (s *Server) respondGroup(w http.ResponseWriter, r *http.Request, status int, id uuid.UUID) {
	g, err := s.store.Group(r.Context(), id, userOf(r))
	if errors.Is(err, store.ErrNotMember) || isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	d, err := s.groupDTO(r, g, true)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, status, d)
}

func groupID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return id, false
	}
	return id, true
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	if id, ok := groupID(w, r); ok {
		s.respondGroup(w, r, http.StatusOK, id)
	}
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	gs, err := s.store.MyGroups(r.Context(), userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]groupDTO, 0, len(gs))
	for _, g := range gs {
		d, _ := s.groupDTO(r, g, false)
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) joinGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"inviteCode"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.store.JoinGroup(r.Context(), userOf(r), req.InviteCode)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "invite_invalid", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.respondGroup(w, r, http.StatusOK, id)
}

func (s *Server) patchGroupMe(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	var req struct {
		Budget    *int      `json:"budget"`
		Interests *[]string `json:"interests"`
		Pace      *string   `json:"pace"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if req.Budget != nil && (*req.Budget < 1 || *req.Budget > 3) {
		fe.add("budget", "1..3")
	}
	if req.Interests != nil && !validInterests(*req.Interests) {
		fe.add("interests", "unknown interest")
	}
	if req.Pace != nil && !slices.Contains(paces, *req.Pace) {
		fe.add("pace", strings.Join(paces, " | "))
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SetMemberPrefs(r.Context(), id, userOf(r), store.MemberPrefs{Budget: req.Budget, Interests: req.Interests, Pace: req.Pace})
	if errors.Is(err, store.ErrNotMember) {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.respondGroup(w, r, http.StatusOK, id)
}

func (s *Server) voteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	var req struct {
		PlaceID string `json:"placeId"`
		Vote    int    `json:"vote"` // 1 | -1 | 0 (olib tashlash)
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Vote < -1 || req.Vote > 1 {
		writeValidation(w, r, fieldErrors{"vote": "1 | -1 | 0"})
		return
	}
	p, err := s.store.Place(r.Context(), req.PlaceID, []string{"en"})
	if isNotFound(err) || err == nil && p.Status != "published" {
		writeError(w, r, http.StatusNotFound, "place_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	err = s.store.Vote(r.Context(), id, userOf(r), p.ID, req.Vote)
	if errors.Is(err, store.ErrNotMember) {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.respondGroup(w, r, http.StatusOK, id)
}

func (s *Server) leaveGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	err := s.store.LeaveGroup(r.Context(), id, userOf(r))
	if errors.Is(err, store.ErrNotMember) {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// groupPrefs — a'zolar istaklaridan umumiy profil: qiziqishlar (ko'p uchraganlari), eng kichik byudjet,
// eng sokin temp; ovozlar: ko'pchilik yoqtirgan joy ustun, ko'pchilik rad etgani chiqariladi.
func groupPrefs(g store.Group) (interests []string, budget int, pace string, boost map[string]float64, exclude map[string]bool) {
	count := map[string]int{}
	paceRank := map[string]int{"relaxed": 0, "balanced": 1, "active": 2}
	pace = "active"
	for _, m := range g.Members {
		for _, i := range m.Interests {
			count[i]++
		}
		if m.Budget != nil && (budget == 0 || *m.Budget < budget) {
			budget = *m.Budget
		}
		if m.Pace != nil && paceRank[*m.Pace] < paceRank[pace] {
			pace = *m.Pace
		}
	}
	if len(g.Members) > 0 && !slices.ContainsFunc(g.Members, func(m store.GroupMember) bool { return m.Pace != nil }) {
		pace = "balanced"
	}
	for i := range count {
		interests = append(interests, i)
	}
	sort.Slice(interests, func(a, b int) bool {
		if count[interests[a]] != count[interests[b]] {
			return count[interests[a]] > count[interests[b]]
		}
		return interests[a] < interests[b]
	})
	if len(interests) > 4 {
		interests = interests[:4]
	}
	boost, exclude = map[string]float64{}, map[string]bool{}
	n := float64(max(1, len(g.Members)))
	for _, v := range g.Votes {
		switch net := v.Up - v.Down; {
		case net > 0:
			boost[v.PlaceID] = min(1, float64(net)/n)
		case net < 0 && float64(v.Down) >= n/2:
			exclude[v.PlaceID] = true
		}
	}
	return
}

// groupTrip — guruh uchun umumiy marshrut; barcha a'zolar ko'radi va tahrirlaydi.
func (s *Server) groupTrip(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}
	g, err := s.store.Group(r.Context(), id, userOf(r))
	if errors.Is(err, store.ErrNotMember) || isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "group_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	var req struct {
		CityID        string         `json:"cityId"`
		StartDate     string         `json:"startDate"`
		Days          int            `json:"days"`
		StartLocation *domain.LatLng `json:"startLocation"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CityID == "" && g.CityID != nil {
		req.CityID = *g.CityID
	}
	interests, budget, pace, boost, exclude := groupPrefs(g)
	tr := tripReq{CityID: req.CityID, StartDate: req.StartDate, Days: req.Days, Interests: interests, Companion: "friends",
		Pace: pace, Budget: budget, StartLocation: req.StartLocation}
	start, fe := tr.validate(s.now())
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	prefs := tr.prefs(start)
	prefs.Boost, prefs.Exclude = boost, exclude
	t, ok := s.planAndSave(w, r, tr, prefs, start)
	if !ok {
		return
	}
	if err := s.store.SetTripGroup(r.Context(), t.ID, id); err != nil {
		s.internalError(w, r, err)
		return
	}
	full, err := s.store.Trip(r.Context(), t.ID, userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeTrip(w, r, http.StatusCreated, full)
}
