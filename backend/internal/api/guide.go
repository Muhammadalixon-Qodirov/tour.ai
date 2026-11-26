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
	"github.com/jackc/pgx/v5"

	"yoldosh/internal/assistant"
	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
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

	// suhbat tarixi (faqat o'z suhbati), seq bo'yicha qat'iy tartibda
	var history []assistant.Turn
	isNew := true
	if req.ConversationID != "" {
		var owner string
		err := s.store.DB.QueryRow(ctx, `SELECT user_id FROM guide_conversations WHERE id = $1`, conv).Scan(&owner)
		switch {
		case err == nil && owner != uid:
			writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
			return
		case err == nil:
			isNew = false
			history, err = s.guideHistory(ctx, conv, lang)
			if err != nil {
				s.internalError(w, r, err)
				return
			}
		case !isNotFound(err) && !errors.Is(err, pgx.ErrNoRows):
			s.internalError(w, r, err)
			return
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
	if err := s.saveGuideTurn(context.WithoutCancel(ctx), conv, uid, isNew, req, answer, mode, msgID, ids); err != nil {
		slog.Error("guide: saqlanmadi", "err", err)
	}
	done := map[string]any{"conversationId": conv.String(), "messageId": msgID.String(), "mode": mode, "newConversation": isNew}
	if stream {
		emit("done", done)
		return
	}
	done["text"] = answer
	done["places"] = cards
	writeJSON(w, http.StatusOK, done)
}

func tashkentLoc() *time.Location { return tz("Asia/Tashkent") }

// guideHistory — oxirgi 20 ta xabar (seq tartibida). Yordamchi javobiga o'sha javobda ko'rsatilgan joylar ro'yxati
// qo'shiladi — "birinchisi qachon ochiladi?" kabi keyingi savollar uchun model id ni biladi.
func (s *Server) guideHistory(ctx context.Context, conv uuid.UUID, lang string) ([]assistant.Turn, error) {
	rows, err := s.store.DB.Query(ctx, `SELECT role, text, place_ids FROM (SELECT role, text, place_ids, seq FROM guide_messages
		WHERE conversation_id = $1 ORDER BY seq DESC LIMIT 20) x ORDER BY seq`, conv)
	if err != nil {
		return nil, err
	}
	type row struct {
		role, text string
		ids        []string
	}
	var list []row
	var all []string
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.role, &x.text, &x.ids); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, x)
		all = append(all, x.ids...)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	names := map[string]store.Place{}
	if len(all) > 0 {
		if names, err = s.store.PlacesByIDs(ctx, all, i18n.Chain(lang)); err != nil {
			return nil, err
		}
	}
	out := make([]assistant.Turn, 0, len(list))
	for _, x := range list {
		text := x.text
		if r := []rune(text); len(r) > 1500 {
			text = string(r[:1500]) + "…"
		}
		if x.role == "assistant" && len(x.ids) > 0 {
			var refs []string
			for _, id := range x.ids {
				if p, ok := names[id]; ok {
					refs = append(refs, id+" = "+p.Name)
				}
			}
			if len(refs) > 0 {
				text += "\n[places shown in this answer (id = name): " + strings.Join(refs, "; ") + "]"
			}
		}
		out = append(out, assistant.Turn{Role: x.role, Text: text})
	}
	return out, nil
}

// saveGuideTurn — savol va javob bitta tranzaksiyada, tartib (seq) bilan; suhbat ro'yxati yangilanadi.
func (s *Server) saveGuideTurn(ctx context.Context, conv uuid.UUID, uid string, isNew bool, req guideReq, answer, mode string,
	msgID uuid.UUID, placeIDs []string) error {
	return pgx.BeginFunc(ctx, s.store.DB, func(tx pgx.Tx) error {
		if isNew {
			title := []rune(strings.Join(strings.Fields(req.Message), " "))
			if len(title) > 80 {
				title = append(title[:77], '…')
			}
			if _, err := tx.Exec(ctx, `INSERT INTO guide_conversations (id, user_id, title, city_id) VALUES ($1,$2,$3,NULLIF($4,''))`,
				conv, uid, string(title), req.CityID); err != nil {
				return err
			}
		}
		if placeIDs == nil {
			placeIDs = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO guide_messages (id, conversation_id, user_id, role, text, mode) VALUES ($1,$2,$3,'user',$4,$5)`,
			uuid.Must(uuid.NewV7()), conv, uid, req.Message, mode); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO guide_messages (id, conversation_id, user_id, role, text, mode, place_ids)
			VALUES ($1,$2,$3,'assistant',$4,$5,$6)`, msgID, conv, uid, answer, mode, placeIDs); err != nil {
			return err
		}
		last := []rune(answer)
		if len(last) > 160 {
			last = append(last[:157], '…')
		}
		_, err := tx.Exec(ctx, `UPDATE guide_conversations SET message_count = message_count + 2, last_text = $2, updated_at = now(),
			city_id = COALESCE(NULLIF($3,''), city_id) WHERE id = $1`, conv, string(last), req.CityID)
		return err
	})
}

