// Package store — PostgreSQL so'rovlari. Faqat parametrli SQL.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
	"yoldosh/internal/planner"
)

var ErrNotFound = errors.New("not found")

type Store struct{ DB *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{DB: db} }

// ---------- Shaharlar ----------

type City struct {
	ID             string
	Name           string
	Country        string
	CoverURL       *string
	Center         domain.LatLng
	Timezone       string
	TaxiBase       int64
	TaxiPerKm      int64
	Emergency      map[string]string
	PlacesCount    int
	NewPlacesCount int
}

const citySelect = `
SELECT c.id, ci.name, c.country, c.cover_url, c.lat, c.lng, c.timezone, c.taxi_base_fare, c.taxi_per_km, c.emergency,
  (SELECT count(*) FROM places p WHERE p.city_id = c.id AND p.status = 'published'),
  (SELECT count(*) FROM places p WHERE p.city_id = c.id AND p.status = 'published' AND p.novelty IN ('new','hidden'))
FROM cities c
CROSS JOIN LATERAL (
  SELECT name FROM city_i18n i WHERE i.city_id = c.id AND i.lang = ANY($1::text[])
  ORDER BY array_position($1::text[], i.lang) LIMIT 1) ci`

func scanCity(row pgx.Row) (City, error) {
	var c City
	err := row.Scan(&c.ID, &c.Name, &c.Country, &c.CoverURL, &c.Center.Lat, &c.Center.Lng, &c.Timezone,
		&c.TaxiBase, &c.TaxiPerKm, &c.Emergency, &c.PlacesCount, &c.NewPlacesCount)
	return c, err
}

