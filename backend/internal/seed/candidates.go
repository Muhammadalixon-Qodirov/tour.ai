package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
)

// Nomzod joylar (seed/candidates/*.json) — ochiq manbalardan qo'lda topilgan joylar.
// Bazaga status='pending', source='research' bilan tushadi; admin tekshirib publish qiladi.
// Qoidalar: seed/candidates/README.md.

type candFile struct {
	Region string `json:"region"`
	Cities []struct {
		ID     string            `json:"id"`
		Name   map[string]string `json:"name"`
		Center domain.LatLng     `json:"center"`
	} `json:"cities"`
	Places []candPlace `json:"places"`
}

type candPlace struct {
	ID          string            `json:"id"`
	CityID      string            `json:"cityId"`
	Category    string            `json:"category"`
	Novelty     domain.Novelty    `json:"novelty"`
	OpenedAt    *string           `json:"openedAt"`
	Name        map[string]string `json:"name"`
	Tagline     map[string]string `json:"tagline"`
	Description map[string]string `json:"description"`
	Address     string            `json:"address"`
	Lat         *float64          `json:"lat"`
	Lng         *float64          `json:"lng"`
	CoordsSrc   string            `json:"coordsSource"`
	Interests   []string          `json:"interests"`
	PriceLevel  *int              `json:"priceLevel"`
	PriceNote   string            `json:"priceNote"`
	Hours       *string           `json:"hours"`
	Instagram   string            `json:"instagram"`
	Telegram    string            `json:"telegram"`
	Website     string            `json:"website"`
	Sources     []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Date  string `json:"date"`
	} `json:"sources"`
	Confidence string `json:"confidence"`
	Notes      string `json:"notes"`
}

type CandidateReport struct {
	Imported []string
	Updated  []string
	Skipped  []string // sabab bilan
}

var candIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// O'zbekistondagi favqulodda raqamlar (tekshirilgan: 112 2025-yildan butun mamlakatda; 1173 — turistlar call-markazi).
var defaultEmergency = map[string]string{"unified": "112", "fire": "101", "police": "102", "ambulance": "103", "gas": "104",
	"emergency_ministry": "1050", "tourist_hotline": "1173"}

