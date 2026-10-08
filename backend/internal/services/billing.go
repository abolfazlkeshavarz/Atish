package services

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/models"
	"atish/internal/payments"
	"atish/internal/repository"
	"atish/internal/telegram"

	"github.com/google/uuid"
)

type BillingService struct {
	cfg       *config.Config
	repo      *repository.Repo
	settings  *Settings
	providers map[string]payments.Provider
}

func NewBilling(cfg *config.Config, repo *repository.Repo, settings *Settings, providers ...payments.Provider) *BillingService {
	m := map[string]payments.Provider{}
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &BillingService{cfg: cfg, repo: repo, settings: settings, providers: m}
}

type PlanView struct {
	models.Plan
	Providers []string `json:"providers"`
}

func (b *BillingService) Plans(ctx context.Context) ([]PlanView, error) {
	if !b.settings.Bool(ctx, "premium_enabled", true) {
		return []PlanView{}, nil
	}
	plans, err := b.repo.ListPlans(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]PlanView, 0, len(plans))
	for _, p := range plans {
		pv := PlanView{Plan: p, Providers: []string{}}
		for name, pr := range b.providers {
			if pr.Supports(p) {
				pv.Providers = append(pv.Providers, name)
			}
		}
		if len(pv.Providers) > 0 {
			out = append(out, pv)
		}
	}
	return out, nil
}

var allFeatures = []string{"unlimited_likes", "see_likes", "rewind", "badge"}

// Entitlements resolves what a user may do. When premium is globally
// disabled (free launch) every feature is unlocked for everyone.
func (b *BillingService) Entitlements(ctx context.Context, uid uuid.UUID) models.Entitlements {
	e := models.Entitlements{Features: map[string]bool{}}
	if !b.settings.Bool(ctx, "premium_enabled", true) {
		for _, f := range allFeatures {
			e.Features[f] = true
		}
		return e
	}
	sub, err := b.repo.ActiveSubscription(ctx, uid)
	if err != nil {
		return e
	}
	e.Premium, e.PlanCode, e.Until = true, sub.PlanCode, sub.CurrentPeriodEnd
	for k, v := range sub.Features {
		if on, ok := v.(bool); ok && on {
			e.Features[k] = true
		}
	}
	return e
}

func (b *BillingService) Has(ctx context.Context, uid uuid.UUID, feature string) bool {
	return b.Entitlements(ctx, uid).Features[feature]
}

type CheckoutResult struct {
	PaymentID uuid.UUID `json:"payment_id"`
	payments.CheckoutOutput
	Provider string `json:"provider"`
}

func (b *BillingService) Checkout(ctx context.Context, uid uuid.UUID, planCode, provider string) (*CheckoutResult, error) {
	if !b.settings.Bool(ctx, "premium_enabled", true) {
		return nil, apperr.BadRequest("premium_disabled", "Premium is not available")
	}
	plan, err := b.repo.GetPlanByCode(ctx, planCode)
	if err != nil || !plan.IsActive {
		return nil, apperr.NotFound("plan not found")
	}
	pr, ok := b.providers[provider]
	if !ok || !pr.Supports(*plan) {
		return nil, apperr.BadRequest("provider_unavailable", "That payment method is not available for this plan")
	}
	amount, currency := plan.PriceCents, plan.Currency
	if provider == "telegram_stars" && plan.StarsPrice != nil {
		amount, currency = *plan.StarsPrice, "XTR"
	}
	pid, err := b.repo.CreatePayment(ctx, uid, plan.ID, provider, amount, currency)
	if err != nil {
		return nil, err
	}
	out, err := pr.Checkout(ctx, payments.CheckoutInput{PaymentID: pid, UserID: uid, Plan: *plan, ReturnURL: b.cfg.MiniAppURL})
	if err != nil {
		_ = b.repo.FailPayment(ctx, pid)
		log.Printf("checkout %s failed: %v", provider, err)
		return nil, apperr.New(502, "checkout_failed", "Could not start checkout. Please try again.")
	}
	if out.ProviderRef != "" {
		_ = b.repo.SetPaymentRef(ctx, pid, out.ProviderRef)
	}
	return &CheckoutResult{PaymentID: pid, CheckoutOutput: *out, Provider: provider}, nil
}

// ---- Telegram Stars (driven by bot updates) ----

func (b *BillingService) ValidatePreCheckout(ctx context.Context, q *telegram.PreCheckoutQuery) (bool, string) {
	pid, err := uuid.Parse(q.InvoicePayload)
	if err != nil {
		return false, "Invalid payment"
	}
	p, err := b.repo.GetPayment(ctx, pid)
	if err != nil || p.Provider != "telegram_stars" || q.Currency != "XTR" || q.TotalAmount != p.AmountCents {
		return false, "Invalid payment"
	}
	if p.Status != "pending" && p.Status != "paid" { // "paid" = recurring renewal
		return false, "This payment is no longer valid"
	}
	return true, ""
}

