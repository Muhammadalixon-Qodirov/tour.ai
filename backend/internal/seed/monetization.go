package seed

// Daromad funksiyalari seed i: seed/monetization.json (docs/MONETIZATION.md).
//
//   - shaharlararo yo'nalishlar: to'g'ridan-to'g'ri raqamlar faylda (manba va sana bilan); mashina va Toshkent
//     orqali yo'nalishlar shu yerda hisoblanadi — sayt (web/src/lib/monetization/intercity.ts) bilan aynan bir xil;
//   - namuna hamkorlar va takliflar (sample = true).
//
// Idempotent: yo'nalishlar to'liq qayta yoziladi, hamkor va takliflar id bo'yicha UPSERT.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
)

// MonetizationFile — LoadDir shu nomdagi faylni shahar katalogi sifatida emas, alohida yuklaydi.
const MonetizationFile = "monetization.json"

type monetizationFile struct {
	Intercity intercitySeed `json:"intercity"`
	Partners  []partnerSeed `json:"partners"`
	Offers    []offerSeed   `json:"offers"`
}

type routeSource struct {
	Source    string `json:"source"`
	SourceURL string `json:"sourceUrl"`
	UpdatedAt string `json:"updatedAt"`
	Estimate  bool   `json:"estimate"`
}

type directRoute struct {
	Mode        string `json:"mode"`
	DurationMin int    `json:"durationMin"`
	PriceFrom   *int64 `json:"priceFrom"`
	Source      string `json:"source"` // sources kaliti
	Estimate    bool   `json:"estimate"`
}

type intercitySeed struct {
	Sources     map[string]routeSource `json:"sources"`
	Hub         string                 `json:"hub"`
	TransferMin int                    `json:"transferMin"`
	Car         struct {
		Source    string  `json:"source"`
		KmPerHour float64 `json:"kmPerHour"`
		SomPerKm  float64 `json:"somPerKm"`
	} `json:"car"`
	RoadKm   map[string]float64       `json:"roadKm"`
	Direct   map[string][]directRoute `json:"direct"`
	ViaModes []string                 `json:"viaModes"`
}

type partnerSeed struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	CityID      *string        `json:"cityId"`
	Location    *domain.LatLng `json:"location"`
	DiscountPct *int           `json:"discountPct"`
	Website     *string        `json:"website"`
	Sample      bool           `json:"sample"`
}

type offerSeed struct {
	ID        string            `json:"id"`
	PartnerID string            `json:"partnerId"`
	Slot      string            `json:"slot"`
	CityID    *string           `json:"cityId"`
	Mode      *string           `json:"mode"`
	DaysMin   *int              `json:"daysMin"`
	DaysMax   *int              `json:"daysMax"`
	Resident  *bool             `json:"resident"`
	Title     map[string]string `json:"title"`
	Body      map[string]string `json:"body"`
	CTA       map[string]string `json:"cta"`
	PriceFrom *int64            `json:"priceFrom"`
	PriceUnit string            `json:"priceUnit"`
	Link      *string           `json:"linkTemplate"`
	Sample    bool              `json:"sample"`
}

// IntercityRoute — bitta yo'nalish qatori (GET /routes/intercity shakli).
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

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// jsRound — JavaScript Math.round (yarimini yuqoriga), sayt bilan bir xil yaxlitlash uchun.
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

func (ic intercitySeed) route(d directRoute) (IntercityRoute, error) {
	src, ok := ic.Sources[d.Source]
	if !ok {
		return IntercityRoute{}, fmt.Errorf("intercity: noma'lum manba %q", d.Source)
	}
	r := IntercityRoute{Mode: d.Mode, DurationMin: d.DurationMin, PriceFrom: d.PriceFrom, Source: src.Source,
		UpdatedAt: src.UpdatedAt, Estimate: src.Estimate || d.Estimate}
	if src.SourceURL != "" {
		u := src.SourceURL
		r.SourceURL = &u
	}
	return r, nil
}

func (ic intercitySeed) car(a, b string) (IntercityRoute, error) {
	km := ic.RoadKm[pairKey(a, b)]
	price := int64(jsRound(km*ic.Car.SomPerKm/10000) * 10000)
	return ic.route(directRoute{Mode: "car", DurationMin: int(jsRound(km/ic.Car.KmPerHour*6)) * 10, PriceFrom: &price, Source: ic.Car.Source})
}

func (ic intercitySeed) leg(a, b, mode string) (IntercityRoute, bool, error) {
	rows := ic.Direct[pairKey(a, b)]
	i := slices.IndexFunc(rows, func(r directRoute) bool { return r.Mode == mode })
	if i < 0 && mode == "afrosiyob" {
		i = slices.IndexFunc(rows, func(r directRoute) bool { return r.Mode == "train" })
	}
	if i < 0 {
		return IntercityRoute{}, false, nil
	}
	r, err := ic.route(rows[i])
	return r, err == nil, err
}

