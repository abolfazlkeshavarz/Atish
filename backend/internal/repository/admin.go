package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"atish/internal/apperr"
	"atish/internal/models"

	"github.com/google/uuid"
)

// ---- settings ----

func (r *Repo) AllSettings(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (r *Repo) SetSetting(ctx context.Context, key string, value json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO app_settings (key, value) VALUES ($1,$2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, string(value))
	return err
}

// ---- audit ----

func (r *Repo) Audit(ctx context.Context, actor, action, targetType, targetID string, details any) {
	b, _ := json.Marshal(details)
	_, _ = r.pool.Exec(ctx, `INSERT INTO audit_logs (actor, action, target_type, target_id, details) VALUES ($1,$2,$3,$4,$5)`,
		actor, action, targetType, targetID, string(b))
}

// jsonRows runs a query and returns its rows as one JSON array (admin screens render it as-is).
func (r *Repo) jsonRows(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	var out []byte
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(json_agg(t), '[]'::json) FROM (`+query+`) t`, args...).Scan(&out)
	return out, err
}

func (r *Repo) jsonRow(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	var out []byte
	err := r.pool.QueryRow(ctx, `SELECT row_to_json(t) FROM (`+query+`) t`, args...).Scan(&out)
	return out, NotFound(err)
}

func (r *Repo) AuditLog(ctx context.Context, limit, offset int) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT id, actor, action, target_type, target_id, details, created_at FROM audit_logs ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
}

// ---- stats ----

func (r *Repo) Stats(ctx context.Context) (json.RawMessage, error) {
	return r.jsonRow(ctx, `SELECT
		(SELECT count(*) FROM users WHERE status<>'deleted') AS users_total,
		(SELECT count(*) FROM users WHERE onboarded_at IS NOT NULL AND status='active') AS users_onboarded,
		(SELECT count(*) FROM users WHERE last_active_at > now() - interval '24 hours') AS active_24h,
		(SELECT count(*) FROM users WHERE last_active_at > now() - interval '7 days') AS active_7d,
		(SELECT count(*) FROM users WHERE created_at > now() - interval '24 hours') AS new_24h,
		(SELECT count(*) FROM users WHERE status IN ('suspended','banned')) AS restricted,
		(SELECT count(*) FROM matches) AS matches_total,
		(SELECT count(*) FROM matches WHERE created_at > now() - interval '24 hours') AS matches_24h,
		(SELECT count(*) FROM messages) AS messages_total,
		(SELECT count(*) FROM messages WHERE created_at > now() - interval '24 hours') AS messages_24h,
		(SELECT count(*) FROM swipes WHERE created_at > now() - interval '24 hours') AS swipes_24h,
		(SELECT count(*) FROM reports WHERE status IN ('open','reviewing')) AS reports_open,
		(SELECT count(DISTINCT user_id) FROM subscriptions WHERE status='active' AND (current_period_end IS NULL OR current_period_end>now())) AS premium_users,
		(SELECT COALESCE(sum(amount_cents),0) FROM payments WHERE status='paid' AND currency<>'XTR' AND paid_at > now() - interval '30 days') AS revenue_30d_cents,
		(SELECT COALESCE(sum(amount_cents),0) FROM payments WHERE status='paid' AND currency='XTR' AND paid_at > now() - interval '30 days') AS stars_30d,
		(SELECT COALESCE(json_agg(d ORDER BY d.day), '[]'::json) FROM (
			SELECT g::date AS day, (SELECT count(*) FROM users u WHERE u.created_at::date = g::date) AS signups,
				(SELECT count(*) FROM matches m WHERE m.created_at::date = g::date) AS matches
			FROM generate_series(current_date - 13, current_date, interval '1 day') g) d) AS daily`)
}

// ---- users ----

type UserQuery struct {
	Q       string
	Status  string
	Role    string
	Premium string // "", "yes", "no"
	Limit   int
	Offset  int
}

func (r *Repo) AdminUsers(ctx context.Context, q UserQuery) (json.RawMessage, int, error) {
	args := []any{}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	where := []string{"1=1"}
	if s := strings.TrimSpace(q.Q); s != "" {
		p := arg("%" + strings.ToLower(s) + "%")
		e := arg(s)
		where = append(where, fmt.Sprintf(`(lower(u.atish_username) LIKE %s OR lower(p.display_name) LIKE %s OR lower(u.telegram_username) LIKE %s OR u.telegram_user_id::text=%s OR u.id::text=%s)`, p, p, p, e, e))
	}
	if q.Status != "" {
		where = append(where, "u.status="+arg(q.Status))
	}
	if q.Role != "" {
		where = append(where, "u.role="+arg(q.Role))
	}
	prem := `EXISTS (SELECT 1 FROM subscriptions s WHERE s.user_id=u.id AND s.status='active' AND (s.current_period_end IS NULL OR s.current_period_end>now()))`
	switch q.Premium {
	case "yes":
		where = append(where, prem)
	case "no":
		where = append(where, "NOT "+prem)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users u LEFT JOIN profiles p ON p.user_id=u.id WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	lim, off := arg(q.Limit), arg(q.Offset)
	rows, err := r.jsonRows(ctx, `SELECT u.id, u.atish_username, u.telegram_user_id, u.telegram_username, u.role, u.status, u.status_reason,
			u.telegram_verified, u.phone_verified, u.photo_verified, u.identity_verified, u.onboarded_at, u.last_active_at, u.created_at,
			p.display_name, l.city, l.country_code, `+prem+` AS premium,
			(SELECT count(*) FROM reports rp WHERE rp.reported_id=u.id AND rp.status IN ('open','reviewing')) AS open_reports,
			(SELECT id FROM user_photos ph WHERE ph.user_id=u.id AND ph.status='active' ORDER BY position LIMIT 1) AS photo_id
		FROM users u LEFT JOIN profiles p ON p.user_id=u.id LEFT JOIN locations l ON l.id=p.location_id
		WHERE `+w+` ORDER BY u.created_at DESC LIMIT `+lim+` OFFSET `+off, args...)
	return rows, total, err
}

func (r *Repo) AdminUserExtras(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	return r.jsonRow(ctx, `SELECT
		(SELECT count(*) FROM matches WHERE (user_a=$1 OR user_b=$1) AND unmatched_at IS NULL) AS matches,
		(SELECT count(*) FROM swipes WHERE from_user=$1 AND action='like') AS likes_sent,
		(SELECT count(*) FROM swipes WHERE to_user=$1 AND action='like') AS likes_received,
		(SELECT count(*) FROM messages WHERE sender_id=$1) AS messages_sent,
		(SELECT count(*) FROM blocks WHERE blocked_id=$1) AS blocked_by,
		(SELECT count(*) FROM reports WHERE reported_id=$1) AS reports_against,
		(SELECT count(*) FROM reports WHERE reporter_id=$1) AS reports_filed,
		(SELECT COALESCE(json_agg(m ORDER BY m.created_at DESC), '[]'::json) FROM (
			SELECT id, CASE WHEN user_a=$1 THEN user_b ELSE user_a END AS other_id, connection_types, created_at, unmatched_at FROM matches
			WHERE user_a=$1 OR user_b=$1 ORDER BY created_at DESC LIMIT 30) m) AS recent_matches`, id)
}

func (r *Repo) AdminSetUser(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	allowed := map[string]bool{"status": true, "status_reason": true, "role": true, "telegram_verified": true, "phone_verified": true, "photo_verified": true, "identity_verified": true}
	sets, args := []string{}, []any{id}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", k, len(args)))
	}
	if len(sets) == 0 {
		return apperr.BadRequest("no_fields", "nothing to update")
	}
	_, err := r.pool.Exec(ctx, `UPDATE users SET `+strings.Join(sets, ",")+`, updated_at=now() WHERE id=$1 AND status<>'deleted'`, args...)
	return err
}

func (r *Repo) AdminEditProfileText(ctx context.Context, id uuid.UUID, name, bio *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE profiles SET display_name=COALESCE($2,display_name), bio=COALESCE($3,bio), updated_at=now() WHERE user_id=$1`, id, name, bio)
	return err
}

