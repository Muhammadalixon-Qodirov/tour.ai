package store

// Daromad funksiyalari (docs/MONETIZATION.md): hamkorlar, takliflar, bosish/ko'rsatish/sotuv, shaharlararo
// yo'nalishlar, Lens Story va chegirma kodlari, rollar. Insights so'rovlari insights.go da.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"yoldosh/internal/domain"
)

// ErrInvalid — havola qilingan yozuv (shahar, hamkor, hudud) yo'q.
var ErrInvalid = errors.New("invalid reference")

// codeAlphabet — chalkashmaydigan harflar (0/O, 1/I yo'q), kodlar ovoz chiqarib aytilganda ham tushunarli.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// RandomCode — n belgili tasodifiy kod (crypto/rand).
func RandomCode(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func isFKViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23503"
}

// ---------- Hamkorlar ----------

type Partner struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	CityID      *string        `json:"cityId"`
	Location    *domain.LatLng `json:"location"`
	DiscountPct *int           `json:"discountPct"`
	Website     *string        `json:"website"`
	Active      bool           `json:"active"`
	Sample      bool           `json:"sample"`
	CreatedAt   time.Time      `json:"createdAt"`
}

const partnerCols = `id, name, city_id, lat, lng, discount_pct, website, active, sample, created_at`

func scanPartner(row pgx.Row) (Partner, error) {
	var p Partner
	var lat, lng *float64
	err := row.Scan(&p.ID, &p.Name, &p.CityID, &lat, &lng, &p.DiscountPct, &p.Website, &p.Active, &p.Sample, &p.CreatedAt)
	if lat != nil && lng != nil {
		p.Location = &domain.LatLng{Lat: *lat, Lng: *lng}
	}
	return p, err
}