// Routes — from dan to ga barcha ma'lum yo'llar (sayt intercityRoutes bilan bir xil tartib va raqamlar).
func (ic intercitySeed) Routes(from, to string) ([]IntercityRoute, error) {
	if from == to || ic.RoadKm[pairKey(from, to)] == 0 {
		return nil, nil
	}
	var out []IntercityRoute
	if direct, ok := ic.Direct[pairKey(from, to)]; ok {
		for _, d := range direct {
			r, err := ic.route(d)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
	} else {
		for _, mode := range ic.ViaModes {
			l1, ok1, err := ic.leg(from, ic.Hub, mode)
			if err != nil {
				return nil, err
			}
			l2, ok2, err := ic.leg(ic.Hub, to, mode)
			if err != nil {
				return nil, err
			}
			if !ok1 || !ok2 {
				continue
			}
			r := IntercityRoute{Mode: mode, DurationMin: l1.DurationMin + l2.DurationMin + ic.TransferMin,
				Source: l1.Source + "; " + l2.Source, UpdatedAt: max(l1.UpdatedAt, l2.UpdatedAt), Estimate: true}
			if l1.PriceFrom != nil && l2.PriceFrom != nil {
				p := *l1.PriceFrom + *l2.PriceFrom
				r.PriceFrom = &p
			}
			hub := ic.Hub
			r.Via = &hub
			out = append(out, r)
		}
	}
	c, err := ic.car(from, to)
	if err != nil {
		return nil, err
	}
	return append(out, c), nil
}

// Cities — roadKm dagi shaharlar.
func (ic intercitySeed) Cities() []string {
	set := map[string]bool{}
	for k := range ic.RoadKm {
		a, b, _ := strings.Cut(k, "|")
		set[a], set[b] = true, true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

var (
	offerSlots = []string{"transport.cheapest", "transport.fastest", "transport.comfort", "transport.bus",
		"stay", "food", "event", "prep.esim", "prep.insurance", "prep.sim"}
	transportModes = []string{"train", "afrosiyob", "plane", "car", "bus"}
)

func (mf monetizationFile) validate() error {
	for k, rows := range mf.Intercity.Direct {
		if mf.Intercity.RoadKm[k] == 0 {
			return fmt.Errorf("intercity.direct[%s]: roadKm da yo'q", k)
		}
		for _, r := range rows {
			if !slices.Contains(transportModes, r.Mode) || r.DurationMin <= 0 {
				return fmt.Errorf("intercity.direct[%s]: mode/durationMin noto'g'ri", k)
			}
		}
	}
	partners := map[string]bool{}
	for _, p := range mf.Partners {
		if p.ID == "" || p.Name == "" {
			return fmt.Errorf("partner: id va name majburiy")
		}
		if p.Location != nil && !p.Location.Valid() {
			return fmt.Errorf("partner %s: location noto'g'ri", p.ID)
		}
		partners[p.ID] = true
	}
	for _, o := range mf.Offers {
		if !partners[o.PartnerID] {
			return fmt.Errorf("offer %s: hamkor %q yo'q", o.ID, o.PartnerID)
		}
		if !slices.Contains(offerSlots, o.Slot) {
			return fmt.Errorf("offer %s: slot %q", o.ID, o.Slot)
		}
		if o.Title["uz"] == "" || o.CTA["uz"] == "" {
			return fmt.Errorf("offer %s: title.uz va cta.uz majburiy", o.ID)
		}
	}
	return nil
}

func loadMonetization(ctx context.Context, db *pgxpool.Pool, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var mf monetizationFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := mf.validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM intercity_routes`); err != nil {
			return err
		}
		ic := mf.Intercity
		cities := ic.Cities()
		n := 0
		for _, from := range cities {
			for _, to := range cities {
				routes, err := ic.Routes(from, to)
				if err != nil {
					return err
				}
				for pos, r := range routes {
					if _, err := tx.Exec(ctx, `INSERT INTO intercity_routes (from_city, to_city, position, mode, duration_min, price_from,
						source, source_url, updated_at, estimate, via) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
						from, to, pos, r.Mode, r.DurationMin, r.PriceFrom, r.Source, r.SourceURL, r.UpdatedAt, r.Estimate, r.Via); err != nil {
						return fmt.Errorf("intercity %s→%s: %w", from, to, err)
					}
					n++
				}
			}
		}
		for _, p := range mf.Partners {
			var lat, lng *float64
			if p.Location != nil {
				lat, lng = &p.Location.Lat, &p.Location.Lng
			}
			if _, err := tx.Exec(ctx, `INSERT INTO partners (id, name, city_id, lat, lng, discount_pct, website, sample)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, city_id = EXCLUDED.city_id, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
				  discount_pct = EXCLUDED.discount_pct, website = EXCLUDED.website, sample = EXCLUDED.sample, updated_at = now()`,
				p.ID, p.Name, p.CityID, lat, lng, p.DiscountPct, p.Website, p.Sample); err != nil {
				return fmt.Errorf("partner %s: %w", p.ID, err)
			}
		}
		for _, o := range mf.Offers {
			unit := o.PriceUnit
			if unit == "" {
				unit = "trip"
			}
			var body any
			if len(o.Body) > 0 {
				body = o.Body
			}
			if _, err := tx.Exec(ctx, `INSERT INTO offers (id, partner_id, slot, city_id, mode, days_min, days_max, resident,
				title, body, cta, price_from, price_unit, link_template, sample)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
				ON CONFLICT (id) DO UPDATE SET partner_id = EXCLUDED.partner_id, slot = EXCLUDED.slot, city_id = EXCLUDED.city_id,
				  mode = EXCLUDED.mode, days_min = EXCLUDED.days_min, days_max = EXCLUDED.days_max, resident = EXCLUDED.resident,
				  title = EXCLUDED.title, body = EXCLUDED.body, cta = EXCLUDED.cta, price_from = EXCLUDED.price_from,
				  price_unit = EXCLUDED.price_unit, link_template = EXCLUDED.link_template, sample = EXCLUDED.sample, updated_at = now()`,
				o.ID, o.PartnerID, o.Slot, o.CityID, o.Mode, o.DaysMin, o.DaysMax, o.Resident, o.Title, body, o.CTA,
				o.PriceFrom, unit, o.Link, o.Sample); err != nil {
				return fmt.Errorf("offer %s: %w", o.ID, err)
			}
		}
		slog.Info("yuklandi", "file", path, "intercity", n, "partners", len(mf.Partners), "offers", len(mf.Offers))
		return nil
	})
}
