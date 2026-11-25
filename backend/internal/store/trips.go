package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"yoldosh/internal/domain"
)

type Trip struct {
	ID           uuid.UUID
	UserID       string
	CityID       string
	Status       string
	StartDate    time.Time
	Prefs        json.RawMessage
	Summary      *string
	NoveltyShare float64
	Planner      string
	CreatedAt    time.Time
	Days         []TripDay
}

type TripDay struct {
	Index int
	Date  time.Time
	Stops []Stop
}

type Stop struct {
	ID     uuid.UUID
	Status string
	domain.PlanStop
}

// PlanDays — invariant tekshiruvi uchun.
func (t Trip) PlanDays() []domain.PlanDay {
	out := make([]domain.PlanDay, len(t.Days))
	for i, d := range t.Days {
		out[i] = domain.PlanDay{Index: d.Index}
		for _, s := range d.Stops {
			out[i].Stops = append(out[i].Stops, s.PlanStop)
		}
	}
	return out
}

func (s *Store) CreateTrip(ctx context.Context, t *Trip) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO trips (id, user_id, city_id, status, start_date, prefs, summary, novelty_share, planner)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			t.ID, t.UserID, t.CityID, t.Status, t.StartDate, t.Prefs, t.Summary, t.NoveltyShare, t.Planner); err != nil {
			return err
		}
		for _, d := range t.Days {
			if _, err := tx.Exec(ctx, `INSERT INTO trip_days (trip_id, idx, date) VALUES ($1,$2,$3)`, t.ID, d.Index, d.Date); err != nil {
				return err
			}
			if err := insertStops(ctx, tx, t.ID, d.Index, d.Stops); err != nil {
				return err
			}
		}
		return nil
	})
}

func insertStops(ctx context.Context, tx pgx.Tx, tripID uuid.UUID, day int, stops []Stop) error {
	for pos, st := range stops {
		var transit []byte
		if st.Transit != nil {
			transit, _ = json.Marshal(st.Transit)
		}
		var reasonText *string
		if st.ReasonText != "" {
			reasonText = &st.ReasonText
		}
		status := st.Status
		if status == "" {
			status = "upcoming"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stops (id, trip_id, day_idx, position, kind, place_id, arrive_min, duration_min,
			reason_code, reason_arg, reason_text, transit, status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			st.ID, tripID, day, pos, st.Kind, st.PlaceID, st.ArriveMin, st.DurationMin,
			st.ReasonCode, st.ReasonArg, reasonText, transit, status); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceDayStops — bir kunning to'xtashlarini almashtiradi va ulushni yangilaydi.
func (s *Store) ReplaceDayStops(ctx context.Context, tripID uuid.UUID, day int, stops []Stop, share float64) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM stops WHERE trip_id = $1 AND day_idx = $2`, tripID, day); err != nil {
			return err
		}
		if err := insertStops(ctx, tx, tripID, day, stops); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE trips SET novelty_share = $2, updated_at = now() WHERE id = $1`, tripID, share)
		return err
	})
}

// Trip — foydalanuvchining marshruti (boshqa foydalanuvchiniki bo'lsa ErrNotFound).
func (s *Store) Trip(ctx context.Context, id uuid.UUID, userID string) (Trip, error) {
	var t Trip
	err := s.DB.QueryRow(ctx, `SELECT id, user_id, city_id, status, start_date, prefs, summary, novelty_share, planner, created_at
		FROM trips WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&t.ID, &t.UserID, &t.CityID, &t.Status, &t.StartDate, &t.Prefs, &t.Summary, &t.NoveltyShare, &t.Planner, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	rows, err := s.DB.Query(ctx, `SELECT idx, date FROM trip_days WHERE trip_id = $1 ORDER BY idx`, id)
	if err != nil {
		return t, err
	}
	for rows.Next() {
		var d TripDay
		if err := rows.Scan(&d.Index, &d.Date); err != nil {
			rows.Close()
			return t, err
		}
		t.Days = append(t.Days, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return t, err
	}

	rows, err = s.DB.Query(ctx, `SELECT s.id, s.day_idx, s.kind, s.place_id, p.novelty, p.is_partner, s.arrive_min, s.duration_min,
		s.reason_code, s.reason_arg, COALESCE(s.reason_text, ''), s.transit, s.status
		FROM stops s JOIN places p ON p.id = s.place_id WHERE s.trip_id = $1 ORDER BY s.day_idx, s.position`, id)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	for rows.Next() {
		var st Stop
		var day int
		var transit []byte
		if err := rows.Scan(&st.ID, &day, &st.Kind, &st.PlaceID, &st.Novelty, &st.IsPartner, &st.ArriveMin, &st.DurationMin,
			&st.ReasonCode, &st.ReasonArg, &st.ReasonText, &transit, &st.Status); err != nil {
			return t, err
		}
		if len(transit) > 0 {
			st.Transit = &domain.Transit{}
			if err := json.Unmarshal(transit, st.Transit); err != nil {
				return t, err
			}
		}
		if day >= 0 && day < len(t.Days) {
			t.Days[day].Stops = append(t.Days[day].Stops, st)
		}
	}
	return t, rows.Err()
}

type TripListItem struct {
	ID           uuid.UUID
	CityID       string
	Status       string
	StartDate    time.Time
	Days         int
	NoveltyShare float64
	CreatedAt    time.Time
}

func (s *Store) Trips(ctx context.Context, userID string, limit int) ([]TripListItem, error) {
	rows, err := s.DB.Query(ctx, `SELECT t.id, t.city_id, t.status, t.start_date,
		(SELECT count(*) FROM trip_days d WHERE d.trip_id = t.id), t.novelty_share, t.created_at
		FROM trips t WHERE t.user_id = $1 ORDER BY t.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TripListItem
	for rows.Next() {
		var it TripListItem
		if err := rows.Scan(&it.ID, &it.CityID, &it.Status, &it.StartDate, &it.Days, &it.NoveltyShare, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) SetStopStatus(ctx context.Context, tripID, stopID uuid.UUID, status string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE stops SET status = $3 WHERE trip_id = $1 AND id = $2`, tripID, stopID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.DB.Exec(ctx, `UPDATE trips SET updated_at = now() WHERE id = $1`, tripID)
	return err
}
