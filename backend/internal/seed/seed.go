// Package seed — seed/*.json fayllarini bazaga yuklaydi (idempotent UPSERT).
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
)

// LoadDir — papkadagi barcha *.json fayllarni yuklaydi. Har fayl alohida tranzaksiyada.
func LoadDir(ctx context.Context, db *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s: seed fayllari topilmadi", dir)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var sf seedFile
		if err := json.Unmarshal(raw, &sf); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if err := validate(sf); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error { return load(ctx, tx, sf) }); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		slog.Info("yuklandi", "file", f, "city", sf.City.ID, "places", len(sf.Places))
	}
	return nil
}

type seedFile struct {
	City struct {
		ID           string            `json:"id"`
		Name         map[string]string `json:"name"`
		Center       domain.LatLng     `json:"center"`
		TaxiBaseFare int64             `json:"taxiBaseFare"`
		TaxiPerKm    int64             `json:"taxiPerKm"`
		Emergency    map[string]string `json:"emergency"`
	} `json:"city"`
	Places []seedPlace `json:"places"`
}

type seedText struct {
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	History     string   `json:"history"`
	Facts       []string `json:"facts"`
}

type seedPlace struct {
	ID              string              `json:"id"`
	Category        string              `json:"category"`
	Novelty         domain.Novelty      `json:"novelty"`
	OpenedAt        *string             `json:"openedAt"`
	Lat             float64             `json:"lat"`
	Lng             float64             `json:"lng"`
	Interests       []string            `json:"interests"`
	VisitMin        int                 `json:"visitMin"`
	PriceLevel      int                 `json:"priceLevel"`
	Hours           string              `json:"hours"`  // "09:00-18:00" har kuni
	Closed          []int               `json:"closed"` // dam olish kunlari (0 = yakshanba)
	Rating          float64             `json:"rating"`
	Reviews         int                 `json:"reviews"`
	Wheelchair      bool                `json:"wheelchair"`
	FamilyFriendly  *bool               `json:"familyFriendly"`
	EasyWalk        *bool               `json:"easyWalk"`
	PayCard         bool                `json:"payCard"`
	CashOnly        bool                `json:"cashOnly"`
	IsPartner       bool                `json:"isPartner"`
	GeofenceRadiusM int                 `json:"geofenceRadiusM"`
	Text            map[string]seedText `json:"text"`
	Rules           []struct {
		Code     string            `json:"code"`
		Severity string            `json:"severity"`
		Priority int               `json:"priority"`
		Text     map[string]string `json:"text"`
	} `json:"rules"`
}

func validate(sf seedFile) error {
	if sf.City.ID == "" || sf.City.Name["uz"] == "" {
		return fmt.Errorf("city.id va city.name.uz shart")
	}
	seen := map[string]bool{}
	for _, p := range sf.Places {
		switch {
		case p.ID == "" || seen[p.ID]:
			return fmt.Errorf("bo'sh yoki takror id: %q", p.ID)
		case !domain.ValidCategory(p.Category):
			return fmt.Errorf("%s: noma'lum kategoriya %q", p.ID, p.Category)
		case !p.Novelty.Valid():
			return fmt.Errorf("%s: noma'lum novelty %q", p.ID, p.Novelty)
		case p.Text["uz"].Name == "" || p.Text["uz"].Tagline == "":
			return fmt.Errorf("%s: uz nom va tagline shart", p.ID)
		case !(domain.LatLng{Lat: p.Lat, Lng: p.Lng}).Valid() || p.Lat == 0:
			return fmt.Errorf("%s: koordinata noto'g'ri", p.ID)
		}
		for _, in := range p.Interests {
			if !domain.ValidInterest(in) {
				return fmt.Errorf("%s: noma'lum qiziqish %q", p.ID, in)
			}
		}
		if _, err := hours(p.Hours, p.Closed); err != nil {
			return fmt.Errorf("%s: %w", p.ID, err)
		}
		seen[p.ID] = true
	}
	return nil
}

func hours(spec string, closed []int) ([]domain.OpeningHours, error) {
	if spec == "" {
		return nil, nil
	}
	o, c, ok := strings.Cut(spec, "-")
	if !ok {
		return nil, fmt.Errorf("hours %q: HH:MM-HH:MM", spec)
	}
	if _, err := domain.ParseHHMM(o); err != nil {
		return nil, err
	}
	if _, err := domain.ParseHHMM(c); err != nil {
		return nil, err
	}
	var out []domain.OpeningHours
	for d := range 7 {
		isClosed := false
		for _, x := range closed {
			isClosed = isClosed || x == d
		}
		if !isClosed {
			out = append(out, domain.OpeningHours{Day: d, Open: o, Close: c})
		}
	}
	return out, nil
}