type conversationDTO struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	LastText     string    `json:"lastText"`
	MessageCount int       `json:"messageCount"`
	CityID       *string   `json:"cityId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// GET /guide/conversations — foydalanuvchining suhbatlari, eng yangisi birinchi.
func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	off, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeValidation(w, r, fieldErrors{"cursor": "invalid"})
		return
	}
	rows, err := s.store.DB.Query(r.Context(), `SELECT id, title, last_text, message_count, city_id, created_at, updated_at
		FROM guide_conversations WHERE user_id = $1 ORDER BY updated_at DESC, id DESC LIMIT 30 OFFSET $2`, userOf(r), off)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	defer rows.Close()
	items := []conversationDTO{}
	for rows.Next() {
		var c conversationDTO
		if err := rows.Scan(&c.ID, &c.Title, &c.LastText, &c.MessageCount, &c.CityID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			s.internalError(w, r, err)
			return
		}
		items = append(items, c)
	}
	var next *string
	if len(items) == 30 {
		next = encodeCursor(off + 30)
	}
	writeJSON(w, http.StatusOK, page[conversationDTO]{Items: items, NextCursor: next})
}

// GET /guide/conversations/{id} — suhbat va barcha xabarlar (tartib bilan), yordamchi javoblarida joy kartochkalari.
func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	var c conversationDTO
	err = s.store.DB.QueryRow(ctx, `SELECT id, title, last_text, message_count, city_id, created_at, updated_at
		FROM guide_conversations WHERE id = $1 AND user_id = $2`, id, userOf(r)).
		Scan(&c.ID, &c.Title, &c.LastText, &c.MessageCount, &c.CityID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	rows, err := s.store.DB.Query(ctx, `SELECT id, role, text, place_ids, mode, created_at FROM guide_messages
		WHERE conversation_id = $1 ORDER BY seq`, id)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	type msg struct {
		ID        uuid.UUID         `json:"id"`
		Role      string            `json:"role"`
		Text      string            `json:"text"`
		Mode      *string           `json:"mode,omitempty"`
		PlaceIDs  []string          `json:"placeIds"`
		Places    []placeSummaryDTO `json:"places"`
		CreatedAt time.Time         `json:"createdAt"`
	}
	out := []msg{}
	var all []string
	for rows.Next() {
		var m msg
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &m.PlaceIDs, &m.Mode, &m.CreatedAt); err != nil {
			rows.Close()
			s.internalError(w, r, err)
			return
		}
		all = append(all, m.PlaceIDs...)
		out = append(out, m)
	}
	rows.Close()
	places, err := s.store.PlacesByIDs(ctx, all, i18n.Chain(langOf(r)))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for i := range out {
		out[i].Places = []placeSummaryDTO{}
		for _, pid := range out[i].PlaceIDs {
			if p, ok := places[pid]; ok && p.Status == "published" {
				out[i].Places = append(out[i].Places, s.summaryDTO(p, tashkentLoc()))
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": c, "conversationId": id.String(), "messages": out})
}

// PATCH /guide/conversations/{id} {title}
func (s *Server) renameConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if n := utf8.RuneCountInString(req.Title); n < 1 || n > 120 {
		writeValidation(w, r, fieldErrors{"title": "1..120 chars"})
		return
	}
	id, _ := uuid.Parse(r.PathValue("id"))
	tag, err := s.store.DB.Exec(r.Context(), `UPDATE guide_conversations SET title = $3 WHERE id = $1 AND user_id = $2`, id, userOf(r), req.Title)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /guide/conversations/{id} — suhbat va xabarlari butunlay o'chiriladi.
func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(r.PathValue("id"))
	var n int64
	err := pgx.BeginFunc(r.Context(), s.store.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM guide_conversations WHERE id = $1 AND user_id = $2`, id, userOf(r))
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		_, err = tx.Exec(r.Context(), `DELETE FROM guide_messages WHERE conversation_id = $1 AND user_id = $2`, id, userOf(r))
		return err
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if n == 0 {
		writeError(w, r, http.StatusNotFound, "conversation_not_found", false)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
