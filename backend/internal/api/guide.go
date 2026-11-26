package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"yoldosh/internal/assistant"
	"yoldosh/internal/domain"
)

// POST /guide/chat — AI yordamchi. Javob SSE oqimi (Accept: text/event-stream) yoki ?stream=false bo'lsa bitta JSON:
//
//	event: delta   data: {"text":"..."}
//	event: places  data: {"places":[PlaceSummary...]}   — javobda tilga olingan joylar (ilova kartochka qilib ko'rsatadi)
//	event: done    data: {"conversationId","messageId","mode":"ai|fallback"}
//
// Kalit (ANTHROPIC_API_KEY) bo'lmasa yoki model ishlamasa — bazadan qoidaga asoslangan qidiruv (mode=fallback).

type guideReq struct {
	ConversationID string         `json:"conversationId"`
	Message        string         `json:"message"`
	PlaceID        string         `json:"placeId"`
	CityID         string         `json:"cityId"`
	Location       *domain.LatLng `json:"location"`
}

func (s *Server) guideChat(w http.ResponseWriter, r *http.Request) {
	var req guideReq
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	fe := fieldErrors{}
	if n := utf8.RuneCountInString(req.Message); n < 1 || n > 2000 {
		fe.add("message", "1..2000 chars")
	}
	if req.Location != nil && !req.Location.Valid() {
		fe.add("location", "invalid")
	}
	conv := uuid.Must(uuid.NewV7())
	if req.ConversationID != "" {
		id, err := uuid.Parse(req.ConversationID)
		if err != nil {
			fe.add("conversationId", "uuid")
		}
		conv = id
	}
	if len(fe) > 0 {
		writeValidation(w, r, fe)
		return
	}
	ctx := r.Context()
	uid := userOf(r)
	lang := langOf(r)

	// suhbat tarixi (faqat o'z suhbati)
	var history []assistant.Turn
	if req.ConversationID != "" {
		var owner string
		err := s.store.DB.QueryRow(ctx, `SELECT user_id FROM guide_messages WHERE conversation_id = $1 LIMIT 1`, conv).Scan(&owner)
		switch {
		case err == nil && owner != uid:
			writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
			return
		case err == nil:
			rows, err := s.store.DB.Query(ctx, `SELECT role, text FROM (SELECT role, text, created_at FROM guide_messages
				WHERE conversation_id = $1 ORDER BY created_at DESC LIMIT 12) x ORDER BY created_at`, conv)
			if err != nil {
				s.internalError(w, r, err)
				return
			}
			for rows.Next() {
				var t assistant.Turn
				if err := rows.Scan(&t.Role, &t.Text); err == nil {
					history = append(history, t)
				}
			}
			rows.Close()
		}
	}
	areq := assistant.Request{Message: req.Message, History: history, Lang: lang, CityID: req.CityID, PlaceID: req.PlaceID,
		LangGuessed: r.Header.Get("Accept-Language") == ""}
	if areq.LangGuessed { // joy nomlari ham savol tilida chiqsin
		lang = assistant.DetectLang(req.Message, lang)
		areq.Lang = lang
	}
	if req.Location != nil {
		areq.Location = &[2]float64{req.Location.Lat, req.Location.Lng}
	}
	if u, err := s.store.User(ctx, uid); err == nil {
		var parts []string
		if u.Budget != nil {
			parts = append(parts, fmt.Sprintf("budget=%d (1 cheap..3 comfort)", *u.Budget))
		}
		if len(u.Interests) > 0 {
			parts = append(parts, "interests="+strings.Join(u.Interests, ","))
		}
		if u.Companion != nil {
			parts = append(parts, "travels_with="+*u.Companion)
		}
		areq.Profile = strings.Join(parts, "; ")
	}

	stream := r.URL.Query().Get("stream") != "false" && strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(3 * time.Minute)) // server WriteTimeout (30 s) dan uzun javoblar uchun
	emit := func(string, any) {}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no") // nginx buferlamasin
		w.WriteHeader(http.StatusOK)
		emit = func(event string, data any) {
			b, _ := json.Marshal(data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
			_ = rc.Flush()
		}
	}

	tools := assistant.NewTools(s.store, lang, s.now)
	mode := "fallback"
	var answer string
	var err error
	if s.ai != nil {
		mode = "ai"
		actx, cancel := context.WithTimeout(ctx, 150*time.Second)
		answer, err = s.ai.Answer(actx, areq, tools, emit)
		cancel()
		if errors.Is(err, assistant.ErrUnavailable) { // hech narsa yuborilmagan — qidiruv bilan javob beramiz
			slog.Warn("guide: model ishlamadi, fallback", "err", err)
			mode, err = "fallback", nil
		}
	}
	if mode == "fallback" {
		answer, err = assistant.Fallback(ctx, areq, tools)
		if err == nil {
			emit("delta", map[string]string{"text": answer})
		}
	}
	if err != nil && answer == "" {
		if stream {
			emit("error", map[string]any{"code": "internal", "retryable": true})
			return
		}
		s.internalError(w, r, err)
		return
	}

	// javobda nomi tilga olingan joylar (fallback — topilganlarning hammasi)
	var cards []placeSummaryDTO
	var ids []string
	low := strings.ToLower(answer)
	for _, p := range tools.FoundOrdered() {
		if len(cards) == 6 {
			break
		}
		if mode == "fallback" || strings.Contains(low, strings.ToLower(p.Name)) {
			cards = append(cards, s.summaryDTO(p, tashkentLoc()))
			ids = append(ids, p.ID)
		}
	}
	if cards == nil {
		cards = []placeSummaryDTO{}
	}
	emit("places", map[string]any{"places": cards})

	msgID := uuid.Must(uuid.NewV7())
	bg := context.WithoutCancel(ctx)
	if _, err := s.store.DB.Exec(bg, `INSERT INTO guide_messages (id, conversation_id, user_id, role, text, mode) VALUES ($1,$2,$3,'user',$4,$5),
		($6,$2,$3,'assistant',$7,$5)`, uuid.Must(uuid.NewV7()), conv, uid, req.Message, mode, msgID, answer); err != nil {
		slog.Error("guide: saqlanmadi", "err", err)
	} else if len(ids) > 0 {
		_, _ = s.store.DB.Exec(bg, `UPDATE guide_messages SET place_ids = $2 WHERE id = $1`, msgID, ids)
	}
	done := map[string]any{"conversationId": conv.String(), "messageId": msgID.String(), "mode": mode}
	if stream {
		emit("done", done)
		return
	}
	done["text"] = answer
	done["places"] = cards
	writeJSON(w, http.StatusOK, done)
}

func tashkentLoc() *time.Location { return tz("Asia/Tashkent") }

// GET /guide/conversations/{id} — suhbat tarixi (o'ziniki).
func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	rows, err := s.store.DB.Query(r.Context(), `SELECT id, role, text, place_ids, created_at FROM guide_messages
		WHERE conversation_id = $1 AND user_id = $2 ORDER BY created_at`, id, userOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	defer rows.Close()
	type msg struct {
		ID        uuid.UUID `json:"id"`
		Role      string    `json:"role"`
		Text      string    `json:"text"`
		PlaceIDs  []string  `json:"placeIds"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := []msg{}
	for rows.Next() {
		var m msg
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &m.PlaceIDs, &m.CreatedAt); err != nil {
			s.internalError(w, r, err)
			return
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversationId": id.String(), "messages": out})
}
