// Package seed — seed/*.json fayllarini bazaga yuklaydi (idempotent UPSERT).
//
// Ikki xil fayl:
//   - shahar katalogi (`city` + places/prices/events/stays) — scripts/catalog/build_seed.py yasaydi;
//   - faqat xizmatlar (`cityId` + services) — SOS uchun demo shifoxona, dorixona va h.k.
//
// Katalog qayta yuklanganda shu shahardagi avvalgi seed joylari (source = demo | open_data), agar yangi
// faylda bo'lmasa, `draft` holatiga o'tkaziladi (o'chirilmaydi: eski marshrutlar ularga havola qiladi).
//
// Ishonch darajasi (official | public | community | sample): joy, tarix va qoida uchun `trust`, ish vaqti uchun
// `hoursTrust`, kirish narxi uchun `feeTrust`, narxlar va turar joy narxi uchun `trust` / `priceTrust`.
// Ko'rsatilmasa: source = demo bo'lsa sample, aks holda public.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
	"yoldosh/internal/i18n"
)

// LoadDir — papkadagi barcha *.json fayllarni nom tartibida yuklaydi. Har fayl alohida tranzaksiyada.
func LoadDir(ctx context.Context, db *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s: seed fayllari topilmadi", dir)
	}
	slices.Sort(files)
	// Daromad funksiyalari fayli (hamkorlar, takliflar, yo'nalishlar) alohida va oxirida: hamkor shaharga havola qiladi.
	// Shahar muqovalari ham alohida va kataloglardan keyin: katalog `city.cover` ini ustidan yozadi.
	monetization, covers := "", ""
	files = slices.DeleteFunc(files, func(f string) bool {
		switch filepath.Base(f) {
		case MonetizationFile:
			monetization = f
		case CityCoversFile:
			covers = f
		default:
			return false
		}
		return true
	})
	// Shahar kataloglari xizmat fayllaridan oldin (xizmat shaharga havola qiladi).
	sort := func(f string) int {
		if strings.HasPrefix(filepath.Base(f), "sos_") {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(files, func(a, b string) int { return sort(a) - sort(b) })
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
		var retired int64
		if err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			var err error
			retired, err = load(ctx, tx, sf)
			return err
		}); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		slog.Info("yuklandi", "file", f, "city", sf.cityID(), "places", len(sf.Places), "prices", len(sf.Prices),
			"events", len(sf.Events), "stays", len(sf.Stays), "services", len(sf.Services), "retired", retired)
	}
	if covers != "" {
		if err := loadCityCovers(ctx, db, covers); err != nil {
			return err
		}
	}
	if monetization != "" {
		return loadMonetization(ctx, db, monetization)
	}
	return nil
}

type seedCity struct {
	ID           string            `json:"id"`
	Name         map[string]string `json:"name"`
	Center       domain.LatLng     `json:"center"`
	Cover        string            `json:"cover"` // static/places dagi fayl nomi
	TaxiBaseFare int64             `json:"taxiBaseFare"`
	TaxiPerKm    int64             `json:"taxiPerKm"`
	Emergency    map[string]string `json:"emergency"`
}

type seedFile struct {
	City     *seedCity     `json:"city"`
	CityID   string        `json:"cityId"` // faqat xizmatlar fayli uchun
	Places   []seedPlace   `json:"places"`
	Prices   []seedPrice   `json:"prices"`
	Events   []seedEvent   `json:"events"`
	Stays    []seedStay    `json:"stays"`
	Services []seedService `json:"services"`
}

func (sf seedFile) cityID() string {
	if sf.City != nil {
		return sf.City.ID
	}
	return sf.CityID
}

type seedService struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind"`
	Name    map[string]string `json:"name"`
	Address map[string]string `json:"address"`
	Lat     float64           `json:"lat"`
	Lng     float64           `json:"lng"`
	Phone   *string           `json:"phone"`
	Hours   string            `json:"hours"`
	Open24h bool              `json:"open24h"`
	Accepts []string          `json:"accepts"`
}

type seedText struct {
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	History     string   `json:"history"`
	Facts       []string `json:"facts"`
	BestTime    string   `json:"bestTime"`
	HoursNote   string   `json:"hoursNote"`
	FeeNote     string   `json:"feeNote"`
}