func (r *Repo) AdminRecentPhotos(ctx context.Context, limit, offset int) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT ph.id, ph.user_id, u.atish_username, ph.created_at FROM user_photos ph JOIN users u ON u.id=ph.user_id
		WHERE ph.status='active' ORDER BY ph.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
}

// ---- reports ----

func (r *Repo) AdminReports(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT rp.id, rp.reason, rp.details, rp.status, rp.resolution, rp.resolved_by, rp.resolved_at, rp.created_at, rp.match_id,
			rp.reporter_id, ru.atish_username AS reporter_username, rp.reported_id, tu.atish_username AS reported_username, tu.status AS reported_status,
			(SELECT count(*) FROM reports x WHERE x.reported_id=rp.reported_id) AS reported_total_reports
		FROM reports rp JOIN users ru ON ru.id=rp.reporter_id JOIN users tu ON tu.id=rp.reported_id
		WHERE ($1::text='' OR rp.status=$1) ORDER BY (rp.status IN ('open','reviewing')) DESC, rp.created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
}

func (r *Repo) AdminReport(ctx context.Context, id int64) (json.RawMessage, error) {
	return r.jsonRow(ctx, `SELECT rp.*, ru.atish_username AS reporter_username, tu.atish_username AS reported_username, tu.status AS reported_status
		FROM reports rp JOIN users ru ON ru.id=rp.reporter_id JOIN users tu ON tu.id=rp.reported_id WHERE rp.id=$1`, id)
}

