package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------- Rasmlar ----------

type DiaryPhoto struct {
	ID        uuid.UUID
	UserID    string
	TripID    uuid.UUID
	DayIdx    int
	StopID    *uuid.UUID
	FileName  string
	URL       string
	Width     int
	Height    int
	CreatedAt time.Time
}

func (s *Store) AddDiaryPhoto(ctx context.Context, p DiaryPhoto) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO diary_photos (id, user_id, trip_id, day_idx, stop_id, file_name, url, width, height)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, p.UserID, p.TripID, p.DayIdx, p.StopID, p.FileName, p.URL, p.Width, p.Height)
	return err
}

func (s *Store) CountDayPhotos(ctx context.Context, tripID uuid.UUID, day int) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `SELECT count(*) FROM diary_photos WHERE trip_id = $1 AND day_idx = $2`, tripID, day).Scan(&n)
	return n, err
}

const photoCols = `id, user_id, trip_id, day_idx, stop_id, file_name, url, width, height, created_at`

func scanPhotos(rows pgx.Rows) ([]DiaryPhoto, error) {
	defer rows.Close()
	out := []DiaryPhoto{}
	for rows.Next() {
		var p DiaryPhoto
		if err := rows.Scan(&p.ID, &p.UserID, &p.TripID, &p.DayIdx, &p.StopID, &p.FileName, &p.URL, &p.Width, &p.Height, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DayPhotos(ctx context.Context, tripID uuid.UUID, day int) ([]DiaryPhoto, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+photoCols+` FROM diary_photos WHERE trip_id = $1 AND day_idx = $2 ORDER BY created_at`, tripID, day)
	if err != nil {
		return nil, err
	}
	return scanPhotos(rows)
}

func (s *Store) PhotosByIDs(ctx context.Context, ids []uuid.UUID) ([]DiaryPhoto, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+photoCols+` FROM diary_photos WHERE id = ANY($1) ORDER BY array_position($1, id)`, ids)
	if err != nil {
		return nil, err
	}
	return scanPhotos(rows)
}

// DeleteDiaryPhoto — faqat egasi; o'chirilgan rasm yozuvlardan ham chiqariladi. Fayl nomini qaytaradi.
func (s *Store) DeleteDiaryPhoto(ctx context.Context, userID string, id uuid.UUID) (string, error) {
	var file string
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `DELETE FROM diary_photos WHERE id = $1 AND user_id = $2 RETURNING file_name`, id, userID).Scan(&file)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE diary_entries SET photo_ids = array_remove(photo_ids, $1), updated_at = now() WHERE $1 = ANY(photo_ids)`, id)
		return err
	})
	return file, err
}

// ---------- Yozuvlar ----------

type DiaryEntry struct {
	ID            uuid.UUID
	UserID        string
	TripID        uuid.UUID
	DayIdx        int
	Lang          string
	Tone          string
	Title         string
	Body          string
	Caption       string
	Highlights    []string
	Hashtags      []string
	PlaceIDs      []string
	PhotoIDs      []uuid.UUID
	GeneratedBy   string
	Edited        bool
	Regenerations int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// bog'liq
	CityID string
	Date   time.Time
	Post   *Post
}

const entryCols = `e.id, e.user_id, e.trip_id, e.day_idx, e.lang, e.tone, e.title, e.body, e.caption, e.highlights, e.hashtags,
  e.place_ids, e.photo_ids, e.generated_by, e.edited, e.regenerations, e.created_at, e.updated_at, t.city_id, d.date`

const entryFrom = ` FROM diary_entries e JOIN trips t ON t.id = e.trip_id
  JOIN trip_days d ON d.trip_id = e.trip_id AND d.idx = e.day_idx`

func scanEntry(row pgx.Row) (DiaryEntry, error) {
	var e DiaryEntry
	err := row.Scan(&e.ID, &e.UserID, &e.TripID, &e.DayIdx, &e.Lang, &e.Tone, &e.Title, &e.Body, &e.Caption, &e.Highlights,
		&e.Hashtags, &e.PlaceIDs, &e.PhotoIDs, &e.GeneratedBy, &e.Edited, &e.Regenerations, &e.CreatedAt, &e.UpdatedAt, &e.CityID, &e.Date)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

func (s *Store) withPost(ctx context.Context, e DiaryEntry, err error) (DiaryEntry, error) {
	if err != nil {
		return e, err
	}
	p, err := s.PostByEntry(ctx, e.ID)
	if err == nil {
		e.Post = &p
	} else if !errors.Is(err, ErrNotFound) {
		return e, err
	}
	return e, nil
}

func (s *Store) DiaryEntry(ctx context.Context, id uuid.UUID, userID string) (DiaryEntry, error) {
	e, err := scanEntry(s.DB.QueryRow(ctx, `SELECT `+entryCols+entryFrom+` WHERE e.id = $1 AND e.user_id = $2`, id, userID))
	return s.withPost(ctx, e, err)
}

func (s *Store) DiaryEntryByDay(ctx context.Context, tripID uuid.UUID, day int, userID string) (DiaryEntry, error) {
	e, err := scanEntry(s.DB.QueryRow(ctx, `SELECT `+entryCols+entryFrom+` WHERE e.trip_id = $1 AND e.day_idx = $2 AND e.user_id = $3`,
		tripID, day, userID))
	return s.withPost(ctx, e, err)
}

// UpsertDiaryEntry — kun uchun yozuvni yaratadi yoki AI/shablon natijasi bilan almashtiradi.
func (s *Store) UpsertDiaryEntry(ctx context.Context, e DiaryEntry, regenerated bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.DB.QueryRow(ctx, `INSERT INTO diary_entries (id, user_id, trip_id, day_idx, lang, tone, title, body, caption,
		highlights, hashtags, place_ids, photo_ids, generated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (trip_id, day_idx) DO UPDATE SET lang = EXCLUDED.lang, tone = EXCLUDED.tone, title = EXCLUDED.title,
		  body = EXCLUDED.body, caption = EXCLUDED.caption, highlights = EXCLUDED.highlights, hashtags = EXCLUDED.hashtags,
		  place_ids = EXCLUDED.place_ids, photo_ids = EXCLUDED.photo_ids, generated_by = EXCLUDED.generated_by,
		  edited = false, regenerations = diary_entries.regenerations + CASE WHEN $15 THEN 1 ELSE 0 END, updated_at = now()
		RETURNING id`,
		e.ID, e.UserID, e.TripID, e.DayIdx, e.Lang, e.Tone, e.Title, e.Body, e.Caption, e.Highlights, e.Hashtags,
		e.PlaceIDs, e.PhotoIDs, e.GeneratedBy, regenerated).Scan(&id)
	return id, err
}

type EntryPatch struct {
	Title    *string
	Body     *string
	Caption  *string
	Hashtags *[]string
	PhotoIDs *[]uuid.UUID
}

func (s *Store) UpdateDiaryEntry(ctx context.Context, id uuid.UUID, userID string, p EntryPatch) error {
	tag, err := s.DB.Exec(ctx, `UPDATE diary_entries SET title = COALESCE($3, title), body = COALESCE($4, body),
		caption = COALESCE($5, caption), hashtags = COALESCE($6, hashtags), photo_ids = COALESCE($7, photo_ids),
		edited = true, updated_at = now() WHERE id = $1 AND user_id = $2`, id, userID, p.Title, p.Body, p.Caption, p.Hashtags, p.PhotoIDs)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddShareEvent(ctx context.Context, entryID uuid.UUID, postID *uuid.UUID, target string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO share_events (diary_entry_id, post_id, target) VALUES ($1,$2,$3)`, entryID, postID, target)
	return err
}

