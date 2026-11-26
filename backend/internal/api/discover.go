package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
)

// Joy taklifi: foydalanuvchi bilgan joyni (masalan Instagram/Telegramda ko'rgan yangi kafeni) yuboradi.
// Tizim kategoriyasini o'zi aniqlaydi, takrorini tekshiradi va admin navbatiga qo'yadi (foydalanuvchi taklifi
// hech qachon tekshiruvsiz chop etilmaydi).

type suggestReq struct {
	Name      string        `json:"name"`
	Location  domain.LatLng `json:"location"`
	Category  string        `json:"category"`
	Note      string        `json:"note"`
	Instagram string        `json:"instagram"`
	Telegram  string        `json:"telegram"`
	Website   string        `json:"website"`
}

type discoveryDTO struct {
	ID         int64             `json:"id"`
	Source     string            `json:"source"`
	Ref        string            `json:"ref"`
	Name       map[string]string `json:"name"`
	Location   domain.LatLng     `json:"location"`
	CityID     *string           `json:"cityId"`
	Category   *string           `json:"category"`
	Confidence float64           `json:"confidence"`
	Novelty    *string           `json:"novelty"`
	Interests  []string          `json:"interests"`
	Reasons    []string          `json:"reasons"`
	Data       json.RawMessage   `json:"data"`
	Status     string            `json:"status"`
	PlaceID    *string           `json:"placeId"`
	FirstSeen  time.Time         `json:"firstSeen"`
	LastSeen   time.Time         `json:"lastSeen"`
}

const discoveryCols = `id, source, ref, name, lat, lng, city_id, category, confidence, novelty, interests, reasons, data, status,
  place_id, first_seen, last_seen`

func scanDiscovery(row pgx.Row) (discoveryDTO, error) {
	var d discoveryDTO
	var conf float32
	err := row.Scan(&d.ID, &d.Source, &d.Ref, &d.Name, &d.Location.Lat, &d.Location.Lng, &d.CityID, &d.Category, &conf,
		&d.Novelty, &d.Interests, &d.Reasons, &d.Data, &d.Status, &d.PlaceID, &d.FirstSeen, &d.LastSeen)
	d.Confidence = float64(int(conf*100+0.5)) / 100
	return d, err
}

