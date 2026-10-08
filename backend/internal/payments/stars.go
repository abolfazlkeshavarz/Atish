package payments

import (
	"context"

	"atish/internal/models"
	"atish/internal/telegram"
)

// Stars sells plans with Telegram Stars (XTR), the native in-Telegram payment
// method. Monthly plans become real recurring Star subscriptions.
type Stars struct{ bot *telegram.Bot }

func NewStars(bot *telegram.Bot) *Stars { return &Stars{bot: bot} }

func (s *Stars) Name() string { return "telegram_stars" }

func (s *Stars) Supports(p models.Plan) bool {
	return s.bot.Enabled() && p.StarsPrice != nil && *p.StarsPrice > 0
}

func (s *Stars) Checkout(ctx context.Context, in CheckoutInput) (*CheckoutOutput, error) {
	period := 0
	if in.Plan.Interval == "month" {
		period = 2592000 // 30 days: the only recurring period Telegram supports
	}
	link, err := s.bot.CreateStarsInvoice(ctx, in.Plan.Name, orDefault(in.Plan.Description, in.Plan.Name),
		in.PaymentID.String(), *in.Plan.StarsPrice, period)
	if err != nil {
		return nil, err
	}
	return &CheckoutOutput{InvoiceLink: link}, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
