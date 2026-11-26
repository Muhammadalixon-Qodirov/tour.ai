package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
)

type Report struct {
	Source       string   `json:"source"`
	Seen         int      `json:"seen"`
	New          int      `json:"new"`
	Published    int      `json:"published"`
	Queued       int      `json:"queued"`
	Duplicate    int      `json:"duplicate"`
	OutOfArea    int      `json:"outOfArea"`
	Enriched     int      `json:"enriched"` // mavjud joyga Wikidata bog'landi
	PublishedIDs []string `json:"publishedIds,omitempty"`
}

type city struct {
	id       string
	loc      domain.LatLng
	radiusKm float64
}

type known struct {
	id, wikidata, osm string
	loc               domain.LatLng
	names             []string
}

type index struct {
	cities []city
	places []known
	byWD   map[string]string
	byOSM  map[string]string
}

func loadIndex(ctx context.Context, tx pgx.Tx) (*index, error) {
	ix := &index{byWD: map[string]string{}, byOSM: map[string]string{}}
	rows, err := tx.Query(ctx, `SELECT id, lat, lng, radius_km FROM cities`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c city
		if err := rows.Scan(&c.id, &c.loc.Lat, &c.loc.Lng, &c.radiusKm); err != nil {
			return nil, err
		}
		ix.cities = append(ix.cities, c)
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT p.id, p.lat, p.lng, COALESCE(p.wikidata, ''), COALESCE(p.osm_id, ''),
		COALESCE(array_agg(i.name) FILTER (WHERE i.name IS NOT NULL), '{}') || p.aliases
		FROM places p LEFT JOIN place_i18n i ON i.place_id = p.id
		WHERE p.status <> 'rejected' GROUP BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k known
		var names []string
		if err := rows.Scan(&k.id, &k.loc.Lat, &k.loc.Lng, &k.wikidata, &k.osm, &names); err != nil {
			return nil, err
		}
		for _, n := range names {
			if n = NormName(n); len([]rune(n)) >= 4 {
				k.names = append(k.names, n)
			}
		}
		if k.wikidata != "" {
			ix.byWD[k.wikidata] = k.id
		}
		if k.osm != "" {
			ix.byOSM[k.osm] = k.id
		}
		ix.places = append(ix.places, k)
	}
	return ix, rows.Err()
}

func (ix *index) nearestCity(p domain.LatLng) string {
	best, bestD := "", math.MaxFloat64
	for _, c := range ix.cities {
		d := domain.DistanceM(c.loc, p) / 1000
		if d <= c.radiusKm && d < bestD {
			best, bestD = c.id, d
		}
	}
	return best
}

// generic — joy turini bildiruvchi umumiy so'z boshlari: "Ulug'bek madrasasi" va "Медресе Улугбека" dan faqat
// "ulugbek"/"улугбек" qoladi. Aks holda "Медресе Абдулазизхана" va "Медресе Амир Олимхана" bir xil ko'rinadi.
var generic = []string{"madras", "медрес", "медрас", "masjid", "мечет", "mosque", "maqbar", "мавзоле", "mausole", "muzey", "музе",
	"museum", "majmua", "комплекс", "complex", "ensemble", "ansambl", "ансамбл", "memorial", "мемориал", "haykal", "памятник",
	"monument", "ziyoratgoh", "mazor", "мазар", "xonaqoh", "ханак", "khanaka", "jome", "джума", "жоме", "juma", "tomb", "sagana",
	"гробниц", "nomidagi", "имени", "named", "of", "the", "va", "и", "and", "shrine", "church", "cherkov", "церк", "собор",
	"park", "парк", "bogi", "teatr", "театр", "theatre", "theater", "bozor", "базар", "bazaar", "market", "рынок", "kafe", "кафе",
	"cafe", "restoran", "ресторан", "restaurant", "choyxona", "чайхан", "hotel", "mehmonxona", "гостиниц", "city", "shahar", "город",
	"samarqand", "самарканд", "samarkand", "buxoro", "бухар", "bukhara", "toshkent", "ташкент", "tashkent", "xiva", "хив", "khiva"}