type seedImage struct {
	File      string `json:"file"`
	Credit    string `json:"credit"`
	License   string `json:"license"`
	SourceURL string `json:"sourceUrl"`
	Source    string `json:"source"`
}

func (im *seedImage) image() *domain.Image {
	if im == nil || im.File == "" {
		return nil
	}
	return &domain.Image{URL: "/static/places/" + im.File, Credit: im.Credit, License: im.License, SourceURL: im.SourceURL, Source: im.Source}
}

type seedSource struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
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
	Hours           string              `json:"hours"`        // "09:00-18:00" har kuni
	Lunch           string              `json:"lunch"`        // "13:00-14:00" — har kungi tanaffus
	FridayPrayer    string              `json:"fridayPrayer"` // "12:00-14:00" — juma kuni yopiq
	Aliases         []string            `json:"aliases"`
	Closed          []int               `json:"closed"` // dam olish kunlari (0 = yakshanba)
	Rating          float64             `json:"rating"`
	Reviews         int                 `json:"reviews"`
	Crowd           string              `json:"crowd"`
	Wheelchair      bool                `json:"wheelchair"`
	FamilyFriendly  *bool               `json:"familyFriendly"`
	EasyWalk        *bool               `json:"easyWalk"`
	PayCard         bool                `json:"payCard"`
	CashOnly        bool                `json:"cashOnly"`
	IsPartner       bool                `json:"isPartner"`
	GeofenceRadiusM int                 `json:"geofenceRadiusM"`
	FeeLocal        *int64              `json:"feeLocal"`
	FeeForeigner    *int64              `json:"feeForeigner"`
	Source          string              `json:"source"`
	Trust           string              `json:"trust"`      // joy, tarix va qoidalar
	HoursTrust      string              `json:"hoursTrust"` // ish vaqti (tadqiqotda namuna)
	FeeTrust        string              `json:"feeTrust"`   // kirish narxi
	Image           *seedImage          `json:"image"`
	HistorySources  []seedSource        `json:"historySources"`
	Research        json.RawMessage     `json:"research"`
	Text            map[string]seedText `json:"text"`
	Rules           []struct {
		Code     string            `json:"code"`
		Severity string            `json:"severity"`
		Priority int               `json:"priority"`
		Text     map[string]string `json:"text"`
		Source   string            `json:"source"`
		Trust    string            `json:"trust"`
	} `json:"rules"`
}

type seedPrice struct {
	ID        string            `json:"id"`
	Category  string            `json:"category"`
	Name      map[string]string `json:"name"`
	Unit      map[string]string `json:"unit"`
	Min       int64             `json:"min"`
	Max       int64             `json:"max"`
	Tourist   *int64            `json:"tourist"`
	Source    string            `json:"source"`
	Year      *int              `json:"year"`
	UpdatedAt string            `json:"updatedAt"` // YYYY-MM-DD
	PlaceIDs  []string          `json:"placeIds"`
	Note      string            `json:"note"`
	Trust     string            `json:"trust"`
}

type seedEvent struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Title     map[string]string `json:"title"`
	Venue     map[string]string `json:"venue"`
	Start     string            `json:"start"` // mahalliy vaqt "2026-12-11T19:30"
	End       string            `json:"end"`
	Lat       float64           `json:"lat"`
	Lng       float64           `json:"lng"`
	Price     *int64            `json:"price"`
	IsFree    bool              `json:"isFree"`
	TicketURL *string           `json:"ticketUrl"`
	Source    *string           `json:"source"`
	Recurring *string           `json:"recurring"`
	Image     *seedImage        `json:"image"`
}

