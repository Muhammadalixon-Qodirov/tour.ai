package osmimport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GeofabrikURL — O'zbekiston OSM ma'lumoti (har kuni yangilanadi, ~130 MB).
const GeofabrikURL = "https://download.geofabrik.de/asia/uzbekistan-latest.osm.pbf"

// USDRate — Markaziy bank kursi (turar joy narx bahosi uchun); olinmasa 0.
func USDRate(ctx context.Context) float64 {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://cbu.uz/uz/arkhiv-kursov-valyut/json/USD/", nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var body []struct {
		Rate string `json:"Rate"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil || len(body) == 0 {
		return 0
	}
	r, _ := strconv.ParseFloat(body[0].Rate, 64)
	return r
}

// Refresh — PBF ni yuklab, ajratib, bazaga yozadi. Yangi ochilgan joylar qo'shiladi, OSM dan o'chganlari yashiriladi.
func Refresh(ctx context.Context, db *pgxpool.Pool, url string) (Report, error) {
	f, err := os.CreateTemp("", "osm-*.pbf")
	if err != nil {
		return Report{}, err
	}
	path := f.Name()
	defer os.Remove(path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := (&http.Client{Timeout: 20 * time.Minute}).Do(req)
	if err != nil {
		f.Close()
		return Report{}, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		f.Close()
		return Report{}, fmt.Errorf("geofabrik: %d", resp.StatusCode)
	}
	n, err := io.Copy(f, resp.Body)
	resp.Body.Close()
	f.Close()
	if err != nil {
		return Report{}, err
	}
	if n < 10<<20 { // to'liq bo'lmagan fayl — eski ma'lumotni buzmaslik uchun
		return Report{}, fmt.Errorf("PBF juda kichik (%d bayt)", n)
	}
	pois, err := ExtractFile(ctx, path)
	if err != nil {
		return Report{}, err
	}
	return LoadPOIs(ctx, db, pois, USDRate(ctx))
}

// Schedule — fon ishi: har `every` da OSM yangilanadi. Oxirgi muvaffaqiyatli yangilash discovery_runs da (source='osm'),
// shuning uchun qayta ishga tushirishda keraksiz takrorlanmaydi. Bir nechta API nusxasi bo'lsa ham bittasi ishlaydi.
func Schedule(ctx context.Context, db *pgxpool.Pool, log *slog.Logger, first, every time.Duration) {
	const lockKey = 7_342_012
	t := time.NewTimer(first)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		var last *time.Time
		_ = db.QueryRow(ctx, `SELECT max(finished_at) FROM discovery_runs WHERE source = 'osm' AND error IS NULL`).Scan(&last)
		if last != nil && time.Since(*last) < every-time.Hour {
			t.Reset(every - time.Since(*last))
			continue
		}
		conn, err := db.Acquire(ctx)
		if err != nil {
			t.Reset(time.Hour)
			continue
		}
		var got bool
		_ = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&got)
		if !got {
			conn.Release()
			t.Reset(time.Hour)
			continue
		}
		var runID int64
		_ = db.QueryRow(ctx, `INSERT INTO discovery_runs (source) VALUES ('osm') RETURNING id`).Scan(&runID)
		log.Info("osm: yangilash boshlandi")
		rep, err := Refresh(ctx, db, GeofabrikURL)
		rj, _ := json.Marshal(rep)
		var msg *string
		if err != nil {
			m := err.Error()
			msg = &m
			log.Error("osm: yangilash xatosi", "err", err)
		} else {
			log.Info("osm: yangilandi", "joylar", rep.Places, "yashirildi", rep.PlacesRetired, "turar_joy", rep.Stays, "xizmatlar", rep.Services)
		}
		_, _ = db.Exec(context.Background(), `UPDATE discovery_runs SET finished_at = now(), report = $2, error = $3 WHERE id = $1`, runID, rj, msg)
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)
		conn.Release()
		if err != nil {
			t.Reset(6 * time.Hour)
		} else {
			t.Reset(every)
		}
	}
}
