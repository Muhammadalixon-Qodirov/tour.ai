// Package speech — ovozni matnga aylantirish (STT).
//
// O'zbekcha — faqat Kotib modeli (o'zbekcha uchun eng aniq, WER ~13 %, tinish belgilari bilan):
//  1. GPU Kotib (stt-relay orqali, unix-socket; GPU kompyuter yoqiq bo'lsa, eng tez);
//  2. serverning o'zidagi Kotib (stt-kotib xizmati, CPU).
//
// Ruscha va inglizcha — Groq Whisper (Kotib faqat o'zbekcha). Kirill o'zbekcha natija lotinga o'giriladi.
package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type Result struct {
	Text   string `json:"text"`
	Lang   string `json:"lang"`
	Engine string `json:"engine"` // kotib-gpu | kotib-cpu | whisper
}

type STT struct {
	relay     *http.Client // nil — relay yo'q
	kotibURL  string       // serverdagi Kotib (stt-kotib), bo'sh — yo'q
	groqKeys  []string
	groqURL   string
	hc        *http.Client
	next      atomic.Uint32
	RelayURL  string // test uchun: relay HTTP manzili (socket o'rniga)
	GroqModel string
}

// New — hech bir dvigatel sozlanmagan bo'lsa nil.
func New(relaySocket, kotibURL, groqKeys string) *STT {
	s := &STT{groqURL: "https://api.groq.com/openai/v1/audio/transcriptions", GroqModel: "whisper-large-v3",
		hc: &http.Client{Timeout: 100 * time.Second}, RelayURL: "http://relay", kotibURL: strings.TrimRight(kotibURL, "/")}
	for _, k := range strings.Split(groqKeys, ",") {
		if k = strings.TrimSpace(k); k != "" {
			s.groqKeys = append(s.groqKeys, k)
		}
	}
	if relaySocket != "" {
		s.relay = &http.Client{Timeout: 100 * time.Second, Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", relaySocket)
			}}}
	}
	if s.relay == nil && s.kotibURL == "" && len(s.groqKeys) == 0 {
		return nil
	}
	return s
}

// WithTestEndpoints — testlar uchun: relay, serverdagi Kotib va Groq manzillari.
func (s *STT) WithTestEndpoints(relayURL, kotibURL, groqURL string) *STT {
	if kotibURL != "" {
		s.kotibURL = kotibURL
	}
	if relayURL != "" {
		s.relay, s.RelayURL = &http.Client{Timeout: 10 * time.Second}, relayURL
	}
	if groqURL != "" {
		s.groqURL = groqURL
	}
	return s
}

// ErrUnavailable — hech bir dvigatel ishlamadi.
var ErrUnavailable = errors.New("stt unavailable")

// ErrBadAudio — audio o'qilmadi yoki juda qisqa.
var ErrBadAudio = errors.New("bad audio")

// Transcribe — lang: uz | ru | en. O'zbekcha faqat Kotib (Whisper ishlatilmaydi).
func (s *STT) Transcribe(ctx context.Context, audio []byte, filename, lang string) (Result, error) {
	var errs []string
	if lang == "uz" {
		if s.relay != nil { // GPU Kotib
			r, err := s.viaRelay(ctx, audio)
			if err == nil && strings.TrimSpace(r.Text) != "" {
				r.Lang, r.Text = lang, Clean(r.Text, lang)
				return r, nil
			}
			if errors.Is(err, ErrBadAudio) {
				return Result{}, err
			}
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
		if s.kotibURL != "" { // serverdagi Kotib (CPU)
			r, err := s.viaKotib(ctx, audio)
			if err == nil {
				r.Lang, r.Text = lang, Clean(r.Text, lang)
				return r, nil
			}
			if errors.Is(err, ErrBadAudio) {
				return Result{}, err
			}
			errs = append(errs, err.Error())
		}
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, strings.Join(errs, "; "))
	}
	if len(s.groqKeys) > 0 {
		text, err := s.viaGroq(ctx, audio, filename, lang)
		if err == nil {
			return Result{Text: Clean(text, lang), Lang: lang, Engine: "whisper"}, nil
		}
		errs = append(errs, err.Error())
	}
	return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, strings.Join(errs, "; "))
}