// ---------- Postlar va lenta ----------

type Post struct {
	ID         uuid.UUID
	UserID     string
	EntryID    uuid.UUID
	CityID     string
	Slug       string
	Visibility string
	Status     string
	LikesCount int
	ViewsCount int
	CreatedAt  time.Time
	// lenta uchun
	Entry      *DiaryEntry
	AuthorName *string
	LikedByMe  bool
}

const postCols = `p.id, p.user_id, p.diary_entry_id, p.city_id, p.slug, p.visibility, p.status, p.likes_count, p.views_count, p.created_at`

func scanPost(row pgx.Row, extra ...any) (Post, error) {
	var p Post
	err := row.Scan(append([]any{&p.ID, &p.UserID, &p.EntryID, &p.CityID, &p.Slug, &p.Visibility, &p.Status, &p.LikesCount,
		&p.ViewsCount, &p.CreatedAt}, extra...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) PostByEntry(ctx context.Context, entryID uuid.UUID) (Post, error) {
	return scanPost(s.DB.QueryRow(ctx, `SELECT `+postCols+` FROM posts p WHERE p.diary_entry_id = $1`, entryID))
}

// PublishEntry — yozuvdan post yaratadi yoki ko'rinishini yangilaydi (bitta yozuv = bitta post).
func (s *Store) PublishEntry(ctx context.Context, e DiaryEntry, slug, visibility string) (Post, error) {
	_, err := s.DB.Exec(ctx, `INSERT INTO posts (id, user_id, diary_entry_id, city_id, slug, visibility)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (diary_entry_id) DO UPDATE SET visibility = EXCLUDED.visibility,
		  status = CASE WHEN posts.status = 'removed' THEN 'visible' ELSE posts.status END`,
		uuid.Must(uuid.NewV7()), e.UserID, e.ID, e.CityID, slug, visibility)
	if err != nil {
		return Post{}, err
	}
	return s.PostByEntry(ctx, e.ID)
}

func (s *Store) attachEntries(ctx context.Context, posts []Post) error {
	for i := range posts {
		e, err := scanEntry(s.DB.QueryRow(ctx, `SELECT `+entryCols+entryFrom+` WHERE e.id = $1`, posts[i].EntryID))
		if err != nil {
			return err
		}
		posts[i].Entry = &e
	}
	return nil
}

func (s *Store) Feed(ctx context.Context, viewer, cityID string, limit, offset int) ([]Post, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+postCols+` FROM posts p WHERE p.status = 'visible' AND p.visibility = 'public'
		AND ($1 = '' OR p.city_id = $1) ORDER BY p.created_at DESC, p.id LIMIT $2 OFFSET $3`, cityID, limit, offset)
	if err != nil {
		return nil, err
	}
	var out []Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.enrich(ctx, viewer, out)
}

func (s *Store) enrich(ctx context.Context, viewer string, posts []Post) ([]Post, error) {
	for i := range posts {
		err := s.DB.QueryRow(ctx, `SELECT u.display_name, EXISTS (SELECT 1 FROM post_likes l WHERE l.post_id = $2 AND l.user_id = $1)
			FROM users u WHERE u.id = $3`, viewer, posts[i].ID, posts[i].UserID).Scan(&posts[i].AuthorName, &posts[i].LikedByMe)
		if err != nil {
			return nil, err
		}
	}
	if err := s.attachEntries(ctx, posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// Post — ko'rish huquqi: ko'rinadigan public/link post yoki muallifning o'zi.
func (s *Store) Post(ctx context.Context, viewer string, id uuid.UUID) (Post, error) {
	p, err := scanPost(s.DB.QueryRow(ctx, `SELECT `+postCols+` FROM posts p WHERE p.id = $1
		AND (p.user_id = $2 OR (p.status = 'visible' AND p.visibility IN ('public','link')))`, id, viewer))
	if err != nil {
		return p, err
	}
	out, err := s.enrich(ctx, viewer, []Post{p})
	if err != nil {
		return p, err
	}
	return out[0], nil
}

// PostBySlug — ochiq sahifa uchun (ko'rishlar soni oshadi).
func (s *Store) PostBySlug(ctx context.Context, slug string) (Post, error) {
	p, err := scanPost(s.DB.QueryRow(ctx, `UPDATE posts p SET views_count = views_count + 1 WHERE p.slug = $1
		AND p.status = 'visible' AND p.visibility IN ('public','link') RETURNING `+postCols, slug))
	if err != nil {
		return p, err
	}
	out, err := s.enrich(ctx, "", []Post{p})
	if err != nil {
		return p, err
	}
	return out[0], nil
}

func (s *Store) SetLike(ctx context.Context, postID uuid.UUID, userID string, liked bool) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var err error
		if liked {
			_, err = tx.Exec(ctx, `INSERT INTO post_likes (post_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, postID, userID)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`, postID, userID)
		}
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `UPDATE posts SET likes_count = (SELECT count(*) FROM post_likes WHERE post_id = $1)
			WHERE id = $1 RETURNING likes_count`, postID).Scan(&n)
	})
	return n, err
}

