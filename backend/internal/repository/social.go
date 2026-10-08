package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---- discovery candidates ----

type CandidateFilter struct {
	MeID          uuid.UUID
	Type          string
	AgeMin        int
	AgeMax        int
	Country       string // "" = anywhere
	LocationCity  string // optional explicit city filter (lowercase)
	Interests     []string
	Language      string
	ResurfaceDays int
	Limit         int
}

type Candidate struct {
	ID      uuid.UUID
	LikedMe bool
}

// Candidates returns a coarse, index-friendly candidate pool. Fine-grained
// compatibility rules and ranking are applied by the discovery service.
func (r *Repo) Candidates(ctx context.Context, f CandidateFilter) ([]Candidate, error) {
	args := []any{f.MeID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where := []string{
		`u.status='active'`, `u.onboarded_at IS NOT NULL`, `u.id<>$1`, `ps.discoverable`,
		`NOT EXISTS (SELECT 1 FROM swipes s WHERE s.from_user=$1 AND s.to_user=u.id
			AND (s.action='like' OR s.created_at > now() - make_interval(days => ` + arg(f.ResurfaceDays) + `)))`,
		`NOT EXISTS (SELECT 1 FROM blocks b WHERE (b.blocker_id=$1 AND b.blocked_id=u.id) OR (b.blocker_id=u.id AND b.blocked_id=$1))`,
		`NOT EXISTS (SELECT 1 FROM matches m WHERE (m.user_a=$1 AND m.user_b=u.id) OR (m.user_a=u.id AND m.user_b=$1))`,
		`p.birth_date <= current_date - make_interval(years => ` + arg(f.AgeMin) + `)`,
		`p.birth_date > current_date - make_interval(years => ` + arg(f.AgeMax+1) + `)`,
	}
	typeCond := `EXISTS (SELECT 1 FROM user_connection_types a JOIN user_connection_types b ON a.type_slug=b.type_slug
		WHERE a.user_id=$1 AND b.user_id=u.id`
	if f.Type != "" {
		typeCond += ` AND a.type_slug=` + arg(f.Type)
	}
	where = append(where, typeCond+`)`)
	if f.Country != "" {
		where = append(where, `l.country_code=`+arg(f.Country))
	}
	if f.LocationCity != "" {
		where = append(where, `lower(l.city)=`+arg(f.LocationCity))
	}
	if len(f.Interests) > 0 {
		where = append(where, `EXISTS (SELECT 1 FROM user_interests ui JOIN interests i ON i.id=ui.interest_id
			WHERE ui.user_id=u.id AND i.slug = ANY(`+arg(f.Interests)+`))`)
	}
	if f.Language != "" {
		where = append(where, `EXISTS (SELECT 1 FROM user_languages ul WHERE ul.user_id=u.id AND ul.language_code=`+arg(f.Language)+`)`)
	}
	limit := arg(f.Limit)
	sql := `SELECT u.id,
			EXISTS (SELECT 1 FROM swipes s2 WHERE s2.from_user=u.id AND s2.to_user=$1 AND s2.action='like')
		FROM users u
		JOIN profiles p ON p.user_id=u.id
		JOIN locations l ON l.id=p.location_id
		JOIN privacy_settings ps ON ps.user_id=u.id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY u.last_active_at DESC LIMIT ` + limit
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.LikedMe); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- swipes & matches ----

type MatchRow struct {
	ID            uuid.UUID
	UserA, UserB  uuid.UUID
	Types         []string
	CreatedAt     time.Time
	LastMessageAt *time.Time
	UnmatchedAt   *time.Time
}

func (m *MatchRow) Other(me uuid.UUID) uuid.UUID {
	if m.UserA == me {
		return m.UserB
	}
	return m.UserA
}

func (m *MatchRow) Has(u uuid.UUID) bool { return m.UserA == u || m.UserB == u }

func orderPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if strings.Compare(a.String(), b.String()) < 0 {
		return a, b
	}
	return b, a
}

const matchCols = `m.id, m.user_a, m.user_b, m.connection_types, m.created_at, m.last_message_at, m.unmatched_at`

func scanMatch(row pgx.Row) (*MatchRow, error) {
	var m MatchRow
	if err := row.Scan(&m.ID, &m.UserA, &m.UserB, &m.Types, &m.CreatedAt, &m.LastMessageAt, &m.UnmatchedAt); err != nil {
		return nil, NotFound(err)
	}
	return &m, nil
}