func (r *Repo) ReportedUser(ctx context.Context, id int64) (uuid.UUID, error) {
	var u uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT reported_id FROM reports WHERE id=$1`, id).Scan(&u)
	return u, NotFound(err)
}

func (r *Repo) ResolveReport(ctx context.Context, id int64, status, resolution, by string) error {
	ct, err := r.pool.Exec(ctx, `UPDATE reports SET status=$2, resolution=NULLIF($3,''), resolved_by=$4,
		resolved_at=CASE WHEN $2 IN ('resolved','dismissed') THEN now() ELSE NULL END WHERE id=$1`, id, status, resolution, by)
	if err == nil && ct.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return err
}

func (r *Repo) AdminConversation(ctx context.Context, matchID uuid.UUID) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT m.id, m.sender_id, u.atish_username AS sender_username, m.body, m.created_at
		FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.match_id=$1 ORDER BY m.id ASC LIMIT 500`, matchID)
}

// ---- billing views ----

func (r *Repo) AdminPayments(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT pay.id, pay.user_id, u.atish_username, pl.code AS plan, pay.provider, pay.provider_ref, pay.amount_cents, pay.currency, pay.status, pay.created_at, pay.paid_at
		FROM payments pay JOIN users u ON u.id=pay.user_id JOIN plans pl ON pl.id=pay.plan_id
		WHERE ($1::text='' OR pay.status=$1) ORDER BY pay.created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
}

