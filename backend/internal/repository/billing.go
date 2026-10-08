package repository

import (
	"context"
	"encoding/json"
	"time"

	"atish/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const planCols = `id, code, name, description, interval, price_cents, currency, stars_price, stripe_price_id, features, position, is_active`

func scanPlan(row pgx.Row) (*models.Plan, error) {
	var p models.Plan
	var feat []byte
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.Interval, &p.PriceCents, &p.Currency,
		&p.StarsPrice, &p.StripePrice, &feat, &p.Position, &p.IsActive); err != nil {
		return nil, NotFound(err)
	}
	_ = json.Unmarshal(feat, &p.Features)
	if p.Features == nil {
		p.Features = map[string]any{}
	}
	return &p, nil
}

func (r *Repo) ListPlans(ctx context.Context, onlyActive bool) ([]models.Plan, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+planCols+` FROM plans WHERE ($1::boolean = false OR is_active) ORDER BY position, id`, onlyActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Plan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *Repo) GetPlanByCode(ctx context.Context, code string) (*models.Plan, error) {
	return scanPlan(r.pool.QueryRow(ctx, `SELECT `+planCols+` FROM plans WHERE code=$1`, code))
}

type PaymentRow struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	PlanID      int
	Provider    string
	ProviderRef string
	AmountCents int
	Currency    string
	Status      string
}

func (r *Repo) CreatePayment(ctx context.Context, uid uuid.UUID, planID int, provider string, amount int, currency string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `INSERT INTO payments (user_id, plan_id, provider, amount_cents, currency) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		uid, planID, provider, amount, currency).Scan(&id)
	return id, err
}

func (r *Repo) GetPayment(ctx context.Context, id uuid.UUID) (*PaymentRow, error) {
	var p PaymentRow
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, plan_id, provider, COALESCE(provider_ref,''), amount_cents, currency, status FROM payments WHERE id=$1`, id).
		Scan(&p.ID, &p.UserID, &p.PlanID, &p.Provider, &p.ProviderRef, &p.AmountCents, &p.Currency, &p.Status)
	return &p, NotFound(err)
}

func (r *Repo) SetPaymentRef(ctx context.Context, id uuid.UUID, ref string) error {
	_, err := r.pool.Exec(ctx, `UPDATE payments SET provider_ref=$2 WHERE id=$1 AND status='pending'`, id, ref)
	return err
}

func (r *Repo) FailPayment(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE payments SET status='failed' WHERE id=$1 AND status='pending'`, id)
	return err
}

// ActivatePayment marks a pending payment as paid (idempotent) and creates or
// extends the user's subscription. Returns false when it was already processed.
// explicitEnd overrides the computed period end (used when the provider reports one).
func (r *Repo) ActivatePayment(ctx context.Context, paymentID uuid.UUID, providerRef string, explicitEnd *time.Time, raw []byte) (bool, error) {
	activated := false
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		var uid uuid.UUID
		var planID int
		var provider string
		err := tx.QueryRow(ctx, `UPDATE payments SET status='paid', paid_at=now(), provider_ref=COALESCE(NULLIF($2,''), provider_ref), raw=$3
			WHERE id=$1 AND status='pending' RETURNING user_id, plan_id, provider`, paymentID, providerRef, nullJSON(raw)).Scan(&uid, &planID, &provider)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		activated = true

		var interval string
		if err := tx.QueryRow(ctx, `SELECT interval FROM plans WHERE id=$1`, planID).Scan(&interval); err != nil {
			return err
		}
		var subID uuid.UUID
		var curEnd *time.Time
		err = tx.QueryRow(ctx, `SELECT id, current_period_end FROM subscriptions WHERE user_id=$1 AND status='active'
			AND (current_period_end IS NULL OR current_period_end>now()) ORDER BY current_period_end DESC NULLS FIRST LIMIT 1`, uid).Scan(&subID, &curEnd)
		hasSub := err == nil
		if err != nil && err != pgx.ErrNoRows {
			return err
		}

		var newEnd *time.Time
		switch {
		case interval == "lifetime":
			newEnd = nil
		case explicitEnd != nil:
			newEnd = explicitEnd
		default:
			base := time.Now()
			if hasSub && curEnd != nil && curEnd.After(base) {
				base = *curEnd
			}
			e := base.AddDate(0, 1, 0)
			if interval == "year" {
				e = base.AddDate(1, 0, 0)
			}
			newEnd = &e
		}
		if hasSub {
			_, err = tx.Exec(ctx, `UPDATE subscriptions SET plan_id=$2, provider=$3, provider_ref=NULLIF($4,''), current_period_end=$5 WHERE id=$1`,
				subID, planID, provider, providerRef, newEnd)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO subscriptions (user_id, plan_id, provider, provider_ref, current_period_end) VALUES ($1,$2,$3,NULLIF($4,''),$5)`,
				uid, planID, provider, providerRef, newEnd)
		}
		return err
	})
	return activated, err
}

