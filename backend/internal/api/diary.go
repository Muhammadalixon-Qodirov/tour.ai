package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/diary"
	"yoldosh/internal/i18n"
	"yoldosh/internal/media"
	"yoldosh/internal/store"
)

const (
	maxPhotosPerDay   = 30
	maxRegenerations  = 5
	minVisitedForPush = 2
)

// ---------- Rasmlar ----------

type photoDTO struct {
	ID     string  `json:"id"`
	URL    string  `json:"url"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	StopID *string `json:"stopId,omitempty"`
}

func toPhotoDTO(p store.DiaryPhoto) photoDTO {
	d := photoDTO{ID: p.ID.String(), URL: p.URL, Width: p.Width, Height: p.Height}
	if p.StopID != nil {
		v := p.StopID.String()
		d.StopID = &v
	}
	return d
}

// uploadDiaryPhoto — multipart: image, tripId, dayIndex, stopId?. EXIF/GPS olib tashlanadi.
func (s *Server) uploadDiaryPhoto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", false)
		return
	}
	defer r.MultipartForm.RemoveAll()
	fe := fieldErrors{}
	tripID, err := uuid.Parse(r.FormValue("tripId"))
	if err != nil {
		fe.add("tripId", "uuid")
	}
	day, err := strconv.Atoi(r.FormValue("dayIndex"))
	if err != nil {
		fe.add("dayIndex", "number")
	}
	var stopID *uuid.UUID
	if v := r.FormValue("stopId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fe.add("stopId", "uuid")
		}
		stopID = &id
	}
	f, _, err := r.FormFile("image")
	if err != nil {
		fe.add("image", "multipart file required")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	defer f.Close()

	t, err := s.store.Trip(r.Context(), tripID, userOf(r))
	if err == nil && t.UserID != userOf(r) {
		err = store.ErrNotFound
	}
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "trip_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if day < 0 || day >= len(t.Days) {
		writeValidation(w, r, fieldErrors{"dayIndex": "0.." + strconv.Itoa(len(t.Days)-1)})
		return
	}
	if stopID != nil && !slices.ContainsFunc(t.Days[day].Stops, func(st store.Stop) bool { return st.ID == *stopID }) {
		writeError(w, r, http.StatusNotFound, "stop_not_found", false)
		return
	}
	if n, err := s.store.CountDayPhotos(r.Context(), t.ID, day); err != nil {
		s.internalError(w, r, err)
		return
	} else if n >= maxPhotosPerDay {
		writeError(w, r, http.StatusConflict, "too_many_photos", false)
		return
	}
	data, width, height, err := media.Process(f)
	switch {
	case errors.Is(err, media.ErrUnsupported):
		writeError(w, r, http.StatusUnsupportedMediaType, "image_unsupported", false)
		return
	case errors.Is(err, media.ErrTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", false)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	name := media.NewName()
	url, err := s.media.Save(name, data)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	p := store.DiaryPhoto{ID: uuid.Must(uuid.NewV7()), UserID: userOf(r), TripID: t.ID, DayIdx: day, StopID: stopID,
		FileName: name, URL: url, Width: width, Height: height}
	if err := s.store.AddDiaryPhoto(r.Context(), p); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPhotoDTO(p))
}

func (s *Server) deleteDiaryPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	}
	file, err := s.store.DeleteDiaryPhoto(r.Context(), userOf(r), id)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.media.Delete(file); err != nil {
		slog.Warn("rasm faylini o'chirib bo'lmadi", "file", file, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) dayPhotos(w http.ResponseWriter, r *http.Request) {
	t, day, ok := s.loadTripDay(w, r)
	if !ok {
		return
	}
	photos, err := s.store.DayPhotos(r.Context(), t.ID, day)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]photoDTO, 0, len(photos))
	for _, p := range photos {
		out = append(out, toPhotoDTO(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) loadTripDay(w http.ResponseWriter, r *http.Request) (store.Trip, int, bool) {
	t, ok := s.loadTrip(w, r)
	if !ok {
		return t, 0, false
	}
	if t.UserID != userOf(r) { // guruh a'zosi umumiy marshrutni ko'radi, lekin kundalik — egasiniki
		writeError(w, r, http.StatusNotFound, "trip_not_found", false)
		return t, 0, false
	}
	day, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil || day < 0 || day >= len(t.Days) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return t, 0, false
	}
	return t, day, true
}

// ---------- Hikoya ----------

var errDayNotStarted = errors.New("diary_day_not_started")
var errNothingVisited = errors.New("diary_nothing_visited")

// buildStory — kun faktlaridan hikoya yozadi (AI bo'lsa AI, aks holda shablon) va yozuvni saqlaydi.
func (s *Server) buildStory(ctx context.Context, t store.Trip, day int, lang, tone string, regenerated bool) (uuid.UUID, error) {
	city, err := s.store.City(ctx, t.CityID, i18n.Chain(lang))
	if err != nil {
		return uuid.Nil, err
	}
	now := s.now().In(tz(city.Timezone))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	d := t.Days[day]
	if d.Date.After(today) {
		return uuid.Nil, errDayNotStarted
	}
	// Borilgan joylar; hech biri belgilanmagan bo'lsa — o'tkazib yuborilmaganlari.
	var chosen []store.Stop
	for _, st := range d.Stops {
		if st.Status == "visited" {
			chosen = append(chosen, st)
		}
	}
	if len(chosen) == 0 {
		for _, st := range d.Stops {
			if st.Status != "skipped" {
				chosen = append(chosen, st)
			}
		}
	}
	if len(chosen) == 0 {
		return uuid.Nil, errNothingVisited
	}
	in := diary.StoryInput{Lang: lang, Tone: tone, City: city.Name, CityID: city.ID, Date: d.Date}
	var prefs tripReq
	_ = json.Unmarshal(t.Prefs, &prefs)
	in.Interests = prefs.Interests
	for _, st := range chosen {
		p, err := s.store.Place(ctx, st.PlaceID, i18n.Chain(lang))
		if err != nil {
			return uuid.Nil, err
		}
		fact := ""
		if len(p.HistoryFacts) > 0 {
			fact = p.HistoryFacts[0]
		}
		in.Stops = append(in.Stops, diary.StopFact{PlaceID: p.ID, Name: p.Name, Category: p.Category,
			Novel: st.CountsForNovelty(), Fact: fact})
		if st.Transit != nil && st.Transit.Mode == "walk" {
			in.WalkingM += st.Transit.DistanceM
		}
	}
	photos, err := s.store.DayPhotos(ctx, t.ID, day)
	if err != nil {
		return uuid.Nil, err
	}
	in.Photos = len(photos)
	story, err := s.story.Write(ctx, in)
	if err == nil {
		err = diary.CheckGrounded(story, in)
	}
	if err != nil {
		if _, isTpl := s.story.(diary.Template); !isTpl {
			slog.Warn("AI hikoya rad etildi — shablon ishlatiladi", "err", err)
		}
		if story, err = (diary.Template{}).Write(ctx, in); err != nil {
			return uuid.Nil, err
		}
	}
	var photoIDs []uuid.UUID
	for i, p := range photos {
		if i == 4 {
			break
		}
		photoIDs = append(photoIDs, p.ID)
	}
	e := store.DiaryEntry{ID: uuid.Must(uuid.NewV7()), UserID: t.UserID, TripID: t.ID, DayIdx: day, Lang: lang, Tone: tone,
		Title: story.Title, Body: story.Body, Caption: story.Caption, Highlights: story.Highlights, Hashtags: story.Hashtags,
		PlaceIDs: story.PlaceIDs, PhotoIDs: photoIDs, GeneratedBy: story.By}
	if photoIDs == nil {
		e.PhotoIDs = []uuid.UUID{}
	}
	return s.store.UpsertDiaryEntry(ctx, e, regenerated)
}

func (s *Server) storyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errDayNotStarted):
		writeError(w, r, http.StatusUnprocessableEntity, "diary_day_not_started", false)
	case errors.Is(err, errNothingVisited):
		writeError(w, r, http.StatusUnprocessableEntity, "diary_nothing_visited", false)
	default:
		s.internalError(w, r, err)
	}
}

func parseTone(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Tone string `json:"tone"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return "", false
	}
	if req.Tone == "" {
		req.Tone = "warm"
	}
	if !slices.Contains(diary.Tones, req.Tone) {
		writeValidation(w, r, fieldErrors{"tone": strings.Join(diary.Tones, " | ")})
		return "", false
	}
	return req.Tone, true
}