// seedStay — upstream stays jadvali shaklida (0008): nom 3 tilda, manzil matn, narx oralig'i, qulaylik kodlari.
type seedStay struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Name         map[string]string `json:"name"`
	Lat          float64           `json:"lat"`
	Lng          float64           `json:"lng"`
	Address      string            `json:"address"`
	Stars        *int              `json:"stars"`
	PriceMin     *int64            `json:"priceMin"` // so'm / kecha
	PriceMax     *int64            `json:"priceMax"`
	PriceTrust   string            `json:"priceTrust"`
	PriceNote    string            `json:"priceNote"`
	PriceSource  string            `json:"priceSource"` // URL
	Rating       *float64          `json:"rating"`      // 5 ballik
	RatingSource string            `json:"ratingSource"`
	ReviewsCount int               `json:"reviewsCount"`
	Amenities    []string          `json:"amenities"` // wifi, breakfast, ac, parking, transfer, courtyard, ...
	Contacts     map[string]string `json:"contacts"`
	Image        *seedImage        `json:"image"`
	Trust        string            `json:"trust"`
}

var (
	priceCats = []string{"taxi", "food", "souvenir", "market", "ticket", "transport"}
	eventKind = []string{"festival", "concert", "fair", "exhibition", "sport"}
	stayKind  = []string{"hotel", "boutique", "guesthouse", "hostel", "motel", "apartment"}
	trusts    = []string{"official", "public", "community", "sample"}
	ruleCodes = []string{"no_flash", "no_photo", "photo_fee", "dress_code", "shoes_off", "quiet", "no_drone", "no_food",
		"no_smoking", "no_pets", "ticket_required", "prayer_time_limit"}
	// Seed yozuvlarida eventlar mahalliy vaqtda (barcha shaharlar Asia/Tashkent).
	tashkent = func() *time.Location {
		if l, err := time.LoadLocation("Asia/Tashkent"); err == nil {
			return l
		}
		return time.FixedZone("UZT", 5*3600)
	}()
)

func localTime(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02T15:04", s, tashkent)
}

func validLL(lat, lng float64) bool { return (domain.LatLng{Lat: lat, Lng: lng}).Valid() && lat != 0 }