func (s *Store) Cities(ctx context.Context, langs []string) ([]City, error) {
	rows, err := s.DB.Query(ctx, citySelect+` ORDER BY c.id`, langs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []City
	for rows.Next() {
		c, err := scanCity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) City(ctx context.Context, id string, langs []string) (City, error) {
	c, err := scanCity(s.DB.QueryRow(ctx, citySelect+` WHERE c.id = $2`, langs, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// ---------- Joylar ----------

type Place struct {
	ID              string
	CityID          string
	Name            string
	Tagline         string
	Category        string
	Novelty         domain.Novelty
	OpenedAt        *time.Time
	Images          []domain.Image
	Loc             domain.LatLng
	Rating          float64
	Reviews         int
	PriceLevel      int
	VisitMin        int
	Hours           []domain.OpeningHours
	IsPartner       bool
	Interests       []string
	Wheelchair      bool
	FamilyFriendly  bool
	EasyWalk        bool
	PayCard         bool
	CashOnly        bool
	GeofenceRadiusM int
	RulesVersion    int
	Status          string
	DistanceM       *float64

	// faqat batafsil so'rovda
	Description    string
	Address        string
	HistorySummary *string
	HistoryFacts   []string
	Sources        json.RawMessage
	EntryFee       json.RawMessage
}

const placeCols = `p.id, p.city_id, t.name, t.tagline, p.category, p.novelty, p.opened_at, p.images, p.lat, p.lng,
  p.rating::float8, p.reviews_count, p.price_level, p.visit_duration_min, p.opening_hours, p.is_partner, p.interests,
  p.wheelchair, p.family_friendly, p.easy_walk, p.pay_card, p.cash_only, p.geofence_radius_m, p.rules_version, p.status`

// placeText — $1 = til zanjiri.
const placeText = `CROSS JOIN LATERAL (
  SELECT name, tagline, description, address, history_summary, history_facts, sources
  FROM place_i18n i WHERE i.place_id = p.id AND i.lang = ANY($1::text[])
  ORDER BY array_position($1::text[], i.lang) LIMIT 1) t`

func placeDest(p *Place) []any {
	return []any{&p.ID, &p.CityID, &p.Name, &p.Tagline, &p.Category, &p.Novelty, &p.OpenedAt, &p.Images,
		&p.Loc.Lat, &p.Loc.Lng, &p.Rating, &p.Reviews, &p.PriceLevel, &p.VisitMin, &p.Hours, &p.IsPartner, &p.Interests,
		&p.Wheelchair, &p.FamilyFriendly, &p.EasyWalk, &p.PayCard, &p.CashOnly, &p.GeofenceRadiusM, &p.RulesVersion, &p.Status}
}

type PlaceFilter struct {
	City       string
	Category   string
	Novelty    string // iconic | new | hidden | novel (= new + hidden)
	Interest   string
	Q          string
	Near       *domain.LatLng
	RadiusM    float64
	Wheelchair bool
	Limit      int
	Offset     int
}

func (s *Store) ListPlaces(ctx context.Context, f PlaceFilter, langs []string) ([]Place, error) {
	args := []any{langs}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"p.status = 'published'"}
	if f.City != "" {
		where = append(where, "p.city_id = "+arg(f.City))
	}
	if f.Category != "" {
		where = append(where, "p.category = "+arg(f.Category))
	}
	switch f.Novelty {
	case "":
	case "novel":
		where = append(where, "p.novelty IN ('new','hidden')")
	default:
		where = append(where, "p.novelty = "+arg(f.Novelty))
	}
	if f.Interest != "" {
		where = append(where, arg(f.Interest)+" = ANY(p.interests)")
	}
	if f.Q != "" {
		where = append(where, "EXISTS (SELECT 1 FROM place_i18n q WHERE q.place_id = p.id AND (q.name ILIKE "+
			arg("%"+escapeLike(f.Q)+"%")+" OR q.tagline ILIKE $"+fmt.Sprint(len(args))+"))")
	}
	if f.Wheelchair {
		where = append(where, "p.wheelchair")
	}
	dist := "NULL::float8"
	order := "CASE p.novelty WHEN 'iconic' THEN 1 ELSE 0 END, p.rating DESC, p.id"
	if f.Near != nil {
		pt := "ll_to_earth(" + arg(f.Near.Lat) + ", " + arg(f.Near.Lng) + ")"
		dist = "earth_distance(" + pt + ", ll_to_earth(p.lat, p.lng))"
		if f.RadiusM > 0 {
			r := arg(f.RadiusM)
			where = append(where, "earth_box("+pt+", "+r+") @> ll_to_earth(p.lat, p.lng)", dist+" <= "+r)
		}
		order = dist + ", p.id"
	}
	sql := fmt.Sprintf(`SELECT %s, %s FROM places p %s WHERE %s ORDER BY %s LIMIT %s OFFSET %s`,
		placeCols, dist, placeText, strings.Join(where, " AND "), order, arg(f.Limit), arg(f.Offset))
	rows, err := s.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(append(placeDest(&p), &p.DistanceM)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Place — bitta joy. published bo'lmasa ham qaytaradi (admin uchun); chaqiruvchi status ni tekshiradi.
func (s *Store) Place(ctx context.Context, id string, langs []string) (Place, error) {
	var p Place
	dest := append(placeDest(&p), &p.Description, &p.Address, &p.HistorySummary, &p.HistoryFacts, &p.Sources, &p.EntryFee)
	err := s.DB.QueryRow(ctx, `SELECT `+placeCols+`, t.description, t.address, t.history_summary, t.history_facts, t.sources, p.entry_fee
		FROM places p `+placeText+` WHERE p.id = $2`, langs, id).Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) PlacesByIDs(ctx context.Context, ids []string, langs []string) (map[string]Place, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+placeCols+` FROM places p `+placeText+` WHERE p.id = ANY($2::text[])`, langs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Place{}
	for rows.Next() {
		var p Place
		if err := rows.Scan(placeDest(&p)...); err != nil {
			return nil, err
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

// Candidates — rejalashtirgich uchun shahardagi barcha chop etilgan joylar (matnsiz).
func (s *Store) Candidates(ctx context.Context, cityID string) ([]planner.Candidate, float64, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, category, novelty, is_partner, interests, lat, lng, rating::float8, reviews_count,
		visit_duration_min, opening_hours, price_level, wheelchair, family_friendly
		FROM places WHERE city_id = $1 AND status = 'published' ORDER BY id`, cityID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []planner.Candidate
	sum := 0.0
	for rows.Next() {
		var c planner.Candidate
		if err := rows.Scan(&c.ID, &c.Category, &c.Novelty, &c.IsPartner, &c.Interests, &c.Loc.Lat, &c.Loc.Lng,
			&c.Rating, &c.Reviews, &c.VisitMin, &c.Hours, &c.PriceLevel, &c.Wheelchair, &c.FamilyFriendly); err != nil {
			return nil, 0, err
		}
		sum += c.Rating
		out = append(out, c)
	}
	avg := 4.3
	if len(out) > 0 {
		avg = sum / float64(len(out))
	}
	return out, avg, rows.Err()
}

// ---------- Qoidalar va geofence ----------

type Rule struct {
	PlaceID    string
	Code       string
	Severity   string
	Priority   int
	Text       map[string]string
	VerifiedBy *string
	VerifiedAt *time.Time
}

// Rules — joylar qoidalari: forbidden > warn > info, keyin priority.
func (s *Store) Rules(ctx context.Context, placeIDs []string) (map[string][]Rule, error) {
	rows, err := s.DB.Query(ctx, `SELECT place_id, code, severity, priority, text, verified_by, verified_at
		FROM place_rules WHERE place_id = ANY($1::text[])
		ORDER BY place_id, CASE severity WHEN 'forbidden' THEN 0 WHEN 'warn' THEN 1 ELSE 2 END, priority DESC, id`, placeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.PlaceID, &r.Code, &r.Severity, &r.Priority, &r.Text, &r.VerifiedBy, &r.VerifiedAt); err != nil {
			return nil, err
		}
		out[r.PlaceID] = append(out[r.PlaceID], r)
	}
	return out, rows.Err()
}

// GeofencePlaces — shahardagi qoidasi bor chop etilgan joylar.
func (s *Store) GeofencePlaces(ctx context.Context, cityID string, langs []string) ([]Place, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+placeCols+` FROM places p `+placeText+`
		WHERE p.city_id = $2 AND p.status = 'published' AND EXISTS (SELECT 1 FROM place_rules r WHERE r.place_id = p.id)
		ORDER BY p.id`, langs, cityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(placeDest(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------- Foydalanuvchilar ----------

type User struct {
	ID            string
	Lang          string
	Role          string
	Interests     []string
	Companion     *string
	Pace          *string
	Budget        *int
	DisplayName   *string
	Email         *string
	EmailVerified bool
	GoogleSub     *string
	AvatarURL     *string
}

// DeviceSession — qurilma bo'yicha foydalanuvchini topadi yoki yaratadi.
func (s *Store) DeviceSession(ctx context.Context, deviceID, platform, lang string) (User, error) {
	var u User
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var userID string
		var registered bool
		err := tx.QueryRow(ctx, `UPDATE devices d SET last_seen_at = now(), platform = $2 FROM users u
			WHERE d.id = $1 AND u.id = d.user_id RETURNING d.user_id, (u.email IS NOT NULL OR u.google_sub IS NOT NULL)`,
			deviceID, platform).Scan(&userID, &registered)
		if err == nil && registered {
			// Qurilma ro'yxatdan o'tgan hisobga bog'langan: faqat deviceId bilan hisobga kirib bo'lmaydi
			// (hisobga refresh token yoki qayta login orqali kiriladi). Qurilma yangi anonim sessiyaga o'tadi.
			userID = NewUserID()
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, lang) VALUES ($1, $2)`, userID, lang); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE devices SET user_id = $2 WHERE id = $1`, deviceID, userID)
			if err != nil {
				return err
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			userID = NewUserID()
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, lang) VALUES ($1, $2)`, userID, lang); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO devices (id, user_id, platform) VALUES ($1, $2, $3)`, deviceID, userID, platform); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		u, err = getUser(ctx, tx, userID)
		return err
	})
	return u, err
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getUser(ctx context.Context, q querier, id string) (User, error) {
	return getUserWhere(ctx, q, "id = $1", id)
}

func getUserWhere(ctx context.Context, q querier, where string, arg any) (User, error) {
	var u User
	err := q.QueryRow(ctx, `SELECT id, lang, role, interests, companion, pace, budget, display_name, email, email_verified,
		google_sub, avatar_url FROM users WHERE `+where, arg).
		Scan(&u.ID, &u.Lang, &u.Role, &u.Interests, &u.Companion, &u.Pace, &u.Budget, &u.DisplayName, &u.Email,
			&u.EmailVerified, &u.GoogleSub, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) User(ctx context.Context, id string) (User, error) { return getUser(ctx, s.DB, id) }

type UserPatch struct {
	Lang        *string
	Interests   *[]string
	Companion   *string
	Pace        *string
	Budget      *int
	DisplayName *string
}

func (s *Store) UpdateUser(ctx context.Context, id string, p UserPatch) (User, error) {
	_, err := s.DB.Exec(ctx, `UPDATE users SET
		lang = COALESCE($2, lang), interests = COALESCE($3, interests), companion = COALESCE($4, companion),
		pace = COALESCE($5, pace), budget = COALESCE($6, budget), display_name = COALESCE($7, display_name)
		WHERE id = $1`, id, p.Lang, p.Interests, p.Companion, p.Pace, p.Budget, p.DisplayName)
	if err != nil {
		return User{}, err
	}
	return s.User(ctx, id)
}

func (s *Store) UserStats(ctx context.Context, id string) (visited, newVisited int, err error) {
	err = s.DB.QueryRow(ctx, `SELECT count(DISTINCT v.place_id),
		count(DISTINCT v.place_id) FILTER (WHERE p.novelty IN ('new','hidden'))
		FROM visits v JOIN places p ON p.id = v.place_id WHERE v.user_id = $1`, id).Scan(&visited, &newVisited)
	return
}

// ---------- Tashriflar va sharhlar ----------

func (s *Store) AddVisit(ctx context.Context, userID, placeID string, tripID *uuid.UUID, rating *int, comment *string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO visits (id, user_id, place_id, trip_id, rating, comment) VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (user_id, place_id, visit_date) DO UPDATE
			SET rating = COALESCE(EXCLUDED.rating, visits.rating), comment = COALESCE(EXCLUDED.comment, visits.comment)`,
			uuid.Must(uuid.NewV7()), userID, placeID, tripID, rating, comment)
		if err != nil {
			return err
		}
		// Reyting foydalanuvchi baholaridan qayta hisoblanadi (seed dagi boshlang'ich qiymat baholar bo'lmaguncha qoladi).
		_, err = tx.Exec(ctx, `UPDATE places p SET rating = round(a.avg, 1), reviews_count = a.n, updated_at = now()
			FROM (SELECT avg(rating)::numeric AS avg, count(*) AS n FROM visits WHERE place_id = $1 AND rating IS NOT NULL) a
			WHERE p.id = $1 AND a.n >= 3`, placeID)
		return err
	})
}

type Review struct {
	ID        uuid.UUID
	Author    string
	Rating    int
	Text      string
	CreatedAt time.Time
}

func (s *Store) Reviews(ctx context.Context, placeID string, limit int) ([]Review, error) {
	rows, err := s.DB.Query(ctx, `SELECT v.id, COALESCE(u.display_name, ''), v.rating, v.comment, v.created_at
		FROM visits v JOIN users u ON u.id = v.user_id
		WHERE v.place_id = $1 AND v.rating IS NOT NULL AND v.comment IS NOT NULL AND v.comment <> ''
		ORDER BY v.created_at DESC LIMIT $2`, placeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.Author, &r.Rating, &r.Text, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
