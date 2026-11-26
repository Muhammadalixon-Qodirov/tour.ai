package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/discover"
	"yoldosh/internal/domain"
)

type Options struct {
	Limit  int  // har bosqichda ko'pi bilan nechta joy (0 — cheksiz)
	Geo    bool // Wikidata'siz obidalar uchun koordinata atrofidagi Commons rasmlari
	Desc   bool // Vikipediya tavsiflari
	DryRun bool
}

type Report struct {
	WikidataPlaces  int `json:"wikidataPlaces"`
	ImagesWikidata  int `json:"imagesFromWikidata"`
	ImagesGeo       int `json:"imagesFromNearbyCommons"`
	GeoChecked      int `json:"geoChecked"`
	LicenseRejected int `json:"licenseRejected"`
	Descriptions    int `json:"descriptions"`
}

// geoCategories — atrofdagi Commons rasmlari qidiriladigan toifalar (kafe/restoran emas: ular uchun ochiq rasm deyarli yo'q
// va qo'shni binoning rasmini olib qo'yish xavfi katta).
var geoCategories = []string{"monument", "museum", "mosque", "theatre", "park", "viewpoint", "gallery", "bazaar", "workshop", "ecovillage"}

func imageJSON(p Photo) []byte {
	b, _ := json.Marshal([]domain.Image{{URL: p.URL, Credit: p.Credit, License: p.License, SourceURL: p.PageURL, Source: "Wikimedia Commons"}})
	return b
}