// Swipe records an action. For a like that completes a mutual like, it creates
// the match (with the given compatible types) and returns it; otherwise nil.
func (r *Repo) Swipe(ctx context.Context, from, to uuid.UUID, action string, types []string) (*MatchRow, error) {
	var match *MatchRow
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO swipes (from_user, to_user, action) VALUES ($1,$2,$3)
			ON CONFLICT (from_user, to_user) DO UPDATE SET action=EXCLUDED.action, created_at=now()`, from, to, action); err != nil {
			return err
		}
		if action != "like" {
			return nil
		}
		var liked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM swipes WHERE from_user=$1 AND to_user=$2 AND action='like')`, to, from).Scan(&liked); err != nil {
			return err
		}
		if !liked {
			return nil
		}
		a, b := orderPair(from, to)
		m, err := scanMatch(tx.QueryRow(ctx, `
			WITH ins AS (
				INSERT INTO matches (user_a, user_b, connection_types) VALUES ($1,$2,$3)
				ON CONFLICT (user_a, user_b) DO NOTHING RETURNING *)
			SELECT `+matchCols+` FROM ins m
			UNION ALL
			SELECT `+matchCols+` FROM matches m WHERE user_a=$1 AND user_b=$2
			LIMIT 1`, a, b, types))
		if err != nil {
			return err
		}
		match = m
		return nil
	})
	return match, err
}

// AlreadyLiked is used to avoid counting repeat likes toward the daily limit.
func (r *Repo) SwipeAction(ctx context.Context, from, to uuid.UUID) string {
	var a string
	_ = r.pool.QueryRow(ctx, `SELECT action FROM swipes WHERE from_user=$1 AND to_user=$2`, from, to).Scan(&a)
	return a
}

func (r *Repo) GetMatch(ctx context.Context, id uuid.UUID) (*MatchRow, error) {
	return scanMatch(r.pool.QueryRow(ctx, `SELECT `+matchCols+` FROM matches m WHERE m.id=$1`, id))
}

func (r *Repo) GetMatchBetween(ctx context.Context, x, y uuid.UUID) (*MatchRow, error) {
	a, b := orderPair(x, y)
	return scanMatch(r.pool.QueryRow(ctx, `SELECT `+matchCols+` FROM matches m WHERE m.user_a=$1 AND m.user_b=$2`, a, b))
}

