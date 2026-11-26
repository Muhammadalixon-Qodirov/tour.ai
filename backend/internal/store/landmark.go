package store

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"yoldosh/internal/domain"
)

// LandmarkPlace — Lens joy moslash uchun nomzod: barcha tillardagi nomlar, tavsif/tarix matni va masofa.
type LandmarkPlace struct {
	ID        string
	CityID    string
	Category  string
	Source    string
	Names     []string // barcha tillardagi nomlar + taxalluslar
	Text      string   // barcha tillardagi tavsif va tarix
	DistanceM *float64
}

// LandmarkPlaces — chop etilgan joylar: near berilsa radius ichidagilar (yaqini birinchi), aks holda nomi yoki
// taxallusi words dagi so'z boshlaridan biri bilan boshlanadiganlar. Apostrof turlari nomda e'tiborga olinmaydi.
func (s *Store) LandmarkPlaces(ctx context.Context, near *domain.LatLng, radiusM float64, words []string, limit int) ([]LandmarkPlace, error) {
	args := []any{}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	where := []string{"p.status = 'published'"}
	dist, order := "NULL::float8", "p.id"
	if near != nil {
		pt := "ll_to_earth(" + arg(near.Lat) + ", " + arg(near.Lng) + ")"
		dist = "earth_distance(" + pt + ", ll_to_earth(p.lat, p.lng))"
		r := arg(radiusM)
		where = append(where, "earth_box("+pt+", "+r+") @> ll_to_earth(p.lat, p.lng)", dist+" <= "+r)
		order = dist + ", p.id"
	} else {
		var pats []string
		for _, w := range words {
			if w = strings.TrimSpace(strings.ToLower(w)); w != "" {
				pats = append(pats, `\m`+regexp.QuoteMeta(w))
			}
		}
		if len(pats) == 0 {
			return nil, nil
		}
		re := arg(strings.Join(pats, "|"))
		norm := func(col string) string {
			return `regexp_replace(lower(` + col + `), '[''ʻ’‘` + "`" + `ʼ]', '', 'g')`
		}
		where = append(where, "(EXISTS (SELECT 1 FROM place_i18n w WHERE w.place_id = p.id AND "+norm("w.name")+" ~ "+re+
			") OR EXISTS (SELECT 1 FROM unnest(p.aliases) al WHERE "+norm("al")+" ~ "+re+"))")
		order = "CASE WHEN p.source IN " + BulkSources + " THEN 1 ELSE 0 END, p.id"
	}
	sql := `SELECT p.id, p.city_id, p.category, p.source,
		  array(SELECT i.name FROM place_i18n i WHERE i.place_id = p.id) || p.aliases,
		  coalesce((SELECT string_agg(coalesce(i.description, '') || ' ' || coalesce(i.history_summary, ''), ' ')
		    FROM place_i18n i WHERE i.place_id = p.id), ''),
		  ` + dist + `
		FROM places p WHERE ` + strings.Join(where, " AND ") + ` ORDER BY ` + order + ` LIMIT ` + arg(limit)
	rows, err := s.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LandmarkPlace
	for rows.Next() {
		var p LandmarkPlace
		if err := rows.Scan(&p.ID, &p.CityID, &p.Category, &p.Source, &p.Names, &p.Text, &p.DistanceM); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlaceDataSource — joyning o'z ma'lumot manbasi (tarix manbasi bo'lmaganda): tadqiqotdagi joylashuv manbasi,
// OSM obyekti yoki Wikidata. Hech biri bo'lmasa "".
func (s *Store) PlaceDataSource(ctx context.Context, id string) (string, error) {
	var loc, osm, wd *string
	err := s.DB.QueryRow(ctx, `SELECT research->>'locationSource', osm_id, wikidata FROM places WHERE id = $1`, id).Scan(&loc, &osm, &wd)
	if err != nil {
		return "", err
	}
	switch {
	case loc != nil && strings.HasPrefix(*loc, "http"):
		return *loc, nil
	case osm != nil && *osm != "":
		return "https://www.openstreetmap.org/" + *osm, nil
	case wd != nil && *wd != "":
		return "https://www.wikidata.org/wiki/" + *wd, nil
	}
	return "", nil
}