func (s *STT) viaKotib(ctx context.Context, audio []byte) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.kotibURL+"/transcribe?lang=uz", bytes.NewReader(audio))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.hc.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 400 {
		return Result{}, fmt.Errorf("%w: %.200s", ErrBadAudio, body)
	}
	if resp.StatusCode != 200 {
		return Result{}, fmt.Errorf("kotib %d: %.200s", resp.StatusCode, body)
	}
	var r Result
	return r, json.Unmarshal(body, &r)
}

func (s *STT) viaRelay(ctx context.Context, audio []byte) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.RelayURL+"/stt", bytes.NewReader(audio))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.relay.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 400 {
		return Result{}, fmt.Errorf("%w: %.200s", ErrBadAudio, body)
	}
	if resp.StatusCode != 200 {
		return Result{}, fmt.Errorf("relay %d: %.200s", resp.StatusCode, body)
	}
	var r Result
	return r, json.Unmarshal(body, &r)
}

// prompts — Whisper ga til va yozuvni ko'rsatuvchi namuna (lotin o'zbekcha, sayohat so'zlari).
var prompts = map[string]string{
	"uz": "Assalomu alaykum. Samarqanddagi Registon maydoniga qanday boraman? Buxoroda arzon mehmonxona qayerda? Toshkentda osh qayerda yeyish mumkin?",
	"ru": "Здравствуйте. Как добраться до площади Регистан в Самарканде? Где недорогая гостиница в Бухаре?",
	"en": "Hello. How do I get to Registan Square in Samarkand? Where is a cheap hotel in Bukhara?",
}

func (s *STT) viaGroq(ctx context.Context, audio []byte, filename, lang string) (string, error) {
	if filename == "" {
		filename = "audio.m4a"
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("model", s.GroqModel)
	mw.WriteField("language", lang)
	mw.WriteField("temperature", "0")
	mw.WriteField("response_format", "json")
	if p := prompts[lang]; p != "" {
		mw.WriteField("prompt", p)
	}
	fw, _ := mw.CreateFormFile("file", filename)
	fw.Write(audio)
	mw.Close()
	body := buf.Bytes()
	start := int(s.next.Add(1)-1) % len(s.groqKeys)
	var last error
	for i := range s.groqKeys {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.groqURL, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+s.groqKeys[(start+i)%len(s.groqKeys)])
		resp, err := s.hc.Do(req)
		if err != nil {
			last = err
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode == 401 || resp.StatusCode >= 500 {
			last = fmt.Errorf("groq %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode == 400 {
			return "", fmt.Errorf("%w: %.200s", ErrBadAudio, data)
		}
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("groq %d: %.200s", resp.StatusCode, data)
		}
		var out struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return "", err
		}
		return out.Text, nil
	}
	return "", last
}

// Clean — bo'sh joylar; o'zbekcha bo'lsa kirill -> lotin.
func Clean(s, lang string) string {
	s = strings.Join(strings.Fields(s), " ")
	if lang == "uz" {
		s = CyrToLat(s)
	}
	return s
}

var cyrLat = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "j", 'з': "z", 'и': "i", 'й': "y",
	'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "x", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sh", 'ъ': "'", 'ы': "i", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'ў': "o'", 'қ': "q", 'ғ': "g'", 'ҳ': "h",
}

// CyrToLat — o'zbek kirill yozuvini lotinga (rasmiy imlo, asosiy qoidalar). Lotin matnga tegmaydi.
func CyrToLat(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool { _, ok := cyrLat[toLower(r)]; return ok }) {
		return s
	}
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		lr := toLower(r)
		v, ok := cyrLat[lr]
		if !ok {
			b.WriteRune(r)
			continue
		}
		// "е" so'z boshida yoki unlidan keyin "ye"
		if lr == 'е' && (i == 0 || !isLetter(rs[i-1]) || strings.ContainsRune("аеёиоуэюяў", toLower(rs[i-1]))) {
			v = "ye"
		}
		if r != lr && v != "" { // katta harf
			v = strings.ToUpper(v[:1]) + v[1:]
		}
		b.WriteString(v)
	}
	return b.String()
}

func toLower(r rune) rune {
	if r >= 'А' && r <= 'Я' {
		return r + 32
	}
	switch r {
	case 'Ё':
		return 'ё'
	case 'Ў':
		return 'ў'
	case 'Қ':
		return 'қ'
	case 'Ғ':
		return 'ғ'
	case 'Ҳ':
		return 'ҳ'
	}
	return r
}

func isLetter(r rune) bool {
	_, ok := cyrLat[toLower(r)]
	return ok || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}
