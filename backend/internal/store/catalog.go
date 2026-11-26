package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"yoldosh/internal/domain"
)

// ---------- Narxlar ----------

type PriceItem struct {
	ID            string
	CityID        string
	Category      string
	Name          map[string]string
	Unit          map[string]string
	BaseMin       int64
	BaseMax       int64
	Tourist       *int64
	PlaceIDs      []string
	SourceURL     *string
	SourceYear    *int
	Note          *string
	BaseUpdatedAt time.Time
	Trust         string     // base_min/base_max ishonch darajasi (tadqiqot narxlari: sample)
	Observed      []int64    // tasdiqlangan kuzatuvlar (so'm)
	LastReport    *time.Time // oxirgi tasdiqlangan kuzatuv
}

// Range — adolatli oraliq, kuzatuvlar soni va yangilangan vaqt (domain.FairRange qoidasi bo'yicha).
func (p PriceItem) Range() (lo, hi int64, sample int, updated time.Time) {
	lo, hi = domain.FairRange(p.BaseMin, p.BaseMax, p.Observed)
	updated = p.BaseUpdatedAt
	if p.LastReport != nil && p.LastReport.After(updated) {
		updated = *p.LastReport
	}
	return lo, hi, len(p.Observed), updated
}

type PriceFilter struct {
	City     string
	Category string
	Q        string
	IDs      []string
}

const priceSelect = `SELECT i.id, i.city_id, i.category, i.name, i.unit, i.base_min, i.base_max, i.tourist, i.place_ids,
  i.source_url, i.source_year, i.note, i.base_updated_at, i.trust,
  COALESCE((SELECT array_agg(r.paid ORDER BY r.paid) FROM price_reports r WHERE r.price_item_id = i.id AND r.status = 'approved'), '{}'),
  (SELECT max(r.created_at) FROM price_reports r WHERE r.price_item_id = i.id AND r.status = 'approved')
FROM price_items i`