func (r *Repo) AdminSubscriptions(ctx context.Context, status string, limit, offset int) (json.RawMessage, error) {
	return r.jsonRows(ctx, `SELECT s.id, s.user_id, u.atish_username, pl.code AS plan, s.provider, s.status, s.started_at, s.current_period_end, s.canceled_at
		FROM subscriptions s JOIN users u ON u.id=s.user_id JOIN plans pl ON pl.id=s.plan_id
		WHERE ($1::text='' OR s.status=$1) ORDER BY s.created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
}

func (r *Repo) PlanIDByCode(ctx context.Context, code string) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `SELECT id FROM plans WHERE code=$1`, code).Scan(&id)
	return id, NotFound(err)
}

// BroadcastTargets lists Telegram ids of active, onboarded users.
func (r *Repo) BroadcastTargets(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT telegram_user_id FROM users WHERE status='active' AND telegram_user_id IS NOT NULL AND onboarded_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---- generic reference-data CRUD (whitelisted tables/columns only) ----

type Col struct{ Name, Type string } // Type: text | int | bool | jsonb

type CatalogSpec struct {
	Table  string
	PK     string
	PKType string // text | int
	Auto   bool   // pk generated by the database
	Cols   []Col
	Order  string
	Search string // column used for ?q= search
}

var CatalogSpecs = map[string]CatalogSpec{
	"interests": {Table: "interests", PK: "id", PKType: "int", Auto: true, Order: "category, position, name", Search: "name",
		Cols: []Col{{"slug", "text"}, {"category", "text"}, {"name", "text"}, {"emoji", "text"}, {"position", "int"}, {"is_active", "bool"}}},
	"languages": {Table: "languages", PK: "code", PKType: "text", Order: "name", Search: "name",
		Cols: []Col{{"name", "text"}, {"native_name", "text"}, {"is_active", "bool"}}},
	"connection-types": {Table: "connection_types", PK: "slug", PKType: "text", Order: "position", Search: "name",
		Cols: []Col{{"name", "text"}, {"emoji", "text"}, {"position", "int"}, {"is_active", "bool"}}},
	"friendship-kinds": {Table: "friendship_kinds", PK: "slug", PKType: "text", Order: "position", Search: "name",
		Cols: []Col{{"name", "text"}, {"emoji", "text"}, {"position", "int"}, {"is_active", "bool"}}},
	"personality-questions": {Table: "personality_questions", PK: "id", PKType: "int", Auto: true, Order: "position, id", Search: "text",
		Cols: []Col{{"key", "text"}, {"text", "text"}, {"options", "jsonb"}, {"position", "int"}, {"is_active", "bool"}}},
	"plans": {Table: "plans", PK: "id", PKType: "int", Auto: true, Order: "position, id", Search: "name",
		Cols: []Col{{"code", "text"}, {"name", "text"}, {"description", "text"}, {"interval", "text"}, {"price_cents", "int"}, {"currency", "text"},
			{"stars_price", "int"}, {"stripe_price_id", "text"}, {"features", "jsonb"}, {"position", "int"}, {"is_active", "bool"}}},
	"locations": {Table: "locations", PK: "id", PKType: "int", Auto: true, Order: "country_code, city, area", Search: "city",
		Cols: []Col{{"country_code", "text"}, {"country", "text"}, {"region", "text"}, {"city", "text"}, {"area", "text"}, {"is_approved", "bool"}}},
}

func coerce(typ string, v any) (any, string, error) {
	cast := ""
	if v == nil {
		return nil, cast, nil
	}
	switch typ {
	case "text":
		s, ok := v.(string)
		if !ok {
			return nil, "", fmt.Errorf("expected string")
		}
		return s, cast, nil
	case "int":
		switch n := v.(type) {
		case float64:
			return int64(n), cast, nil
		case string:
			i, err := strconv.ParseInt(n, 10, 64)
			return i, cast, err
		}
		return nil, "", fmt.Errorf("expected number")
	case "bool":
		b, ok := v.(bool)
		if !ok {
			return nil, "", fmt.Errorf("expected boolean")
		}
		return b, cast, nil
	case "jsonb":
		b, err := json.Marshal(v)
		return string(b), "::jsonb", err
	}
	return nil, "", fmt.Errorf("unsupported type")
}

func (s CatalogSpec) pk(id string) (any, error) {
	if s.PKType == "int" {
		return strconv.ParseInt(id, 10, 64)
	}
	return id, nil
}

func (r *Repo) CatalogList(ctx context.Context, s CatalogSpec, q string, limit, offset int) (json.RawMessage, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE ($1::text='' OR %s::text ILIKE '%%'||$1||'%%')`, s.Table, s.Search), q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.jsonRows(ctx, fmt.Sprintf(`SELECT * FROM %s WHERE ($1::text='' OR %s::text ILIKE '%%'||$1||'%%') ORDER BY %s LIMIT $2 OFFSET $3`, s.Table, s.Search, s.Order), q, limit, offset)
	return rows, total, err
}

