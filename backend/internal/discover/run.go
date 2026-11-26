package discover

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// lockKey — bir vaqtda faqat bitta yig'ish ishlaydi (bir nechta API nusxasi bo'lsa ham).
const lockKey = 7_342_011

// RunWikidata — Wikidata dan yig'adi va natijani discovery_runs ga yozadi.
func RunWikidata(ctx context.Context, db *pgxpool.Pool, dryRun bool) (Report, error) {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return Report{}, err
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&got); err != nil || !got {
		return Report{Source: "wikidata"}, err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)

	hc := &http.Client{Timeout: 3 * time.Minute}
	items, err := FetchWikidata(ctx, hc)
	var rep Report
	if err == nil {
		rep, err = Ingest(ctx, db, "wikidata", items, dryRun)
	}
	if dryRun { // sinov — jurnalga yozilmaydi (aks holda jadval "yaqinda ishlagan" deb o'tkazib yuboradi)
		return rep, err
	}
	var runID int64
	if err := db.QueryRow(ctx, `INSERT INTO discovery_runs (source) VALUES ('wikidata') RETURNING id`).Scan(&runID); err != nil {
		return rep, err
	}
	rj, _ := json.Marshal(rep)
	var msg *string
	if err != nil {
		m := err.Error()
		msg = &m
	}
	_, _ = db.Exec(context.Background(), `UPDATE discovery_runs SET finished_at = now(), report = $2, error = $3 WHERE id = $1`, runID, rj, msg)
	return rep, err
}

// Schedule — fon ishi: har `every` da Wikidata dan yangi joylarni yig'adi. Birinchi ishga tushish `first` dan keyin.
func Schedule(ctx context.Context, db *pgxpool.Pool, log *slog.Logger, first, every time.Duration) {
	t := time.NewTimer(first)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// oxirgi muvaffaqiyatli yig'ishdan beri `every` o'tmagan bo'lsa (masalan qayta ishga tushirilgan) — kutamiz
		var last *time.Time
		_ = db.QueryRow(ctx, `SELECT max(finished_at) FROM discovery_runs WHERE source = 'wikidata' AND error IS NULL`).Scan(&last)
		if last != nil && time.Since(*last) < every-time.Minute {
			t.Reset(every - time.Since(*last))
			continue
		}
		rep, err := RunWikidata(ctx, db, false)
		if err != nil {
			log.Error("discovery wikidata", "err", err)
			t.Reset(time.Hour) // vaqtinchalik xato — bir soatdan keyin qayta
			continue
		} else {
			log.Info("discovery wikidata", "seen", rep.Seen, "new", rep.New, "published", rep.Published, "queued", rep.Queued,
				"duplicate", rep.Duplicate, "enriched", rep.Enriched)
		}
		t.Reset(every)
	}
}