func validate(sf seedFile) error {
	if sf.City == nil {
		if sf.CityID == "" || len(sf.Places)+len(sf.Prices)+len(sf.Events)+len(sf.Stays) > 0 {
			return fmt.Errorf("city (katalog) yoki cityId + services (xizmatlar fayli) shart")
		}
	} else if sf.City.ID == "" || sf.City.Name["uz"] == "" || !validLL(sf.City.Center.Lat, sf.City.Center.Lng) {
		return fmt.Errorf("city.id, city.name.uz va city.center shart")
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
		case !validLL(p.Lat, p.Lng):
			return fmt.Errorf("%s: koordinata noto'g'ri", p.ID)
		case p.Crowd != "" && !slices.Contains([]string{"low", "medium", "high"}, p.Crowd):
			return fmt.Errorf("%s: crowd low | medium | high", p.ID)
		case !validTrust(p.Trust) || !validTrust(p.HoursTrust) || !validTrust(p.FeeTrust):
			return fmt.Errorf("%s: trust %s", p.ID, strings.Join(trusts, " | "))
		}
		for _, b := range []string{p.Lunch, p.FridayPrayer} {
			if b == "" {
				continue
			}
			f, t, ok := strings.Cut(b, "-")
			fm, err1 := domain.ParseHHMM(f)
			tm, err2 := domain.ParseHHMM(t)
			if !ok || err1 != nil || err2 != nil || tm <= fm {
				return fmt.Errorf("%s: tanaffus %q: HH:MM-HH:MM", p.ID, b)
			}
		}
		for _, in := range p.Interests {
			if !domain.ValidInterest(in) {
				return fmt.Errorf("%s: noma'lum qiziqish %q", p.ID, in)
			}
		}
		for _, r := range p.Rules {
			if !slices.Contains(ruleCodes, r.Code) || !slices.Contains([]string{"info", "warn", "forbidden"}, r.Severity) ||
				!validTrust(r.Trust) {
				return fmt.Errorf("%s: noto'g'ri qoida %s/%s/%s", p.ID, r.Code, r.Severity, r.Trust)
			}
		}
		for lang := range p.Text {
			if !i18n.Valid(lang) {
				return fmt.Errorf("%s: noma'lum til %q", p.ID, lang)
			}
		}
		if _, err := hours(p.Hours, p.Closed); err != nil {
			return fmt.Errorf("%s: %w", p.ID, err)
		}
		seen[p.ID] = true
	}
	for _, x := range sf.Prices {
		switch {
		case x.ID == "" || seen[x.ID]:
			return fmt.Errorf("bo'sh yoki takror narx id: %q", x.ID)
		case !slices.Contains(priceCats, x.Category):
			return fmt.Errorf("%s: noma'lum narx turi %q", x.ID, x.Category)
		case x.Name["uz"] == "" || x.Unit["uz"] == "":
			return fmt.Errorf("%s: uz nom va birlik shart", x.ID)
		case x.Min < 0 || x.Max < x.Min:
			return fmt.Errorf("%s: min <= max", x.ID)
		case !validTrust(x.Trust):
			return fmt.Errorf("%s: trust %s", x.ID, strings.Join(trusts, " | "))
		}
		seen[x.ID] = true
	}
	for _, e := range sf.Events {
		st, err1 := localTime(e.Start)
		en, err2 := localTime(e.End)
		switch {
		case e.ID == "" || seen[e.ID]:
			return fmt.Errorf("bo'sh yoki takror tadbir id: %q", e.ID)
		case !slices.Contains(eventKind, e.Kind):
			return fmt.Errorf("%s: noma'lum tadbir turi %q", e.ID, e.Kind)
		case e.Title["uz"] == "":
			return fmt.Errorf("%s: uz nom shart", e.ID)
		case err1 != nil || err2 != nil || en.Before(st):
			return fmt.Errorf("%s: start/end YYYY-MM-DDTHH:MM, start <= end", e.ID)
		case !validLL(e.Lat, e.Lng):
			return fmt.Errorf("%s: koordinata noto'g'ri", e.ID)
		}
		seen[e.ID] = true
	}
	for _, s := range sf.Stays {
		switch {
		case s.ID == "" || seen[s.ID]:
			return fmt.Errorf("bo'sh yoki takror turar joy id: %q", s.ID)
		case !slices.Contains(stayKind, s.Kind):
			return fmt.Errorf("%s: noma'lum turar joy turi %q", s.ID, s.Kind)
		case s.Name["uz"] == "":
			return fmt.Errorf("%s: uz nom shart", s.ID)
		case s.PriceMin != nil && *s.PriceMin <= 0, s.PriceMin != nil && s.PriceMax != nil && *s.PriceMax < *s.PriceMin:
			return fmt.Errorf("%s: priceMin > 0, priceMax >= priceMin", s.ID)
		case !validTrust(s.Trust) || !validTrust(s.PriceTrust):
			return fmt.Errorf("%s: trust %s", s.ID, strings.Join(trusts, " | "))
		case s.Rating != nil && (*s.Rating < 0 || *s.Rating > 5):
			return fmt.Errorf("%s: reyting 0..5 (10 ballik bo'lsa 2 ga bo'ling)", s.ID)
		case !validLL(s.Lat, s.Lng):
			return fmt.Errorf("%s: koordinata noto'g'ri", s.ID)
		}
		seen[s.ID] = true
	}
	kinds := map[string]bool{"hospital": true, "clinic": true, "pharmacy": true, "police": true, "embassy": true, "atm": true,
		"exchange": true, "fuel": true, "ev_charging": true}
	for _, sv := range sf.Services {
		switch {
		case sv.ID == "" || seen[sv.ID]:
			return fmt.Errorf("bo'sh yoki takror xizmat id: %q", sv.ID)
		case !kinds[sv.Kind]:
			return fmt.Errorf("%s: noma'lum xizmat turi %q", sv.ID, sv.Kind)
		case sv.Name["uz"] == "":
			return fmt.Errorf("%s: uz nom shart", sv.ID)
		case !validLL(sv.Lat, sv.Lng):
			return fmt.Errorf("%s: koordinata noto'g'ri", sv.ID)
		}
		if _, err := hours(sv.Hours, nil); err != nil {
			return fmt.Errorf("%s: %w", sv.ID, err)
		}
		seen[sv.ID] = true
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
	om, err := domain.ParseHHMM(o)
	if err != nil {
		return nil, err
	}
	cm, err := domain.ParseHHMM(c)
	if err != nil {
		return nil, err
	}
	if cm <= om {
		return nil, fmt.Errorf("hours %q: yopilish ochilishdan keyin bo'lsin (yarim tun = 24:00)", spec)
	}
	var out []domain.OpeningHours
	for d := range 7 {
		if !slices.Contains(closed, d) {
			out = append(out, domain.OpeningHours{Day: d, Open: o, Close: c})
		}
	}
	return out, nil
}

