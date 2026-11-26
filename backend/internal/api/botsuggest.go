package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
)

// Ichki endpoint Telegram crowdsourcing boti uchun: foydalanuvchi botga yuborgan joy (nom, lokatsiya, izoh, rasm)
// admin navbatiga tushadi (discover.Ingest, source=user — hech qachon avtomatik nashr qilinmaydi).
// Himoya: X-Bot-Key = BOT_KEY (bo'sh — endpoint o'chiq).

func (s *Server) requireBot(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.BotKey == "" {
			writeError(w, r, http.StatusNotFound, "not_found", false)
			return
		}
		if subtleEqual(r.Header.Get("X-Bot-Key"), s.cfg.BotKey) {
			next(w, r)
			return
		}
		writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
	})
}

type botSuggestReq struct {
	Name     string        `json:"name"`
	Location domain.LatLng `json:"location"`
	Note     string        `json:"note"`
	PhotoURL string        `json:"photoUrl"` // Telegram rasm havolasi (admin ko'radi)
	From     string        `json:"from"`     // Telegram foydalanuvchi (@username yoki id)
}

func (s *Server) botSuggest(w http.ResponseWriter, r *http.Request) {
	var req botSuggestReq
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	fe := fieldErrors{}
	if n := utf8.RuneCountInString(req.Name); n < 2 || n > 120 {
		fe.add("name", "2..120")
	}
	if !req.Location.Valid() || req.Location.Lat < 37 || req.Location.Lat > 46 || req.Location.Lng < 55 || req.Location.Lng > 74 {
		fe.add("location", "O'zbekiston ichida")
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	names := map[string]string{"uz": req.Name}
	data := map[string]any{"note": req.Note, "via": "telegram", "from": req.From}
	if req.PhotoURL != "" {
		data["photoUrl"] = req.PhotoURL
	}
	f := discover.Found{Source: "user", Ref: "tg-" + uuid.Must(uuid.NewV7()).String(), Names: names, Loc: req.Location,
		Data: data, Signals: discover.Signals{Names: names, Text: req.Note}}
	if _, err := discover.Ingest(r.Context(), s.store.DB, "user", []discover.Found{f}, false); err != nil {
		s.internalError(w, r, err)
		return
	}
	var d discoveryDTO
	d, _ = scanDiscovery(s.store.DB.QueryRow(r.Context(), `SELECT `+discoveryCols+` FROM discoveries WHERE source='user' AND ref=$1`, f.Ref))
	writeJSON(w, http.StatusAccepted, map[string]any{"status": d.Status, "placeId": d.PlaceID,
		"suggestedCategory": d.Category})
}
