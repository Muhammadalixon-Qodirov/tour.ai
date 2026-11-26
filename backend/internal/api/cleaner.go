package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
)

// Ichki "ma'lumot tozalovchi" API. Ichki tarmoqdagi AI server (Ollama/llama.cpp/...) bu endpointlardan
// navbatdagi topilmalarni tortib oladi, modeli bilan tozalaydi (kategoriya, tavsif, junk ajratish) va
// natijani qaytaradi. Faqat chiquvchi ulanish — AI serverga port ochish shart emas.
// Himoya: X-Cleaner-Key sarlavhasi CLEANER_KEY ga teng bo'lishi kerak; kalit o'rnatilmagan bo'lsa endpointlar o'chiq.

func subtleEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) requireCleaner(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := s.cfg.CleanerKey
		if key == "" {
			writeError(w, r, http.StatusNotFound, "not_found", false)
			return
		}
		if subtleEqual(r.Header.Get("X-Cleaner-Key"), key) {
			next(w, r)
			return
		}
		writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
	})
}

// cleanPendingItem — modelga yuboriladigan xom signallar (fakt bazadan, model faqat tozalaydi).
type cleanPendingItem struct {
	ID         int64             `json:"id"`
	Source     string            `json:"source"`
	Ref        string            `json:"ref"`
	Names      map[string]string `json:"names"`
	CityID     *string           `json:"cityId"`
	Lat        float64           `json:"lat"`
	Lng        float64           `json:"lng"`
	Category   *string           `json:"currentCategory"`
	Confidence float64           `json:"confidence"`
	Reasons    []string          `json:"reasons"`
	Desc       map[string]string `json:"desc,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Classes    []string          `json:"classes,omitempty"`
	Sitelinks  int               `json:"sitelinks,omitempty"`
	Website    string            `json:"website,omitempty"`
}

func (s *Server) cleanPending(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			limit = n
		}
	}
	rows, err := s.store.DB.Query(r.Context(), `SELECT `+discoveryCols+` FROM discoveries
		WHERE status = 'queued' AND ai_cleaned_at IS NULL ORDER BY confidence DESC, id LIMIT $1`, limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	defer rows.Close()
	items := []cleanPendingItem{}
	for rows.Next() {
		d, err := scanDiscovery(rows)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		var raw struct {
			Desc      map[string]string `json:"desc"`
			Tags      map[string]string `json:"tags"`
			Classes   []string          `json:"classes"`
			Sitelinks int               `json:"sitelinks"`
			Website   string            `json:"website"`
		}
		_ = json.Unmarshal(d.Data, &raw)
		items = append(items, cleanPendingItem{ID: d.ID, Source: d.Source, Ref: d.Ref, Names: d.Name, CityID: d.CityID,
			Lat: d.Location.Lat, Lng: d.Location.Lng, Category: d.Category, Confidence: d.Confidence, Reasons: d.Reasons,
			Desc: raw.Desc, Tags: raw.Tags, Classes: raw.Classes, Sitelinks: raw.Sitelinks, Website: raw.Website})
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

// cleanResult — model qaytargan tozalangan ma'lumot.
type cleanResult struct {
	ID           int64             `json:"id"`
	Decision     string            `json:"decision"` // publish | review | reject
	Category     string            `json:"category"`
	Names        map[string]string `json:"names,omitempty"`
	Descriptions map[string]string `json:"descriptions,omitempty"`
	Confidence   float64           `json:"confidence,omitempty"`
	Reasons      []string          `json:"reasons,omitempty"`
}

type cleanResultsReq struct {
	Results []cleanResult `json:"results"`
}

func (s *Server) cleanResults(w http.ResponseWriter, r *http.Request) {
	var req cleanResultsReq
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Results) == 0 || len(req.Results) > 100 {
		writeValidation(w, r, fieldErrors{"results": "1..100"})
		return
	}
	ctx := r.Context()
	var published, rejected, review, skipped int
	for _, res := range req.Results {
		applied, err := s.applyClean(ctx, res)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		switch applied {
		case "published":
			published++
		case "rejected":
			rejected++
		case "review":
			review++
		default:
			skipped++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"published": published, "rejected": rejected, "review": review, "skipped": skipped})
}

// applyClean — bitta natijani qo'llaydi. Fakt (koordinata, manba) bazadan o'zgarmaydi; model faqat kategoriya,
// nom va tavsifni tozalaydi. Foydalanuvchi taklifi hech qachon avtomatik nashr qilinmaydi (review ga tushadi).
func (s *Server) applyClean(ctx context.Context, res cleanResult) (string, error) {
	decision := res.Decision
	outcome := "skipped"
	err := pgx.BeginFunc(ctx, s.store.DB, func(tx pgx.Tx) error {
		d, err := scanDiscovery(tx.QueryRow(ctx, `SELECT `+discoveryCols+` FROM discoveries WHERE id = $1 FOR UPDATE`, res.ID))
		if isNotFound(err) || err == pgx.ErrNoRows {
			return nil // yo'q — o'tkazamiz
		}
		if err != nil {
			return err
		}
		if d.Status != "queued" { // admin yoki boshqa jarayon allaqachon hal qilgan
			return nil
		}
		cat := strings.TrimSpace(res.Category)
		if cat == "" && d.Category != nil {
			cat = *d.Category
		}
		// foydalanuvchi taklifi va noto'g'ri kategoriya — hech qachon avto-nashr emas
		if decision == "publish" && (d.Source == "user" || !domain.ValidCategory(cat) || d.CityID == nil) {
			decision = "review"
		}
		// AI tavsiyasini har doim saqlaymiz (admin ko'radi) + ko'rilgan deb belgilaymiz
		ai, _ := json.Marshal(map[string]any{"decision": res.Decision, "category": cat, "descriptions": res.Descriptions,
			"confidence": res.Confidence, "reasons": res.Reasons})
		if _, err := tx.Exec(ctx, `UPDATE discoveries SET data = jsonb_set(data, '{ai}', $2::jsonb), ai_cleaned_at = now(),
			category = COALESCE(NULLIF($3,''), category) WHERE id = $1`, d.ID, ai, cat); err != nil {
			return err
		}
		switch decision {
		case "reject":
			outcome = "rejected"
			_, err = tx.Exec(ctx, `UPDATE discoveries SET status = 'rejected', reviewed_by = 'ai-cleaner', reviewed_at = now() WHERE id = $1`, d.ID)
			return err
		case "publish":
			names := d.Name
			for k, v := range res.Names {
				if v = strings.TrimSpace(v); v != "" && (k == "uz" || k == "ru" || k == "en") {
					names[k] = v
				}
			}
			f := discover.Found{Source: d.Source, Ref: d.Ref, Names: names, Loc: d.Location, Desc: cleanDesc(res.Descriptions),
				Data: cleanContacts(d.Data)}
			rr := discover.Result{Category: cat, Novelty: string(domain.NoveltyHidden), Interests: domain.CategoryInterests[cat],
				Confidence: d.Confidence, Reasons: append(d.Reasons, "ai-cleaner")}
			if d.Novelty != nil && domain.Novelty(*d.Novelty).Valid() {
				rr.Novelty = *d.Novelty
			}
			var placeID string
			if d.Source == "research" { // nomzod bazada bor — chop etamiz (+ tozalangan tavsif)
				tag, err := tx.Exec(ctx, `UPDATE places SET status = 'published', category = $2, published_at = now(), updated_at = now() WHERE id = $1`, d.Ref, cat)
				if err != nil {
					return err
				}
				if tag.RowsAffected() == 0 { // joy o'chirilgan/yo'q — navbatga qoldiramiz
					outcome = "review"
					return nil
				}
				placeID = d.Ref
				if err := applyCleanDesc(ctx, tx, placeID, res.Descriptions); err != nil {
					return err
				}
			} else {
				placeID, err = discover.CreatePlace(ctx, tx, f, rr, *d.CityID, "public")
				if err != nil {
					return err
				}
			}
			outcome = "published"
			_, err = tx.Exec(ctx, `UPDATE discoveries SET status = 'published', place_id = $2, reviewed_by = 'ai-cleaner', reviewed_at = now() WHERE id = $1`, d.ID, placeID)
			return err
		default: // review — navbatda qoladi, admin ko'radi
			outcome = "review"
			return nil
		}
	})
	return outcome, err
}

func cleanDesc(m map[string]string) map[string]string {
	out := map[string]string{}
	for _, l := range []string{"uz", "ru", "en"} {
		if v := strings.TrimSpace(m[l]); v != "" {
			out[l] = v
		}
	}
	return out
}

// applyCleanDesc — research joyining i18n tavsifini tozalangani bilan yangilaydi (bo'sh bo'lsa tegmaydi).
func applyCleanDesc(ctx context.Context, tx pgx.Tx, placeID string, desc map[string]string) error {
	for l, v := range cleanDesc(desc) {
		if _, err := tx.Exec(ctx, `UPDATE place_i18n SET description = $3 WHERE place_id = $1 AND lang = $2 AND (description IS NULL OR description = '')`,
			placeID, l, v); err != nil {
			return err
		}
	}
	return nil
}

func cleanContacts(data json.RawMessage) map[string]any {
	var raw struct {
		Website   string `json:"website"`
		Instagram string `json:"instagram"`
		Telegram  string `json:"telegram"`
	}
	_ = json.Unmarshal(data, &raw)
	return map[string]any{"website": raw.Website, "instagram": raw.Instagram, "telegram": raw.Telegram}
}