func (r *Repo) CatalogCreate(ctx context.Context, s CatalogSpec, body map[string]any) (json.RawMessage, error) {
	cols, ph, args := []string{}, []string{}, []any{}
	add := func(name, typ string, v any) error {
		cv, cast, err := coerce(typ, v)
		if err != nil {
			return apperr.BadRequest("invalid_field", name+": "+err.Error())
		}
		args = append(args, cv)
		cols = append(cols, name)
		ph = append(ph, fmt.Sprintf("$%d%s", len(args), cast))
		return nil
	}
	if !s.Auto {
		if v, ok := body[s.PK]; ok {
			if err := add(s.PK, s.PKType, v); err != nil {
				return nil, err
			}
		} else {
			return nil, apperr.BadRequest("missing_field", s.PK+" is required")
		}
	}
	for _, c := range s.Cols {
		if v, ok := body[c.Name]; ok {
			if err := add(c.Name, c.Type, v); err != nil {
				return nil, err
			}
		}
	}
	out, err := r.rowJSON(ctx, fmt.Sprintf(`WITH ins AS (INSERT INTO %s (%s) VALUES (%s) RETURNING *) SELECT row_to_json(ins) FROM ins`, s.Table, strings.Join(cols, ","), strings.Join(ph, ",")), args...)
	if IsUnique(err) {
		return nil, apperr.Conflict("duplicate", "A record with the same unique value already exists")
	}
	if err != nil && strings.Contains(err.Error(), "violates") {
		return nil, apperr.BadRequest("invalid_value", "Invalid value: "+err.Error())
	}
	return out, err
}

func (r *Repo) CatalogUpdate(ctx context.Context, s CatalogSpec, id string, body map[string]any) (json.RawMessage, error) {
	pk, err := s.pk(id)
	if err != nil {
		return nil, apperr.BadRequest("invalid_id", "invalid id")
	}
	sets, args := []string{}, []any{pk}
	for _, c := range s.Cols {
		v, ok := body[c.Name]
		if !ok {
			continue
		}
		cv, cast, err := coerce(c.Type, v)
		if err != nil {
			return nil, apperr.BadRequest("invalid_field", c.Name+": "+err.Error())
		}
		args = append(args, cv)
		sets = append(sets, fmt.Sprintf("%s=$%d%s", c.Name, len(args), cast))
	}
	if len(sets) == 0 {
		return nil, apperr.BadRequest("no_fields", "nothing to update")
	}
	out, err := r.rowJSON(ctx, fmt.Sprintf(`WITH upd AS (UPDATE %s SET %s WHERE %s=$1 RETURNING *) SELECT row_to_json(upd) FROM upd`, s.Table, strings.Join(sets, ","), s.PK), args...)
	if IsUnique(err) {
		return nil, apperr.Conflict("duplicate", "A record with the same unique value already exists")
	}
	if err != nil && strings.Contains(err.Error(), "violates") {
		return nil, apperr.BadRequest("invalid_value", "Invalid value: "+err.Error())
	}
	return out, err
}

func (r *Repo) CatalogDelete(ctx context.Context, s CatalogSpec, id string) error {
	pk, err := s.pk(id)
	if err != nil {
		return apperr.BadRequest("invalid_id", "invalid id")
	}
	ct, err := r.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s=$1`, s.Table, s.PK), pk)
	if IsFK(err) {
		return apperr.Conflict("in_use", "This item is in use. Deactivate it instead of deleting.")
	}
	if err == nil && ct.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return err
}

// rowJSON runs a statement that already ends in SELECT row_to_json(...) (needed for data-modifying CTEs).
func (r *Repo) rowJSON(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	var out []byte
	err := r.pool.QueryRow(ctx, query, args...).Scan(&out)
	return out, NotFound(err)
}

// FindUser resolves an operator-supplied reference: user id, Atish username
// (with or without the Atish_ prefix), Telegram numeric id or Telegram username.
func (r *Repo) FindUser(ctx context.Context, ref string) (*models.User, error) {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "@"))
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u
		WHERE u.id::text=$1 OR u.telegram_user_id::text=$1 OR lower(u.atish_username)=lower($1)
		   OR lower(u.atish_username)=lower('Atish_'||$1) OR lower(u.telegram_username)=lower($1)
		ORDER BY u.created_at LIMIT 1`, ref))
}

// PromoteTelegram gives a Telegram user a staff role, creating a placeholder
// account if they have not opened the app yet (it is completed at first login).
func (r *Repo) PromoteTelegram(ctx context.Context, tgID int64, role string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `INSERT INTO users (telegram_user_id, role, telegram_verified) VALUES ($1,$2,true)
		ON CONFLICT (telegram_user_id) DO UPDATE SET role=EXCLUDED.role, updated_at=now() RETURNING id`, tgID, role).Scan(&id)
	return id, err
}