func orTrue(b *bool) bool { return b == nil || *b }

// validTrust — bo'sh (standart qo'llanadi) yoki ma'lum daraja.
func validTrust(t string) bool { return t == "" || slices.Contains(trusts, t) }

// trustOr — ko'rsatilgan daraja, bo'lmasa def.
func trustOr(t, def string) string {
	if t != "" {
		return t
	}
	return def
}

// withBreaks — "13:00-14:00" tushlik (har kuni) va juma namozi tanaffusi ish vaqtiga qo'shiladi.
func withBreaks(h []domain.OpeningHours, lunch, friday string) []domain.OpeningHours {
	add := func(spec, reason string, onlyDay int) {
		f, t, ok := strings.Cut(spec, "-")
		if !ok {
			return
		}
		for i := range h {
			if onlyDay < 0 || h[i].Day == onlyDay {
				h[i].Breaks = append(h[i].Breaks, domain.Break{From: f, To: t, Reason: reason})
			}
		}
	}
	add(lunch, "lunch", -1)
	add(friday, "prayer", 5)
	return h
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func jsonOf(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func load(ctx context.Context, tx pgx.Tx, sf seedFile) (int64, error) {
	var retired int64
	if c := sf.City; c != nil {
		var cover *string
		if c.Cover != "" {
			v := "/static/places/" + c.Cover
			cover = &v
		}
		if _, err := tx.Exec(ctx, `INSERT INTO cities (id, lat, lng, cover_url, taxi_base_fare, taxi_per_km, emergency)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng, cover_url = COALESCE(EXCLUDED.cover_url, cities.cover_url),
			  taxi_base_fare = EXCLUDED.taxi_base_fare, taxi_per_km = EXCLUDED.taxi_per_km, emergency = EXCLUDED.emergency`,
			c.ID, c.Center.Lat, c.Center.Lng, cover, c.TaxiBaseFare, c.TaxiPerKm, jsonOf(c.Emergency)); err != nil {
			return 0, err
		}
		for lang, name := range c.Name {
			if _, err := tx.Exec(ctx, `INSERT INTO city_i18n (city_id, lang, name) VALUES ($1,$2,$3)
				ON CONFLICT (city_id, lang) DO UPDATE SET name = EXCLUDED.name`, c.ID, lang, name); err != nil {
				return 0, err
			}
		}
		ids := make([]string, 0, len(sf.Places))
		for _, p := range sf.Places {
			if err := loadPlace(ctx, tx, c.ID, p); err != nil {
				return 0, fmt.Errorf("place %s: %w", p.ID, err)
			}
			ids = append(ids, p.ID)
		}
		// Avvalgi seed dagi, endi katalogda yo'q joylar (o'ylab topilgan demo yoki boshqa id dagi takror) yashiriladi.
		tag, err := tx.Exec(ctx, `UPDATE places SET status = 'draft', updated_at = now()
			WHERE city_id = $1 AND source IN ('demo','open_data') AND status = 'published' AND id <> ALL($2::text[])`, c.ID, ids)
		if err != nil {
			return 0, err
		}
		retired = tag.RowsAffected()
		if err := loadPrices(ctx, tx, c.ID, sf.Prices); err != nil {
			return 0, err
		}
		if err := loadEvents(ctx, tx, c.ID, sf.Events); err != nil {
			return 0, err
		}
		if err := loadStays(ctx, tx, c.ID, sf.Stays); err != nil {
			return 0, err
		}
	}
	if len(sf.Services) > 0 {
		// osm-import demo xizmatlarni haqiqiylariga almashtiradi; shundan keyin seed ularni qayta qo'shmaydi.
		var real bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM city_services WHERE city_id = $1 AND osm_id IS NOT NULL)`,
			sf.cityID()).Scan(&real); err != nil {
			return 0, err
		}
		if real {
			slog.Info("demo xizmatlar o'tkazib yuborildi: shaharda OSM xizmatlari bor", "city", sf.cityID())
			return retired, nil
		}
	}
	for _, sv := range sf.Services {
		h, _ := hours(sv.Hours, nil)
		addr := []byte("{}")
		if sv.Address != nil {
			addr = jsonOf(sv.Address)
		}
		if sv.Accepts == nil {
			sv.Accepts = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO city_services (id, city_id, kind, name, address, lat, lng, phone, hours, open_24h, accepts, source)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'demo')
			ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, kind = EXCLUDED.kind, name = EXCLUDED.name,
			  address = EXCLUDED.address, lat = EXCLUDED.lat, lng = EXCLUDED.lng, phone = EXCLUDED.phone, hours = EXCLUDED.hours,
			  open_24h = EXCLUDED.open_24h, accepts = EXCLUDED.accepts, updated_at = now()`,
			sv.ID, sf.cityID(), sv.Kind, jsonOf(sv.Name), addr, sv.Lat, sv.Lng, sv.Phone, jsonOf(h), sv.Open24h, sv.Accepts); err != nil {
			return 0, fmt.Errorf("service %s: %w", sv.ID, err)
		}
	}
	return retired, nil
}

