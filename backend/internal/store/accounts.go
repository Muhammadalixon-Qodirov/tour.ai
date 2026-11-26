package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCodeInvalid = errors.New("code_invalid")
	ErrCodeLocked  = errors.New("code_locked") // juda ko'p noto'g'ri urinish
	ErrTokenReuse  = errors.New("refresh_reuse")
)

// MaxCodeAttempts — bitta kod uchun noto'g'ri urinishlar chegarasi.
const MaxCodeAttempts = 5

func NewUserID() string { return "u_" + uuid.Must(uuid.NewV7()).String() }

// IsRegistered — Google yoki email orqali kirgan hisob.
func (u User) IsRegistered() bool { return u.Email != nil || u.GoogleSub != nil }

func (s *Store) userBy(ctx context.Context, where string, arg any) (User, error) {
	return getUserWhere(ctx, s.DB, where, arg)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.userBy(ctx, "lower(email) = lower($1)", email)
}

func (s *Store) UserByGoogle(ctx context.Context, sub string) (User, error) {
	return s.userBy(ctx, "google_sub = $1", sub)
}

// ---------- Email kodlari ----------

func (s *Store) CountRecentCodes(ctx context.Context, email string, since time.Time) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT count(*) FROM auth_codes WHERE email = $1 AND created_at > $2`, email, since).Scan(&n)
	return n, err
}

// CreateAuthCode — yangi kod; shu email ning oldingi ishlatilmagan kodlari bekor qilinadi.
func (s *Store) CreateAuthCode(ctx context.Context, email, hash string, expires time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE auth_codes SET consumed_at = now() WHERE email = $1 AND consumed_at IS NULL`, email); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO auth_codes (email, code_hash, expires_at) VALUES ($1,$2,$3)`, email, hash, expires)
		return err
	})
}

// ConsumeAuthCode — eng oxirgi faol kodni tekshiradi. Noto'g'ri bo'lsa urinish soni oshadi.
func (s *Store) ConsumeAuthCode(ctx context.Context, email, hash string, now time.Time) error {
	var result error // commit dan keyin qaytariladi (urinishlar soni saqlanib qolishi uchun)
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var id int64
		var want string
		var attempts int
		err := tx.QueryRow(ctx, `SELECT id, code_hash, attempts FROM auth_codes
			WHERE email = $1 AND consumed_at IS NULL AND expires_at > $2 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`,
			email, now).Scan(&id, &want, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			result = ErrCodeInvalid
			return nil
		}
		if err != nil {
			return err
		}
		if attempts >= MaxCodeAttempts {
			result = ErrCodeLocked
			return nil
		}
		if want != hash {
			result = ErrCodeInvalid
			_, err := tx.Exec(ctx, `UPDATE auth_codes SET attempts = attempts + 1 WHERE id = $1`, id)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE auth_codes SET consumed_at = now() WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return err
	}
	return result
}

// ---------- Hisoblar ----------

type Identity struct {
	Email     *string
	GoogleSub *string
	Name      string
	AvatarURL string
}

// Login — identity bo'yicha hisobni topadi yoki yaratadi. anonID berilsa (anonim qurilma foydalanuvchisi):
// hisob yo'q bo'lsa anonim foydalanuvchi o'zi hisobga aylanadi; bor bo'lsa — anonim ma'lumotlar hisobga ko'chiriladi.
func (s *Store) Login(ctx context.Context, id Identity, anonID string, deviceID string, lang string) (User, bool, error) {
	var userID string
	created := false
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var existing string
		var err error
		if id.GoogleSub != nil {
			err = tx.QueryRow(ctx, `SELECT id FROM users WHERE google_sub = $1`, *id.GoogleSub).Scan(&existing)
			if errors.Is(err, pgx.ErrNoRows) && id.Email != nil {
				// shu email bilan avval email-kod orqali kirgan hisob bo'lsa — Google ni unga ulaymiz
				err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, *id.Email).Scan(&existing)
			}
		} else if id.Email != nil {
			err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, *id.Email).Scan(&existing)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		anonOK := false
		if anonID != "" && anonID != existing {
			var registered bool
			e := tx.QueryRow(ctx, `SELECT email IS NOT NULL OR google_sub IS NOT NULL FROM users WHERE id = $1`, anonID).Scan(&registered)
			anonOK = e == nil && !registered
		}

		switch {
		case existing != "":
			userID = existing
			if anonOK {
				if err := mergeUsers(ctx, tx, anonID, existing); err != nil {
					return err
				}
			}
		case anonOK:
			userID = anonID
		default:
			userID = NewUserID()
			created = true
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, lang) VALUES ($1, $2)`, userID, lang); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET
			email = COALESCE(email, $2), email_verified = email_verified OR $2::text IS NOT NULL,
			google_sub = COALESCE(google_sub, $3),
			display_name = COALESCE(display_name, NULLIF($4, '')),
			avatar_url = COALESCE(avatar_url, NULLIF($5, '')),
			last_login_at = now() WHERE id = $1`,
			userID, id.Email, id.GoogleSub, id.Name, id.AvatarURL); err != nil {
			return err
		}
		if deviceID != "" {
			if _, err := tx.Exec(ctx, `UPDATE devices SET user_id = $2, last_seen_at = now() WHERE id = $1`, deviceID, userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return User{}, false, err
	}
	u, err := s.User(ctx, userID)
	return u, created, err
}

// mergeUsers — anonim foydalanuvchining barcha ma'lumotlarini hisobga ko'chiradi va anonim yozuvni o'chiradi.
func mergeUsers(ctx context.Context, tx pgx.Tx, from, into string) error {
	stmts := []string{
		`UPDATE trips SET user_id = $2 WHERE user_id = $1`,
		`UPDATE diary_photos SET user_id = $2 WHERE user_id = $1`,
		`UPDATE diary_entries SET user_id = $2 WHERE user_id = $1`,
		`UPDATE posts SET user_id = $2 WHERE user_id = $1`,
		`UPDATE devices SET user_id = $2 WHERE user_id = $1`,
		// takrorlanadigan yozuvlar: hisobda bor bo'lsa anonimnikini tashlaymiz
		`UPDATE visits v SET user_id = $2 WHERE v.user_id = $1 AND NOT EXISTS
		   (SELECT 1 FROM visits x WHERE x.user_id = $2 AND x.place_id = v.place_id AND x.visit_date = v.visit_date)`,
		`INSERT INTO post_likes (post_id, user_id, created_at) SELECT post_id, $2, created_at FROM post_likes WHERE user_id = $1
		   ON CONFLICT DO NOTHING`,
		`INSERT INTO post_reports (post_id, user_id, reason, created_at) SELECT post_id, $2, reason, created_at FROM post_reports
		   WHERE user_id = $1 ON CONFLICT DO NOTHING`,
		`INSERT INTO notification_log (user_id, kind, day, sent_at) SELECT $2, kind, day, sent_at FROM notification_log
		   WHERE user_id = $1 ON CONFLICT DO NOTHING`,
		`UPDATE users u SET interests = CASE WHEN cardinality(u.interests) = 0 THEN a.interests ELSE u.interests END,
		   companion = COALESCE(u.companion, a.companion), pace = COALESCE(u.pace, a.pace), budget = COALESCE(u.budget, a.budget),
		   display_name = COALESCE(u.display_name, a.display_name)
		   FROM users a WHERE u.id = $2 AND a.id = $1`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(ctx, q, from, into); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, from)
	return err
}

// DeleteUser — hisob va unga tegishli hamma narsa (CASCADE). Rasm fayllari nomlarini qaytaradi (diskdan o'chirish uchun).
func (s *Store) DeleteUser(ctx context.Context, userID string) ([]string, error) {
	var files []string
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT file_name FROM diary_photos WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f string
			if err := rows.Scan(&f); err != nil {
				rows.Close()
				return err
			}
			files = append(files, f)
		}
		rows.Close()
		tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return err
	})
	return files, err
}

// ---------- Refresh tokenlar ----------

func (s *Store) IssueRefresh(ctx context.Context, userID, deviceID, hash string, expires time.Time) error {
	var dev *string
	if deviceID != "" {
		dev = &deviceID
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, device_id, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		uuid.Must(uuid.NewV7()), userID, hash, dev, expires)
	return err
}

// RotateRefresh — eski tokenni almashtiradi. Bekor qilingan token qayta ishlatilsa (o'g'irlangan bo'lishi mumkin)
// foydalanuvchining barcha refresh tokenlari bekor qilinadi.
func (s *Store) RotateRefresh(ctx context.Context, oldHash, newHash string, now, expires time.Time) (userID string, err error) {
	var result error
	err = pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var id uuid.UUID
		var dev *string
		var revoked *time.Time
		var exp time.Time
		e := tx.QueryRow(ctx, `SELECT id, user_id, device_id, revoked_at, expires_at FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`,
			oldHash).Scan(&id, &userID, &dev, &revoked, &exp)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
		if revoked != nil {
			result = ErrTokenReuse
			_, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
			return err
		}
		if !exp.After(now) {
			return ErrNotFound
		}
		newID := uuid.Must(uuid.NewV7())
		if _, err := tx.Exec(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, device_id, expires_at) VALUES ($1,$2,$3,$4,$5)`,
			newID, userID, newHash, dev, expires); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE id = $1`, id, newID)
		return err
	})
	if err == nil {
		err = result
	}
	return userID, err
}

func (s *Store) RevokeRefresh(ctx context.Context, hash string) error {
	_, err := s.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	return err
}

func (s *Store) RevokeAllRefresh(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&ok)
	return ok, err
}
