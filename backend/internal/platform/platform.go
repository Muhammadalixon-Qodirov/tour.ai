// Package platform: konfiguratsiya, ma'lumotlar bazasi ulanishi va migratsiyalar.
package platform

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/db/migrations"
)

type Config struct {
	Addr          string
	DatabaseURL   string
	TokenSecret   []byte
	PublicBaseURL string
	WebBaseURL    string // sayt manzili: story havolasi, joy sahifasiga va namuna hamkor sahifasiga yo'naltirish
	CORSOrigins   []string
	AdminOrigin   string
	AIBaseURL     string
	AIKey         string
	UploadDir     string

	// AI yordamchi (POST /guide/chat). Kalit bo'sh — bazadan qoidaga asoslangan qidiruv.
	AnthropicKey    string
	GroqKey         string // Anthropic kaliti bo'lmasa Groq (openai/gpt-oss-120b)
	LensVisionModel string // "Adolatli narx" rasm tanish modeli (Groq; standart qwen/qwen3.8-27b)
	STTKotibURL     string // serverdagi Kotib STT (stt-kotib), masalan http://stt-kotib:8000
	STTRelaySocket  string // stt-relay unix-socket (serverdagi o'zbekcha Kotib STT); bo'sh — faqat Groq Whisper
	UzumBearer      string // Uzum anonim Bearer token (eskiradi; bo'sh — Uzum o'chiq)
	MarketFetchURL  string // market-fetch yordamchisi (curl, brauzer izi); bo'sh bo'lsa MarketProxy
	MarketProxy     string // market qidiruvi uchun HTTP proksi (O'zbekistondagi server); bo'sh — to'g'ridan-to'g'ri
	BotKey          string // Telegram crowdsourcing boti kaliti (bo'sh — /internal/suggest o'chiq)
	CleanerKey      string // ichki AI tozalovchi ishchi uchun kalit (bo'sh — /internal/clean/* o'chiq)
	AIModel         string // standart claude-opus-5-5
	AIEffort        string // low | medium | high (standart low — suhbat tez bo'lsin)

	GoogleClientIDs []string // web, android, ios OAuth client id lar (maxfiy emas)
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPassword    string
	MailFrom        string // "Yo'ldosh AI <noreply@domen.uz>"
	MailDevLog      bool   // faqat lokal: xat yuborilmaydi, kod logga yoziladi
	AppName         string // foydalanuvchiga ko'rinadigan brend nomi (xatlarda)

	// TrustedProxies — X-Forwarded-For ga ishoniladigan proksilar (nginx 127.0.0.1, sayt serveri IP si).
	// Bo'sh bo'lsa sarlavha e'tiborsiz qoldiriladi va RemoteAddr ishlatiladi.
	TrustedProxies []netip.Prefix
}

// DefaultWebBaseURL — production sayt manzili (WEB_BASE_URL berilmasa).
const DefaultWebBaseURL = "https://safar.e-investment.uz"

func LoadConfig() (Config, error) {
	c := Config{
		Addr:            env("ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		TokenSecret:     []byte(os.Getenv("TOKEN_SECRET")),
		PublicBaseURL:   strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		AIBaseURL:       os.Getenv("AI_BASE_URL"),
		AIKey:           os.Getenv("AI_INTERNAL_KEY"),
		UploadDir:       env("UPLOAD_DIR", "data/uploads"),
		AnthropicKey:    os.Getenv("ANTHROPIC_API_KEY"),
		GroqKey:         os.Getenv("GROQ_API_KEY"),
		CleanerKey:      os.Getenv("CLEANER_KEY"),
		BotKey:          os.Getenv("BOT_KEY"),
		LensVisionModel: os.Getenv("LENS_VISION_MODEL"),
		MarketProxy:     os.Getenv("MARKET_PROXY"),
		MarketFetchURL:  os.Getenv("MARKET_FETCH_URL"),
		UzumBearer:      os.Getenv("UZUM_BEARER"),
		STTRelaySocket:  os.Getenv("STT_RELAY_SOCKET"),
		STTKotibURL:     os.Getenv("STT_KOTIB_URL"),
		AIModel:         os.Getenv("AI_MODEL"),
		AIEffort:        os.Getenv("AI_EFFORT"),
		WebBaseURL:      strings.TrimRight(env("WEB_BASE_URL", DefaultWebBaseURL), "/"),
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CORSOrigins = append(c.CORSOrigins, o)
			}
		}
	}
	c.GoogleClientIDs = splitList(os.Getenv("GOOGLE_CLIENT_IDS"))
	c.SMTPHost, c.SMTPUser, c.SMTPPassword = os.Getenv("SMTP_HOST"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD")
	c.SMTPPort = 587
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &c.SMTPPort); err != nil {
			return c, fmt.Errorf("SMTP_PORT noto'g'ri")
		}
	}
	c.MailFrom = env("MAIL_FROM", "Safar <noreply@e-investment.uz>")
	c.MailDevLog = os.Getenv("MAIL_DEV_LOG") == "true"
	c.AppName = env("APP_NAME", "Safar")
	for _, v := range splitList(os.Getenv("TRUSTED_PROXIES")) {
		p, err := ParsePrefix(v)
		if err != nil {
			return c, fmt.Errorf("TRUSTED_PROXIES: %q noto'g'ri (IP yoki CIDR)", v)
		}
		c.TrustedProxies = append(c.TrustedProxies, p)
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL o'rnatilmagan")
	}
	if len(c.TokenSecret) < 32 {
		return c, fmt.Errorf("TOKEN_SECRET kamida 32 belgi bo'lishi kerak")
	}
	return c, nil
}

func splitList(v string) []string {
	var out []string
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func OpenDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("bazaga ulanib bo'lmadi: %w", err)
	}
	return pool, nil
}

// Migrate db/migrations dagi hali qo'llanmagan .sql fayllarni nom tartibida bajaradi.
// Har bir fayl alohida tranzaksiyada.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migratsiya %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		slog.Info("migratsiya qo'llandi", "version", name)
	}
	return nil
}

// ParsePrefix — "10.0.0.0/8" yoki bitta IP ("127.0.0.1" → /32).
func ParsePrefix(v string) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(v); err == nil {
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(v)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()), nil
}