// ReportHideThreshold — shuncha turli foydalanuvchi shikoyat qilsa post avtomatik yashiriladi.
const ReportHideThreshold = 3

func (s *Store) ReportPost(ctx context.Context, postID uuid.UUID, userID, reason string) (hidden bool, err error) {
	err = pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO post_reports (post_id, user_id, reason) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			postID, userID, reason); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM post_reports WHERE post_id = $1`, postID).Scan(&n); err != nil {
			return err
		}
		if n >= ReportHideThreshold {
			tag, err := tx.Exec(ctx, `UPDATE posts SET status = 'hidden' WHERE id = $1 AND status = 'visible'`, postID)
			hidden = err == nil && tag.RowsAffected() > 0
			return err
		}
		return nil
	})
	return hidden, err
}

func (s *Store) DeletePost(ctx context.Context, postID uuid.UUID, userID string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE posts SET status = 'removed' WHERE id = $1 AND user_id = $2`, postID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Admin: postlar moderatsiyasi ----------

type ReportedPost struct {
	Post
	Reports int
	Reasons []string
}

func (s *Store) AdminPosts(ctx context.Context, status string, limit, offset int) ([]ReportedPost, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+postCols+`,
		(SELECT count(*) FROM post_reports r WHERE r.post_id = p.id),
		COALESCE((SELECT array_agg(r.reason ORDER BY r.created_at) FROM post_reports r WHERE r.post_id = p.id), '{}')
		FROM posts p WHERE ($1 = '' AND EXISTS (SELECT 1 FROM post_reports r WHERE r.post_id = p.id)) OR p.status = $1
		ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	var out []ReportedPost
	for rows.Next() {
		var rp ReportedPost
		p, err := scanPost(rows, &rp.Reports, &rp.Reasons)
		if err != nil {
			rows.Close()
			return nil, err
		}
		rp.Post = p
		out = append(out, rp)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		e, err := scanEntry(s.DB.QueryRow(ctx, `SELECT `+entryCols+entryFrom+` WHERE e.id = $1`, out[i].EntryID))
		if err != nil {
			return nil, err
		}
		out[i].Entry = &e
	}
	return out, nil
}

func (s *Store) AdminSetPostStatus(ctx context.Context, adminID string, postID uuid.UUID, status string) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var prev string
		err := tx.QueryRow(ctx, `SELECT status FROM posts WHERE id = $1 FOR UPDATE`, postID).Scan(&prev)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE posts SET status = $2 WHERE id = $1`, postID, status); err != nil {
			return err
		}
		if status == "visible" {
			// tiklanganda eski shikoyatlar yopiladi, aks holda keyingi bitta shikoyat yana yashiradi
			if _, err := tx.Exec(ctx, `DELETE FROM post_reports WHERE post_id = $1`, postID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, adminID, "post.status", "post", postID.String(), map[string]string{"status": prev}, map[string]string{"status": status})
	})
}