// NormName — taqqoslash uchun: kichik harf, umumiy so'zlarsiz, faqat harf va raqam.
func NormName(s string) string {
	var b strings.Builder
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\'' && r != 'ʻ' && r != '’'
	}) {
		w = strings.NewReplacer("'", "", "ʻ", "", "’", "").Replace(w)
		skip := false
		for _, g := range generic {
			lg, lw := len([]rune(g)), len([]rune(w))
			if lg <= 3 && w == g || lg > 3 && strings.HasPrefix(w, g) && (lw <= lg+4 || lg >= 5) {
				skip = true
				break
			}
		}
		if !skip {
			b.WriteString(w)
		}
	}
	return b.String()
}

// SameName — ikki nom bitta joynikimi (umumiy so'zlarsiz): teng, biri ikkinchisini o'z ichiga oladi (≥5 harf)
// yoki tahrir masofasi kichik (≥ 85% o'xshash).
func SameName(rawA, rawB string) bool { return sameName(NormName(rawA), NormName(rawB)) }

func sameName(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) < 4 || len(rb) < 4 {
		return false
	}
	if a == b {
		return true
	}
	// biri ikkinchisining ichida — faqat qisqasi uzunining ≥60% i bo'lsa ("Kalta Minor" ≠ "Minor Cafe")
	short, long := len(ra), len(rb)
	if short > long {
		short, long = long, short
	}
	if short >= 5 && float64(short) >= 0.6*float64(long) && (strings.Contains(a, b) || strings.Contains(b, a)) {
		return true
	}
	if short >= 7 && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a)) { // "Dorut Tilovat" ~ "Dorut Tilovat va Ko'k Gumbaz"
		return true
	}
	d := levenshtein(ra, rb)
	return float64(d) <= 0.15*float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// duplicate — bazada shu joy bormi: bir xil Wikidata/OSM id, yoki radius ichida o'xshash nom.
func (ix *index) duplicate(f Found, skipID string) string {
	if f.Source == "wikidata" {
		if id := ix.byWD[f.Ref]; id != "" && id != skipID {
			return id
		}
	}
	if q := f.Signals.OSM["wikidata"]; q != "" {
		if id := ix.byWD[q]; id != "" && id != skipID {
			return id
		}
	}
	if f.Source == "osm" {
		if id := ix.byOSM[f.Ref]; id != "" && id != skipID {
			return id
		}
	}
	radius := 150.0
	if f.Source == "wikidata" || f.Source == "user" || f.Source == "research" {
		radius = 350 // Wikidata va qo'lda kiritilgan koordinatalar aniqligi pastroq
	}
	var names []string
	for _, n := range f.Names {
		if n = NormName(n); len([]rune(n)) >= 4 {
			names = append(names, n)
		}
	}
	for _, k := range ix.places {
		if k.id == skipID || domain.DistanceM(k.loc, f.Loc) > radius {
			continue
		}
		for _, a := range names {
			for _, b := range k.names {
				if sameName(a, b) {
					return k.id
				}
			}
		}
	}
	return ""
}

// Ingest — topilmalarni saqlaydi va har biri uchun qaror qiladi. Admin ko'rib chiqqan (rejected/published) yozuvlarga
// qaror qayta qo'llanmaydi — faqat last_seen yangilanadi.
func Ingest(ctx context.Context, db *pgxpool.Pool, source string, items []Found, dryRun bool) (Report, error) {
	rep := Report{Source: source}
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		ix, err := loadIndex(ctx, tx)
		if err != nil {
			return err
		}
		for _, f := range items {
			rep.Seen++
			if err := ingestOne(ctx, tx, ix, f, &rep); err != nil {
				return fmt.Errorf("%s %s: %w", f.Source, f.Ref, err)
			}
		}
		if dryRun {
			return errDryRun
		}
		return nil
	})
	if err == errDryRun {
		err = nil
	}
	return rep, err
}