// LoadCandidates — papkadagi barcha nomzod fayllarini yuklaydi. Admin allaqachon ko'rib chiqqan
// (source='research' bo'lmagan yoki pending emas) joylarga tegmaydi.
func LoadCandidates(ctx context.Context, db *pgxpool.Pool, dir string) (CandidateReport, error) {
	var rep CandidateReport
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return rep, err
	}
	sort.Strings(files)
	seen := map[string]string{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return rep, err
		}
		var cf candFile
		if err := json.Unmarshal(raw, &cf); err != nil {
			return rep, fmt.Errorf("%s: %w", f, err)
		}
		err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			for _, c := range cf.Cities {
				if c.ID == "" || c.Name["uz"] == "" || !c.Center.Valid() || c.Center.Lat == 0 {
					rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: shahar %q — nom yoki markaz yo'q", filepath.Base(f), c.ID))
					continue
				}
				em, _ := json.Marshal(defaultEmergency)
				// Yangi shahar: Yandex Go "Start" tarifiga yaqin (4 600 so'm + ~1 050/km + daqiqa ≈ 2 400/km o'rtacha).
				if _, err := tx.Exec(ctx, `INSERT INTO cities (id, lat, lng, taxi_base_fare, taxi_per_km, emergency)
					VALUES ($1,$2,$3,4600,2400,$4) ON CONFLICT (id) DO NOTHING`, c.ID, c.Center.Lat, c.Center.Lng, em); err != nil {
					return err
				}
				for lang, name := range c.Name {
					if i18n.Valid(lang) && name != "" {
						if _, err := tx.Exec(ctx, `INSERT INTO city_i18n (city_id, lang, name) VALUES ($1,$2,$3)
							ON CONFLICT (city_id, lang) DO NOTHING`, c.ID, lang, name); err != nil {
							return err
						}
					}
				}
			}
			for _, p := range cf.Places {
				if prev, dup := seen[p.ID]; dup {
					rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: takror id (%s da ham bor)", p.ID, prev))
					continue
				}
				seen[p.ID] = filepath.Base(f)
				if why := checkCandidate(p); why != "" {
					rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s (%s): %s", p.ID, p.Name["uz"], why))
					continue
				}
				var exists bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cities WHERE id = $1)`, p.CityID).Scan(&exists); err != nil {
					return err
				}
				if !exists {
					rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: shahar %q bazada yo'q", p.ID, p.CityID))
					continue
				}
				var status, source string
				err := tx.QueryRow(ctx, `SELECT status, source FROM places WHERE id = $1`, p.ID).Scan(&status, &source)
				isNew := err == pgx.ErrNoRows
				if err != nil && !isNew {
					return err
				}
				if !isNew && (source != "research" || status != "pending") {
					rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: admin allaqachon ko'rib chiqqan (%s/%s) — tegilmadi", p.ID, source, status))
					continue
				}
				if err := upsertCandidate(ctx, tx, p, isNew); err != nil {
					return fmt.Errorf("%s: %w", p.ID, err)
				}
				if isNew {
					rep.Imported = append(rep.Imported, p.ID)
				} else {
					rep.Updated = append(rep.Updated, p.ID)
				}
			}
			return nil
		})
		if err != nil {
			return rep, fmt.Errorf("%s: %w", f, err)
		}
	}
	return rep, nil
}

func checkCandidate(p candPlace) string {
	switch {
	case !candIDRe.MatchString(p.ID):
		return "id formati noto'g'ri"
	case !domain.ValidCategory(p.Category):
		return "noma'lum kategoriya " + p.Category
	case !p.Novelty.Valid():
		return "noma'lum novelty"
	case strings.TrimSpace(p.Name["uz"]) == "" || strings.TrimSpace(p.Tagline["uz"]) == "":
		return "uz nom yoki tagline yo'q"
	case p.Lat == nil || p.Lng == nil || *p.Lat == 0 || !(domain.LatLng{Lat: *p.Lat, Lng: *p.Lng}).Valid():
		return "koordinata yo'q — admin panelda qo'lda qo'shish kerak"
	case *p.Lat < 37 || *p.Lat > 46 || *p.Lng < 55 || *p.Lng > 74:
		return "koordinata O'zbekistondan tashqarida"
	case len(p.Sources) == 0:
		return "manba yo'q"
	case p.Confidence == "low":
		return "ishonch past — qo'lda tekshirib, admin panelda qo'shing"
	}
	for _, in := range p.Interests {
		if !domain.ValidInterest(in) {
			return "noma'lum qiziqish " + in
		}
	}
	if p.Hours != nil && *p.Hours != "" {
		if _, err := hours(*p.Hours, nil); err != nil {
			return "ish vaqti formati noto'g'ri"
		}
	}
	return ""
}

func upsertCandidate(ctx context.Context, tx pgx.Tx, p candPlace, isNew bool) error {
	var h []domain.OpeningHours
	if p.Hours != nil {
		h, _ = hours(*p.Hours, nil)
	}
	hj, _ := json.Marshal(h)
	if h == nil {
		hj = []byte("[]")
	}
	price := 0
	if p.PriceLevel != nil && *p.PriceLevel >= 0 && *p.PriceLevel <= 3 {
		price = *p.PriceLevel
	}
	interests := p.Interests
	if interests == nil {
		interests = []string{}
	}
	var opened *string
	if p.OpenedAt != nil && *p.OpenedAt != "" {
		d := *p.OpenedAt
		if len(d) == 7 { // YYYY-MM
			d += "-01"
		}
		if _, err := time.Parse(time.DateOnly, d); err == nil {
			opened = &d
		}
	}
	contacts := map[string]string{}
	for k, v := range map[string]string{"instagram": p.Instagram, "telegram": p.Telegram, "website": p.Website} {
		if v = strings.TrimSpace(v); v != "" {
			contacts[k] = v
		}
	}
	cj, _ := json.Marshal(contacts)
	research, _ := json.Marshal(map[string]any{"confidence": p.Confidence, "coordsSource": p.CoordsSrc, "priceNote": p.PriceNote,
		"notes": p.Notes, "sources": p.Sources, "importedAt": time.Now().UTC().Format(time.RFC3339)})
	visit := 45
	switch p.Category {
	case "restaurant":
		visit = 60
	case "viewpoint", "mosque":
		visit = 30
	case "ecovillage", "park":
		visit = 90
	case "museum":
		visit = 60
	}
	if isNew {
		if _, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, opened_at, status, lat, lng, interests,
			price_level, visit_duration_min, opening_hours, source, contacts, research)
			VALUES ($1,$2,$3,$4,$5::date,'pending',$6,$7,$8,$9,$10,$11,'research',$12,$13)`,
			p.ID, p.CityID, p.Category, p.Novelty, opened, *p.Lat, *p.Lng, interests, price, visit, hj, cj, research); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE places SET city_id=$2, category=$3, novelty=$4, opened_at=$5::date, lat=$6, lng=$7,
			interests=$8, price_level=$9, opening_hours=$10, contacts=$11, research=$12, updated_at=now() WHERE id=$1`,
			p.ID, p.CityID, p.Category, p.Novelty, opened, *p.Lat, *p.Lng, interests, price, hj, cj, research); err != nil {
			return err
		}
	}
	src, _ := json.Marshal(p.Sources)
	if _, err := tx.Exec(ctx, `DELETE FROM place_i18n WHERE place_id = $1`, p.ID); err != nil {
		return err
	}
	for _, lang := range i18n.Supported {
		name := strings.TrimSpace(p.Name[lang])
		tag := strings.TrimSpace(p.Tagline[lang])
		if name == "" || tag == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address, sources)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, p.ID, lang, name, tag, strings.TrimSpace(p.Description[lang]), p.Address, src); err != nil {
			return err
		}
	}
	return nil
}