func (r *Repo) ListMatches(ctx context.Context, uid uuid.UUID) ([]MatchRow, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+matchCols+` FROM matches m
		JOIN users ou ON ou.id = CASE WHEN m.user_a=$1 THEN m.user_b ELSE m.user_a END AND ou.status='active'
		WHERE (m.user_a=$1 OR m.user_b=$1) AND m.unmatched_at IS NULL
		ORDER BY COALESCE(m.last_message_at, m.created_at) DESC LIMIT 500`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchRow
	for rows.Next() {
		var m MatchRow
		if err := rows.Scan(&m.ID, &m.UserA, &m.UserB, &m.Types, &m.CreatedAt, &m.LastMessageAt, &m.UnmatchedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) Unmatch(ctx context.Context, id, by uuid.UUID) error {
	ct, err := r.pool.Exec(ctx, `UPDATE matches SET unmatched_at=now(), unmatched_by=$2
		WHERE id=$1 AND (user_a=$2 OR user_b=$2) AND unmatched_at IS NULL`, id, by)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// LikesReceived returns users who liked uid and are still undecided (newest first).
func (r *Repo) LikesReceived(ctx context.Context, uid uuid.UUID, limit int) (ids []uuid.UUID, total int, err error) {
	const cond = `FROM swipes s JOIN users u ON u.id=s.from_user AND u.status='active' AND u.onboarded_at IS NOT NULL
		WHERE s.to_user=$1 AND s.action='like'
		AND NOT EXISTS (SELECT 1 FROM swipes s2 WHERE s2.from_user=$1 AND s2.to_user=s.from_user)
		AND NOT EXISTS (SELECT 1 FROM blocks b WHERE (b.blocker_id=$1 AND b.blocked_id=s.from_user) OR (b.blocker_id=s.from_user AND b.blocked_id=$1))`
	if err = r.pool.QueryRow(ctx, `SELECT count(*) `+cond, uid).Scan(&total); err != nil {
		return
	}
	rows, err := r.pool.Query(ctx, `SELECT s.from_user `+cond+` ORDER BY s.created_at DESC LIMIT $2`, uid, limit)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	return
}

// ---- messages ----

type ChatRow struct {
	Match       MatchRow
	LastMessage *models.Message
	Unread      int
}

func (r *Repo) ListChats(ctx context.Context, uid uuid.UUID) ([]ChatRow, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+matchCols+`,
			lm.id, lm.sender_id, lm.body, lm.created_at, lm.read_at,
			(SELECT count(*) FROM messages x WHERE x.match_id=m.id AND x.sender_id<>$1 AND x.read_at IS NULL)
		FROM matches m
		JOIN users ou ON ou.id = CASE WHEN m.user_a=$1 THEN m.user_b ELSE m.user_a END AND ou.status='active'
		LEFT JOIN LATERAL (SELECT * FROM messages WHERE match_id=m.id ORDER BY id DESC LIMIT 1) lm ON true
		WHERE (m.user_a=$1 OR m.user_b=$1) AND m.unmatched_at IS NULL
		ORDER BY COALESCE(m.last_message_at, m.created_at) DESC LIMIT 500`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatRow
	for rows.Next() {
		var c ChatRow
		var mid *int64
		var sender *uuid.UUID
		var body *string
		var at, readAt *time.Time
		m := &c.Match
		if err := rows.Scan(&m.ID, &m.UserA, &m.UserB, &m.Types, &m.CreatedAt, &m.LastMessageAt, &m.UnmatchedAt,
			&mid, &sender, &body, &at, &readAt, &c.Unread); err != nil {
			return nil, err
		}
		if mid != nil {
			c.LastMessage = &models.Message{ID: *mid, MatchID: m.ID, SenderID: *sender, Body: *body, CreatedAt: *at, ReadAt: readAt}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) ListMessages(ctx context.Context, matchID uuid.UUID, afterID, beforeID int64, limit int) ([]models.Message, error) {
	var rows pgx.Rows
	var err error
	if afterID > 0 {
		rows, err = r.pool.Query(ctx, `SELECT id, match_id, sender_id, body, created_at, read_at FROM messages
			WHERE match_id=$1 AND id>$2 ORDER BY id ASC LIMIT $3`, matchID, afterID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT * FROM (SELECT id, match_id, sender_id, body, created_at, read_at FROM messages
			WHERE match_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT $3) t ORDER BY id ASC`, matchID, beforeID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Message{}
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.MatchID, &m.SenderID, &m.Body, &m.CreatedAt, &m.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) InsertMessage(ctx context.Context, matchID, sender uuid.UUID, body string) (*models.Message, error) {
	var m models.Message
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO messages (match_id, sender_id, body) VALUES ($1,$2,$3)
			RETURNING id, match_id, sender_id, body, created_at, read_at`, matchID, sender, body).
			Scan(&m.ID, &m.MatchID, &m.SenderID, &m.Body, &m.CreatedAt, &m.ReadAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE matches SET last_message_at=$2 WHERE id=$1`, matchID, m.CreatedAt)
		return err
	})
	return &m, err
}

func (r *Repo) MarkRead(ctx context.Context, matchID, reader uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE messages SET read_at=now() WHERE match_id=$1 AND sender_id<>$2 AND read_at IS NULL`, matchID, reader)
	return err
}

func (r *Repo) TotalUnread(ctx context.Context, uid uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM messages x JOIN matches m ON m.id=x.match_id AND m.unmatched_at IS NULL
		WHERE (m.user_a=$1 OR m.user_b=$1) AND x.sender_id<>$1 AND x.read_at IS NULL`, uid).Scan(&n)
	return n, err
}

// ---- blocks & reports ----

func (r *Repo) Block(ctx context.Context, blocker, blocked uuid.UUID) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, blocker, blocked); err != nil {
			return err
		}
		a, b := orderPair(blocker, blocked)
		_, err := tx.Exec(ctx, `UPDATE matches SET unmatched_at=now(), unmatched_by=$3 WHERE user_a=$1 AND user_b=$2 AND unmatched_at IS NULL`, a, b, blocker)
		return err
	})
}

func (r *Repo) Unblock(ctx context.Context, blocker, blocked uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`, blocker, blocked)
	return err
}

func (r *Repo) IsBlockedEither(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, a, b).Scan(&ok)
	return ok, err
}

func (r *Repo) ListBlocked(ctx context.Context, uid uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT blocked_id FROM blocks WHERE blocker_id=$1 ORDER BY created_at DESC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repo) CreateReport(ctx context.Context, reporter, reported uuid.UUID, matchID *uuid.UUID, reason, details string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO reports (reporter_id, reported_id, match_id, reason, details) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		reporter, reported, matchID, reason, details).Scan(&id)
	return id, err
}

func (r *Repo) RecentReportCount(ctx context.Context, reporter, reported uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE reporter_id=$1 AND reported_id=$2 AND created_at > now() - interval '24 hours'`, reporter, reported).Scan(&n)
	return n, err
}

// RewindLastPass deletes the user's most recent pass (within 24h) and returns the target id.
func (r *Repo) RewindLastPass(ctx context.Context, uid uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `DELETE FROM swipes WHERE (from_user, to_user) = (
			SELECT from_user, to_user FROM swipes WHERE from_user=$1 AND action='pass' AND created_at > now() - interval '24 hours'
			ORDER BY created_at DESC LIMIT 1) RETURNING to_user`, uid).Scan(&id)
	return id, NotFound(err)
}