// createDiary — kun hikoyasi: bor bo'lsa qaytaradi (200), bo'lmasa yaratadi (201).
func (s *Server) createDiary(w http.ResponseWriter, r *http.Request) {
	t, day, ok := s.loadTripDay(w, r)
	if !ok {
		return
	}
	tone, ok := parseTone(w, r)
	if !ok {
		return
	}
	if e, err := s.store.DiaryEntryByDay(r.Context(), t.ID, day, t.UserID); err == nil {
		s.writeEntry(w, r, http.StatusOK, e)
		return
	} else if !isNotFound(err) {
		s.internalError(w, r, err)
		return
	}
	id, err := s.buildStory(r.Context(), t, day, langOf(r), tone, false)
	if err != nil {
		s.storyError(w, r, err)
		return
	}
	s.respondEntry(w, r, http.StatusCreated, id)
}

func (s *Server) getDayDiary(w http.ResponseWriter, r *http.Request) {
	t, day, ok := s.loadTripDay(w, r)
	if !ok {
		return
	}
	e, err := s.store.DiaryEntryByDay(r.Context(), t.ID, day, t.UserID)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "diary_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeEntry(w, r, http.StatusOK, e)
}

func (s *Server) loadEntry(w http.ResponseWriter, r *http.Request) (store.DiaryEntry, bool) {
	id, err := uuid.Parse(r.PathValue("entryId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "diary_not_found", false)
		return store.DiaryEntry{}, false
	}
	e, err := s.store.DiaryEntry(r.Context(), id, userOf(r))
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "diary_not_found", false)
		return e, false
	}
	if err != nil {
		s.internalError(w, r, err)
		return e, false
	}
	return e, true
}