func (s *Store) Partner(ctx context.Context, id string) (Partner, error) {
	p, err := scanPartner(s.DB.QueryRow(ctx, `SELECT `+partnerCols+` FROM partners WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) Partners(ctx context.Context) ([]Partner, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+partnerCols+` FROM partners ORDER BY sample, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Partner{}
	for rows.Next() {
		p, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SavePartner — yangi (create = true) yoki mavjud hamkorni to'liq yozuv bilan saqlaydi, audit bilan.
func (s *Store) SavePartner(ctx context.Context, adminID string, p Partner, create bool) error {
	var lat, lng *float64
	if p.Location != nil {
		lat, lng = &p.Location.Lat, &p.Location.Lng
	}
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var err error
		action := "partner.update"
		if create {
			action = "partner.create"
			_, err = tx.Exec(ctx, `INSERT INTO partners (id, name, city_id, lat, lng, discount_pct, website, active, sample)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, p.Name, p.CityID, lat, lng, p.DiscountPct, p.Website, p.Active, p.Sample)
		} else {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `UPDATE partners SET name=$2, city_id=$3, lat=$4, lng=$5, discount_pct=$6, website=$7, active=$8,
				sample=$9, updated_at=now() WHERE id=$1`, p.ID, p.Name, p.CityID, lat, lng, p.DiscountPct, p.Website, p.Active, p.Sample)
			if err == nil && tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
		switch {
		case isUniqueViolation(err):
			return ErrConflict
		case isFKViolation(err):
			return ErrInvalid
		case err != nil:
			return err
		}
		return audit(ctx, tx, adminID, action, "partner", p.ID, nil, p)
	})
}

// ---------- Hamkor kalitlari ----------

type PartnerKey struct {
	ID         uuid.UUID  `json:"id"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

func (s *Store) CreatePartnerKey(ctx context.Context, adminID, partnerID, hash, prefix string) (PartnerKey, error) {
	k := PartnerKey{ID: uuid.Must(uuid.NewV7()), Prefix: prefix}
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO partner_keys (id, partner_id, key_hash, prefix) VALUES ($1,$2,$3,$4) RETURNING created_at`,
			k.ID, partnerID, hash, prefix).Scan(&k.CreatedAt)
		if isFKViolation(err) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, adminID, "partner.key.create", "partner", partnerID, nil, map[string]string{"keyId": k.ID.String(), "prefix": prefix})
	})
	return k, err
}

func (s *Store) PartnerKeys(ctx context.Context, partnerID string) ([]PartnerKey, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, prefix, created_at, last_used_at, revoked_at FROM partner_keys
		WHERE partner_id = $1 ORDER BY created_at DESC`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PartnerKey{}
	for rows.Next() {
		var k PartnerKey
		if err := rows.Scan(&k.ID, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) RevokePartnerKey(ctx context.Context, adminID, partnerID string, keyID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE partner_keys SET revoked_at = now() WHERE id = $1 AND partner_id = $2 AND revoked_at IS NULL`, keyID, partnerID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return audit(ctx, tx, adminID, "partner.key.revoke", "partner", partnerID, nil, map[string]string{"keyId": keyID.String()})
	})
}

// PartnerByKeyHash — faol kalit egasi (faol hamkor). Kalit ishlatilgan vaqt yangilanadi.
func (s *Store) PartnerByKeyHash(ctx context.Context, hash string) (Partner, error) {
	var id string
	err := s.DB.QueryRow(ctx, `UPDATE partner_keys k SET last_used_at = now() FROM partners p
		WHERE k.key_hash = $1 AND k.revoked_at IS NULL AND p.id = k.partner_id AND p.active RETURNING p.id`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Partner{}, ErrNotFound
	}
	if err != nil {
		return Partner{}, err
	}
	return s.Partner(ctx, id)
}

// ---------- Takliflar ----------

type Offer struct {
	ID           string            `json:"id"`
	PartnerID    string            `json:"partnerId"`
	Slot         string            `json:"slot"`
	CityID       *string           `json:"cityId"`
	RouteFrom    *string           `json:"routeFrom"`
	RouteTo      *string           `json:"routeTo"`
	Mode         *string           `json:"mode"`
	ValidFrom    *string           `json:"validFrom"` // YYYY-MM-DD
	ValidTo      *string           `json:"validTo"`
	DaysMin      *int              `json:"daysMin"`
	DaysMax      *int              `json:"daysMax"`
	Resident     *bool             `json:"resident"`
	Title        map[string]string `json:"title"`
	Body         map[string]string `json:"body"`
	CTA          map[string]string `json:"cta"`
	PriceFrom    *int64            `json:"priceFrom"`
	PriceUnit    string            `json:"priceUnit"`
	LinkTemplate *string           `json:"linkTemplate"`
	Priority     int               `json:"priority"`
	Active       bool              `json:"active"`
	Sample       bool              `json:"sample"`

	// o'qishda: hamkor
	PartnerName   string `json:"-"`
	PartnerSample bool   `json:"-"`
}

const offerCols = `o.id, o.partner_id, o.slot, o.city_id, o.route_from, o.route_to, o.mode, to_char(o.valid_from, 'YYYY-MM-DD'),
  to_char(o.valid_to, 'YYYY-MM-DD'), o.days_min, o.days_max, o.resident, o.title, o.body, o.cta, o.price_from, o.price_unit,
  o.link_template, o.priority, o.active, o.sample, p.name, p.sample`

func scanOffer(row pgx.Row) (Offer, error) {
	var o Offer
	err := row.Scan(&o.ID, &o.PartnerID, &o.Slot, &o.CityID, &o.RouteFrom, &o.RouteTo, &o.Mode, &o.ValidFrom, &o.ValidTo,
		&o.DaysMin, &o.DaysMax, &o.Resident, &o.Title, &o.Body, &o.CTA, &o.PriceFrom, &o.PriceUnit, &o.LinkTemplate,
		&o.Priority, &o.Active, &o.Sample, &o.PartnerName, &o.PartnerSample)
	return o, err
}

func (s *Store) Offer(ctx context.Context, id string) (Offer, error) {
	o, err := scanOffer(s.DB.QueryRow(ctx, `SELECT `+offerCols+` FROM offers o JOIN partners p ON p.id = o.partner_id WHERE o.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

func (s *Store) Offers(ctx context.Context, partnerID string) ([]Offer, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+offerCols+` FROM offers o JOIN partners p ON p.id = o.partner_id
		WHERE $1 = '' OR o.partner_id = $1 ORDER BY o.slot, o.priority DESC, o.id`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OfferQuery — GET /offers konteksti. Mode — transport slotida yo'nalish bo'yicha aniqlangan transport turi.
type OfferQuery struct {
	Slot     string
	City     string
	From     string
	To       string
	Mode     string
	Date     time.Time
	Days     int
	Resident bool
}

// MatchingOffers — kontekstga mos faol takliflar, faqat eng yuqori priority guruhi (id tartibida).
func (s *Store) MatchingOffers(ctx context.Context, q OfferQuery) ([]Offer, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+offerCols+` FROM offers o JOIN partners p ON p.id = o.partner_id
		WHERE o.active AND p.active AND o.slot = $1
		  AND (o.city_id IS NULL OR o.city_id = $2)
		  AND (o.route_from IS NULL OR (o.route_from = $3 AND o.route_to = $4) OR (o.route_from = $4 AND o.route_to = $3))
		  AND (o.mode IS NULL OR o.mode = $5)
		  AND (o.valid_from IS NULL OR o.valid_from <= $6) AND (o.valid_to IS NULL OR o.valid_to >= $6)
		  AND (o.days_min IS NULL OR o.days_min <= $7) AND (o.days_max IS NULL OR o.days_max >= $7)
		  AND (o.resident IS NULL OR o.resident = $8)
		ORDER BY o.priority DESC, o.id`, q.Slot, q.City, q.From, q.To, q.Mode, q.Date, q.Days, q.Resident)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Offer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 && o.Priority < out[0].Priority {
			break
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SaveOffer — yangi yoki mavjud taklifni to'liq yozuv bilan saqlaydi, audit bilan.
func (s *Store) SaveOffer(ctx context.Context, adminID string, o Offer, create bool) error {
	var body any
	if len(o.Body) > 0 {
		body = o.Body
	}
	args := []any{o.ID, o.PartnerID, o.Slot, o.CityID, o.RouteFrom, o.RouteTo, o.Mode, o.ValidFrom, o.ValidTo, o.DaysMin, o.DaysMax,
		o.Resident, o.Title, body, o.CTA, o.PriceFrom, o.PriceUnit, o.LinkTemplate, o.Priority, o.Active, o.Sample}
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var err error
		action := "offer.update"
		if create {
			action = "offer.create"
			_, err = tx.Exec(ctx, `INSERT INTO offers (id, partner_id, slot, city_id, route_from, route_to, mode, valid_from, valid_to,
				days_min, days_max, resident, title, body, cta, price_from, price_unit, link_template, priority, active, sample)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9::date,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`, args...)
		} else {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `UPDATE offers SET partner_id=$2, slot=$3, city_id=$4, route_from=$5, route_to=$6, mode=$7,
				valid_from=$8::date, valid_to=$9::date, days_min=$10, days_max=$11, resident=$12, title=$13, body=$14, cta=$15,
				price_from=$16, price_unit=$17, link_template=$18, priority=$19, active=$20, sample=$21, updated_at=now() WHERE id=$1`, args...)
			if err == nil && tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
		switch {
		case isUniqueViolation(err):
			return ErrConflict
		case isFKViolation(err):
			return ErrInvalid
		case err != nil:
			return err
		}
		return audit(ctx, tx, adminID, action, "offer", o.ID, nil, o)
	})
}

// AddImpressions — kunlik anonim hisob.
func (s *Store) AddImpressions(ctx context.Context, offerID string, day time.Time, count int) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO offer_impressions (offer_id, day, count) VALUES ($1, $2, $3)
		ON CONFLICT (offer_id, day) DO UPDATE SET count = offer_impressions.count + EXCLUDED.count`, offerID, day, count)
	if isFKViolation(err) {
		return ErrNotFound
	}
	return err
}

type Click struct {
	ID        string
	OfferID   string
	PartnerID string
	UserID    *string
	TripID    *uuid.UUID
	IssuedAt  time.Time
	Sample    bool
}

// CreateClick — bosishni yozadi. Foydalanuvchi yoki safar o'chirilgan bo'lsa ular bo'sh qoldiriladi.
func (s *Store) CreateClick(ctx context.Context, c Click) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO offer_clicks (id, offer_id, partner_id, user_id, trip_id, issued_at, sample)
		VALUES ($1, $2, $3, (SELECT id FROM users WHERE id = $4), (SELECT id FROM trips WHERE id = $5), $6, $7)`,
		c.ID, c.OfferID, c.PartnerID, c.UserID, c.TripID, c.IssuedAt, c.Sample)
	return err
}

type Conversion struct {
	ID         uuid.UUID
	ClickID    string
	Amount     int64
	Currency   string
	ExternalID string
	CreatedAt  time.Time
}

// AddConversion — sotuv xabari. Bosish shu hamkornikimasa ErrNotFound. Bir xil externalId qayta kelsa
// avvalgi yozuv qaytadi (dup = true): hamkor xabarni qayta yuborsa ikki marta sanalmaydi.
func (s *Store) AddConversion(ctx context.Context, partnerID string, c Conversion) (Conversion, bool, error) {
	var out Conversion
	dup := false
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM offer_clicks WHERE id = $1 AND partner_id = $2)`, c.ClickID, partnerID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		err := tx.QueryRow(ctx, `INSERT INTO offer_conversions (id, click_id, partner_id, amount, currency, external_id)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (partner_id, external_id) DO NOTHING
			RETURNING id, click_id, amount, currency, external_id, created_at`,
			uuid.Must(uuid.NewV7()), c.ClickID, partnerID, c.Amount, c.Currency, c.ExternalID).
			Scan(&out.ID, &out.ClickID, &out.Amount, &out.Currency, &out.ExternalID, &out.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			dup = true
			return tx.QueryRow(ctx, `SELECT id, click_id, amount, currency, external_id, created_at FROM offer_conversions
				WHERE partner_id = $1 AND external_id = $2`, partnerID, c.ExternalID).
				Scan(&out.ID, &out.ClickID, &out.Amount, &out.Currency, &out.ExternalID, &out.CreatedAt)
		}
		return err
	})
	return out, dup, err
}

// ---------- Hamkor statistikasi ----------

type MoneyRow struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type StatsRow struct {
	Key         string     `json:"-"`
	Impressions int64      `json:"impressions"`
	Clicks      int64      `json:"clicks"`
	Conversions int64      `json:"conversions"`
	Amounts     []MoneyRow `json:"amounts"`
	CodesIssued int64      `json:"codesIssued"`
	CodesUsed   int64      `json:"codesRedeemed"`
}

type PartnerStats struct {
	Total  StatsRow
	Months []StatsRow // Key = YYYY-MM
	Offers []StatsRow // Key = offer id
}

// PartnerStats — [from, to] (sanalar, Asia/Tashkent bo'yicha emas, UTC kunlari) oralig'idagi ko'rsatkichlar.
func (s *Store) PartnerStats(ctx context.Context, partnerID string, from, to time.Time) (PartnerStats, error) {
	end := to.AddDate(0, 0, 1)
	months := map[string]*StatsRow{}
	offers := map[string]*StatsRow{}
	var st PartnerStats
	get := func(m map[string]*StatsRow, k string) *StatsRow {
		if r, ok := m[k]; ok {
			return r
		}
		r := &StatsRow{Key: k, Amounts: []MoneyRow{}}
		m[k] = r
		return r
	}
	addAmount := func(r *StatsRow, cur string, amt int64) {
		for i := range r.Amounts {
			if r.Amounts[i].Currency == cur {
				r.Amounts[i].Amount += amt
				return
			}
		}
		r.Amounts = append(r.Amounts, MoneyRow{Amount: amt, Currency: cur})
	}
	st.Total.Amounts = []MoneyRow{}

	type q struct {
		sql  string
		each func(month, offer, cur string, n, amt int64)
	}
	queries := []q{
		{`SELECT to_char(i.day, 'YYYY-MM'), i.offer_id, '', sum(i.count), 0 FROM offer_impressions i JOIN offers o ON o.id = i.offer_id
			WHERE o.partner_id = $1 AND i.day >= $2::date AND i.day < $3::date GROUP BY 1, 2`,
			func(m, o, _ string, n, _ int64) {
				st.Total.Impressions += n
				get(months, m).Impressions += n
				get(offers, o).Impressions += n
			}},
		{`SELECT to_char(clicked_at, 'YYYY-MM'), offer_id, '', count(*), 0 FROM offer_clicks
			WHERE partner_id = $1 AND clicked_at >= $2 AND clicked_at < $3 GROUP BY 1, 2`,
			func(m, o, _ string, n, _ int64) {
				st.Total.Clicks += n
				get(months, m).Clicks += n
				get(offers, o).Clicks += n
			}},
		{`SELECT to_char(v.created_at, 'YYYY-MM'), c.offer_id, v.currency, count(*), sum(v.amount) FROM offer_conversions v
			JOIN offer_clicks c ON c.id = v.click_id
			WHERE v.partner_id = $1 AND v.created_at >= $2 AND v.created_at < $3 GROUP BY 1, 2, 3`,
			func(m, o, cur string, n, amt int64) {
				st.Total.Conversions += n
				get(months, m).Conversions += n
				get(offers, o).Conversions += n
				addAmount(&st.Total, cur, amt)
				addAmount(get(months, m), cur, amt)
				addAmount(get(offers, o), cur, amt)
			}},
		{`SELECT to_char(issued_at, 'YYYY-MM'), '', '', count(*), 0 FROM discount_codes
			WHERE partner_id = $1 AND issued_at >= $2 AND issued_at < $3 GROUP BY 1`,
			func(m, _, _ string, n, _ int64) {
				st.Total.CodesIssued += n
				get(months, m).CodesIssued += n
			}},
		{`SELECT to_char(used_at, 'YYYY-MM'), '', '', count(*), 0 FROM discount_codes
			WHERE partner_id = $1 AND used_at >= $2 AND used_at < $3 GROUP BY 1`,
			func(m, _, _ string, n, _ int64) {
				st.Total.CodesUsed += n
				get(months, m).CodesUsed += n
			}},
	}
	for _, qq := range queries {
		rows, err := s.DB.Query(ctx, qq.sql, partnerID, from, end)
		if err != nil {
			return st, err
		}
		for rows.Next() {
			var m, o, cur string
			var n, amt int64
			if err := rows.Scan(&m, &o, &cur, &n, &amt); err != nil {
				rows.Close()
				return st, err
			}
			qq.each(m, o, cur, n, amt)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return st, err
		}
	}
	for _, r := range months {
		st.Months = append(st.Months, *r)
	}
	for _, r := range offers {
		st.Offers = append(st.Offers, *r)
	}
	sortRows(st.Months)
	sortRows(st.Offers)
	return st, nil
}

func sortRows(r []StatsRow) {
	for i := 1; i < len(r); i++ {
		for j := i; j > 0 && r[j].Key < r[j-1].Key; j-- {
			r[j], r[j-1] = r[j-1], r[j]
		}
	}
}

// ---------- Shaharlararo yo'nalishlar ----------

type IntercityRoute struct {
	Mode        string
	DurationMin int
	PriceFrom   *int64
	Source      string
	SourceURL   *string
	UpdatedAt   string
	Estimate    bool
	Via         *string
}

// IntercityBooking — tur bo'yicha rasmiy chipta sotib olish sahifasi (deep-link). Mavjud bo'lmasa nil.
// Poyezd/Afrosiyob — O'zbekiston temir yo'llari e-chiptasi; samolyot — Uzbekistan Airways. Avtobus/taksi — yo'q.
func IntercityBooking(mode string) *string {
	var u string
	switch mode {
	case "train", "afrosiyob":
		u = "https://eticket.railway.uz/"
	case "plane":
		u = "https://www.uzairways.com/"
	default:
		return nil
	}
	return &u
}

func (s *Store) IntercityRoutes(ctx context.Context, from, to string) ([]IntercityRoute, error) {
	rows, err := s.DB.Query(ctx, `SELECT mode, duration_min, price_from, source, source_url, to_char(updated_at, 'YYYY-MM-DD'), estimate, via
		FROM intercity_routes WHERE from_city = $1 AND to_city = $2 ORDER BY position`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IntercityRoute{}
	for rows.Next() {
		var r IntercityRoute
		if err := rows.Scan(&r.Mode, &r.DurationMin, &r.PriceFrom, &r.Source, &r.SourceURL, &r.UpdatedAt, &r.Estimate, &r.Via); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- Lens Story va kodlar ----------

type Story struct {
	ShareID   string
	UserID    string
	PlaceID   string
	Loc       *domain.LatLng
	CreatedAt time.Time
	SharedAt  *time.Time
}

// CreateStory — shareId tasodifiy (8 belgi); to'qnashsa qayta urinadi.
func (s *Store) CreateStory(ctx context.Context, userID, placeID string, loc *domain.LatLng) (string, error) {
	var lat, lng *float64
	if loc != nil {
		// ~10 m gacha yaxlitlanadi: "joydami" tekshiruvi uchun yetarli, aniq joylashuv saqlanmaydi.
		la, ln := roundTo(loc.Lat, 4), roundTo(loc.Lng, 4)
		lat, lng = &la, &ln
	}
	for range 5 {
		id := strings.ToLower(RandomCode(8))
		_, err := s.DB.Exec(ctx, `INSERT INTO stories (share_id, user_id, place_id, lat, lng) VALUES ($1,$2,$3,$4,$5)`, id, userID, placeID, lat, lng)
		if isUniqueViolation(err) {
			continue
		}
		return id, err
	}
	return "", fmt.Errorf("story id to'qnashuvi")
}

func roundTo(v float64, digits int) float64 {
	p := 1.0
	for range digits {
		p *= 10
	}
	if v < 0 {
		return -float64(int64(-v*p+0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}

func (s *Store) Story(ctx context.Context, shareID, userID string) (Story, error) {
	var st Story
	var lat, lng *float64
	err := s.DB.QueryRow(ctx, `SELECT share_id, user_id, place_id, lat, lng, created_at, shared_at FROM stories
		WHERE share_id = $1 AND user_id = $2`, shareID, userID).Scan(&st.ShareID, &st.UserID, &st.PlaceID, &lat, &lng, &st.CreatedAt, &st.SharedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, ErrNotFound
	}
	if lat != nil && lng != nil {
		st.Loc = &domain.LatLng{Lat: *lat, Lng: *lng}
	}
	return st, err
}

// MarkStoryShared — ulashildi (birinchi vaqt saqlanadi); yangi joylashuv kelsa u yoziladi.
func (s *Store) MarkStoryShared(ctx context.Context, shareID string, loc *domain.LatLng) error {
	var lat, lng *float64
	if loc != nil {
		la, ln := roundTo(loc.Lat, 4), roundTo(loc.Lng, 4)
		lat, lng = &la, &ln
	}
	_, err := s.DB.Exec(ctx, `UPDATE stories SET shared_at = COALESCE(shared_at, now()), lat = COALESCE($2, lat), lng = COALESCE($3, lng)
		WHERE share_id = $1`, shareID, lat, lng)
	return err
}

// CountStoryRef — havola orqali kelgan tashrif. Topilsa true.
func (s *Store) CountStoryRef(ctx context.Context, shareID, placeID string) (bool, error) {
	tag, err := s.DB.Exec(ctx, `UPDATE stories SET ref_count = ref_count + 1 WHERE share_id = $1 AND place_id = $2`, shareID, placeID)
	return err == nil && tag.RowsAffected() > 0, err
}

// AtPlaceToday — foydalanuvchining (yoki guruhining) bugungi kun marshrutida shu joy "current" yoki "visited" holatida.
func (s *Store) AtPlaceToday(ctx context.Context, userID, placeID string, day time.Time) (bool, error) {
	var ok bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stops s
		JOIN trips t ON t.id = s.trip_id
		JOIN trip_days d ON d.trip_id = s.trip_id AND d.idx = s.day_idx
		WHERE s.place_id = $2 AND s.status IN ('current','visited') AND d.date = $3::date
		  AND (t.user_id = $1 OR t.group_id IN (SELECT group_id FROM group_members WHERE user_id = $1)))`,
		userID, placeID, day.Format(time.DateOnly)).Scan(&ok)
	return ok, err
}

// NearestStoryPartner — story dasturidagi faol hamkor, radius ichida eng yaqini.
func (s *Store) NearestStoryPartner(ctx context.Context, at domain.LatLng, radiusM float64) (Partner, float64, error) {
	var p Partner
	var d float64
	var lat, lng *float64
	err := s.DB.QueryRow(ctx, `SELECT `+partnerCols+`, earth_distance(ll_to_earth($1, $2), ll_to_earth(lat, lng)) AS d
		FROM partners WHERE active AND discount_pct IS NOT NULL AND lat IS NOT NULL
		  AND earth_box(ll_to_earth($1, $2), $3) @> ll_to_earth(lat, lng)
		  AND earth_distance(ll_to_earth($1, $2), ll_to_earth(lat, lng)) <= $3
		ORDER BY d, id LIMIT 1`, at.Lat, at.Lng, radiusM).
		Scan(&p.ID, &p.Name, &p.CityID, &lat, &lng, &p.DiscountPct, &p.Website, &p.Active, &p.Sample, &p.CreatedAt, &d)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, 0, ErrNotFound
	}
	if lat != nil && lng != nil {
		p.Location = &domain.LatLng{Lat: *lat, Lng: *lng}
	}
	return p, d, err
}

type DiscountCode struct {
	ID          string
	Value       string
	UserID      *string
	PartnerID   string
	PartnerName string
	PlaceID     *string
	DiscountPct int
	IssuedAt    time.Time
	ExpiresAt   time.Time
	UsedAt      *time.Time
	Sample      bool
}

const codeCols = `c.id, c.value, c.user_id, c.partner_id, p.name, c.place_id, c.discount_pct, c.issued_at, c.expires_at, c.used_at, c.sample`

func scanCode(row pgx.Row) (DiscountCode, error) {
	var c DiscountCode
	err := row.Scan(&c.ID, &c.Value, &c.UserID, &c.PartnerID, &c.PartnerName, &c.PlaceID, &c.DiscountPct, &c.IssuedAt, &c.ExpiresAt, &c.UsedAt, &c.Sample)
	return c, err
}

// IssueCode — bir martalik kod. Shu hamkor shu foydalanuvchiga `window` ichida kod bergan bo'lsa o'sha qaytadi
// (recent = true). Bir vaqtdagi ikki so'rov ikki kod olmasligi uchun (foydalanuvchi, hamkor) bo'yicha qulf.
func (s *Store) IssueCode(ctx context.Context, userID string, p Partner, placeID, shareID string, now time.Time, window, ttl time.Duration) (DiscountCode, bool, error) {
	var out DiscountCode
	recent := false
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('code:' || $1 || ':' || $2))`, userID, p.ID); err != nil {
			return err
		}
		c, err := scanCode(tx.QueryRow(ctx, `SELECT `+codeCols+` FROM discount_codes c JOIN partners p ON p.id = c.partner_id
			WHERE c.user_id = $1 AND c.partner_id = $2 AND c.issued_at > $3 ORDER BY c.issued_at DESC LIMIT 1`, userID, p.ID, now.Add(-window)))
		if err == nil {
			out, recent = c, true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		for range 5 {
			c = DiscountCode{ID: "c_" + strings.ToLower(RandomCode(10)), Value: "SF-" + RandomCode(4) + "-" + RandomCode(4), UserID: &userID,
				PartnerID: p.ID, PartnerName: p.Name, PlaceID: &placeID, DiscountPct: *p.DiscountPct, IssuedAt: now, ExpiresAt: now.Add(ttl), Sample: p.Sample}
			_, err = tx.Exec(ctx, `SAVEPOINT code`)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO discount_codes (id, value, user_id, partner_id, place_id, share_id, discount_pct, issued_at, expires_at, sample)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, c.ID, c.Value, userID, p.ID, placeID, shareID, c.DiscountPct, c.IssuedAt, c.ExpiresAt, c.Sample)
			if isUniqueViolation(err) {
				if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT code`); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			out = c
			return nil
		}
		return fmt.Errorf("kod to'qnashuvi")
	})
	return out, recent, err
}