func ingestOne(ctx context.Context, tx pgx.Tx, ix *index, f Found, rep *Report) error {
	if f.Signals.Names == nil {
		f.Signals.Names = f.Names
	}
	res := Classify(f.Signals)
	if f.ForceCategory != "" && domain.ValidCategory(f.ForceCategory) {
		// Rasmiy reyestr kategoriyani aniq biladi — ehtimollik ovozini almashtiramiz.
		res.Category = f.ForceCategory
		res.Confidence = max(res.Confidence, 0.9)
		res.Reasons = append([]string{"official:" + f.Source}, res.Reasons...)
		if len(res.Interests) == 0 {
			res.Interests = nonNil(domain.CategoryInterests[f.ForceCategory])
		}
	}
	cityID := ix.nearestCity(f.Loc)
	name, _ := json.Marshal(f.Names)
	data := map[string]any{}
	for k, v := range f.Data {
		data[k] = v
	}
	if len(f.Desc) > 0 {
		data["desc"] = f.Desc
	}
	if len(f.Signals.OSM) > 0 {
		data["tags"] = f.Signals.OSM
	}
	if len(f.Signals.WDClasses) > 0 {
		data["classes"] = f.Signals.WDClasses
	}
	if f.Signals.Sitelinks > 0 {
		data["sitelinks"] = f.Signals.Sitelinks
	}
	dj, _ := json.Marshal(data)
	var city *string
	if cityID != "" {
		city = &cityID
	}
	var user *string
	if f.UserID != "" {
		user = &f.UserID
	}
	var id int64
	var status string
	var inserted bool
	err := tx.QueryRow(ctx, `INSERT INTO discoveries (source, ref, name, lat, lng, city_id, category, confidence, novelty,
		interests, reasons, data, submitted_by)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13)
		ON CONFLICT (source, ref) DO UPDATE SET name = EXCLUDED.name, lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		  city_id = EXCLUDED.city_id, category = EXCLUDED.category, confidence = EXCLUDED.confidence, novelty = EXCLUDED.novelty,
		  interests = EXCLUDED.interests, reasons = EXCLUDED.reasons, data = EXCLUDED.data, last_seen = now()
		RETURNING id, status, (xmax = 0)`,
		f.Source, f.Ref, name, f.Loc.Lat, f.Loc.Lng, city, res.Category, res.Confidence, res.Novelty, nonNil(res.Interests),
		res.Reasons, dj, user).Scan(&id, &status, &inserted)
	if err != nil {
		return err
	}
	if inserted {
		rep.New++
	}
	if status != "queued" { // qaror allaqachon qilingan (chop etilgan, rad etilgan, takror)
		return nil
	}
	decide := func(st string, placeID *string) error {
		_, err := tx.Exec(ctx, `UPDATE discoveries SET status = $2, place_id = $3 WHERE id = $1`, id, st, placeID)
		return err
	}
	switch {
	case cityID == "":
		rep.OutOfArea++
		return decide("out_of_area", nil)
	}
	if dup := ix.duplicate(f, ""); dup != "" {
		rep.Duplicate++
		if f.Source == "wikidata" { // mavjud joyga Wikidata id bog'lash (keyingi safar aniq topiladi)
			tag, err := tx.Exec(ctx, `UPDATE places SET wikidata = $2 WHERE id = $1 AND wikidata IS NULL
				AND NOT EXISTS (SELECT 1 FROM places WHERE wikidata = $2)`, dup, f.Ref)
			if err != nil {
				return err
			}
			if tag.RowsAffected() > 0 {
				rep.Enriched++
				ix.byWD[f.Ref] = dup
			}
		}
		return decide("duplicate", &dup)
	}
	if f.Trusted && res.Category != "" && res.Confidence >= AutoPublish && f.Names["uz"]+f.Names["en"]+f.Names["ru"] != "" {
		pid, err := createPlace(ctx, tx, f, res, cityID)
		if err != nil {
			return err
		}
		ix.places = append(ix.places, known{id: pid, loc: f.Loc, wikidata: wdOf(f), names: normNames(f.Names)})
		if q := wdOf(f); q != "" {
			ix.byWD[q] = pid
		}
		rep.Published++
		rep.PublishedIDs = append(rep.PublishedIDs, pid)
		return decide("published", &pid)
	}
	rep.Queued++
	return nil
}