// Run — Wikidata bog'langan joylar (rasm + tavsif), so'ng ixtiyoriy ravishda atrofdagi Commons rasmlari.
func Run(ctx context.Context, db *pgxpool.Pool, w *Wiki, o Options, log *slog.Logger) (Report, error) {
	var rep Report
	limit := ""
	if o.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", o.Limit)
	}
	// ---------- 1. Wikidata bog'langan joylar ----------
	rows, err := db.Query(ctx, `SELECT p.id, p.wikidata, jsonb_array_length(p.images) = 0,
		  ARRAY(SELECT i.lang FROM place_i18n i WHERE i.place_id = p.id AND coalesce(i.description, '') = '')
		FROM places p WHERE p.status = 'published' AND p.wikidata IS NOT NULL
		  AND (jsonb_array_length(p.images) = 0 OR EXISTS (SELECT 1 FROM place_i18n i WHERE i.place_id = p.id AND coalesce(i.description, '') = ''))
		ORDER BY CASE WHEN p.source IN ('osm','wikidata') THEN 1 ELSE 0 END, p.id`+limit)
	if err != nil {
		return rep, err
	}
	type wdPlace struct {
		id, q        string
		needImage    bool
		missingLangs []string
	}
	var wps []wdPlace
	var qids []string
	for rows.Next() {
		var p wdPlace
		if err := rows.Scan(&p.id, &p.q, &p.needImage, &p.missingLangs); err != nil {
			rows.Close()
			return rep, err
		}
		wps = append(wps, p)
		qids = append(qids, p.q)
	}
	rows.Close()
	rep.WikidataPlaces = len(wps)
	ents, err := w.Entities(ctx, qids)
	if err != nil {
		return rep, fmt.Errorf("wikidata: %w", err)
	}
	var files []string
	for _, p := range wps {
		if e := ents[p.q]; p.needImage && e.Image != "" {
			files = append(files, e.Image)
		}
	}
	photos, err := w.Photos(ctx, files)
	if err != nil {
		return rep, fmt.Errorf("commons: %w", err)
	}
	for _, p := range wps {
		e := ents[p.q]
		if p.needImage && e.Image != "" {
			ph, ok := photos[e.Image]
			switch {
			case !ok:
			case !ph.Open:
				rep.LicenseRejected++
			default:
				if !o.DryRun {
					if _, err := db.Exec(ctx, `UPDATE places SET images = $2::jsonb, updated_at = now()
						WHERE id = $1 AND jsonb_array_length(images) = 0`, p.id, imageJSON(ph)); err != nil {
						return rep, err
					}
				}
				rep.ImagesWikidata++
			}
		}
		if !o.Desc {
			continue
		}
		for _, lang := range p.missingLangs {
			title := e.Sitelinks[lang]
			if title == "" {
				continue
			}
			sm, err := w.Summary(ctx, lang, title)
			if err != nil || sm.Extract == "" {
				continue
			}
			src, _ := json.Marshal([]map[string]string{{"title": "Wikipedia (CC BY-SA 4.0)", "url": sm.URL}})
			if !o.DryRun {
				if _, err := db.Exec(ctx, `UPDATE place_i18n SET description = $3,
					sources = CASE WHEN jsonb_typeof(sources) = 'array' THEN sources || $4::jsonb ELSE $4::jsonb END
					WHERE place_id = $1 AND lang = $2 AND coalesce(description, '') = ''`, p.id, lang, sm.Extract, src); err != nil {
					return rep, err
				}
			}
			rep.Descriptions++
		}
	}
	if log != nil {
		log.Info("enrich: wikidata", "joylar", rep.WikidataPlaces, "rasm", rep.ImagesWikidata, "tavsif", rep.Descriptions)
	}
	if !o.Geo {
		return rep, nil
	}

	// ---------- 2. Wikidata'siz obidalar: atrofdagi Commons rasmi, nomi mos kelsa ----------
	rows, err = db.Query(ctx, `SELECT p.id, p.lat, p.lng,
		  COALESCE(array_agg(i.name) FILTER (WHERE i.name IS NOT NULL), '{}') || p.aliases
		FROM places p LEFT JOIN place_i18n i ON i.place_id = p.id
		WHERE p.status = 'published' AND p.wikidata IS NULL AND jsonb_array_length(p.images) = 0 AND p.category = ANY($1)
		GROUP BY p.id ORDER BY CASE WHEN p.source = 'osm' THEN 1 ELSE 0 END, p.id`+limit, geoCategories)
	if err != nil {
		return rep, err
	}
	type geoPlace struct {
		id       string
		lat, lng float64
		names    []string
	}
	var gps []geoPlace
	for rows.Next() {
		var g geoPlace
		if err := rows.Scan(&g.id, &g.lat, &g.lng, &g.names); err != nil {
			rows.Close()
			return rep, err
		}
		gps = append(gps, g)
	}
	rows.Close()
	for _, g := range gps {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		rep.GeoChecked++
		cands, err := w.NearbyFiles(ctx, g.lat, g.lng, 120)
		if err != nil || len(cands) == 0 {
			continue
		}
		var match []string
		for _, f := range cands {
			if fileMatches(f, g.names) {
				match = append(match, f)
			}
		}
		if len(match) == 0 {
			continue
		}
		phs, err := w.Photos(ctx, match)
		if err != nil {
			continue
		}
		for _, f := range match { // atrofdagi birinchi mos va ochiq litsenziyali rasm
			ph, ok := phs[f]
			if !ok {
				continue
			}
			if !ph.Open {
				rep.LicenseRejected++
				continue
			}
			if !o.DryRun {
				if _, err := db.Exec(ctx, `UPDATE places SET images = $2::jsonb, updated_at = now()
					WHERE id = $1 AND jsonb_array_length(images) = 0`, g.id, imageJSON(ph)); err != nil {
					return rep, err
				}
			}
			rep.ImagesGeo++
			break
		}
		if log != nil && rep.GeoChecked%100 == 0 {
			log.Info("enrich: atrof", "tekshirildi", rep.GeoChecked, "rasm", rep.ImagesGeo)
		}
	}
	return rep, nil
}

// fileMatches — Commons fayl nomi joy nomlaridan biriga mosmi ("Sher-Dor Madrasah, Samarkand (3).jpg" ~ "Sherdor madrasa").
// Umumiy so'zlar (madrasa, masjid, shahar nomlari) olib tashlanadi; qolgan o'ziga xos qism fayl nomida bo'lishi kerak.
func fileMatches(file string, names []string) bool {
	base := file
	if i := strings.LastIndexByte(base, '.'); i > 0 {
		base = base[:i]
	}
	fn := discover.NormName(base)
	if fn == "" {
		return false
	}
	for _, n := range names {
		nn := discover.NormName(n)
		if len([]rune(nn)) >= 5 && strings.Contains(fn, nn) {
			return true
		}
		// ko'p so'zli nomlar: har bir o'ziga xos (≥ 6 harf) so'z fayl nomida bo'lsa
		var toks []string
		for _, t := range strings.FieldsFunc(strings.ToLower(n), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			if tn := discover.NormName(t); len([]rune(tn)) >= 6 {
				toks = append(toks, tn)
			}
		}
		if len(toks) > 0 {
			all := true
			for _, t := range toks {
				if !strings.Contains(fn, t) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}