func loadPlace(ctx context.Context, tx pgx.Tx, cityID string, p seedPlace) error {
	h, _ := hours(p.Hours, p.Closed)
	h = withBreaks(h, p.Lunch, p.FridayPrayer)
	if h == nil {
		h = []domain.OpeningHours{}
	}
	if p.Aliases == nil {
		p.Aliases = []string{}
	}
	if p.GeofenceRadiusM == 0 {
		p.GeofenceRadiusM = 80
	}
	if p.VisitMin == 0 {
		p.VisitMin = 45
	}
	if p.Interests == nil {
		p.Interests = []string{}
	}
	if p.Source == "" {
		p.Source = "demo"
	}
	def := "public"
	if p.Source == "demo" {
		def = "sample"
	}
	trust := trustOr(p.Trust, def)
	images := []domain.Image{}
	if im := p.Image.image(); im != nil {
		images = append(images, *im)
	}
	var fee []byte
	if p.FeeLocal != nil || p.FeeForeigner != nil {
		f := map[string]any{}
		if p.FeeLocal != nil {
			f["local"] = domain.UZS(*p.FeeLocal)
		}
		if p.FeeForeigner != nil {
			f["foreigner"] = domain.UZS(*p.FeeForeigner)
		}
		var r struct {
			FeeSource string `json:"feeSource"`
			FeeYear   int    `json:"feeYear"`
		}
		_ = json.Unmarshal(p.Research, &r)
		if r.FeeSource != "" {
			f["source"] = r.FeeSource
		}
		if r.FeeYear > 0 {
			f["year"] = r.FeeYear
		}
		f["trust"] = trustOr(p.FeeTrust, trust)
		fee = jsonOf(f)
	}
	var research []byte
	if len(p.Research) > 0 && string(p.Research) != "null" {
		research = p.Research
	}
	if _, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, opened_at, status, lat, lng,
		geofence_radius_m, interests, price_level, visit_duration_min, opening_hours, entry_fee, wheelchair, family_friendly, easy_walk,
		pay_card, cash_only, is_partner, rating, reviews_count, crowd, images, research, source, published_at, aliases, trust, hours_trust)
		VALUES ($1,$2,$3,$4,$5,'published',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,now(),$26,$27,$28)
		ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, category = EXCLUDED.category, novelty = EXCLUDED.novelty,
		  opened_at = EXCLUDED.opened_at, status = 'published', lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		  geofence_radius_m = EXCLUDED.geofence_radius_m, interests = EXCLUDED.interests, price_level = EXCLUDED.price_level,
		  visit_duration_min = EXCLUDED.visit_duration_min, opening_hours = EXCLUDED.opening_hours, entry_fee = EXCLUDED.entry_fee,
		  wheelchair = EXCLUDED.wheelchair, family_friendly = EXCLUDED.family_friendly, easy_walk = EXCLUDED.easy_walk,
		  pay_card = EXCLUDED.pay_card, cash_only = EXCLUDED.cash_only, is_partner = EXCLUDED.is_partner,
		  -- foydalanuvchi baholari bo'lsa (AddVisit qayta hisoblagan) reyting saqlanadi
		  rating = CASE WHEN places.reviews_count > EXCLUDED.reviews_count THEN places.rating ELSE EXCLUDED.rating END,
		  reviews_count = GREATEST(places.reviews_count, EXCLUDED.reviews_count),
		  crowd = EXCLUDED.crowd, images = EXCLUDED.images, research = EXCLUDED.research, source = EXCLUDED.source,
		  aliases = EXCLUDED.aliases, hours_trust = EXCLUDED.hours_trust,
		  -- obyekt ma'muriyati tasdiqlagan (official) daraja seed bilan pasaymaydi
		  trust = CASE WHEN places.trust = 'official' THEN places.trust ELSE EXCLUDED.trust END,
		  published_at = COALESCE(places.published_at, now()), rules_version = places.rules_version + 1, updated_at = now()`,
		p.ID, cityID, p.Category, p.Novelty, p.OpenedAt, p.Lat, p.Lng, p.GeofenceRadiusM, p.Interests, p.PriceLevel, p.VisitMin,
		jsonOf(h), fee, p.Wheelchair, orTrue(p.FamilyFriendly), orTrue(p.EasyWalk), p.PayCard, p.CashOnly, p.IsPartner,
		p.Rating, p.Reviews, nilIfEmpty(p.Crowd), jsonOf(images), research, p.Source, p.Aliases, trust,
		trustOr(p.HoursTrust, trust)); err != nil {
		return err
	}
	sources := []seedSource{}
	if p.HistorySources != nil {
		sources = p.HistorySources
	}
	if _, err := tx.Exec(ctx, `DELETE FROM place_i18n WHERE place_id = $1`, p.ID); err != nil {
		return err
	}
	for lang, t := range p.Text {
		var hist *string
		src := []byte("[]")
		if t.History != "" {
			hist = &t.History
			src = jsonOf(sources)
		}
		if t.Facts == nil {
			t.Facts = []string{}
		}
		var histTrust *string
		if hist != nil {
			histTrust = &trust
		}
		if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address,
			history_summary, history_facts, sources, history_trust, best_time, hours_note, fee_note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			p.ID, lang, t.Name, t.Tagline, t.Description, t.Address, hist, t.Facts, src, histTrust,
			nilIfEmpty(t.BestTime), nilIfEmpty(t.HoursNote), nilIfEmpty(t.FeeNote)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM place_rules WHERE place_id = $1`, p.ID); err != nil {
		return err
	}
	for _, r := range p.Rules {
		var by *string
		if p.Source == "demo" {
			v := "demo"
			by = &v
		}
		if _, err := tx.Exec(ctx, `INSERT INTO place_rules (place_id, code, severity, priority, text, source, trust, verified_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, p.ID, r.Code, r.Severity, r.Priority, jsonOf(r.Text), nilIfEmpty(r.Source),
			trustOr(r.Trust, trust), by); err != nil {
			return err
		}
	}
	return nil
}

func loadPrices(ctx context.Context, tx pgx.Tx, cityID string, prices []seedPrice) error {
	for _, x := range prices {
		updated := time.Now()
		if t, err := time.Parse(time.DateOnly, x.UpdatedAt); err == nil {
			updated = t
		}
		if x.PlaceIDs == nil {
			x.PlaceIDs = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO price_items (id, city_id, category, name, unit, base_min, base_max, tourist,
			place_ids, source_url, source_year, note, base_updated_at, source, trust)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'open_data',$14)
			ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, category = EXCLUDED.category, name = EXCLUDED.name,
			  unit = EXCLUDED.unit, base_min = EXCLUDED.base_min, base_max = EXCLUDED.base_max, tourist = EXCLUDED.tourist,
			  place_ids = EXCLUDED.place_ids, source_url = EXCLUDED.source_url, source_year = EXCLUDED.source_year,
			  note = EXCLUDED.note, base_updated_at = EXCLUDED.base_updated_at, trust = EXCLUDED.trust`,
			x.ID, cityID, x.Category, jsonOf(x.Name), jsonOf(x.Unit), x.Min, x.Max, x.Tourist, x.PlaceIDs,
			nilIfEmpty(x.Source), x.Year, nilIfEmpty(x.Note), updated, trustOr(x.Trust, "sample")); err != nil {
			return fmt.Errorf("price %s: %w", x.ID, err)
		}
	}
	return nil
}

