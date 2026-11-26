package api

import (
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

type deviceReq struct {
	DeviceID string `json:"deviceId"`
	Platform string `json:"platform"`
	Lang     string `json:"lang"`
}

func (s *Server) authDevice(w http.ResponseWriter, r *http.Request) {
	var req deviceReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if l := len(req.DeviceID); l < 8 || l > 128 {
		fe.add("deviceId", "8..128 chars")
	}
	if !slices.Contains([]string{"ios", "android", "web"}, req.Platform) {
		fe.add("platform", "ios | android | web")
	}
	if req.Lang == "" {
		req.Lang = langOf(r)
	}
	if !i18n.Valid(req.Lang) {
		fe.add("lang", "uz | ru | en")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	u, err := s.store.DeviceSession(r.Context(), req.DeviceID, req.Platform, req.Lang)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	tok, exp, err := s.signer.Issue(u.ID, u.Role, s.now())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok, "userId": u.ID, "expiresAt": exp.UTC().Format(time.RFC3339)})
}

type meDTO struct {
	ID            string   `json:"id"`
	Lang          string   `json:"lang"`
	IsRegistered  bool     `json:"isRegistered"`
	Email         *string  `json:"email,omitempty"`
	AuthProviders []string `json:"authProviders"` // "google", "email"
	AvatarURL     *string  `json:"avatarUrl,omitempty"`
	IsResident    *bool    `json:"isResident"` // O'zbekiston rezidenti: kirish narxi shunga qarab ko'rsatiladi
	DisplayName   *string  `json:"displayName,omitempty"`
	Interests     []string `json:"interests"`
	Companion     *string  `json:"companion,omitempty"`
	Pace          *string  `json:"pace,omitempty"`
	Budget        *int     `json:"budget,omitempty"`
	Stats         struct {
		PlacesVisited    int `json:"placesVisited"`
		NewPlacesVisited int `json:"newPlacesVisited"`
	} `json:"stats"`
}

func (s *Server) meFor(r *http.Request, u store.User) (meDTO, error) {
	d := meDTO{ID: u.ID, Lang: u.Lang, IsRegistered: u.IsRegistered(), Email: u.Email, AvatarURL: u.AvatarURL, IsResident: u.IsResident,
		DisplayName: u.DisplayName, Interests: u.Interests, Companion: u.Companion, Pace: u.Pace, Budget: u.Budget,
		AuthProviders: []string{}}
	if u.GoogleSub != nil {
		d.AuthProviders = append(d.AuthProviders, "google")
	}
	if u.Email != nil && u.EmailVerified {
		d.AuthProviders = append(d.AuthProviders, "email")
	}
	if d.Interests == nil {
		d.Interests = []string{}
	}
	var err error
	d.Stats.PlacesVisited, d.Stats.NewPlacesVisited, err = s.store.UserStats(r.Context(), u.ID)
	return d, err
}

func (s *Server) writeMe(w http.ResponseWriter, r *http.Request, u store.User) {
	d, err := s.meFor(r, u)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.User(r.Context(), userOf(r))
	if isNotFound(err) {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeMe(w, r, u)
}

type mePatch struct {
	Lang        *string   `json:"lang"`
	DisplayName *string   `json:"displayName"`
	Interests   *[]string `json:"interests"`
	Companion   *string   `json:"companion"`
	Pace        *string   `json:"pace"`
	Budget      *int      `json:"budget"`
	IsResident  *bool     `json:"isResident"`
}

var (
	companions = []string{"solo", "couple", "family", "friends"}
	paces      = []string{"relaxed", "balanced", "active"}
)

func validInterests(list []string) bool {
	if len(list) > len(domain.Interests) {
		return false
	}
	for _, i := range list {
		if !domain.ValidInterest(i) {
			return false
		}
	}
	return true
}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	var p mePatch
	if !decodeJSON(w, r, &p) {
		return
	}
	fe := fieldErrors{}
	if p.Lang != nil && !i18n.Valid(*p.Lang) {
		fe.add("lang", "uz | ru | en")
	}
	if p.DisplayName != nil {
		n := strings.TrimSpace(*p.DisplayName)
		if c := utf8.RuneCountInString(n); c < 2 || c > 40 {
			fe.add("displayName", "2..40 chars")
		}
		p.DisplayName = &n
	}
	if p.Interests != nil && !validInterests(*p.Interests) {
		fe.add("interests", "unknown interest")
	}
	if p.Companion != nil && !slices.Contains(companions, *p.Companion) {
		fe.add("companion", strings.Join(companions, " | "))
	}
	if p.Pace != nil && !slices.Contains(paces, *p.Pace) {
		fe.add("pace", strings.Join(paces, " | "))
	}
	if p.Budget != nil && (*p.Budget < 1 || *p.Budget > 3) {
		fe.add("budget", "1..3")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	u, err := s.store.UpdateUser(r.Context(), userOf(r), store.UserPatch{Lang: p.Lang, Interests: p.Interests,
		Companion: p.Companion, Pace: p.Pace, Budget: p.Budget, DisplayName: p.DisplayName, IsResident: p.IsResident})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeMe(w, r, u)
}

type visitReq struct {
	PlaceID string  `json:"placeId"`
	TripID  *string `json:"tripId"`
	Rating  *int    `json:"rating"`
	Comment *string `json:"comment"`
}

func (s *Server) addVisit(w http.ResponseWriter, r *http.Request) {
	var req visitReq
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		fe.add("rating", "1..5")
	}
	if req.Comment != nil {
		c := strings.TrimSpace(*req.Comment)
		if utf8.RuneCountInString(c) > 1000 {
			fe.add("comment", "max 1000 chars")
		}
		req.Comment = &c
	}
	var tripID *uuid.UUID
	if req.TripID != nil {
		id, err := uuid.Parse(*req.TripID)
		if err != nil {
			fe.add("tripId", "uuid")
		}
		tripID = &id
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
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
	if err := s.store.AddVisit(r.Context(), userOf(r), p.ID, tripID, req.Rating, req.Comment); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
