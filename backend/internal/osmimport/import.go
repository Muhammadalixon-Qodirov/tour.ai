package osmimport

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
)

// POI — osm-extract chiqishi.
type POI struct {
	OSM   string            `json:"osm"`
	Group string            `json:"group"`
	Kind  string            `json:"kind"`
	Lat   float64           `json:"lat"`
	Lng   float64           `json:"lng"`
	Tags  map[string]string `json:"tags"`
}

type City struct {
	ID       string
	Name     map[string]string
	Center   domain.LatLng
	RadiusKm float64
}

// Cities — import qamrovi. Mavjud bo'lsa radiusi yangilanadi, bo'lmasa yaratiladi.
var Cities = []City{
	{"tashkent", map[string]string{"uz": "Toshkent", "ru": "Ташкент", "en": "Tashkent"}, domain.LatLng{Lat: 41.3111, Lng: 69.2797}, 22},
	{"samarkand", map[string]string{"uz": "Samarqand", "ru": "Самарканд", "en": "Samarkand"}, domain.LatLng{Lat: 39.6542, Lng: 66.9597}, 12},
	{"bukhara", map[string]string{"uz": "Buxoro", "ru": "Бухара", "en": "Bukhara"}, domain.LatLng{Lat: 39.7747, Lng: 64.4286}, 10},
	{"khiva", map[string]string{"uz": "Xiva", "ru": "Хива", "en": "Khiva"}, domain.LatLng{Lat: 41.3775, Lng: 60.3635}, 6},
	{"urgench", map[string]string{"uz": "Urganch", "ru": "Ургенч", "en": "Urgench"}, domain.LatLng{Lat: 41.5500, Lng: 60.6333}, 8},
	{"nukus", map[string]string{"uz": "Nukus", "ru": "Нукус", "en": "Nukus"}, domain.LatLng{Lat: 42.4600, Lng: 59.6100}, 10},
	{"fergana", map[string]string{"uz": "Farg'ona", "ru": "Фергана", "en": "Fergana"}, domain.LatLng{Lat: 40.3864, Lng: 71.7864}, 10},
	{"kokand", map[string]string{"uz": "Qo'qon", "ru": "Коканд", "en": "Kokand"}, domain.LatLng{Lat: 40.5286, Lng: 70.9425}, 8},
	{"margilan", map[string]string{"uz": "Marg'ilon", "ru": "Маргилан", "en": "Margilan"}, domain.LatLng{Lat: 40.4711, Lng: 71.7247}, 6},
	{"namangan", map[string]string{"uz": "Namangan", "ru": "Наманган", "en": "Namangan"}, domain.LatLng{Lat: 40.9983, Lng: 71.6726}, 10},
	{"andijan", map[string]string{"uz": "Andijon", "ru": "Андижан", "en": "Andijan"}, domain.LatLng{Lat: 40.7821, Lng: 72.3442}, 10},
	{"shahrisabz", map[string]string{"uz": "Shahrisabz", "ru": "Шахрисабз", "en": "Shahrisabz"}, domain.LatLng{Lat: 39.0578, Lng: 66.8342}, 6},
	{"termez", map[string]string{"uz": "Termiz", "ru": "Термез", "en": "Termez"}, domain.LatLng{Lat: 37.2242, Lng: 67.2783}, 8},
	{"navoi", map[string]string{"uz": "Navoiy", "ru": "Навои", "en": "Navoi"}, domain.LatLng{Lat: 40.1039, Lng: 65.3739}, 8},
	{"qarshi", map[string]string{"uz": "Qarshi", "ru": "Карши", "en": "Karshi"}, domain.LatLng{Lat: 38.8606, Lng: 65.7891}, 8},
	{"jizzakh", map[string]string{"uz": "Jizzax", "ru": "Джизак", "en": "Jizzakh"}, domain.LatLng{Lat: 40.1158, Lng: 67.8422}, 7},
	{"bostanliq", map[string]string{"uz": "Bo'stonliq (Chorvoq, Chimyon)", "ru": "Бостанлык (Чарвак, Чимган)", "en": "Bostanliq (Charvak, Chimgan)"}, domain.LatLng{Lat: 41.5900, Lng: 70.0500}, 25},
}