func loadEvents(ctx context.Context, tx pgx.Tx, cityID string, events []seedEvent) error {
	for _, e := range events {
		st, _ := localTime(e.Start)
		en, _ := localTime(e.End)
		var img []byte
		if im := e.Image.image(); im != nil {
			img = jsonOf(im)
		}
		venue := e.Venue
		if venue == nil {
			venue = e.Title
		}
		if _, err := tx.Exec(ctx, `INSERT INTO events (id, city_id, kind, title, venue, starts_at, ends_at, lat, lng, price, is_free,
			ticket_url, source_url, recurring, image) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, kind = EXCLUDED.kind, title = EXCLUDED.title,
			  venue = EXCLUDED.venue, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at, lat = EXCLUDED.lat,
			  lng = EXCLUDED.lng, price = EXCLUDED.price, is_free = EXCLUDED.is_free, ticket_url = EXCLUDED.ticket_url,
			  source_url = EXCLUDED.source_url, recurring = EXCLUDED.recurring, image = EXCLUDED.image, updated_at = now()`,
			e.ID, cityID, e.Kind, jsonOf(e.Title), jsonOf(venue), st, en, e.Lat, e.Lng, e.Price, e.IsFree,
			e.TicketURL, e.Source, e.Recurring, img); err != nil {
			return fmt.Errorf("event %s: %w", e.ID, err)
		}
	}
	return nil
}