func (b *BillingService) HandleStarsPayment(ctx context.Context, sp *telegram.SuccessfulPayment) error {
	pid, err := uuid.Parse(sp.InvoicePayload)
	if err != nil {
		return err
	}
	var end *time.Time
	if sp.SubscriptionExpirationDate > 0 {
		t := time.Unix(sp.SubscriptionExpirationDate, 0)
		end = &t
	}
	raw, _ := json.Marshal(sp)
	activated, err := b.repo.ActivatePayment(ctx, pid, sp.TelegramPaymentChargeID, end, raw)
	if err != nil || activated {
		return err
	}
	// Already paid: this is a recurring renewal of an existing Stars subscription.
	if sp.IsRecurring && !sp.IsFirstRecurring && end != nil {
		orig, err := b.repo.GetPayment(ctx, pid)
		if err != nil {
			return err
		}
		_, err = b.repo.RenewByRef(ctx, "telegram_stars", orig.ProviderRef, sp.TelegramPaymentChargeID, sp.TotalAmount, sp.Currency, *end, raw)
		return err
	}
	return nil
}

// ---- Stripe webhooks ----

type stripeEvent struct {
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

func (b *BillingService) HandleStripeWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if err := payments.VerifyStripeSignature(payload, sigHeader, b.cfg.StripeWebhookSecret, 5*time.Minute); err != nil {
		return apperr.BadRequest("invalid_signature", err.Error())
	}
	var ev stripeEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return apperr.BadRequest("invalid_payload", "bad json")
	}
	switch ev.Type {
	case "checkout.session.completed":
		var s struct {
			ID                string `json:"id"`
			ClientReferenceID string `json:"client_reference_id"`
			PaymentStatus     string `json:"payment_status"`
			Subscription      string `json:"subscription"`
		}
		if err := json.Unmarshal(ev.Data.Object, &s); err != nil {
			return err
		}
		pid, err := uuid.Parse(s.ClientReferenceID)
		if err != nil || (s.PaymentStatus != "paid" && s.PaymentStatus != "no_payment_required") {
			return nil
		}
		ref := s.Subscription
		if ref == "" {
			ref = s.ID
		}
		_, err = b.repo.ActivatePayment(ctx, pid, ref, nil, ev.Data.Object)
		return err
	case "invoice.paid":
		var inv struct {
			ID            string `json:"id"`
			Subscription  string `json:"subscription"`
			BillingReason string `json:"billing_reason"`
			AmountPaid    int    `json:"amount_paid"`
			Currency      string `json:"currency"`
			Parent        struct {
				SubscriptionDetails struct {
					Subscription string `json:"subscription"`
				} `json:"subscription_details"`
			} `json:"parent"`
			Lines struct {
				Data []struct {
					Period struct {
						End int64 `json:"end"`
					} `json:"period"`
				} `json:"data"`
			} `json:"lines"`
		}
		if err := json.Unmarshal(ev.Data.Object, &inv); err != nil {
			return err
		}
		sub := inv.Subscription
		if sub == "" {
			sub = inv.Parent.SubscriptionDetails.Subscription
		}
		if sub == "" || inv.BillingReason != "subscription_cycle" || len(inv.Lines.Data) == 0 {
			return nil // first invoice is handled by checkout.session.completed
		}
		_, err := b.repo.RenewByRef(ctx, "stripe", sub, inv.ID, inv.AmountPaid, strings.ToLower(inv.Currency),
			time.Unix(inv.Lines.Data[0].Period.End, 0), ev.Data.Object)
		return err
	case "customer.subscription.deleted":
		var s struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Data.Object, &s); err != nil {
			return err
		}
		return b.repo.CancelByRef(ctx, "stripe", s.ID)
	}
	return nil
}

// StartExpiryLoop flips lapsed subscriptions to "expired" every hour.
func (b *BillingService) StartExpiryLoop(ctx context.Context) {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			if n, err := b.repo.ExpireSubscriptions(ctx); err != nil {
				log.Printf("expire subscriptions: %v", err)
			} else if n > 0 {
				log.Printf("expired %d subscriptions", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// ---- admin helpers ----

func (b *BillingService) Grant(ctx context.Context, uid uuid.UUID, planCode string, days int) error {
	pid, err := b.repo.PlanIDByCode(ctx, planCode)
	if err != nil {
		return apperr.BadRequest("unknown_plan", "Unknown plan")
	}
	return b.repo.GrantPremium(ctx, uid, pid, days)
}

func (b *BillingService) Revoke(ctx context.Context, uid uuid.UUID) error {
	return b.repo.RevokePremium(ctx, uid)
}