func (s *Server) suggestPlace(w http.ResponseWriter, r *http.Request) {
	var req suggestReq
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Note = strings.TrimSpace(req.Note)
	fe := fieldErrors{}
	if n := utf8.RuneCountInString(req.Name); n < 2 || n > 120 {
		fe.add("name", "2..120 chars")
	}
	if !req.Location.Valid() || req.Location.Lat < 37 || req.Location.Lat > 46 || req.Location.Lng < 55 || req.Location.Lng > 74 {
		fe.add("location", "O'zbekiston ichida lat,lng")
	}
	if req.Category != "" && !domain.ValidCategory(req.Category) {
		fe.add("category", strings.Join(domain.Categories, " | "))
	}
	if utf8.RuneCountInString(req.Note) > 1000 {
		fe.add("note", "max 1000 chars")
	}
	for k, v := range map[string]string{"instagram": req.Instagram, "telegram": req.Telegram, "website": req.Website} {
		if utf8.RuneCountInString(v) > 200 {
			fe.add(k, "max 200 chars")
		}
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	ref := uuid.Must(uuid.NewV7()).String()
	data := map[string]any{"note": req.Note}
	for k, v := range map[string]string{"instagram": req.Instagram, "telegram": req.Telegram, "website": req.Website} {
		if v = strings.TrimSpace(v); v != "" {
			data[k] = v
		}
	}
	names := map[string]string{"uz": req.Name}
	f := discover.Found{Source: "user", Ref: ref, Names: names, Loc: req.Location, UserID: userOf(r), Data: data,
		Signals: discover.Signals{Names: names, Hint: req.Category, Text: req.Note}}
	if _, err := discover.Ingest(r.Context(), s.store.DB, "user", []discover.Found{f}, false); err != nil {
		s.internalError(w, r, err)
		return
	}
	d, err := scanDiscovery(s.store.DB.QueryRow(r.Context(), `SELECT `+discoveryCols+` FROM discoveries WHERE source = 'user' AND ref = $1`, ref))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	// "duplicate" — bu joy bizda allaqachon bor (placeId bilan), "queued" — jamoa tekshiradi
	writeJSON(w, http.StatusAccepted, map[string]any{"id": d.ID, "status": d.Status, "placeId": d.PlaceID,
		"suggestedCategory": d.Category, "confidence": d.Confidence})
}

// ---------- Admin ----------

var discoveryStatuses = []string{"queued", "published", "duplicate", "rejected", "out_of_area"}

func (s *Server) adminListDiscoveries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "queued"
	}
	if !slices.Contains(discoveryStatuses, status) {
		writeValidation(w, r, fieldErrors{"status": strings.Join(discoveryStatuses, " | ")})
		return
	}
	off, _ := decodeCursor(q.Get("cursor"))
	args := []any{status}
	where := "status = $1"
	if v := q.Get("source"); v != "" {
		args = append(args, v)
		where += " AND source = $" + strconv.Itoa(len(args))
	}
	if v := q.Get("city"); v != "" {
		args = append(args, v)
		where += " AND city_id = $" + strconv.Itoa(len(args))
	}
	args = append(args, off)
	rows, err := s.store.DB.Query(r.Context(), `SELECT `+discoveryCols+` FROM discoveries WHERE `+where+
		` ORDER BY confidence DESC, id LIMIT 50 OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	defer rows.Close()
	items := []discoveryDTO{}
	for rows.Next() {
		d, err := scanDiscovery(rows)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		items = append(items, d)
	}
	var next *string
	if len(items) == 50 {
		next = encodeCursor(off + 50)
	}
	writeJSON(w, http.StatusOK, page[discoveryDTO]{Items: items, NextCursor: next})
}

type approveReq struct {
	Category string            `json:"category"`
	CityID   string            `json:"cityId"`
	Name     map[string]string `json:"name"`
}

// adminApproveDiscovery — topilmadan joy yaratadi (kategoriya/nomni admin to'g'rilashi mumkin) va chop etadi.
func (s *Server) adminApproveDiscovery(w http.ResponseWriter, r *http.Request) {
	var req approveReq
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ctx := r.Context()
	var placeID string
	err := pgx.BeginFunc(ctx, s.store.DB, func(tx pgx.Tx) error {
		d, err := scanDiscovery(tx.QueryRow(ctx, `SELECT `+discoveryCols+` FROM discoveries WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if d.Status != "queued" && d.Status != "out_of_area" {
			return errConflict
		}
		cat := req.Category
		if cat == "" && d.Category != nil {
			cat = *d.Category
		}
		city := req.CityID
		if city == "" && d.CityID != nil {
			city = *d.CityID
		}
		if !domain.ValidCategory(cat) || city == "" {
			return errNeedFields
		}
		names := d.Name
		for k, v := range req.Name {
			if v = strings.TrimSpace(v); v != "" {
				names[k] = v
			}
		}
		var raw struct {
			Desc      map[string]string `json:"desc"`
			Website   string            `json:"website"`
			Instagram string            `json:"instagram"`
			Telegram  string            `json:"telegram"`
		}
		_ = json.Unmarshal(d.Data, &raw)
		f := discover.Found{Source: d.Source, Ref: d.Ref, Names: names, Loc: d.Location, Desc: raw.Desc,
			Data: map[string]any{"website": raw.Website, "instagram": raw.Instagram, "telegram": raw.Telegram}}
		res := discover.Result{Category: cat, Novelty: string(domain.NoveltyHidden), Interests: domain.CategoryInterests[cat],
			Confidence: d.Confidence, Reasons: append(d.Reasons, "admin")}
		if d.Novelty != nil && domain.Novelty(*d.Novelty).Valid() {
			res.Novelty = *d.Novelty
		}
		trust := "public"
		if d.Source == "user" {
			trust = "community"
		}
		if d.Source == "research" { // tadqiqot nomzodi bazada allaqachon bor (pending) — faqat chop etiladi
			placeID = d.Ref
			_, err = tx.Exec(ctx, `UPDATE places SET status = 'published', category = $2, city_id = $3, published_at = now(),
				updated_at = now() WHERE id = $1`, placeID, cat, city)
		} else {
			placeID, err = discover.CreatePlace(ctx, tx, f, res, city, trust)
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE discoveries SET status = 'published', place_id = $2, category = $3, reviewed_by = $4,
			reviewed_at = now() WHERE id = $1`, id, placeID, cat, adminOf(r).ID)
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, r, http.StatusNotFound, "discovery_not_found", false)
	case errors.Is(err, errConflict):
		writeError(w, r, http.StatusConflict, "already_reviewed", false)
	case errors.Is(err, errNeedFields):
		writeValidation(w, r, fieldErrors{"category": "required", "cityId": "required"})
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"placeId": placeID, "status": "published"})
	}
}

var (
	errConflict   = errors.New("conflict")
	errNeedFields = errors.New("fields required")
)

func (s *Server) adminRejectDiscovery(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tag, err := s.store.DB.Exec(r.Context(), `UPDATE discoveries SET status = 'rejected', reviewed_by = $2, reviewed_at = now()
		WHERE id = $1 AND status IN ('queued', 'out_of_area', 'duplicate')`, id, adminOf(r).ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, r, http.StatusNotFound, "discovery_not_found", false)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminRunDiscovery — Wikidata yig'ishni hoziroq fonda ishga tushiradi (~30-60 s); natija discovery_runs da va
// GET /admin/discoveries da ko'rinadi.
func (s *Server) adminRunDiscovery(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if _, err := discover.RunWikidata(ctx, s.store.DB, false); err != nil {
			slog.Error("discovery wikidata (admin)", "err", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}