// loadStays — tadqiqot turar joylari upstream stays jadvaliga (source = research). OSM importi va admin tahriri
// bilan to'qnashmaydi: admin tahrir qilgan yozuv (source = manual) seed bilan qayta yozilmaydi.
func loadStays(ctx context.Context, tx pgx.Tx, cityID string, stays []seedStay) error {
	for _, s := range stays {
		images := []domain.Image{}
		if im := s.Image.image(); im != nil {
			images = append(images, *im)
		}
		am := s.Amenities
		if am == nil {
			am = []string{}
		}
		contacts := s.Contacts
		if contacts == nil {
			contacts = map[string]string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stays (id, city_id, kind, status, name, lat, lng, address, stars, price_min, price_max,
			price_trust, price_note, price_source, price_updated_at, amenities, contacts, rating, rating_source, reviews_count, images,
			fair_price, trust, source)
			VALUES ($1,$2,$3,'published',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now(),$14,$15,$16,$17,$18,$19,true,$20,'research')
			ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, kind = EXCLUDED.kind, name = EXCLUDED.name,
			  lat = EXCLUDED.lat, lng = EXCLUDED.lng, address = EXCLUDED.address, stars = EXCLUDED.stars,
			  price_min = EXCLUDED.price_min, price_max = EXCLUDED.price_max, price_trust = EXCLUDED.price_trust,
			  price_note = EXCLUDED.price_note, price_source = EXCLUDED.price_source,
			  price_updated_at = CASE WHEN stays.price_min IS DISTINCT FROM EXCLUDED.price_min
			    OR stays.price_max IS DISTINCT FROM EXCLUDED.price_max THEN now() ELSE stays.price_updated_at END,
			  amenities = EXCLUDED.amenities, contacts = EXCLUDED.contacts, rating = EXCLUDED.rating,
			  rating_source = EXCLUDED.rating_source, reviews_count = EXCLUDED.reviews_count, images = EXCLUDED.images,
			  trust = EXCLUDED.trust, updated_at = now()
			WHERE stays.source = 'research'`,
			s.ID, cityID, s.Kind, jsonOf(s.Name), s.Lat, s.Lng, s.Address, s.Stars, s.PriceMin, s.PriceMax,
			trustOr(s.PriceTrust, "sample"), nilIfEmpty(s.PriceNote), nilIfEmpty(s.PriceSource), am, jsonOf(contacts),
			s.Rating, nilIfEmpty(s.RatingSource), s.ReviewsCount, jsonOf(images), trustOr(s.Trust, "public")); err != nil {
			return fmt.Errorf("stay %s: %w", s.ID, err)
		}
	}
	return nil
}
