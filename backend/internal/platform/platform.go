// Package platform: konfiguratsiya, ma'lumotlar bazasi ulanishi va migratsiyalar.
package platform

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
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
	CORSOrigins   []string
	AdminOrigin   string
	AIBaseURL     string
	AIKey         string
	UploadDir     string

	GoogleClientIDs []string // web, android, ios OAuth client id lar (maxfiy emas)
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPassword    string
	MailFrom        string // "Yo'ldosh AI <noreply@domen.uz>"
	MailDevLog      bool   // faqat lokal: xat yuborilmaydi, kod logga yoziladi
}

func LoadConfig() (Config, error) {
	c := Config{
		Addr:          env("ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		TokenSecret:   []byte(os.Getenv("TOKEN_SECRET")),
		PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		AIBaseURL:     os.Getenv("AI_BASE_URL"),
		AIKey:         os.Getenv("AI_INTERNAL_KEY"),
		UploadDir:     env("UPLOAD_DIR", "data/uploads"),
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
	c.MailFrom = env("MAIL_FROM", "Yo'ldosh AI <noreply@e-investment.uz>")
	c.MailDevLog = os.Getenv("MAIL_DEV_LOG") == "true"
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