type Report struct {
	Places, PlacesSkippedDup, Stays, Services, DemoServicesRemoved, OutOfArea int
	PlacesRetired                                                             int // OSM dan o'chgan yoki endi mos kelmaydigan joylar yashirildi
	PerCity                                                                   map[string]int
}

func Load(ctx context.Context, db *pgxpool.Pool, path string, usdRate float64) (Report, error) {
	rep := Report{PerCity: map[string]int{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return rep, err
	}
	var pois []POI
	if err := json.Unmarshal(raw, &pois); err != nil {
		return rep, err
	}
	if usdRate <= 0 {
		usdRate = 12000
	}
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		cities, err := ensureCities(ctx, tx)
		if err != nil {
			return err
		}
		existing, err := existingPlaces(ctx, tx)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM city_services WHERE source = 'demo'`)
		if err != nil {
			return err
		}
		rep.DemoServicesRemoved = int(tag.RowsAffected())
		var seen []string
		for _, p := range pois {
			city := nearestCity(cities, p.Lat, p.Lng)
			if city == "" {
				rep.OutOfArea++
				continue
			}
			switch p.Group {
			case "stay":
				if err := upsertStay(ctx, tx, p, city, usdRate); err != nil {
					return fmt.Errorf("%s: %w", p.OSM, err)
				}
				rep.Stays++
			case "food", "sight":
				if dupOf(existing, p) {
					rep.PlacesSkippedDup++
					continue
				}
				ok, err := upsertPlace(ctx, tx, p, city)
				if err != nil {
					return fmt.Errorf("%s: %w", p.OSM, err)
				}
				if !ok {
					continue
				}
				seen = append(seen, idFor(p.OSM))
				rep.Places++
			case "service":
				if err := upsertService(ctx, tx, p, city); err != nil {
					return fmt.Errorf("%s: %w", p.OSM, err)
				}
				rep.Services++
			default:
				continue
			}
			rep.PerCity[city]++
		}
		// Bu safar kelmagan OSM joylari (OSM da yopilgan/o'chirilgan, endi turistik deb hisoblanmaydi yoki qo'lda
		// kiritilgan joyning takrori bo'lib qolgan) yashiriladi. Kichik/test fayl bilan hammasini o'chirib yubormaslik uchun
		// faqat to'liq import (≥ 1000 joy) da.
		if len(seen) >= 1000 {
			tag, err := tx.Exec(ctx, `UPDATE places SET status = 'draft', updated_at = now()
				WHERE source = 'osm' AND status = 'published' AND id <> ALL($1)`, seen)
			if err != nil {
				return err
			}
			rep.PlacesRetired = int(tag.RowsAffected())
		}
		return nil
	})
	return rep, err
}

func ensureCities(ctx context.Context, tx pgx.Tx) ([]City, error) {
	em, _ := json.Marshal(map[string]string{"unified": "112", "fire": "101", "police": "102", "ambulance": "103", "gas": "104",
		"emergency_ministry": "1050", "tourist_hotline": "1173"})
	for _, c := range Cities {
		if _, err := tx.Exec(ctx, `INSERT INTO cities (id, lat, lng, taxi_base_fare, taxi_per_km, emergency, radius_km)
			VALUES ($1,$2,$3,4600,2400,$4,$5) ON CONFLICT (id) DO UPDATE SET radius_km = EXCLUDED.radius_km`,
			c.ID, c.Center.Lat, c.Center.Lng, em, c.RadiusKm); err != nil {
			return nil, err
		}
		for lang, n := range c.Name {
			if _, err := tx.Exec(ctx, `INSERT INTO city_i18n (city_id, lang, name) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, c.ID, lang, n); err != nil {
				return nil, err
			}
		}
	}
	rows, err := tx.Query(ctx, `SELECT id, lat, lng, radius_km FROM cities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []City
	for rows.Next() {
		var c City
		if err := rows.Scan(&c.ID, &c.Center.Lat, &c.Center.Lng, &c.RadiusKm); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func nearestCity(cities []City, lat, lng float64) string {
	best, bestD := "", math.MaxFloat64
	for _, c := range cities {
		d := domain.DistanceM(c.Center, domain.LatLng{Lat: lat, Lng: lng}) / 1000
		if d <= c.RadiusKm && d < bestD {
			best, bestD = c.ID, d
		}
	}
	return best
}

type existingPlace struct {
	loc  domain.LatLng
	name string
}

func existingPlaces(ctx context.Context, tx pgx.Tx) ([]existingPlace, error) {
	rows, err := tx.Query(ctx, `SELECT p.lat, p.lng, i.name FROM places p JOIN place_i18n i ON i.place_id = p.id
		WHERE p.osm_id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []existingPlace
	for rows.Next() {
		var e existingPlace
		if err := rows.Scan(&e.loc.Lat, &e.loc.Lng, &e.name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// dupOf — bazada (qo'lda yoki tadqiqot orqali kiritilgan) xuddi shu joy bormi: 150 m ichida va nomi o'xshash
// (umumiy so'zlar — "madrasasi", "masjid" — hisobga olinmaydi, discover.SameName).
func dupOf(ex []existingPlace, p POI) bool {
	loc := domain.LatLng{Lat: p.Lat, Lng: p.Lng}
	names := []string{p.Tags["name"], p.Tags["name:uz"], p.Tags["name:ru"], p.Tags["name:en"]}
	for _, e := range ex {
		if domain.DistanceM(e.loc, loc) > 150 {
			continue
		}
		for _, n := range names {
			if n != "" && discover.SameName(n, e.name) {
				return true
			}
		}
	}
	return false
}

func names(t map[string]string) map[string]string {
	base := t["name"]
	pick := func(k string) string {
		if v := strings.TrimSpace(t[k]); v != "" {
			return v
		}
		return base
	}
	return map[string]string{"uz": pick("name:uz"), "ru": pick("name:ru"), "en": pick("name:en")}
}

func idFor(osm string) string { return "osm-" + strings.ReplaceAll(osm, "/", "-") }

func contacts(t map[string]string) map[string]string {
	out := map[string]string{}
	for k, keys := range map[string][]string{"phone": {"phone", "contact:phone"}, "website": {"website", "contact:website"},
		"telegram": {"contact:telegram"}, "instagram": {"contact:instagram"}, "email": {"email", "contact:email"}} {
		for _, key := range keys {
			if v := strings.TrimSpace(t[key]); v != "" {
				out[k] = strings.Split(v, ";")[0]
				break
			}
		}
	}
	return out
}

func address(t map[string]string) string {
	parts := []string{}
	if s := t["addr:street"]; s != "" {
		if h := t["addr:housenumber"]; h != "" {
			s += ", " + h
		}
		parts = append(parts, s)
	}
	if c := t["addr:city"]; c != "" {
		parts = append(parts, c)
	}
	return strings.Join(parts, ", ")
}

// ---------- Joylar ----------

type catInfo struct {
	category string
	tag      [3]string // uz, ru, en
	visit    int
}

// placeCategory — umumiy klassifikator (discover.Classify): OSM teglari + nom (uz/ru/en kalit so'zlar).
func placeCategory(p POI) (catInfo, bool) {
	tags := map[string]string{}
	for k, v := range p.Tags {
		tags[k] = v
	}
	// eski extract fayllarida teg o'rniga faqat Kind bo'lishi mumkin
	switch k := p.Kind; {
	case strings.HasPrefix(k, "historic:"):
		if tags["historic"] == "" {
			tags["historic"] = strings.TrimPrefix(k, "historic:")
		}
	case k == "restaurant" || k == "cafe" || k == "theatre" || k == "marketplace" || k == "concert_hall" || k == "arts_centre" ||
		k == "place_of_worship":
		if tags["amenity"] == "" {
			tags["amenity"] = k
		}
	case k == "museum" || k == "gallery" || k == "viewpoint" || k == "theme_park" || k == "zoo" || k == "attraction":
		if tags["tourism"] == "" {
			tags["tourism"] = k
		}
	}
	res := discover.Classify(discover.Signals{Names: names(p.Tags), OSM: tags})
	if res.Category == "" || res.Confidence < 0.5 {
		return catInfo{}, false
	}
	l := discover.CategoryLabel[res.Category]
	return catInfo{res.Category, [3]string{l["uz"], l["ru"], l["en"]}, discover.DefaultVisitMin(res.Category)}, true
}

func upsertPlace(ctx context.Context, tx pgx.Tx, p POI, city string) (bool, error) {
	ci, ok := placeCategory(p)
	if !ok {
		return false, nil
	}
	nm := names(p.Tags)
	if nm["uz"] == "" {
		return false, nil
	}
	// Yangilik darajasi: Vikipediya/Wikidata/meros belgisi bor obidalar — mashhur; boshqa obidalar — kam tanilgan;
	// ovqat joylari — oddiy (yangilik kvotasiga ta'sir qilmaydi, chunki ular `meal` to'xtashi).
	novelty := "iconic"
	if ci.category != "restaurant" && ci.category != "cafe" && p.Tags["wikidata"] == "" && p.Tags["wikipedia"] == "" && p.Tags["heritage"] == "" {
		novelty = "hidden"
	}
	hours, _, _ := ParseHours(p.Tags["opening_hours"])
	if hours == nil {
		hours = []domain.OpeningHours{}
	}
	hj, _ := json.Marshal(hours)
	interests := domain.CategoryInterests[ci.category]
	cj, _ := json.Marshal(contacts(p.Tags))
	priceLevel := 0
	if ci.category == "restaurant" || ci.category == "cafe" {
		priceLevel = 2
	} else if ci.category == "museum" || ci.category == "theatre" {
		priceLevel = 1
	}
	id := idFor(p.OSM)
	if _, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, status, lat, lng, interests, price_level,
		visit_duration_min, opening_hours, wheelchair, source, trust, osm_id, contacts, published_at, wikidata)
		VALUES ($1,$2,$3,$4,'published',$5,$6,$7,$8,$9,$10,$11,'osm','public',$12,$13,now(),
		  (SELECT $14::text WHERE $14 <> '' AND NOT EXISTS (SELECT 1 FROM places WHERE wikidata = $14)))
		ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id,
		  status = CASE WHEN places.source = 'osm' AND places.status = 'draft' THEN 'published' ELSE places.status END, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		  category = CASE WHEN places.source = 'osm' THEN EXCLUDED.category ELSE places.category END,
		  visit_duration_min = CASE WHEN places.source = 'osm' THEN EXCLUDED.visit_duration_min ELSE places.visit_duration_min END,
		  interests = CASE WHEN places.source = 'osm' THEN EXCLUDED.interests ELSE places.interests END,
		  opening_hours = CASE WHEN places.source = 'osm' THEN EXCLUDED.opening_hours ELSE places.opening_hours END,
		  contacts = CASE WHEN places.source = 'osm' THEN EXCLUDED.contacts ELSE places.contacts END, updated_at = now()`,
		id, city, ci.category, novelty, p.Lat, p.Lng, interests, priceLevel, ci.visit, hj, p.Tags["wheelchair"] == "yes",
		p.OSM, cj, p.Tags["wikidata"]); err != nil {
		return false, err
	}
	tagline := ci.tag
	if c := p.Tags["cuisine"]; c != "" && (ci.category == "restaurant" || ci.category == "cafe") {
		c = strings.ReplaceAll(strings.Split(c, ";")[0], "_", " ")
		tagline = [3]string{tagline[0] + " · " + c, tagline[1] + " · " + c, tagline[2] + " · " + c}
	}
	addr := address(p.Tags)
	for i, lang := range []string{"uz", "ru", "en"} {
		desc := strings.TrimSpace(p.Tags["description:"+lang])
		if desc == "" && lang == "uz" {
			desc = strings.TrimSpace(p.Tags["description"])
		}
		if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address, sources)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (place_id, lang) DO UPDATE SET name = EXCLUDED.name, tagline = EXCLUDED.tagline, address = EXCLUDED.address`,
			id, lang, nm[lang], tagline[i], desc, addr,
			`[{"title":"OpenStreetMap","url":"https://www.openstreetmap.org/`+p.OSM+`"}]`); err != nil {
			return false, err
		}
	}
	return true, nil
}

