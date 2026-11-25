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

type Service struct {
	ID        string                `json:"id"`
	CityID    string                `json:"cityId"`
	Kind      string                `json:"kind"`
	Name      map[string]string     `json:"name"`
	Address   map[string]string     `json:"address"`
	Loc       domain.LatLng         `json:"location"`
	Phone     *string               `json:"phone"`
	Hours     []domain.OpeningHours `json:"hours"`
	Open24h   bool                  `json:"open24h"`
	Accepts   []string              `json:"accepts"`
	Source    string                `json:"source"`
	UpdatedAt time.Time             `json:"updatedAt"`
	DistanceM *float64              `json:"-"`
}

const serviceCols = `s.id, s.city_id, s.kind, s.name, s.address, s.lat, s.lng, s.phone, s.hours, s.open_24h, s.accepts, s.source, s.updated_at`

func scanService(row pgx.Row, extra ...any) (Service, error) {
	var v Service
	err := row.Scan(append([]any{&v.ID, &v.CityID, &v.Kind, &v.Name, &v.Address, &v.Loc.Lat, &v.Loc.Lng, &v.Phone, &v.Hours,
		&v.Open24h, &v.Accepts, &v.Source, &v.UpdatedAt}, extra...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

type ServiceFilter struct {
	City    string
	Kinds   []string
	Near    *domain.LatLng
	RadiusM float64
	Limit   int
}

func (s *Store) Services(ctx context.Context, f ServiceFilter) ([]Service, error) {
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"true"}
	if f.City != "" {
		where = append(where, "s.city_id = "+arg(f.City))
	}
	if len(f.Kinds) > 0 {
		where = append(where, "s.kind = ANY("+arg(f.Kinds)+")")
	}
	dist, order := "NULL::float8", "s.kind, s.id"
	if f.Near != nil {
		pt := "ll_to_earth(" + arg(f.Near.Lat) + ", " + arg(f.Near.Lng) + ")"
		dist = "earth_distance(" + pt + ", ll_to_earth(s.lat, s.lng))"
		if f.RadiusM > 0 {
			r := arg(f.RadiusM)
			where = append(where, "earth_box("+pt+", "+r+") @> ll_to_earth(s.lat, s.lng)", dist+" <= "+r)
		}
		order = dist + ", s.id"
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	rows, err := s.DB.Query(ctx, `SELECT `+serviceCols+`, `+dist+` FROM city_services s WHERE `+strings.Join(where, " AND ")+
		` ORDER BY `+order+` LIMIT `+arg(f.Limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		var d *float64
		v, err := scanService(rows, &d)
		if err != nil {
			return nil, err
		}
		v.DistanceM = d
		out = append(out, v)
	}
	return out, rows.Err()
}

// NearestService — har tur bo'yicha eng yaqini (radiussiz).
func (s *Store) NearestService(ctx context.Context, city, kind string, near domain.LatLng) (Service, error) {
	list, err := s.Services(ctx, ServiceFilter{City: city, Kinds: []string{kind}, Near: &near, Limit: 1})
	if err != nil {
		return Service{}, err
	}
	if len(list) == 0 {
		return Service{}, ErrNotFound
	}
	return list[0], nil
}

func (s *Store) Service(ctx context.Context, id string) (Service, error) {
	return scanService(s.DB.QueryRow(ctx, `SELECT `+serviceCols+` FROM city_services s WHERE s.id = $1`, id))
}

func (s *Store) SaveService(ctx context.Context, adminID string, v Service, isNew bool, before any) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		name, _ := json.Marshal(v.Name)
		addr, _ := json.Marshal(v.Address)
		hours, _ := json.Marshal(v.Hours)
		if isNew {
			_, err := tx.Exec(ctx, `INSERT INTO city_services (id, city_id, kind, name, address, lat, lng, phone, hours, open_24h, accepts, source, verified_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'manual',now())`,
				v.ID, v.CityID, v.Kind, name, addr, v.Loc.Lat, v.Loc.Lng, v.Phone, hours, v.Open24h, v.Accepts)
			if err != nil && strings.Contains(err.Error(), "city_services_pkey") {
				return ErrConflict
			}
			if err != nil {
				return err
			}
		} else {
			tag, err := tx.Exec(ctx, `UPDATE city_services SET city_id=$2, kind=$3, name=$4, address=$5, lat=$6, lng=$7, phone=$8,
				hours=$9, open_24h=$10, accepts=$11, source='manual', verified_at=now(), updated_at=now() WHERE id=$1`,
				v.ID, v.CityID, v.Kind, name, addr, v.Loc.Lat, v.Loc.Lng, v.Phone, hours, v.Open24h, v.Accepts)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
		action := map[bool]string{true: "service.create", false: "service.update"}[isNew]
		return audit(ctx, tx, adminID, action, "service", v.ID, before, v)
	})
}

func (s *Store) DeleteService(ctx context.Context, adminID, id string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM city_services WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return audit(ctx, tx, adminID, "service.delete", "service", id, nil, nil)
	})
}

// OfflineVersion — oflayn paket versiyasi: joylar, qoidalar va xizmatlarning oxirgi o'zgarishi va soni.
func (s *Store) OfflineVersion(ctx context.Context, cityID string) (string, error) {
	var places, services int
	var pmax, smax *time.Time
	err := s.DB.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM places WHERE city_id = $1 AND status = 'published'),
		(SELECT max(updated_at) FROM places WHERE city_id = $1),
		(SELECT count(*) FROM city_services WHERE city_id = $1),
		(SELECT max(updated_at) FROM city_services WHERE city_id = $1)`, cityID).Scan(&places, &pmax, &services, &smax)
	if err != nil {
		return "", err
	}
	ts := func(t *time.Time) int64 {
		if t == nil {
			return 0
		}
		return t.UnixNano()
	}
	return fmt.Sprintf("%d.%d.%d.%d", places, ts(pmax), services, ts(smax)), nil
}