// ---------- Push eslatmalar ----------

type PushTarget struct {
	UserID   string
	DeviceID string
	Token    string
	Timezone *string
	Lang     string
}

func (s *Store) SetPushToken(ctx context.Context, userID, deviceID string, token, tz *string) error {
	tag, err := s.DB.Exec(ctx, `UPDATE devices SET push_token = $3, timezone = COALESCE($4, timezone), last_seen_at = now()
		WHERE id = $1 AND user_id = $2`, deviceID, userID, token, tz)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PushTargets(ctx context.Context) ([]PushTarget, error) {
	rows, err := s.DB.Query(ctx, `SELECT d.user_id, d.id, d.push_token, d.timezone, u.lang FROM devices d JOIN users u ON u.id = d.user_id
		WHERE d.push_token IS NOT NULL AND d.push_token <> '' AND d.last_seen_at > now() - interval '30 days'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushTarget
	for rows.Next() {
		var t PushTarget
		if err := rows.Scan(&t.UserID, &t.DeviceID, &t.Token, &t.Timezone, &t.Lang); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DiaryReadyDay — foydalanuvchining shu sanadagi kuni: kamida `minVisited` ta borilgan joy bo'lsa va hikoyasi hali yo'q bo'lsa.
func (s *Store) DiaryReadyDay(ctx context.Context, userID string, date time.Time, minVisited int) (tripID uuid.UUID, day int, ok bool, err error) {
	err = s.DB.QueryRow(ctx, `SELECT d.trip_id, d.idx FROM trip_days d JOIN trips t ON t.id = d.trip_id
		WHERE t.user_id = $1 AND d.date = $2
		  AND (SELECT count(*) FROM stops s WHERE s.trip_id = d.trip_id AND s.day_idx = d.idx AND s.status = 'visited') >= $3
		  AND NOT EXISTS (SELECT 1 FROM diary_entries e WHERE e.trip_id = d.trip_id AND e.day_idx = d.idx)
		ORDER BY t.created_at DESC LIMIT 1`, userID, date, minVisited).Scan(&tripID, &day)
	if errors.Is(err, pgx.ErrNoRows) {
		return tripID, 0, false, nil
	}
	return tripID, day, err == nil, err
}

// MarkNotified — bir kunda bir marta: allaqachon yuborilgan bo'lsa false.
func (s *Store) MarkNotified(ctx context.Context, userID, kind string, day time.Time) (bool, error) {
	tag, err := s.DB.Exec(ctx, `INSERT INTO notification_log (user_id, kind, day) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, userID, kind, day)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