// UserCodes — foydalanuvchi kodlari, yangisi birinchi. all = false: faqat amaldagilar (ishlatilmagan, muddati o'tmagan);
// all = true: oxirgi 30 kundagi hammasi.
func (s *Store) UserCodes(ctx context.Context, userID string, all bool, now time.Time) ([]DiscountCode, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+codeCols+` FROM discount_codes c JOIN partners p ON p.id = c.partner_id
		WHERE c.user_id = $1 AND (CASE WHEN $2 THEN c.issued_at > $3::timestamptz - interval '30 days' ELSE c.used_at IS NULL AND c.expires_at > $3 END)
		ORDER BY c.issued_at DESC LIMIT 100`, userID, all, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DiscountCode{}
	for rows.Next() {
		c, err := scanCode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RedeemCode — hamkor faqat o'z kodini tekshiradi; boshqa hamkorning kodi "not_found" (borligi ham bildirilmaydi).
// status: ok (endi ishlatildi) | used | expired | not_found.
func (s *Store) RedeemCode(ctx context.Context, partnerID, value string, now time.Time) (string, DiscountCode, error) {
	var c DiscountCode
	status := ""
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var err error
		c, err = scanCode(tx.QueryRow(ctx, `SELECT `+codeCols+` FROM discount_codes c JOIN partners p ON p.id = c.partner_id
			WHERE c.value = $1 AND c.partner_id = $2 FOR UPDATE OF c`, value, partnerID))
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			status = "not_found"
			return nil
		case err != nil:
			return err
		case c.UsedAt != nil:
			status = "used"
			return nil
		case !c.ExpiresAt.After(now):
			status = "expired"
			return nil
		}
		status = "ok"
		c.UsedAt = &now
		_, err = tx.Exec(ctx, `UPDATE discount_codes SET used_at = $2 WHERE id = $1`, c.ID, now)
		return err
	})
	return status, c, err
}

// ---------- Rollar ----------

// SetUserRole — tourist | partner (partnerID bilan) | insights (hududlar bilan). Audit bilan.
func (s *Store) SetUserRole(ctx context.Context, adminID, userID, role string, partnerID *string, regions []string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET role = $2, partner_id = $3 WHERE id = $1`, userID, role, partnerID)
		if isFKViolation(err) {
			return ErrInvalid
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM insights_regions WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for _, r := range regions {
			if _, err := tx.Exec(ctx, `INSERT INTO insights_regions (user_id, region) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, r); err != nil {
				if isFKViolation(err) {
					return ErrInvalid
				}
				return err
			}
		}
		return audit(ctx, tx, adminID, "user.role", "user", userID, nil, map[string]any{"role": role, "partnerId": partnerID, "regions": regions})
	})
}

func (s *Store) InsightsRegions(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT region FROM insights_regions WHERE user_id = $1 ORDER BY region`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