func (s *Store) Prices(ctx context.Context, f PriceFilter) ([]PriceItem, error) {
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"true"}
	if f.City != "" {
		where = append(where, "i.city_id = "+arg(f.City))
	}
	if f.Category != "" {
		where = append(where, "i.category = "+arg(f.Category))
	}
	if f.Q != "" {
		q := arg("%" + escapeLike(f.Q) + "%")
		where = append(where, "(i.name->>'uz' ILIKE "+q+" OR i.name->>'ru' ILIKE "+q+" OR i.name->>'en' ILIKE "+q+")")
	}
	if f.IDs != nil {
		where = append(where, "i.id = ANY("+arg(f.IDs)+"::text[])")
	}
	rows, err := s.DB.Query(ctx, priceSelect+` WHERE `+strings.Join(where, " AND ")+
		` ORDER BY i.city_id, array_position(ARRAY['ticket','taxi','transport','food','market','souvenir'], i.category), i.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceItem
	for rows.Next() {
		var p PriceItem
		if err := rows.Scan(&p.ID, &p.CityID, &p.Category, &p.Name, &p.Unit, &p.BaseMin, &p.BaseMax, &p.Tourist, &p.PlaceIDs,
			&p.SourceURL, &p.SourceYear, &p.Note, &p.BaseUpdatedAt, &p.Trust, &p.Observed, &p.LastReport); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PriceItem(ctx context.Context, id string) (PriceItem, error) {
	items, err := s.Prices(ctx, PriceFilter{IDs: []string{id}})
	if err != nil {
		return PriceItem{}, err
	}
	if len(items) == 0 {
		return PriceItem{}, ErrNotFound
	}
	return items[0], nil
}

type PriceReport struct {
	ID          uuid.UUID
	PriceItemID *string
	CityID      *string
	UserID      string // bo'sh = anonim
	Name        string
	Category    string
	Paid        int64
	Buyer       *string
	Loc         *domain.LatLng // ~100 m gacha yaxlitlanadi
	Status      string
}

func (s *Store) AddPriceReport(ctx context.Context, r PriceReport) error {
	var lat, lng *float64
	if r.Loc != nil {
		a, b := roundCoord(r.Loc.Lat), roundCoord(r.Loc.Lng)
		lat, lng = &a, &b
	}
	var user *string
	if r.UserID != "" {
		user = &r.UserID
	}
	// optionalUser token ni bazada tekshirmaydi: o'chirilgan hisob bo'lsa kuzatuv anonim saqlanadi.
	_, err := s.DB.Exec(ctx, `INSERT INTO price_reports (id, price_item_id, city_id, user_id, name, category, paid, buyer, lat, lng, status)
		VALUES ($1,$2,$3,(SELECT id FROM users WHERE id = $4),$5,$6,$7,$8,$9,$10,$11)`,
		r.ID, r.PriceItemID, r.CityID, user, r.Name, r.Category, r.Paid, r.Buyer, lat, lng, r.Status)
	return err
}

func roundCoord(v float64) float64 { return float64(int64(v*1000+0.5*sign(v))) / 1000 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// ---------- Tadbirlar ----------

type Event struct {
	ID        string
	CityID    string
	Kind      string
	Title     map[string]string
	Venue     map[string]string
	StartsAt  time.Time
	EndsAt    time.Time
	Loc       domain.LatLng
	Price     *int64
	IsFree    bool
	TicketURL *string
	SourceURL *string
	Recurring *string
	Image     *domain.Image
}

type EventFilter struct {
	City string
	From time.Time // shu vaqtdan keyin tugaydigan (o'tib ketganlari ko'rinmaydi)
	To   *time.Time
}

func (s *Store) Events(ctx context.Context, f EventFilter) ([]Event, error) {
	args := []any{f.From}
	where := []string{"status = 'published'", "ends_at >= $1"}
	if f.City != "" {
		args = append(args, f.City)
		where = append(where, fmt.Sprintf("city_id = $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("starts_at <= $%d", len(args)))
	}
	rows, err := s.DB.Query(ctx, `SELECT id, city_id, kind, title, venue, starts_at, ends_at, lat, lng, price, is_free,
		ticket_url, source_url, recurring, image FROM events WHERE `+strings.Join(where, " AND ")+` ORDER BY starts_at, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CityID, &e.Kind, &e.Title, &e.Venue, &e.StartsAt, &e.EndsAt, &e.Loc.Lat, &e.Loc.Lng,
			&e.Price, &e.IsFree, &e.TicketURL, &e.SourceURL, &e.Recurring, &e.Image); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------- Marshrut tahriri ----------

// SetStopRating — to'xtash bahosi (1-5). Visits ga yozish chaqiruvchida (AddVisit).
func (s *Store) SetStopRating(ctx context.Context, tripID, stopID uuid.UUID, rating int) error {
	tag, err := s.DB.Exec(ctx, `UPDATE stops SET rating = $3 WHERE trip_id = $1 AND id = $2`, tripID, stopID, rating)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceDays — bir nechta kunning to'xtashlarini bitta tranzaksiyada almashtiradi (to'xtashni boshqa kunga ko'chirish).
func (s *Store) ReplaceDays(ctx context.Context, tripID uuid.UUID, days map[int][]Stop, share float64) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		// Avval hamma kun tozalanadi: ko'chirilayotgan to'xtash id si ikki kunda bir vaqtda bo'lmasin.
		for day := range days {
			if _, err := tx.Exec(ctx, `DELETE FROM stops WHERE trip_id = $1 AND day_idx = $2`, tripID, day); err != nil {
				return err
			}
		}
		for day, stops := range days {
			if err := insertStops(ctx, tx, tripID, day, stops); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE trips SET novelty_share = $2, updated_at = now() WHERE id = $1`, tripID, share)
		return err
	})
}
