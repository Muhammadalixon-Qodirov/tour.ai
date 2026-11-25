package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"yoldosh/internal/domain"
)

// ---------- Admin foydalanuvchilar ----------

type AdminUser struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Role         string
	Disabled     bool
}

func (s *Store) AdminByEmail(ctx context.Context, email string) (AdminUser, error) {
	var a AdminUser
	err := s.DB.QueryRow(ctx, `SELECT id, email, name, password_hash, role, disabled FROM admin_users WHERE lower(email) = lower($1)`, email).
		Scan(&a.ID, &a.Email, &a.Name, &a.PasswordHash, &a.Role, &a.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) AdminByID(ctx context.Context, id string) (AdminUser, error) {
	var a AdminUser
	err := s.DB.QueryRow(ctx, `SELECT id, email, name, password_hash, role, disabled FROM admin_users WHERE id = $1`, id).
		Scan(&a.ID, &a.Email, &a.Name, &a.PasswordHash, &a.Role, &a.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// UpsertAdmin — email bo'yicha yaratadi yoki parol/rolni yangilaydi (CLI uchun).
func (s *Store) UpsertAdmin(ctx context.Context, id, email, name, hash, role string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO admin_users (id, email, name, password_hash, role) VALUES ($1, lower($2), $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role,
		  name = COALESCE(NULLIF(EXCLUDED.name, ''), admin_users.name), disabled = false`, id, email, name, hash, role)
	return err
}

func (s *Store) TouchAdminLogin(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, `UPDATE admin_users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// audit — tranzaksiya ichida o'zgarishni yozadi.
func audit(ctx context.Context, tx pgx.Tx, adminID, action, entity, entityID string, before, after any) error {
	var b, a []byte
	if before != nil {
		b, _ = json.Marshal(before)
	}
	if after != nil {
		a, _ = json.Marshal(after)
	}
	_, err := tx.Exec(ctx, `INSERT INTO admin_audit (admin_id, action, entity, entity_id, before, after) VALUES ($1,$2,$3,$4,$5,$6)`,
		adminID, action, entity, entityID, b, a)
	return err
}

// ---------- Joylar (admin) ----------

// PlaceRecord — joyning tahrirlanadigan to'liq yozuvi (tilsiz).
type PlaceRecord struct {
	ID              string                `json:"id"`
	CityID          string                `json:"cityId"`
	Category        string                `json:"category"`
	Novelty         domain.Novelty        `json:"novelty"`
	OpenedAt        *string               `json:"openedAt"` // YYYY-MM-DD
	Status          string                `json:"status"`
	Location        domain.LatLng         `json:"location"`
	GeofenceRadiusM int                   `json:"geofenceRadiusM"`
	Interests       []string              `json:"interests"`
	PriceLevel      int                   `json:"priceLevel"`
	VisitMin        int                   `json:"visitDurationMin"`
	OpeningHours    []domain.OpeningHours `json:"openingHours"`
	EntryFee        json.RawMessage       `json:"entryFee"`
	Wheelchair      bool                  `json:"wheelchair"`
	FamilyFriendly  bool                  `json:"familyFriendly"`
	EasyWalk        bool                  `json:"easyWalk"`
	PayCard         bool                  `json:"payCard"`
	CashOnly        bool                  `json:"cashOnly"`
	IsPartner       bool                  `json:"isPartner"`
	Images          []domain.Image        `json:"images"`
	Source          string                `json:"source"`
	RejectReason    *string               `json:"rejectReason"`
	CreatedBy       *string               `json:"createdBy"`

	// faqat o'qish
	Rating       float64    `json:"rating"`
	ReviewsCount int        `json:"reviewsCount"`
	RulesVersion int        `json:"rulesVersion"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	PublishedAt  *time.Time `json:"publishedAt"`
}

type PlaceText struct {
	Name        string          `json:"name"`
	Tagline     string          `json:"tagline"`
	Description string          `json:"description"`
	Address     string          `json:"address"`
	History     *string         `json:"history"`
	Facts       []string        `json:"facts"`
	Sources     json.RawMessage `json:"sources"`
}

const recordCols = `id, city_id, category, novelty, to_char(opened_at, 'YYYY-MM-DD'), status, lat, lng, geofence_radius_m,
  interests, price_level, visit_duration_min, opening_hours, entry_fee, wheelchair, family_friendly, easy_walk,
  pay_card, cash_only, is_partner, images, source, reject_reason, created_by,
  rating::float8, reviews_count, rules_version, created_at, updated_at, published_at`

func scanRecord(row pgx.Row) (PlaceRecord, error) {
	var p PlaceRecord
	err := row.Scan(&p.ID, &p.CityID, &p.Category, &p.Novelty, &p.OpenedAt, &p.Status, &p.Location.Lat, &p.Location.Lng,
		&p.GeofenceRadiusM, &p.Interests, &p.PriceLevel, &p.VisitMin, &p.OpeningHours, &p.EntryFee, &p.Wheelchair,
		&p.FamilyFriendly, &p.EasyWalk, &p.PayCard, &p.CashOnly, &p.IsPartner, &p.Images, &p.Source, &p.RejectReason,
		&p.CreatedBy, &p.Rating, &p.ReviewsCount, &p.RulesVersion, &p.CreatedAt, &p.UpdatedAt, &p.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// AdminPlace — yozuv, hamma tillardagi matnlar va qoidalar.
func (s *Store) AdminPlace(ctx context.Context, id string) (PlaceRecord, map[string]PlaceText, []Rule, error) {
	p, err := scanRecord(s.DB.QueryRow(ctx, `SELECT `+recordCols+` FROM places WHERE id = $1`, id))
	if err != nil {
		return p, nil, nil, err
	}
	texts := map[string]PlaceText{}
	rows, err := s.DB.Query(ctx, `SELECT lang, name, tagline, description, address, history_summary, history_facts, sources
		FROM place_i18n WHERE place_id = $1`, id)
	if err != nil {
		return p, nil, nil, err
	}
	for rows.Next() {
		var lang string
		var t PlaceText
		if err := rows.Scan(&lang, &t.Name, &t.Tagline, &t.Description, &t.Address, &t.History, &t.Facts, &t.Sources); err != nil {
			rows.Close()
			return p, nil, nil, err
		}
		texts[lang] = t
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, nil, nil, err
	}
	rules, err := s.Rules(ctx, []string{id})
	return p, texts, rules[id], err
}

// SavePlace — joyni yaratadi (isNew) yoki to'liq yangilaydi; matnlar almashtiriladi; audit yoziladi.
func (s *Store) SavePlace(ctx context.Context, adminID string, p PlaceRecord, texts map[string]PlaceText, isNew bool, before any) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		hours, _ := json.Marshal(p.OpeningHours)
		images, _ := json.Marshal(p.Images)
		var fee []byte
		if len(p.EntryFee) > 0 && string(p.EntryFee) != "null" {
			fee = p.EntryFee
		}
		if isNew {
			_, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, opened_at, status, lat, lng, geofence_radius_m,
				interests, price_level, visit_duration_min, opening_hours, entry_fee, wheelchair, family_friendly, easy_walk,
				pay_card, cash_only, is_partner, images, source, created_by)
				VALUES ($1,$2,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
				p.ID, p.CityID, p.Category, p.Novelty, p.OpenedAt, p.Status, p.Location.Lat, p.Location.Lng, p.GeofenceRadiusM,
				p.Interests, p.PriceLevel, p.VisitMin, hours, fee, p.Wheelchair, p.FamilyFriendly, p.EasyWalk,
				p.PayCard, p.CashOnly, p.IsPartner, images, p.Source, adminID)
			if err != nil {
				if strings.Contains(err.Error(), "places_pkey") {
					return ErrConflict
				}
				return err
			}
		} else {
			tag, err := tx.Exec(ctx, `UPDATE places SET city_id=$2, category=$3, novelty=$4, opened_at=$5::date, lat=$6, lng=$7,
				geofence_radius_m=$8, interests=$9, price_level=$10, visit_duration_min=$11, opening_hours=$12, entry_fee=$13,
				wheelchair=$14, family_friendly=$15, easy_walk=$16, pay_card=$17, cash_only=$18, is_partner=$19, images=$20,
				updated_at=now() WHERE id=$1`,
				p.ID, p.CityID, p.Category, p.Novelty, p.OpenedAt, p.Location.Lat, p.Location.Lng, p.GeofenceRadiusM,
				p.Interests, p.PriceLevel, p.VisitMin, hours, fee, p.Wheelchair, p.FamilyFriendly, p.EasyWalk,
				p.PayCard, p.CashOnly, p.IsPartner, images)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM place_i18n WHERE place_id = $1`, p.ID); err != nil {
			return err
		}
		for lang, t := range texts {
			facts := t.Facts
			if facts == nil {
				facts = []string{}
			}
			src := t.Sources
			if len(src) == 0 || string(src) == "null" {
				src = json.RawMessage("[]")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address,
				history_summary, history_facts, sources) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				p.ID, lang, t.Name, t.Tagline, t.Description, t.Address, t.History, facts, src); err != nil {
				return err
			}
		}
		action := "place.update"
		if isNew {
			action = "place.create"
		}
		return audit(ctx, tx, adminID, action, "place", p.ID, before, map[string]any{"place": p, "texts": texts})
	})
}

var ErrConflict = errors.New("conflict")

// SetPlaceStatus — publish / reject / pending / draft.
func (s *Store) SetPlaceStatus(ctx context.Context, adminID, id, status string, reason *string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var prev string
		if err := tx.QueryRow(ctx, `SELECT status FROM places WHERE id = $1 FOR UPDATE`, id).Scan(&prev); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE places SET status = $2, reject_reason = $3, updated_at = now(),
			published_at = CASE WHEN $2 = 'published' THEN COALESCE(published_at, now()) ELSE published_at END
			WHERE id = $1`, id, status, reason)
		if err != nil {
			return err
		}
		action := map[string]string{"published": "place.publish", "rejected": "place.reject"}[status]
		if action == "" {
			action = "place.status"
		}
		return audit(ctx, tx, adminID, action, "place", id, map[string]string{"status": prev},
			map[string]any{"status": status, "reason": reason})
	})
}

type RuleInput struct {
	Code       string            `json:"code"`
	Severity   string            `json:"severity"`
	Priority   int               `json:"priority"`
	Text       map[string]string `json:"text"`
	VerifiedBy *string           `json:"verifiedBy"`
}

// ReplaceRules — qoidalarni almashtiradi va rules_version ni oshiradi (mijozlar geofence ni yangilaydi).
func (s *Store) ReplaceRules(ctx context.Context, adminID, placeID string, rules []RuleInput) (int, error) {
	var version int
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE places SET rules_version = rules_version + 1, updated_at = now() WHERE id = $1 RETURNING rules_version`,
			placeID).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		before, err := s.Rules(ctx, []string{placeID})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM place_rules WHERE place_id = $1`, placeID); err != nil {
			return err
		}
		for _, r := range rules {
			txt, _ := json.Marshal(r.Text)
			var verifiedAt *time.Time
			if r.VerifiedBy != nil && *r.VerifiedBy != "" {
				now := time.Now()
				verifiedAt = &now
			}
			if _, err := tx.Exec(ctx, `INSERT INTO place_rules (place_id, code, severity, priority, text, verified_by, verified_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`, placeID, r.Code, r.Severity, r.Priority, txt, r.VerifiedBy, verifiedAt); err != nil {
				return err
			}
		}
		return audit(ctx, tx, adminID, "place.rules", "place", placeID, before[placeID], rules)
	})
	return version, err
}

// AppendPlaceImage — yuklangan rasmni ro'yxat oxiriga qo'shadi.
func (s *Store) AppendPlaceImage(ctx context.Context, adminID, placeID string, img domain.Image) ([]domain.Image, error) {
	var images []domain.Image
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		b, _ := json.Marshal([]domain.Image{img})
		err := tx.QueryRow(ctx, `UPDATE places SET images = images || $2::jsonb, updated_at = now() WHERE id = $1 RETURNING images`,
			placeID, b).Scan(&images)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, adminID, "place.image", "place", placeID, nil, img)
	})
	return images, err
}

// ---------- Ro'yxat va analitika ----------

type AdminPlaceRow struct {
	ID          string    `json:"id"`
	CityID      string    `json:"cityId"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Novelty     string    `json:"novelty"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	Langs       []string  `json:"langs"`
	RulesCount  int       `json:"rulesCount"`
	ImagesCount int       `json:"imagesCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AdminPlaceFilter struct {
	Status, City, Q string
	Limit, Offset   int
}

func (s *Store) AdminPlaces(ctx context.Context, f AdminPlaceFilter) ([]AdminPlaceRow, error) {
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"true"}
	if f.Status != "" {
		where = append(where, "p.status = "+arg(f.Status))
	}
	if f.City != "" {
		where = append(where, "p.city_id = "+arg(f.City))
	}
	if f.Q != "" {
		where = append(where, "(p.id ILIKE "+arg("%"+escapeLike(f.Q)+"%")+
			" OR EXISTS (SELECT 1 FROM place_i18n q WHERE q.place_id = p.id AND q.name ILIKE $"+fmt.Sprint(len(args))+"))")
	}
	rows, err := s.DB.Query(ctx, `SELECT p.id, p.city_id,
		COALESCE((SELECT name FROM place_i18n i WHERE i.place_id = p.id ORDER BY array_position(ARRAY['uz','en','ru'], i.lang) LIMIT 1), p.id),
		p.category, p.novelty, p.status, p.source,
		COALESCE((SELECT array_agg(lang ORDER BY lang) FROM place_i18n i WHERE i.place_id = p.id), '{}'),
		(SELECT count(*) FROM place_rules r WHERE r.place_id = p.id), jsonb_array_length(p.images), p.updated_at
		FROM places p WHERE `+strings.Join(where, " AND ")+`
		ORDER BY CASE p.status WHEN 'pending' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, p.updated_at DESC, p.id
		LIMIT `+arg(f.Limit)+` OFFSET `+arg(f.Offset), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminPlaceRow{}
	for rows.Next() {
		var r AdminPlaceRow
		if err := rows.Scan(&r.ID, &r.CityID, &r.Name, &r.Category, &r.Novelty, &r.Status, &r.Source, &r.Langs,
			&r.RulesCount, &r.ImagesCount, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type NoveltyKPI struct {
	CityID               string   `json:"cityId"`
	PlacesPublished      int      `json:"placesPublished"`
	NovelPlacesPublished int      `json:"novelPlacesPublished"`
	PendingPlaces        int      `json:"pendingPlaces"`
	Trips                int      `json:"trips"`
	AvgTripNoveltyShare  *float64 `json:"avgTripNoveltyShare"`
	Visits               int      `json:"visits"`
	NovelVisitShare      *float64 `json:"novelVisitShare"` // asosiy KPI: yangi joylarga tashrif ulushi (maqsad 20–30%)
}

func (s *Store) NoveltyAnalytics(ctx context.Context, since time.Time) ([]NoveltyKPI, error) {
	rows, err := s.DB.Query(ctx, `SELECT c.id,
		(SELECT count(*) FROM places p WHERE p.city_id = c.id AND p.status = 'published'),
		(SELECT count(*) FROM places p WHERE p.city_id = c.id AND p.status = 'published' AND p.novelty <> 'iconic'),
		(SELECT count(*) FROM places p WHERE p.city_id = c.id AND p.status = 'pending'),
		(SELECT count(*) FROM trips t WHERE t.city_id = c.id AND t.created_at >= $1),
		(SELECT round(avg(t.novelty_share)::numeric, 3)::float8 FROM trips t WHERE t.city_id = c.id AND t.created_at >= $1),
		(SELECT count(*) FROM visits v JOIN places p ON p.id = v.place_id WHERE p.city_id = c.id AND v.created_at >= $1),
		(SELECT round(avg(CASE WHEN p.novelty <> 'iconic' AND NOT p.is_partner THEN 1 ELSE 0 END)::numeric, 3)::float8
		   FROM visits v JOIN places p ON p.id = v.place_id WHERE p.city_id = c.id AND v.created_at >= $1)
		FROM cities c ORDER BY c.id`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NoveltyKPI{}
	for rows.Next() {
		var k NoveltyKPI
		if err := rows.Scan(&k.CityID, &k.PlacesPublished, &k.NovelPlacesPublished, &k.PendingPlaces, &k.Trips,
			&k.AvgTripNoveltyShare, &k.Visits, &k.NovelVisitShare); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
