// Package payments defines the provider-agnostic checkout contract.
//
// To add a payment method (PayPal, Apple/Google in-app purchase, ZarinPal,
// crypto…): implement Provider in a new file, register it in router/app wiring,
// and have its webhook call BillingService.Activate. Nothing else changes.
package payments

import (
	"context"

	"atish/internal/models"

	"github.com/google/uuid"
)

type CheckoutInput struct {
	PaymentID uuid.UUID
	UserID    uuid.UUID
	Plan      models.Plan
	ReturnURL string
}

type CheckoutOutput struct {
	// URL is a hosted checkout page to open in a browser (Stripe, PayPal…).
	URL string `json:"url,omitempty"`
	// InvoiceLink is a Telegram invoice link for WebApp.openInvoice (Stars).
	InvoiceLink string `json:"invoice_link,omitempty"`
	// ProviderRef is the provider's id for the pending checkout, if any.
	ProviderRef string `json:"-"`
}

type Provider interface {
	Name() string
	// Supports reports whether the provider is configured and can sell the plan.
	Supports(plan models.Plan) bool
	Checkout(ctx context.Context, in CheckoutInput) (*CheckoutOutput, error)
}
