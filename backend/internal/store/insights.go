package store

import (
	"context"
	"encoding/json"
	"time"
)

// InsightsTrip — Safar Insights uchun bitta safar, faqat ruxsat etilgan anonim maydonlar
// (MONETIZATION.md §5): shahar, sana, kunlar, odamlar, byudjet, qiziqishlar, kim bilan, rezidentlik,
// transport va rejadagi joylar. Foydalanuvchi ID si, ism, email, joylashuv olinmaydi.
type InsightsTrip struct {
	StartDate time.Time
	Days      int
	Prefs     json.RawMessage
	Arrival   json.RawMessage
	Places    []string // kind = place to'xtashlari (takrorlarsiz emas; agregatsiyada bitta safarda bir marta sanaladi)
}

// InsightsTrips — hududdagi [from, to] sanalarda boshlanadigan safarlar; statistikadan chiqqan
// (analytics_opt_out) foydalanuvchilarniki kirmaydi. Anonim qurilma foydalanuvchilari ham sanaladi.
func (s *Store) InsightsTrips(ctx context.Context, region string, from, to time.Time) ([]InsightsTrip, error) {
	rows, err := s.DB.Query(ctx, `SELECT t.start_date,
		  (SELECT count(*) FROM trip_days d WHERE d.trip_id = t.id),
		  t.prefs, t.arrival,
		  COALESCE((SELECT array_agg(s.place_id ORDER BY s.day_idx, s.position) FROM stops s WHERE s.trip_id = t.id AND s.kind = 'place'), '{}')
		FROM trips t JOIN users u ON u.id = t.user_id
		WHERE t.city_id = $1 AND t.start_date BETWEEN $2::date AND $3::date AND t.status = 'ready' AND NOT u.analytics_opt_out`,
		region, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InsightsTrip
	for rows.Next() {
		var t InsightsTrip
		if err := rows.Scan(&t.StartDate, &t.Days, &t.Prefs, &t.Arrival, &t.Places); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CheapestStay — hududdagi eng arzon nashr qilingan turar joy (so'm/kecha, 2 kishi). Yo'q bo'lsa nil.
func (s *Store) CheapestStay(ctx context.Context, cityID string) (*int64, error) {
	var v *int64
	err := s.DB.QueryRow(ctx, `SELECT min(price_min) FROM stays WHERE city_id = $1 AND status = 'published' AND price_min IS NOT NULL`, cityID).Scan(&v)
	return v, err
}