// ---------- Turar joy ----------

// priceEstimateUSD — turi va yulduzi bo'yicha taxminiy narx (2 kishi, oddiy xona, bir kecha), 2025–2026 manbalar.
func priceEstimateUSD(kind string, stars int) (float64, float64) {
	switch kind {
	case "hostel":
		return 15, 30
	case "guesthouse":
		return 20, 45
	case "motel":
		return 15, 30
	case "apartment":
		return 25, 50
	}
	switch stars {
	case 5:
		return 120, 250
	case 4:
		return 70, 130
	case 3:
		return 40, 70
	}
	return 30, 60
}

func roundSum(v float64) int64 { return int64(math.Round(v/10000)) * 10000 }

func upsertStay(ctx context.Context, tx pgx.Tx, p POI, city string, usd float64) error {
	kind := map[string]string{"hotel": "hotel", "guest_house": "guesthouse", "hostel": "hostel", "motel": "motel", "apartment": "apartment"}[p.Kind]
	if kind == "" {
		return nil
	}
	nm := names(p.Tags)
	status := "published"
	if nm["uz"] == "" {
		status = "pending" // nomsiz — tekshiruvga
		nm = map[string]string{"uz": "Nomsiz " + kind, "ru": "Без названия", "en": "Unnamed " + kind}
	}
	var stars *int
	if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(p.Tags["stars"], "S"))); err == nil && n >= 1 && n <= 5 {
		stars = &n
	}
	s := 0
	if stars != nil {
		s = *stars
	}
	lo, hi := priceEstimateUSD(kind, s)
	var amen []string
	if v := p.Tags["internet_access"]; v == "wlan" || v == "yes" || v == "wifi" {
		amen = append(amen, "wifi")
	}
	if amen == nil {
		amen = []string{}
	}
	var wheel *bool
	if v, ok := p.Tags["wheelchair"]; ok {
		b := v == "yes"
		wheel = &b
	}
	nj, _ := json.Marshal(nm)
	cj, _ := json.Marshal(contacts(p.Tags))
	_, err := tx.Exec(ctx, `INSERT INTO stays (id, city_id, kind, status, name, lat, lng, address, stars, price_min, price_max,
		price_trust, price_note, price_updated_at, amenities, contacts, check_in, check_out, wheelchair, trust, source, osm_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'estimate',$12,now(),$13,$14,$15,$16,$17,'public','open_data',$18)
		ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		  name = CASE WHEN stays.source = 'open_data' THEN EXCLUDED.name ELSE stays.name END,
		  contacts = CASE WHEN stays.source = 'open_data' THEN EXCLUDED.contacts ELSE stays.contacts END,
		  stars = COALESCE(stays.stars, EXCLUDED.stars),
		  price_min = CASE WHEN stays.price_trust = 'estimate' THEN EXCLUDED.price_min ELSE stays.price_min END,
		  price_max = CASE WHEN stays.price_trust = 'estimate' THEN EXCLUDED.price_max ELSE stays.price_max END,
		  updated_at = now()`,
		idFor(p.OSM), city, kind, status, nj, p.Lat, p.Lng, address(p.Tags), stars, roundSum(lo*usd), roundSum(hi*usd),
		"Taxminiy narx: turi va yulduzi bo'yicha. Aniq narxni joyning o'zidan so'rang", amen, cj,
		nullable(p.Tags["check_in"]), nullable(p.Tags["check_out"]), wheel, p.OSM)
	return err
}

