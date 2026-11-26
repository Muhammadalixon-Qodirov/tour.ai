package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CityCoversFile — shahar muqovalari (rasm + muallif, litsenziya, manba). LoadDir uni shahar katalogi sifatida emas,
// kataloglardan keyin alohida yuklaydi: katalog faylidagi `city.cover` dan ustun turadi.
// Rasmlar static/cities da (binar ichida, "/static/cities/{file}"). Bazada yo'q shahar o'tkazib yuboriladi
// (OSM yoki nomzodlar importi yaratgandan keyin seed qayta ishga tushirilsa muqova qo'yiladi).
const CityCoversFile = "city_covers.json"

type CityCover struct {
	CityID    string `json:"cityId"`
	File      string `json:"file"`    // static ga nisbatan: "cities/khiva.jpg" yoki "places/registan.jpg"
	Subject   string `json:"subject"` // suratda nima (hujjat uchun, API ga chiqmaydi)
	Credit    string `json:"credit"`
	License   string `json:"license"`
	SourceURL string `json:"sourceUrl"`
	Source    string `json:"source"`
	Note      string `json:"note,omitempty"` // masalan "tuman manzarasi: shaharning o'z surati yo'q"
}

type cityCoversFile struct {
	Covers []CityCover `json:"covers"`
}

var coverFile = regexp.MustCompile(`^(cities|places)/[a-z0-9_]+\.jpg$`)

// ReadCityCovers — faylni o'qiydi va tekshiradi.
func ReadCityCovers(path string) ([]CityCover, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f cityCoversFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, c := range f.Covers {
		switch {
		case c.CityID == "" || seen[c.CityID]:
			return nil, fmt.Errorf("%s: cityId bo'sh yoki takror: %q", path, c.CityID)
		case !coverFile.MatchString(c.File):
			return nil, fmt.Errorf("%s: %s: file noto'g'ri: %q", path, c.CityID, c.File)
		case c.Credit == "" || c.License == "" || c.SourceURL == "":
			return nil, fmt.Errorf("%s: %s: credit, license va sourceUrl majburiy", path, c.CityID)
		}
		seen[c.CityID] = true
	}
	return f.Covers, nil
}

func loadCityCovers(ctx context.Context, db *pgxpool.Pool, path string) error {
	covers, err := ReadCityCovers(path)
	if err != nil {
		return err
	}
	set, missing := 0, []string{}
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		for _, c := range covers {
			tag, err := tx.Exec(ctx, `UPDATE cities SET cover_url = $2, cover_blurhash = NULL, cover_credit = $3,
				cover_license = $4, cover_source_url = $5, cover_source = NULLIF($6, '') WHERE id = $1`,
				c.CityID, "/static/"+c.File, c.Credit, c.License, c.SourceURL, c.Source)
			if err != nil {
				return fmt.Errorf("%s: %w", c.CityID, err)
			}
			if tag.RowsAffected() == 0 {
				missing = append(missing, c.CityID)
				continue
			}
			set++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	slog.Info("shahar muqovalari", "file", path, "set", set, "missing_cities", missing)
	return nil
}
