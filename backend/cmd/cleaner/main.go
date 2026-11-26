// cleaner — "ma'lumot tozalovchi" ishchi. Ichki tarmoqdagi AI serverda ishlaydi: asosiy serverdan (Safar backend)
// navbatdagi topilgan joylarni tortib oladi, lokal AI model (Ollama/llama.cpp/vLLM/LM Studio — OpenAI-mos) bilan
// tozalaydi (haqiqiy turistik joymi, kategoriya, qisqa uz/ru/en tavsif) va natijani qaytaradi. Faqat chiquvchi ulanish.
//
// Sozlash (env):
//
//	BACKEND_URL     asosiy server, masalan https://safar-api.e-investment.uz
//	CLEANER_KEY     asosiy serverdagi CLEANER_KEY bilan bir xil
//	AI_BASE_URL     OpenAI-mos model manzili (default http://localhost:11434/v1 — Ollama)
//	AI_MODEL        model nomi (masalan qwen2.5:7b-instruct, llama3.1:8b)
//	AI_KEY          model kaliti (ixtiyoriy; lokal server talab qilmaydi)
//	CLEAN_BATCH     bir siklda nechta (default 20)
//	CLEAN_INTERVAL  sikllar orasidagi kutish (default 60s)
//
// Ishga tushirish:
//
//	cleaner          # tsikl: har interval da navbatni tozalaydi
//	cleaner once     # bir marta ishlaydi (cron yoki test uchun)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"yoldosh/internal/cleaner"
)

type config struct {
	backendURL string
	key        string
	batch      int
	interval   time.Duration
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config{
		backendURL: trimURL(os.Getenv("BACKEND_URL")),
		key:        os.Getenv("CLEANER_KEY"),
		batch:      envInt("CLEAN_BATCH", 20),
		interval:   envDur("CLEAN_INTERVAL", 60*time.Second),
	}
	if cfg.backendURL == "" || cfg.key == "" {
		fail("BACKEND_URL va CLEANER_KEY kerak")
	}
	model := cleaner.NewOpenAI(envOr("AI_BASE_URL", "http://localhost:11434/v1"), os.Getenv("AI_KEY"), os.Getenv("AI_MODEL"))
	if model.Model == "" {
		fail("AI_MODEL kerak (masalan qwen2.5:7b-instruct)")
	}
	hc := &http.Client{Timeout: 30 * time.Second}
	w := &worker{cfg: cfg, model: model, hc: hc}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("tozalovchi ishga tushdi", "backend", cfg.backendURL, "model", model.Name(), "aiBase", model.BaseURL)

	if len(os.Args) > 1 && os.Args[1] == "once" {
		if err := w.runOnce(ctx); err != nil {
			fail(err.Error())
		}
		return
	}
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("to'xtatildi")
			return
		case <-t.C:
		}
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("sikl xatosi", "err", err)
		}
		t.Reset(cfg.interval)
	}
}

type worker struct {
	cfg   config
	model cleaner.Model
	hc    *http.Client
}

// runOnce — navbatdan bir to'plam oladi, tozalaydi, qaytaradi. To'plam bo'sh bo'lsa to'xtaydi.
func (w *worker) runOnce(ctx context.Context) error {
	items, err := w.pending(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		slog.Info("navbat bo'sh")
		return nil
	}
	results := make([]cleaner.Result, 0, len(items))
	var modelErr int
	for _, it := range items {
		if ctx.Err() != nil {
			break
		}
		res, err := cleaner.Clean(ctx, w.model, it)
		if err != nil {
			modelErr++
			slog.Warn("tozalay olmadi (keyingi siklda qayta)", "id", it.ID, "err", err)
			continue // ai_cleaned_at o'rnatilmaydi — keyingi siklda qayta urinadi
		}
		results = append(results, res)
	}
	if len(results) == 0 {
		return fmt.Errorf("to'plamda hech biri tozalanmadi (model xatosi: %d)", modelErr)
	}
	rep, err := w.submit(ctx, results)
	if err != nil {
		return err
	}
	slog.Info("to'plam yakunlandi", "olingan", len(items), "nashr", rep.Published, "rad", rep.Rejected,
		"navbatda", rep.Review, "o'tkazildi", rep.Skipped, "modelXato", modelErr)
	return nil
}

func (w *worker) pending(ctx context.Context) ([]cleaner.PendingItem, error) {
	u := fmt.Sprintf("%s/api/v1/internal/clean/pending?limit=%d", w.cfg.backendURL, w.cfg.batch)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("X-Cleaner-Key", w.cfg.key)
	resp, err := w.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pending %d: %s", resp.StatusCode, data)
	}
	var out struct {
		Items []cleaner.PendingItem `json:"items"`
	}
	return out.Items, json.Unmarshal(data, &out)
}

type submitReport struct {
	Published, Rejected, Review, Skipped int
}

func (w *worker) submit(ctx context.Context, results []cleaner.Result) (submitReport, error) {
	var rep submitReport
	body, _ := json.Marshal(map[string]any{"results": results})
	u := w.cfg.backendURL + "/api/v1/internal/clean/results"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	req.Header.Set("X-Cleaner-Key", w.cfg.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.hc.Do(req)
	if err != nil {
		return rep, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return rep, fmt.Errorf("results %d: %s", resp.StatusCode, data)
	}
	return rep, json.Unmarshal(data, &rep)
}

func trimURL(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func fail(msg string) {
	slog.Error(msg)
	os.Exit(1)
}