func nullable(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// ---------- Xizmatlar ----------

var serviceKind = map[string]string{"hospital": "hospital", "clinic": "clinic", "pharmacy": "pharmacy", "police": "police",
	"atm": "atm", "bureau_de_change": "exchange", "embassy": "embassy", "fuel": "fuel", "charging_station": "ev_charging"}

var serviceName = map[string][3]string{
	"hospital": {"Shifoxona", "Больница", "Hospital"}, "clinic": {"Klinika", "Клиника", "Clinic"},
	"pharmacy": {"Dorixona", "Аптека", "Pharmacy"}, "police": {"Politsiya", "Полиция", "Police"},
	"atm": {"Bankomat", "Банкомат", "ATM"}, "exchange": {"Valyuta ayirboshlash", "Обмен валют", "Currency exchange"},
	"embassy": {"Elchixona", "Посольство", "Embassy"}, "fuel": {"Yoqilg'i quyish shoxobchasi", "АЗС", "Fuel station"},
	"ev_charging": {"Elektromobil quvvatlash", "Зарядная станция", "EV charging"},
}

func upsertService(ctx context.Context, tx pgx.Tx, p POI, city string) error {
	kind := serviceKind[p.Kind]
	if kind == "" {
		return nil
	}
	def := serviceName[kind]
	nm := names(p.Tags)
	for i, lang := range []string{"uz", "ru", "en"} {
		if nm[lang] == "" {
			if b := p.Tags["brand"]; b != "" {
				nm[lang] = def[i] + " · " + b
			} else if o := p.Tags["operator"]; o != "" {
				nm[lang] = def[i] + " · " + o
			} else {
				nm[lang] = def[i]
			}
		}
	}
	hours, open24, _ := ParseHours(p.Tags["opening_hours"])
	if hours == nil {
		hours = []domain.OpeningHours{}
	}
	if kind == "atm" && p.Tags["opening_hours"] == "" {
		open24 = false
	}
	hj, _ := json.Marshal(hours)
	nj, _ := json.Marshal(nm)
	aj, _ := json.Marshal(map[string]string{"uz": address(p.Tags)})
	var accepts []string
	for _, c := range []string{"uzcard", "humo", "visa", "mastercard", "unionpay", "mir"} {
		if p.Tags["payment:"+c] == "yes" {
			accepts = append(accepts, c)
		}
	}
	if accepts == nil {
		accepts = []string{}
	}
	var phone *string
	if ph := contacts(p.Tags)["phone"]; ph != "" {
		phone = &ph
	}
	_, err := tx.Exec(ctx, `INSERT INTO city_services (id, city_id, kind, name, address, lat, lng, phone, hours, open_24h, accepts, source, osm_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'open_data',$12)
		ON CONFLICT (id) DO UPDATE SET city_id = EXCLUDED.city_id, kind = EXCLUDED.kind, name = EXCLUDED.name, lat = EXCLUDED.lat,
		  lng = EXCLUDED.lng, phone = EXCLUDED.phone, hours = EXCLUDED.hours, open_24h = EXCLUDED.open_24h, updated_at = now()`,
		idFor(p.OSM), city, kind, nj, aj, p.Lat, p.Lng, phone, hj, open24, accepts, p.OSM)
	return err
}
