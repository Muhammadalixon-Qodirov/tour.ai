package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"yoldosh/internal/store"
)

// Kechki eslatma: turistning mahalliy vaqti bilan 20:00–20:59 oralig'ida, bugun kamida 2 ta joyga
// "bordim" belgisi qo'yilgan va hikoyasi hali yo'q bo'lsa — hikoya oldindan yoziladi va push yuboriladi.
// Kuniga bir martadan ko'p emas (notification_log).

type PushSender interface {
	Send(ctx context.Context, tokens []string, title, body string, data map[string]string) error
}

// ExpoPush — Expo push xizmati (React Native / Expo ilova).
type ExpoPush struct {
	URL         string
	AccessToken string
	HTTP        *http.Client
}

func NewExpoPush(accessToken string) *ExpoPush {
	return &ExpoPush{URL: "https://exp.host/--/api/v2/push/send", AccessToken: accessToken, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (e *ExpoPush) Send(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	msgs := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		msgs = append(msgs, map[string]any{"to": t, "title": title, "body": body, "data": data, "sound": "default"})
	}
	b, _ := json.Marshal(msgs)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expo push: status %d", resp.StatusCode)
	}
	return nil
}

var nudgeText = map[string][2]string{
	"uz": {"Bugungi hikoyangiz tayyor ✨", "Ko'rgan joylaringizdan hikoya yozdik — o'qing va do'stlaringiz bilan ulashing"},
	"ru": {"История вашего дня готова ✨", "Мы написали историю о местах, где вы были, — прочитайте и поделитесь"},
	"en": {"Your story of the day is ready ✨", "We wrote a story about today's places — read it and share it with friends"},
}

// NudgeTick — bitta tekshiruv. Ishga tushirgich uni har 5 daqiqada chaqiradi.
func (s *Server) NudgeTick(ctx context.Context, sender PushSender) (sent int, err error) {
	targets, err := s.store.PushTargets(ctx)
	if err != nil {
		return 0, err
	}
	byUser := map[string][]store.PushTarget{}
	for _, t := range targets {
		byUser[t.UserID] = append(byUser[t.UserID], t)
	}
	for userID, devs := range byUser {
		zone := "Asia/Tashkent"
		if devs[0].Timezone != nil {
			zone = *devs[0].Timezone
		}
		now := s.now().In(tz(zone))
		if now.Hour() != 20 {
			continue
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		tripID, day, ok, err := s.store.DiaryReadyDay(ctx, userID, today, minVisitedForPush)
		if err != nil || !ok {
			if err != nil {
				slog.Warn("nudge", "err", err)
			}
			continue
		}
		first, err := s.store.MarkNotified(ctx, userID, "diary_ready", today)
		if err != nil || !first {
			continue
		}
		lang := devs[0].Lang
		t, err := s.store.Trip(ctx, tripID, userID)
		if err != nil {
			slog.Warn("nudge trip", "err", err)
			continue
		}
		entryID, err := s.buildStory(ctx, t, day, lang, "warm", false)
		if err != nil {
			slog.Warn("nudge story", "err", err)
			continue
		}
		tokens := make([]string, 0, len(devs))
		for _, d := range devs {
			tokens = append(tokens, d.Token)
		}
		txt, ok := nudgeText[lang]
		if !ok {
			txt = nudgeText["en"]
		}
		data := map[string]string{"type": "diary_ready", "entryId": entryID.String(), "tripId": tripID.String(), "dayIndex": fmt.Sprint(day)}
		if err := sender.Send(ctx, tokens, txt[0], txt[1], data); err != nil {
			slog.Warn("push yuborilmadi", "err", err)
			continue
		}
		sent++
	}
	return sent, nil
}

// RunNudger — fon jarayoni (EXPO_PUSH_ENABLED=true bo'lsa main ishga tushiradi).
func (s *Server) RunNudger(ctx context.Context, sender PushSender, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if n, err := s.NudgeTick(ctx, sender); err != nil {
			slog.Warn("nudge tick", "err", err)
		} else if n > 0 {
			slog.Info("kechki eslatmalar yuborildi", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