func wdOf(f Found) string {
	if f.Source == "wikidata" {
		return f.Ref
	}
	return f.Signals.OSM["wikidata"]
}

func normNames(m map[string]string) []string {
	var out []string
	for _, n := range m {
		if n = NormName(n); len([]rune(n)) >= 4 {
			out = append(out, n)
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func placeID(f Found) string {
	r := strings.ToLower(f.Ref)
	switch f.Source {
	case "wikidata":
		return "wd-" + r
	case "osm":
		return "osm-" + strings.ReplaceAll(r, "/", "-")
	case "user":
		if len(r) > 12 {
			r = r[:12]
		}
		return "u-" + r
	}
	return r
}

// createPlace — topilmadan chop etilgan joy. Matn: nom, Wikidata qisqa tavsifi yoki kategoriya nomi.
func createPlace(ctx context.Context, tx pgx.Tx, f Found, res Result, cityID string) (string, error) {
	return CreatePlace(ctx, tx, f, res, cityID, "public")
}

// CreatePlace — admin tasdig'ida ham ishlatiladi (trust: public | community).
func CreatePlace(ctx context.Context, tx pgx.Tx, f Found, res Result, cityID, trust string) (string, error) {
	if !domain.ValidCategory(res.Category) {
		return "", errors.New("kategoriya noma'lum")
	}
	id := placeID(f)
	price := 0
	switch res.Category {
	case "restaurant", "cafe":
		price = 2
	case "museum", "theatre":
		price = 1
	}
	var wd *string
	if q := wdOf(f); q != "" {
		wd = &q
	}
	contacts := map[string]string{}
	for _, k := range []string{"website", "instagram", "telegram", "phone"} {
		if v, ok := f.Data[k].(string); ok && v != "" {
			contacts[k] = v
		}
	}
	cj, _ := json.Marshal(contacts)
	research, _ := json.Marshal(map[string]any{"discovery": f.Source, "ref": f.Ref, "confidence": res.Confidence,
		"reasons": res.Reasons, "importedAt": time.Now().UTC().Format(time.RFC3339)})
	source := f.Source
	if source == "user" {
		source = "community"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO places (id, city_id, category, novelty, status, lat, lng, interests, price_level,
		visit_duration_min, opening_hours, source, trust, wikidata, contacts, research, published_at)
		VALUES ($1,$2,$3,$4,'published',$5,$6,$7,$8,$9,'[]',$10,$11,$12,$13,$14,now())`,
		id, cityID, res.Category, res.Novelty, f.Loc.Lat, f.Loc.Lng, nonNil(res.Interests), price, DefaultVisitMin(res.Category),
		source, trust, wd, cj, research); err != nil {
		return "", err
	}
	srcTitle, srcURL := map[string]string{"wikidata": "Wikidata", "osm": "OpenStreetMap", "user": "Foydalanuvchi taklifi"}[f.Source], ""
	switch f.Source {
	case "wikidata":
		srcURL = "https://www.wikidata.org/wiki/" + f.Ref
	case "osm":
		srcURL = "https://www.openstreetmap.org/" + f.Ref
	}
	sources, _ := json.Marshal([]map[string]string{{"title": srcTitle, "url": srcURL}})
	for _, lang := range []string{"uz", "ru", "en"} {
		name := firstNonEmpty(f.Names[lang], f.Names["uz"], f.Names["en"], f.Names["ru"])
		desc := strings.TrimSpace(f.Desc[lang])
		tagline := CategoryLabel[res.Category][lang]
		if desc != "" && len([]rune(desc)) <= 90 {
			tagline = upperFirst(desc)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO place_i18n (place_id, lang, name, tagline, description, address, sources)
			VALUES ($1,$2,$3,$4,$5,'',$6)`, id, lang, name, tagline, desc, sources); err != nil {
			return "", err
		}
	}
	return id, nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
