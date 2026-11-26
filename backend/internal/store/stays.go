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

type Stay struct {
	ID             string            `json:"id"`
	CityID         string            `json:"cityId"`
	Kind           string            `json:"kind"`
	Status         string            `json:"status"`
	Name           map[string]string `json:"name"`
	Loc            domain.LatLng     `json:"location"`
	Address        string            `json:"address"`
	Stars          *int              `json:"stars"`
	PriceMin       *int64            `json:"priceMin"`
	PriceMax       *int64            `json:"priceMax"`
	PriceTrust     *string           `json:"priceTrust"`
	PriceNote      *string           `json:"priceNote"`
	PriceUpdatedAt *time.Time        `json:"priceUpdatedAt"`
	PriceSource    *string           `json:"priceSource"` // narx manbasi (URL), tadqiqot yozuvlarida
	Amenities      []string          `json:"amenities"`
	Contacts       map[string]string `json:"contacts"`
	CheckIn        *string           `json:"checkIn"`
	CheckOut       *string           `json:"checkOut"`
	Wheelchair     *bool             `json:"wheelchair"`
	EMehmon        *bool             `json:"emehmon"`
	Rating         *float64          `json:"rating"`
	RatingSource   *string           `json:"ratingSource"` // reyting manbasi (URL); reyting 5 ballik
	ReviewsCount   int               `json:"reviewsCount"`
	Images         []domain.Image    `json:"images"`
	BookingURL     *string           `json:"bookingUrl"`
	AgodaURL       *string           `json:"agodaUrl"`
	IsPartner      bool              `json:"isPartner"`
	FairPrice      bool              `json:"fairPrice"`
	Verified       bool              `json:"verified"`
	Trust          string            `json:"trust"`
	Source         string            `json:"source"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	DistanceM      *float64          `json:"-"`
}

const stayCols = `s.id, s.city_id, s.kind, s.status, s.name, s.lat, s.lng, s.address, s.stars, s.price_min, s.price_max, s.price_trust,
  s.price_note, s.price_updated_at, s.amenities, s.contacts, s.check_in, s.check_out, s.wheelchair, s.emehmon, s.rating::float8,
  s.reviews_count, s.images, s.booking_url, s.agoda_url, s.is_partner, s.fair_price, s.verified, s.trust, s.source, s.updated_at,
  s.price_source, s.rating_source`

func scanStay(row pgx.Row, extra ...any) (Stay, error) {
	var v Stay
	err := row.Scan(append([]any{&v.ID, &v.CityID, &v.Kind, &v.Status, &v.Name, &v.Loc.Lat, &v.Loc.Lng, &v.Address, &v.Stars,
		&v.PriceMin, &v.PriceMax, &v.PriceTrust, &v.PriceNote, &v.PriceUpdatedAt, &v.Amenities, &v.Contacts, &v.CheckIn,
		&v.CheckOut, &v.Wheelchair, &v.EMehmon, &v.Rating, &v.ReviewsCount, &v.Images, &v.BookingURL, &v.AgodaURL,
		&v.IsPartner, &v.FairPrice, &v.Verified, &v.Trust, &v.Source, &v.UpdatedAt, &v.PriceSource, &v.RatingSource}, extra...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

type StayFilter struct {
	City     string
	Kinds    []string
	Status   string // bo'sh = published
	Near     *domain.LatLng
	RadiusM  float64
	MaxPrice int64
	Q        string
	Limit    int
	Offset   int
}

func (s *Store) Stays(ctx context.Context, f StayFilter) ([]Stay, error) {
	args := []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	status := f.Status
	if status == "" {
		status = "published"
	}
	where := []string{"s.status = " + arg(status)}
	if f.City != "" {
		where = append(where, "s.city_id = "+arg(f.City))
	}
	if len(f.Kinds) > 0 {
		where = append(where, "s.kind = ANY("+arg(f.Kinds)+")")
	}
	if f.MaxPrice > 0 {
		where = append(where, "s.price_min <= "+arg(f.MaxPrice))
	}
	if f.Q != "" {
		where = append(where, "s.name::text ILIKE "+arg("%"+escapeLike(f.Q)+"%"))
	}
	dist := "NULL::float8"
	// saralash: hamkor/tekshirilgan, aniq narxli, reytingli, keyin nom
	order := "s.verified DESC, (s.price_trust <> 'estimate') DESC, s.rating DESC NULLS LAST, s.stars DESC NULLS LAST, s.id"
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
		f.Limit = 20
	}
	rows, err := s.DB.Query(ctx, `SELECT `+stayCols+`, `+dist+` FROM stays s WHERE `+strings.Join(where, " AND ")+
		` ORDER BY `+order+` LIMIT `+arg(f.Limit)+` OFFSET `+arg(f.Offset), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Stay{}
	for rows.Next() {
		var d *float64
		v, err := scanStay(rows, &d)
		if err != nil {
			return nil, err
		}
		v.DistanceM = d
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) Stay(ctx context.Context, id string) (Stay, error) {
	return scanStay(s.DB.QueryRow(ctx, `SELECT `+stayCols+` FROM stays s WHERE s.id = $1`, id))
}

// SaveStay — admin tahriri (to'liq yozuv). Narx qo'lda kiritilsa price_updated_at yangilanadi.
func (s *Store) SaveStay(ctx context.Context, adminID string, v Stay, before any) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		name, _ := json.Marshal(v.Name)
		contacts, _ := json.Marshal(v.Contacts)
		images, _ := json.Marshal(v.Images)
		tag, err := tx.Exec(ctx, `UPDATE stays SET city_id=$2, kind=$3, status=$4, name=$5, lat=$6, lng=$7, address=$8, stars=$9,
			price_min=$10, price_max=$11, price_trust=$12, price_note=$13,
			price_updated_at = CASE WHEN price_min IS DISTINCT FROM $10 OR price_max IS DISTINCT FROM $11 THEN now() ELSE price_updated_at END,
			amenities=$14, contacts=$15, check_in=$16, check_out=$17, wheelchair=$18, emehmon=$19, images=$20,
			booking_url=$21, agoda_url=$22, is_partner=$23, fair_price=$24, verified=$25, trust=$26,
			source = CASE WHEN source IN ('open_data','research') THEN 'manual' ELSE source END, updated_at=now() WHERE id=$1`,
			v.ID, v.CityID, v.Kind, v.Status, name, v.Loc.Lat, v.Loc.Lng, v.Address, v.Stars, v.PriceMin, v.PriceMax, v.PriceTrust,
			v.PriceNote, v.Amenities, contacts, v.CheckIn, v.CheckOut, v.Wheelchair, v.EMehmon, images, v.BookingURL, v.AgodaURL,
			v.IsPartner, v.FairPrice, v.Verified, v.Trust)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return audit(ctx, tx, adminID, "stay.update", "stay", v.ID, before, v)
	})
}
