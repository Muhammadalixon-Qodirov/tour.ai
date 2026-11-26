package store

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------- Saqlangan joylar ----------

func (s *Store) SavePlaceFor(ctx context.Context, userID, placeID string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO saved_places (user_id, place_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, placeID)
	return err
}

func (s *Store) UnsavePlace(ctx context.Context, userID, placeID string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM saved_places WHERE user_id = $1 AND place_id = $2`, userID, placeID)
	return err
}

func (s *Store) SavedPlaceIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT sp.place_id FROM saved_places sp JOIN places p ON p.id = sp.place_id
		WHERE sp.user_id = $1 AND p.status = 'published' ORDER BY sp.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---------- Guruh safari ----------

var ErrNotMember = errors.New("not_member")

type Group struct {
	ID         uuid.UUID
	OwnerID    string
	Name       string
	CityID     *string
	InviteCode string
	CreatedAt  time.Time
	Members    []GroupMember
	Votes      []GroupVote // joy bo'yicha jami
	TripIDs    []uuid.UUID
}

type GroupMember struct {
	UserID      string
	Role        string
	DisplayName *string
	Budget      *int
	Interests   []string
	Pace        *string
	JoinedAt    time.Time
}

type GroupVote struct {
	PlaceID string
	Up      int
	Down    int
	MyVote  int
}

// inviteCode — 8 belgili, chalkashmaydigan harflar (0/O, 1/I yo'q).
func inviteCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func (s *Store) CreateGroup(ctx context.Context, ownerID, name string, cityID *string) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var err error
		for range 5 { // taklif kodi to'qnashsa qayta urinish
			_, err = tx.Exec(ctx, `INSERT INTO groups (id, owner_id, name, city_id, invite_code) VALUES ($1,$2,$3,$4,$5)`,
				id, ownerID, name, cityID, inviteCode())
			if err == nil || !strings.Contains(err.Error(), "invite_code") {
				break
			}
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role, interests, budget, pace)
			SELECT $1, u.id, 'owner', u.interests, u.budget, u.pace FROM users u WHERE u.id = $2`, id, ownerID)
		return err
	})
	return id, err
}

// JoinGroup — taklif kodi bilan qo'shilish (a'zo bo'lsa — o'zgarishsiz).
func (s *Store) JoinGroup(ctx context.Context, userID, code string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT id FROM groups WHERE invite_code = upper($1)`, strings.TrimSpace(code)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, ErrNotFound
	}
	if err != nil {
		return id, err
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role, interests, budget, pace)
		SELECT $1, u.id, 'member', u.interests, u.budget, u.pace FROM users u WHERE u.id = $2
		ON CONFLICT (group_id, user_id) DO NOTHING`, id, userID)
	return id, err
}

func (s *Store) IsGroupMember(ctx context.Context, groupID uuid.UUID, userID string) (role string, err error) {
	err = s.DB.QueryRow(ctx, `SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotMember
	}
	return role, err
}

func (s *Store) Group(ctx context.Context, groupID uuid.UUID, viewer string) (Group, error) {
	if _, err := s.IsGroupMember(ctx, groupID, viewer); err != nil {
		return Group{}, err
	}
	var g Group
	err := s.DB.QueryRow(ctx, `SELECT id, owner_id, name, city_id, invite_code, created_at FROM groups WHERE id = $1`, groupID).
		Scan(&g.ID, &g.OwnerID, &g.Name, &g.CityID, &g.InviteCode, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, ErrNotFound
	}
	if err != nil {
		return g, err
	}
	rows, err := s.DB.Query(ctx, `SELECT m.user_id, m.role, u.display_name, m.budget, m.interests, m.pace, m.joined_at
		FROM group_members m JOIN users u ON u.id = m.user_id WHERE m.group_id = $1 ORDER BY m.joined_at`, groupID)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Role, &m.DisplayName, &m.Budget, &m.Interests, &m.Pace, &m.JoinedAt); err != nil {
			rows.Close()
			return g, err
		}
		g.Members = append(g.Members, m)
	}
	rows.Close()
	rows, err = s.DB.Query(ctx, `SELECT place_id, count(*) FILTER (WHERE vote = 1), count(*) FILTER (WHERE vote = -1),
		COALESCE(max(vote) FILTER (WHERE user_id = $2), 0)
		FROM group_votes WHERE group_id = $1 GROUP BY place_id
		ORDER BY count(*) FILTER (WHERE vote = 1) - count(*) FILTER (WHERE vote = -1) DESC, place_id`, groupID, viewer)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var v GroupVote
		if err := rows.Scan(&v.PlaceID, &v.Up, &v.Down, &v.MyVote); err != nil {
			rows.Close()
			return g, err
		}
		g.Votes = append(g.Votes, v)
	}
	rows.Close()
	rows, err = s.DB.Query(ctx, `SELECT id FROM trips WHERE group_id = $1 ORDER BY created_at DESC`, groupID)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return g, err
		}
		g.TripIDs = append(g.TripIDs, id)
	}
	return g, rows.Err()
}

func (s *Store) MyGroups(ctx context.Context, userID string) ([]Group, error) {
	rows, err := s.DB.Query(ctx, `SELECT g.id, g.owner_id, g.name, g.city_id, g.invite_code, g.created_at,
		(SELECT count(*) FROM group_members x WHERE x.group_id = g.id)
		FROM groups g JOIN group_members m ON m.group_id = g.id WHERE m.user_id = $1 ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		var n int
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.CityID, &g.InviteCode, &g.CreatedAt, &n); err != nil {
			return nil, err
		}
		g.Members = make([]GroupMember, n) // faqat soni kerak
		out = append(out, g)
	}
	return out, rows.Err()
}

type MemberPrefs struct {
	Budget    *int
	Interests *[]string
	Pace      *string
}

func (s *Store) SetMemberPrefs(ctx context.Context, groupID uuid.UUID, userID string, p MemberPrefs) error {
	tag, err := s.DB.Exec(ctx, `UPDATE group_members SET budget = COALESCE($3, budget), interests = COALESCE($4, interests),
		pace = COALESCE($5, pace) WHERE group_id = $1 AND user_id = $2`, groupID, userID, p.Budget, p.Interests, p.Pace)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotMember
	}
	return nil
}

// Vote — +1 / -1; 0 — ovozni olib tashlash.
func (s *Store) Vote(ctx context.Context, groupID uuid.UUID, userID, placeID string, vote int) error {
	if _, err := s.IsGroupMember(ctx, groupID, userID); err != nil {
		return err
	}
	if vote == 0 {
		_, err := s.DB.Exec(ctx, `DELETE FROM group_votes WHERE group_id = $1 AND user_id = $2 AND place_id = $3`, groupID, userID, placeID)
		return err
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO group_votes (group_id, user_id, place_id, vote) VALUES ($1,$2,$3,$4)
		ON CONFLICT (group_id, user_id, place_id) DO UPDATE SET vote = EXCLUDED.vote, created_at = now()`, groupID, userID, placeID, vote)
	return err
}

// LeaveGroup — a'zo chiqadi; egasi chiqsa guruh o'chiriladi (marshrutlar qoladi, guruhsiz).
func (s *Store) LeaveGroup(ctx context.Context, groupID uuid.UUID, userID string) error {
	role, err := s.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if role == "owner" {
		_, err = s.DB.Exec(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
		return err
	}
	_, err = s.DB.Exec(ctx, `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	return err
}

func (s *Store) SetTripGroup(ctx context.Context, tripID, groupID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `UPDATE trips SET group_id = $2 WHERE id = $1`, tripID, groupID)
	return err
}
