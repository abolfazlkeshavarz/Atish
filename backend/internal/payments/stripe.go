package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"atish/internal/models"
)

// Stripe uses hosted Checkout Sessions over plain HTTPS (no SDK dependency).
type Stripe struct {
	secret string
	http   *http.Client
}

func NewStripe(secretKey string) *Stripe {
	return &Stripe{secret: secretKey, http: &http.Client{Timeout: 20 * time.Second}}
}

func (s *Stripe) Name() string { return "stripe" }

func (s *Stripe) Supports(p models.Plan) bool { return s.secret != "" && p.PriceCents > 0 }

func (s *Stripe) Checkout(ctx context.Context, in CheckoutInput) (*CheckoutOutput, error) {
	f := url.Values{}
	f.Set("client_reference_id", in.PaymentID.String())
	f.Set("success_url", in.ReturnURL+"/?payment=success")
	f.Set("cancel_url", in.ReturnURL+"/?payment=cancel")
	f.Set("line_items[0][quantity]", "1")
	f.Set("metadata[payment_id]", in.PaymentID.String())
	f.Set("metadata[user_id]", in.UserID.String())
	switch {
	case in.Plan.StripePrice != nil && *in.Plan.StripePrice != "":
		f.Set("mode", "subscription")
		f.Set("line_items[0][price]", *in.Plan.StripePrice)
		f.Set("subscription_data[metadata][payment_id]", in.PaymentID.String())
	case in.Plan.Interval == "lifetime":
		f.Set("mode", "payment")
		f.Set("line_items[0][price_data][currency]", in.Plan.Currency)
		f.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(in.Plan.PriceCents))
		f.Set("line_items[0][price_data][product_data][name]", in.Plan.Name)
	default:
		f.Set("mode", "subscription")
		f.Set("line_items[0][price_data][currency]", in.Plan.Currency)
		f.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(in.Plan.PriceCents))
		f.Set("line_items[0][price_data][recurring][interval]", in.Plan.Interval)
		f.Set("line_items[0][price_data][product_data][name]", in.Plan.Name)
		f.Set("subscription_data[metadata][payment_id]", in.PaymentID.String())
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(f.Encode()))
	req.Header.Set("Authorization", "Bearer "+s.secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", in.PaymentID.String())
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 || out.URL == "" {
		return nil, fmt.Errorf("stripe checkout failed: %s", out.Error.Message)
	}
	return &CheckoutOutput{URL: out.URL, ProviderRef: out.ID}, nil
}

// VerifyStripeSignature validates the Stripe-Signature header (v1 scheme).
func VerifyStripeSignature(payload []byte, header, secret string, tolerance time.Duration) error {
	if secret == "" {
		return errors.New("stripe webhook secret not configured")
	}
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return errors.New("malformed signature header")
	}
	if d := time.Since(time.Unix(t, 0)); d > tolerance || d < -tolerance {
		return errors.New("signature timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(expected), []byte(s)) {
			return nil
		}
	}
	return errors.New("signature mismatch")
}
