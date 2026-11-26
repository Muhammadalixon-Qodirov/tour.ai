// tgbot — Safar crowdsourcing boti. Foydalanuvchi botga yangi joyni yuboradi (lokatsiya + nom + ixtiyoriy rasm);
// bot uni asosiy serverga yuboradi, u admin navbatiga tushadi (hech qachon avtomatik nashr emas).
//
// Qonuniy: foydalanuvchi o'z joyini ixtiyoriy yuboradi (scraping emas). Begona kanallar o'qilmaydi.
//
// Sozlash (env):
//
//	TELEGRAM_BOT_TOKEN   @BotFather bergan token
//	BACKEND_URL          https://safar-api.e-investment.uz
//	BOT_KEY              asosiy serverdagi BOT_KEY bilan bir xil
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type bot struct {
	token, api, backend, key string
	hc                       *http.Client
	mu                       sync.Mutex
	pending                  map[int64]*pend // chat -> kutilayotgan joy
}

type pend struct {
	lat, lng float64
	name     string
	photo    string // telegram file_id
	at       time.Time
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	tok := os.Getenv("TELEGRAM_BOT_TOKEN")
	b := &bot{token: tok, api: "https://api.telegram.org/bot" + tok, backend: strings.TrimRight(os.Getenv("BACKEND_URL"), "/"),
		key: os.Getenv("BOT_KEY"), hc: &http.Client{Timeout: 70 * time.Second}, pending: map[int64]*pend{}}
	if tok == "" || b.backend == "" || b.key == "" {
		slog.Error("TELEGRAM_BOT_TOKEN, BACKEND_URL va BOT_KEY kerak")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if me, err := b.call(ctx, "getMe", nil); err == nil {
		slog.Info("bot ishga tushdi", "me", string(me))
	}
	go b.gc(ctx)
	var offset int64
	for ctx.Err() == nil {
		ups, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("getUpdates", "err", err)
				time.Sleep(3 * time.Second)
			}
			continue
		}
		for _, u := range ups {
			offset = u.UpdateID + 1
			if u.Message != nil {
				b.handle(ctx, u.Message)
			}
		}
	}
}

// ---- Telegram tiplari (kerakli qismlari) ----

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		Username string `json:"username"`
		ID       int64  `json:"id"`
	} `json:"from"`
	Text     string `json:"text"`
	Caption  string `json:"caption"`
	Location *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
	Photo []struct {
		FileID string `json:"file_id"`
	} `json:"photo"`
}

func (b *bot) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	raw, err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 50, "allowed_updates": []string{"message"}})
	if err != nil {
		return nil, err
	}
	var r struct {
		Result []update `json:"result"`
	}
	return r.Result, json.Unmarshal(raw, &r)
}

func (b *bot) call(ctx context.Context, method string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.api+"/"+method, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if json.Unmarshal(data, &env) == nil && !env.OK {
		return nil, fmt.Errorf("telegram: %s", env.Description)
	}
	return env.Result, nil
}

func (b *bot) send(ctx context.Context, chat int64, text string) {
	_, _ = b.call(ctx, "sendMessage", map[string]any{"chat_id": chat, "text": text, "disable_web_page_preview": true})
}

const helpText = `Assalomu alaykum! Men Safar yordamchisiman.

Yangi joyni (kafe, muzey, tarixiy obida, bog'...) qo'shish uchun:
1. Pastdagi 📎 (skrepka) → Joylashuv (Location) ni yuboring — joyning xaritadagi nuqtasini.
2. Keyin joyning nomini yozing (va xohlasangiz rasmini yuboring).

Rahmat! Har bir joy moderatsiyadan o'tgach ilovada ko'rinadi.`

func (b *bot) handle(ctx context.Context, m *message) {
	chat := m.Chat.ID
	switch {
	case strings.HasPrefix(m.Text, "/start") || strings.HasPrefix(m.Text, "/help"):
		b.send(ctx, chat, helpText)
		return
	case m.Location != nil:
		b.mu.Lock()
		b.pending[chat] = &pend{lat: m.Location.Latitude, lng: m.Location.Longitude, at: time.Now()}
		b.mu.Unlock()
		b.send(ctx, chat, "Joylashuv qabul qilindi. Endi joyning nomini yozing (masalan: \"Choyxona Navro'z\").")
		return
	}
	// lokatsiya kutilyaptimi
	b.mu.Lock()
	p := b.pending[chat]
	b.mu.Unlock()
	if p == nil {
		b.send(ctx, chat, "Avval joyning joylashuvini yuboring (📎 → Location), keyin nomini. /help — yordam.")
		return
	}
	if len(m.Photo) > 0 {
		p.photo = m.Photo[len(m.Photo)-1].FileID // eng katta o'lcham
	}
	name := strings.TrimSpace(firstNonEmpty(m.Text, m.Caption, p.name))
	if name == "" {
		if p.photo != "" {
			b.send(ctx, chat, "Rasm qabul qilindi. Endi joyning nomini yozing.")
		}
		return
	}
	p.name = name
	b.submit(ctx, chat, p, m)
}

func (b *bot) submit(ctx context.Context, chat int64, p *pend, m *message) {
	from := "@" + m.From.Username
	if m.From.Username == "" {
		from = fmt.Sprintf("id%d", m.From.ID)
	}
	photoURL := ""
	if p.photo != "" {
		photoURL = b.fileURL(ctx, p.photo)
	}
	body, _ := json.Marshal(map[string]any{"name": p.name, "location": map[string]float64{"lat": p.lat, "lng": p.lng},
		"from": from, "photoUrl": photoURL})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.backend+"/api/v1/internal/suggest", bytes.NewReader(body))
	req.Header.Set("X-Bot-Key", b.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.hc.Do(req)
	if err != nil {
		b.send(ctx, chat, "Xatolik yuz berdi, birozdan keyin qayta urinib ko'ring.")
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	b.mu.Lock()
	delete(b.pending, chat)
	b.mu.Unlock()
	switch {
	case resp.StatusCode == 202:
		var r struct{ Status string }
		_ = json.Unmarshal(data, &r)
		if r.Status == "duplicate" {
			b.send(ctx, chat, "Bu joy allaqachon bazamizda bor. Baribir rahmat!")
		} else {
			b.send(ctx, chat, "Rahmat! \""+p.name+"\" qabul qilindi va tez orada tekshiriladi. Yana joy qo'shishingiz mumkin.")
		}
	case resp.StatusCode == 400:
		b.send(ctx, chat, "Joylashuv O'zbekiston ichida bo'lishi va nom 2 harfdan uzun bo'lishi kerak. Qaytadan urinib ko'ring.")
	default:
		b.send(ctx, chat, "Xatolik yuz berdi, birozdan keyin qayta urinib ko'ring.")
	}
}

// fileURL — Telegram file_id dan yuklab olish havolasi (token bilan; admin ko'radi).
func (b *bot) fileURL(ctx context.Context, fileID string) string {
	raw, err := b.call(ctx, "getFile", map[string]any{"file_id": fileID})
	if err != nil {
		return ""
	}
	var f struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(raw, &f) != nil || f.FilePath == "" {
		return ""
	}
	return "https://api.telegram.org/file/bot" + b.token + "/" + url.PathEscape(f.FilePath)
}

// gc — 30 daqiqadan eski tugallanmagan kutuvlarni tozalaydi.
func (b *bot) gc(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.mu.Lock()
			for k, p := range b.pending {
				if time.Since(p.at) > 30*time.Minute {
					delete(b.pending, k)
				}
			}
			b.mu.Unlock()
		}
	}
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}