func orTrue(b *bool) bool { return b == nil || *b }

func load(ctx context.Context, tx pgx.Tx, sf seedFile) error {
	c := sf.City
	emergency, _ := json.Marshal(c.Emergency)
	if _, err := tx.Exec(ctx, `INSERT INTO cities (id, lat, lng, taxi_base_fare, taxi_per_km, emergency)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng, taxi_base_fare = EXCLUDED.taxi_base_fare,
		  taxi_per_km = EXCLUDED.taxi_per_km, emergency = EXCLUDED.emergency`,
		c.ID, c.Center.Lat, c.Center.Lng, c.TaxiBaseFare, c.TaxiPerKm, emergency); err != nil {
		return err
	}
	for lang, name := range c.Name {
		if _, err := tx.Exec(ctx, `INSERT INTO city_i18n (city_id, lang, name) VALUES ($1,$2,$3)
			ON CONFLICT (city_id, lang) DO UPDATE SET name = EXCLUDED.name`, c.ID, lang, name); err != nil {
			return err
		}
	}
	sources, _ := json.Marshal([]map[string]string{{"title": "Ochiq manbalar (demo — tekshirilishi kerak)"}})
	for _, p := range sf.Places {
		h, _ := hours(p.Hours, p.Closed)
		hj, _ := json.Marshal(h)
		if p.GeofenceRadiusM == 0 {
			p.GeofenceRadiusM = 80
		}
		if p.VisitMin == 0 {
			p.VisitMin = 45
		}
		if p.Interests == nil {
			p.Interests = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, opened_at, status, lat, lng,
			geofence_radius_m, interests, price_level, visit_duration_min, opening_hours, wheelchair, family_friendly, easy_walk,
			pay_card, cash_only, is_partner, rating, reviews_count, source, published_at)
			VALUES ($1,$2,$3,$4,$5,'published',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,'demo',now())
			ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, category = EXCLUDED.category, novelty = EXCLUDED.novelty,
			  opened_at = EXCLUDED.opened_at, lat = EXCLUDED.lat, lng = EXCLUDED.lng, geofence_radius_m = EXCLUDED.geofence_radius_m,
			  interests = EXCLUDED.interests, price_level = EXCLUDED.price_level, visit_duration_min = EXCLUDED.visit_duration_min,
			  opening_hours = EXCLUDED.opening_hours, wheelchair = EXCLUDED.wheelchair, family_friendly = EXCLUDED.family_friendly,
			  easy_walk = EXCLUDED.easy_walk, pay_card = EXCLUDED.pay_card, cash_only = EXCLUDED.cash_only,
			  is_partner = EXCLUDED.is_partner, updated_at = now()`,
			p.ID, c.ID, p.Category, p.Novelty, p.OpenedAt, p.Lat, p.Lng, p.GeofenceRadiusM, p.Interests, p.PriceLevel, p.VisitMin,
			hj, p.Wheelchair, orTrue(p.FamilyFriendly), orTrue(p.EasyWalk), p.PayCard, p.CashOnly, p.IsPartner,
			p.Rating, p.Reviews); err != nil {
			return fmt.Errorf("place %s: %w", p.ID, err)
		}
		for lang, t := range p.Text {
			if !i18n.Valid(lang) {
				return fmt.Errorf("place %s: noma'lum til %q", p.ID, lang)
			}
			var hist *string
			if t.History != "" {
				hist = &t.History
			}
			if t.Facts == nil {
				t.Facts = []string{}
			}
			src := []byte("[]")
			if hist != nil {
				src = sources
			}
			if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address,
				history_summary, history_facts, sources) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
				ON CONFLICT (place_id, lang) DO UPDATE SET name = EXCLUDED.name, tagline = EXCLUDED.tagline,
				  description = EXCLUDED.description, address = EXCLUDED.address, history_summary = EXCLUDED.history_summary,
				  history_facts = EXCLUDED.history_facts, sources = EXCLUDED.sources`,
				p.ID, lang, t.Name, t.Tagline, t.Description, t.Address, hist, t.Facts, src); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM place_rules WHERE place_id = $1`, p.ID); err != nil {
			return err
		}
		for _, r := range p.Rules {
			txt, _ := json.Marshal(r.Text)
			if _, err := tx.Exec(ctx, `INSERT INTO place_rules (place_id, code, severity, priority, text, verified_by)
				VALUES ($1,$2,$3,$4,$5,'demo')`, p.ID, r.Code, r.Severity, r.Priority, txt); err != nil {
				return err
			}
		}
	}
	return nil
}