// RenewByRef extends the subscription identified by (provider, ref) after a
// recurring charge and records the charge as a payment (idempotent on chargeRef).
func (r *Repo) RenewByRef(ctx context.Context, provider, ref, chargeRef string, amount int, currency string, end time.Time, raw []byte) (bool, error) {
	renewed := false
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		var uid uuid.UUID
		var planID int
		err := tx.QueryRow(ctx, `UPDATE subscriptions SET current_period_end=$3, status='active', canceled_at=NULL
			WHERE provider=$1 AND provider_ref=$2 RETURNING user_id, plan_id`, provider, ref, end).Scan(&uid, &planID)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		ct, err := tx.Exec(ctx, `INSERT INTO payments (user_id, plan_id, provider, provider_ref, amount_cents, currency, status, raw, paid_at)
			VALUES ($1,$2,$3,$4,$5,$6,'paid',$7,now()) ON CONFLICT DO NOTHING`, uid, planID, provider, chargeRef, amount, currency, nullJSON(raw))
		if err != nil {
			return err
		}
		renewed = ct.RowsAffected() > 0
		return nil
	})
	return renewed, err
}

func (r *Repo) CancelByRef(ctx context.Context, provider, ref string) error {
	_, err := r.pool.Exec(ctx, `UPDATE subscriptions SET status='canceled', canceled_at=now() WHERE provider=$1 AND provider_ref=$2 AND status='active'`, provider, ref)
	return err
}

// GrantPremium creates an admin-granted subscription. days<=0 means lifetime.
func (r *Repo) GrantPremium(ctx context.Context, uid uuid.UUID, planID, days int) error {
	var end *time.Time
	if days > 0 {
		e := time.Now().AddDate(0, 0, days)
		end = &e
	}
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE subscriptions SET status='canceled', canceled_at=now() WHERE user_id=$1 AND status='active'`, uid); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO subscriptions (user_id, plan_id, provider, current_period_end) VALUES ($1,$2,'admin',$3)`, uid, planID, end)
		return err
	})
}

func (r *Repo) RevokePremium(ctx context.Context, uid uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE subscriptions SET status='canceled', canceled_at=now() WHERE user_id=$1 AND status='active'`, uid)
	return err
}

func (r *Repo) ExpireSubscriptions(ctx context.Context) (int64, error) {
	ct, err := r.pool.Exec(ctx, `UPDATE subscriptions SET status='expired' WHERE status='active' AND current_period_end IS NOT NULL AND current_period_end<now()`)
	return ct.RowsAffected(), err
}

// ActiveSubscription returns the user's current subscription, if any.
func (r *Repo) ActiveSubscription(ctx context.Context, uid uuid.UUID) (*models.Subscription, error) {
	var s models.Subscription
	var feat []byte
	var ref *string
	err := r.pool.QueryRow(ctx, `SELECT s.id, s.user_id, s.plan_id, p.code, s.provider, s.provider_ref, s.status, s.started_at, s.current_period_end, p.features
		FROM subscriptions s JOIN plans p ON p.id=s.plan_id
		WHERE s.user_id=$1 AND s.status='active' AND (s.current_period_end IS NULL OR s.current_period_end>now())
		ORDER BY s.current_period_end DESC NULLS FIRST LIMIT 1`, uid).
		Scan(&s.ID, &s.UserID, &s.PlanID, &s.PlanCode, &s.Provider, &ref, &s.Status, &s.StartedAt, &s.CurrentPeriodEnd, &feat)
	if err != nil {
		return nil, NotFound(err)
	}
	if ref != nil {
		s.ProviderRef = *ref
	}
	_ = json.Unmarshal(feat, &s.Features)
	return &s, nil
}

func nullJSON(b []byte) any {
	if len(b) == 0 || !json.Valid(b) {
		return nil
	}
	return string(b)
}
