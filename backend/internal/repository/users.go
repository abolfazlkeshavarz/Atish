package repository

import (
	"context"

	"atish/internal/apperr"
	"atish/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const userCols = `u.id, u.telegram_user_id, COALESCE(u.telegram_username,''), COALESCE(u.telegram_photo_url,''),
	u.atish_username, u.role, u.status, COALESCE(u.status_reason,''), u.telegram_verified, u.phone_verified,
	u.photo_verified, u.identity_verified, COALESCE(u.language_code,''), u.onboarded_at, u.last_active_at, u.created_at`

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.TelegramUserID, &u.TelegramUsername, &u.TelegramPhotoURL, &u.AtishUsername, &u.Role,
		&u.Status, &u.StatusReason, &u.TelegramVerified, &u.PhoneVerified, &u.PhotoVerified, &u.IdentityVerified,
		&u.LanguageCode, &u.OnboardedAt, &u.LastActiveAt, &u.CreatedAt)
	if err != nil {
		return nil, NotFound(err)
	}
	return &u, nil
}

func (r *Repo) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE u.id=$1`, id))
}

func (r *Repo) GetUserByTelegramID(ctx context.Context, tgID int64) (*models.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users u WHERE u.telegram_user_id=$1`, tgID))
}

func (r *Repo) CreateTelegramUser(ctx context.Context, tgID int64, tgUsername, photo, lang string, atishUsername *string, role string) (*models.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO users (telegram_user_id, telegram_username, telegram_photo_url, language_code, atish_username, role, telegram_verified)
			VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), $5, $6, true)
			RETURNING *)
		SELECT `+userCols+` FROM ins u`, tgID, tgUsername, photo, lang, atishUsername, role))
}

func (r *Repo) UpdateTelegramInfo(ctx context.Context, id uuid.UUID, username, photo, lang string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET telegram_username=NULLIF($2,''), telegram_photo_url=NULLIF($3,''),
		language_code=NULLIF($4,''), last_active_at=now(), updated_at=now() WHERE id=$1`, id, username, photo, lang)
	return err
}

func (r *Repo) Touch(ctx context.Context, id uuid.UUID) {
	_, _ = r.pool.Exec(ctx, `UPDATE users SET last_active_at=now() WHERE id=$1`, id)
}

func (r *Repo) UsernameTaken(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(atish_username)=lower($1))`, name).Scan(&ok)
	return ok, err
}

func (r *Repo) SetUsername(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET atish_username=$2, updated_at=now() WHERE id=$1`, id, name)
	if IsUnique(err) {
		return apperr.Conflict("username_taken", "That username is already taken")
	}
	return err
}

func (r *Repo) MarkOnboarded(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET onboarded_at=COALESCE(onboarded_at, now()), updated_at=now() WHERE id=$1`, id)
	return err
}

func (r *Repo) SetStatus(ctx context.Context, id uuid.UUID, status, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET status=$2, status_reason=NULLIF($3,''), updated_at=now() WHERE id=$1 AND status<>'deleted'`, id, status, reason)
	return err
}

func (r *Repo) TelegramIDOf(ctx context.Context, id uuid.UUID) (int64, error) {
	var tg *int64
	if err := r.pool.QueryRow(ctx, `SELECT telegram_user_id FROM users WHERE id=$1 AND status='active'`, id).Scan(&tg); err != nil {
		return 0, NotFound(err)
	}
	if tg == nil {
		return 0, apperr.ErrNotFound
	}
	return *tg, nil
}

// SavePhone stores an encrypted phone number and flips the verified flag.
func (r *Repo) SavePhone(ctx context.Context, uid uuid.UUID, enc []byte, hash string) error {
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO phone_verifications (user_id, phone_enc, phone_hash) VALUES ($1,$2,$3)
			ON CONFLICT (user_id) DO UPDATE SET phone_enc=EXCLUDED.phone_enc, phone_hash=EXCLUDED.phone_hash, verified_at=now()`, uid, enc, hash); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE users SET phone_verified=true, updated_at=now() WHERE id=$1`, uid)
		return err
	})
	if IsUnique(err) {
		return apperr.Conflict("phone_in_use", "This phone number is already linked to another account")
	}
	return err
}

func (r *Repo) DeletePhone(ctx context.Context, uid uuid.UUID) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM phone_verifications WHERE user_id=$1`, uid); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE users SET phone_verified=false, updated_at=now() WHERE id=$1`, uid)
		return err
	})
}

func (r *Repo) NotifyPrefs(ctx context.Context, uid uuid.UUID) (matches, messages bool) {
	matches, messages = true, true
	_ = r.pool.QueryRow(ctx, `SELECT notify_matches, notify_messages FROM privacy_settings WHERE user_id=$1`, uid).Scan(&matches, &messages)
	return
}

// DeleteAccount removes personal data and anonymizes the user row (kept as a
// tombstone so payment records and moderation reports keep referential integrity).
// It returns storage keys of photos so the caller can delete the files.
func (r *Repo) DeleteAccount(ctx context.Context, uid uuid.UUID) ([]string, error) {
	var keys []string
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT storage_key FROM user_photos WHERE user_id=$1`, uid)
		if err != nil {
			return err
		}
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				rows.Close()
				return err
			}
			keys = append(keys, k)
		}
		rows.Close()

		stmts := []string{
			`DELETE FROM user_photos WHERE user_id=$1`,
			`DELETE FROM user_interests WHERE user_id=$1`,
			`DELETE FROM user_languages WHERE user_id=$1`,
			`DELETE FROM user_connection_types WHERE user_id=$1`,
			`DELETE FROM user_friendship_kinds WHERE user_id=$1`,
			`DELETE FROM personality_answers WHERE user_id=$1`,
			`DELETE FROM availability WHERE user_id=$1`,
			`DELETE FROM preferences WHERE user_id=$1`,
			`DELETE FROM privacy_settings WHERE user_id=$1`,
			`DELETE FROM swipes WHERE from_user=$1 OR to_user=$1`,
			`DELETE FROM matches WHERE user_a=$1 OR user_b=$1`, // cascades messages
			`DELETE FROM blocks WHERE blocker_id=$1 OR blocked_id=$1`,
			`DELETE FROM phone_verifications WHERE user_id=$1`,
			`UPDATE subscriptions SET status='canceled', canceled_at=now() WHERE user_id=$1 AND status='active'`,
			`DELETE FROM profiles WHERE user_id=$1`,
			`UPDATE users SET status='deleted', deleted_at=now(), telegram_user_id=NULL, telegram_username=NULL,
				telegram_photo_url=NULL, language_code=NULL, status_reason=NULL, phone_verified=false,
				atish_username='Deleted_'||left(id::text,8), updated_at=now() WHERE id=$1`,
		}
		for _, s := range stmts {
			if _, err := tx.Exec(ctx, s, uid); err != nil {
				return err
			}
		}
		return nil
	})
	return keys, err
}