func (s *Server) respondEntry(w http.ResponseWriter, r *http.Request, status int, id uuid.UUID) {
	e, err := s.store.DiaryEntry(r.Context(), id, userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeEntry(w, r, status, e)
}

func (s *Server) getDiary(w http.ResponseWriter, r *http.Request) {
	if e, ok := s.loadEntry(w, r); ok {
		s.writeEntry(w, r, http.StatusOK, e)
	}
}

func (s *Server) regenerateDiary(w http.ResponseWriter, r *http.Request) {
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	tone, ok := parseTone(w, r)
	if !ok {
		return
	}
	if e.Regenerations >= maxRegenerations {
		writeError(w, r, http.StatusTooManyRequests, "diary_regenerate_limit", false)
		return
	}
	t, err := s.store.Trip(r.Context(), e.TripID, e.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	id, err := s.buildStory(r.Context(), t, e.DayIdx, langOf(r), tone, true)
	if err != nil {
		s.storyError(w, r, err)
		return
	}
	s.respondEntry(w, r, http.StatusOK, id)
}

type diaryPatch struct {
	Title    *string   `json:"title"`
	Body     *string   `json:"body"`
	Caption  *string   `json:"caption"`
	Hashtags *[]string `json:"hashtags"`
	PhotoIDs *[]string `json:"photoIds"`
}

func (s *Server) patchDiary(w http.ResponseWriter, r *http.Request) {
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	var req diaryPatch
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	p := store.EntryPatch{Title: trimPtr(req.Title), Body: trimPtr(req.Body), Caption: trimPtr(req.Caption), Hashtags: req.Hashtags}
	if p.Title != nil && !runeLen(*p.Title, 2, 120) {
		fe.add("title", "2..120 chars")
	}
	if p.Body != nil && !runeLen(*p.Body, 10, 3000) {
		fe.add("body", "10..3000 chars")
	}
	if p.Caption != nil && !runeLen(*p.Caption, 0, 500) {
		fe.add("caption", "max 500 chars")
	}
	if p.Hashtags != nil {
		if len(*p.Hashtags) > 10 {
			fe.add("hashtags", "max 10")
		}
		for _, h := range *p.Hashtags {
			if !strings.HasPrefix(h, "#") || !runeLen(h, 2, 40) || strings.ContainsAny(h, " \t\n") {
				fe.add("hashtags", "#word, no spaces")
				break
			}
		}
	}
	if req.PhotoIDs != nil {
		if len(*req.PhotoIDs) > 4 {
			fe.add("photoIds", "max 4")
		}
		ids := []uuid.UUID{}
		for _, v := range *req.PhotoIDs {
			id, err := uuid.Parse(v)
			if err != nil {
				fe.add("photoIds", "uuid")
				break
			}
			ids = append(ids, id)
		}
		if len(fe) == 0 {
			photos, err := s.store.PhotosByIDs(r.Context(), ids)
			if err != nil {
				s.internalError(w, r, err)
				return
			}
			for _, ph := range photos {
				if ph.TripID != e.TripID || ph.DayIdx != e.DayIdx {
					fe.add("photoIds", "photos must belong to this day")
				}
			}
			if len(photos) != len(ids) {
				fe.add("photoIds", "unknown photo")
			}
		}
		p.PhotoIDs = &ids
	}
	for _, v := range []*string{p.Title, p.Body, p.Caption} {
		if v != nil && !contentOK(*v) {
			fe.add("content", "links are not allowed")
			break
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	if err := s.store.UpdateDiaryEntry(r.Context(), e.ID, e.UserID, p); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.respondEntry(w, r, http.StatusOK, e.ID)
}

// contentOK — oddiy spam filtri: postlarda havolalar va Telegram kanallar reklamasi taqiqlangan.
func contentOK(s string) bool {
	l := strings.ToLower(s)
	for _, bad := range []string{"http://", "https://", "www.", "t.me/", "@gmail", "bit.ly"} {
		if strings.Contains(l, bad) {
			return false
		}
	}
	return true
}

// ---------- DTO ----------

type postRefDTO struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	URL        string `json:"url"`
	Visibility string `json:"visibility"`
	Status     string `json:"status"`
	LikesCount int    `json:"likesCount"`
	ViewsCount int    `json:"viewsCount"`
}

type diaryDTO struct {
	ID                string            `json:"id"`
	TripID            string            `json:"tripId"`
	DayIndex          int               `json:"dayIndex"`
	Date              string            `json:"date"`
	CityID            string            `json:"cityId"`
	Lang              string            `json:"lang"`
	Tone              string            `json:"tone"`
	Title             string            `json:"title"`
	Body              string            `json:"body"`
	Caption           string            `json:"caption"`
	Highlights        []string          `json:"highlights"`
	Hashtags          []string          `json:"hashtags"`
	Places            []placeSummaryDTO `json:"places"`
	Photos            []photoDTO        `json:"photos"`
	GeneratedBy       string            `json:"generatedBy"`
	Edited            bool              `json:"edited"`
	RegenerationsLeft int               `json:"regenerationsLeft"`
	ShareCard         map[string]string `json:"shareCard"`
	Post              *postRefDTO       `json:"post"`
	CreatedAt         string            `json:"createdAt"`
	UpdatedAt         string            `json:"updatedAt"`
}

func (s *Server) postURL(slug string) string { return s.cfg.PublicBaseURL + "/s/" + slug }

func (s *Server) placesFor(r *http.Request, ids []string) ([]placeSummaryDTO, error) {
	places, err := s.store.PlacesByIDs(r.Context(), ids, i18n.Chain(langOf(r)))
	if err != nil {
		return nil, err
	}
	out := []placeSummaryDTO{}
	for _, id := range ids {
		if p, ok := places[id]; ok && p.Status == "published" {
			out = append(out, s.summaryDTO(p, nil))
		}
	}
	return out, nil
}

func (s *Server) photosFor(r *http.Request, ids []uuid.UUID) ([]photoDTO, error) {
	out := []photoDTO{}
	if len(ids) == 0 {
		return out, nil
	}
	photos, err := s.store.PhotosByIDs(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	for _, p := range photos {
		out = append(out, toPhotoDTO(p))
	}
	return out, nil
}

func (s *Server) writeEntry(w http.ResponseWriter, r *http.Request, status int, e store.DiaryEntry) {
	places, err := s.placesFor(r, e.PlaceIDs)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	photos, err := s.photosFor(r, e.PhotoIDs)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	base := s.cfg.PublicBaseURL + "/api/v1/diary/" + e.ID.String() + "/share-card?format="
	d := diaryDTO{ID: e.ID.String(), TripID: e.TripID.String(), DayIndex: e.DayIdx, Date: e.Date.Format(time.DateOnly),
		CityID: e.CityID, Lang: e.Lang, Tone: e.Tone, Title: e.Title, Body: e.Body, Caption: e.Caption,
		Highlights: nonNil(e.Highlights), Hashtags: nonNil(e.Hashtags), Places: places, Photos: photos,
		GeneratedBy: e.GeneratedBy, Edited: e.Edited, RegenerationsLeft: max(0, maxRegenerations-e.Regenerations),
		ShareCard: map[string]string{"story": base + "story", "post": base + "post"},
		CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: e.UpdatedAt.UTC().Format(time.RFC3339)}
	if e.Post != nil {
		d.Post = &postRefDTO{ID: e.Post.ID.String(), Slug: e.Post.Slug, URL: s.postURL(e.Post.Slug), Visibility: e.Post.Visibility,
			Status: e.Post.Status, LikesCount: e.Post.LikesCount, ViewsCount: e.Post.ViewsCount}
	}
	writeJSON(w, status, d)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ---------- Ulashish ----------

// newSlug — 10 belgili tasodifiy ochiq havola kaliti.
func newSlug() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return strings.ToLower(strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="))
}

func (s *Server) publishDiary(w http.ResponseWriter, r *http.Request) {
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	var req struct {
		Visibility string `json:"visibility"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	if !slices.Contains([]string{"public", "link", "private"}, req.Visibility) {
		writeValidation(w, r, fieldErrors{"visibility": "public | link | private"})
		return
	}
	if !contentOK(e.Title) || !contentOK(e.Body) {
		writeError(w, r, http.StatusUnprocessableEntity, "content_rejected", false)
		return
	}
	if e.Post != nil && e.Post.Status == "hidden" {
		writeError(w, r, http.StatusConflict, "post_hidden", false)
		return
	}
	slug := newSlug()
	if e.Post != nil {
		slug = e.Post.Slug
	}
	post, err := s.store.PublishEntry(r.Context(), e, slug, req.Visibility)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if req.Visibility == "public" && (e.Post == nil || e.Post.Visibility != "public") {
		_ = s.store.AddShareEvent(r.Context(), e.ID, &post.ID, "feed")
	}
	s.respondEntry(w, r, http.StatusOK, e.ID)
}

func (s *Server) recordShare(w http.ResponseWriter, r *http.Request) {
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !slices.Contains([]string{"instagram", "telegram", "other", "copy"}, req.Target) {
		writeValidation(w, r, fieldErrors{"target": "instagram | telegram | other | copy"})
		return
	}
	var postID *uuid.UUID
	if e.Post != nil {
		postID = &e.Post.ID
	}
	if err := s.store.AddShareEvent(r.Context(), e.ID, postID, req.Target); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// shareCard — egasi uchun PNG karta (story | post).
func (s *Server) shareCard(w http.ResponseWriter, r *http.Request) {
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	s.renderCard(w, r, e, r.URL.Query().Get("format"))
}

func (s *Server) renderCard(w http.ResponseWriter, r *http.Request, e store.DiaryEntry, format string) {
	if format == "" {
		format = "story"
	}
	if format != "story" && format != "post" {
		writeValidation(w, r, fieldErrors{"format": "story | post"})
		return
	}
	sum := sha256.Sum256([]byte(fmt.Sprint(e.ID, e.UpdatedAt.UnixNano(), format, e.Post != nil)))
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	city, err := s.store.City(r.Context(), e.CityID, i18n.Chain(e.Lang))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	in := diary.CardInput{Format: format, Title: e.Title, Highlights: e.Highlights, City: city.Name, Date: e.Date.Format("02.01.2006")}
	if e.Post != nil && e.Post.Visibility != "private" {
		in.Link = strings.TrimPrefix(strings.TrimPrefix(s.postURL(e.Post.Slug), "https://"), "http://")
	}
	photos, err := s.store.PhotosByIDs(r.Context(), e.PhotoIDs)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for _, p := range photos {
		img, err := s.openImage(p.FileName)
		if err != nil {
			slog.Warn("kartaga rasm qo'shilmadi", "photo", p.ID, "err", err)
			continue
		}
		in.Photos = append(in.Photos, img)
	}
	png, err := diary.RenderCard(in)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(png)
}

func (s *Server) openImage(name string) (image.Image, error) {
	f, err := s.media.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(&buf)
	return img, err
}

// ---------- Push token ----------

func (s *Server) setPushToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string  `json:"deviceId"`
		PushToken *string `json:"pushToken"` // null — o'chirish
		Timezone  *string `json:"timezone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	fe := fieldErrors{}
	if req.PushToken != nil && !runeLen(*req.PushToken, 10, 300) {
		fe.add("pushToken", "10..300 chars")
	}
	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil || *req.Timezone == "" || utf8.RuneCountInString(*req.Timezone) > 64 {
			fe.add("timezone", "IANA, e.g. Asia/Tashkent")
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	err := s.store.SetPushToken(r.Context(), userOf(r), req.DeviceID, req.PushToken, req.Timezone)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
