package discover

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/domain"
)

// PublishResearch — tadqiqot nomzodlarini (source='research', status='pending') ko'rib chiqadi:
//   - bazada shu joy bor bo'lsa (boshqa katalog yoki OSM) — takror, chop etilmaydi (status 'draft');
//   - tadqiqotchi qo'ygan kategoriya bilan klassifikator keskin rozi bo'lmasa — admin navbatida qoladi;
//   - qolganlari chop etiladi (ishonch: high/medium; low allaqachon import qilinmagan).
func PublishResearch(ctx context.Context, db *pgxpool.Pool, dryRun bool) (Report, error) {
	rep := Report{Source: "research"}
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		ix, err := loadIndex(ctx, tx)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT p.id, p.category, p.lat, p.lng, p.city_id, COALESCE(p.research->>'confidence', ''),
			jsonb_object_agg(i.lang, i.name), jsonb_object_agg(i.lang, COALESCE(i.tagline, '') || ' ' || COALESCE(i.description, ''))
			FROM places p JOIN place_i18n i ON i.place_id = p.id
			WHERE p.source = 'research' AND p.status = 'pending' GROUP BY p.id ORDER BY p.id`)
		if err != nil {
			return err
		}
		type cand struct {
			id, cat, city, conf string
			loc                 domain.LatLng
			names, text         map[string]string
		}
		var cands []cand
		for rows.Next() {
			var c cand
			if err := rows.Scan(&c.id, &c.cat, &c.loc.Lat, &c.loc.Lng, &c.city, &c.conf, &c.names, &c.text); err != nil {
				return err
			}
			cands = append(cands, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, c := range cands {
			rep.Seen++
			f := Found{Source: "research", Ref: c.id, Names: c.names, Loc: c.loc,
				Signals: Signals{Names: c.names, Hint: c.cat, Text: c.text["uz"] + " " + c.text["en"]}}
			res := Classify(f.Signals)
			status, placeID := "published", &c.id
			switch dup := ix.duplicate(f, c.id); {
			case dup != "":
				status, placeID = "duplicate", &dup
				rep.Duplicate++
			case res.Category != c.cat && res.Confidence >= 0.75 && !compatible(res.Category, c.cat):
				status, placeID = "queued", nil // tadqiqotchi va klassifikator rozi emas — admin hal qiladi
				rep.Queued++
			default:
				rep.Published++
				rep.PublishedIDs = append(rep.PublishedIDs, c.id)
			}
			data, _ := json.Marshal(map[string]any{"researchCategory": c.cat, "confidence": c.conf})
			name, _ := json.Marshal(c.names)
			if _, err := tx.Exec(ctx, `INSERT INTO discoveries (source, ref, name, lat, lng, city_id, category, confidence, novelty,
				interests, reasons, data, status, place_id)
				SELECT 'research', $1, $2, $3, $4, $5, $6, $7, novelty, interests, $8, $9, $10, $11 FROM places WHERE id = $1
				ON CONFLICT (source, ref) DO UPDATE SET status = EXCLUDED.status, place_id = EXCLUDED.place_id, last_seen = now()`,
				c.id, name, c.loc.Lat, c.loc.Lng, c.city, firstNonEmpty(res.Category, c.cat), res.Confidence, res.Reasons, data,
				status, placeID); err != nil {
				return fmt.Errorf("%s: %w", c.id, err)
			}
			switch status {
			case "published":
				if _, err := tx.Exec(ctx, `UPDATE places SET status = 'published', published_at = now(), updated_at = now() WHERE id = $1`, c.id); err != nil {
					return err
				}
			case "duplicate":
				if _, err := tx.Exec(ctx, `UPDATE places SET status = 'draft', updated_at = now() WHERE id = $1`, c.id); err != nil {
					return err
				}
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

var errDryRun = fmt.Errorf("dry run")
