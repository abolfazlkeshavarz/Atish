package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Bot is a tiny Telegram Bot API client (no third-party dependency).
type Bot struct {
	token string
	http  *http.Client
}

func NewBot(token string) *Bot {
	return &Bot{token: token, http: &http.Client{Timeout: 70 * time.Second}}
}

func (b *Bot) Enabled() bool { return b != nil && b.token != "" }

type Update struct {
	UpdateID         int64             `json:"update_id"`
	Message          *Message          `json:"message"`
	PreCheckoutQuery *PreCheckoutQuery `json:"pre_checkout_query"`
}

type Message struct {
	MessageID         int64              `json:"message_id"`
	From              *User              `json:"from"`
	Chat              Chat               `json:"chat"`
	Text              string             `json:"text"`
	Contact           *Contact           `json:"contact"`
	SuccessfulPayment *SuccessfulPayment `json:"successful_payment"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`
	UserID      int64  `json:"user_id"`
}

type PreCheckoutQuery struct {
	ID             string `json:"id"`
	From           User   `json:"from"`
	Currency       string `json:"currency"`
	TotalAmount    int    `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
}

type SuccessfulPayment struct {
	Currency                   string `json:"currency"`
	TotalAmount                int    `json:"total_amount"`
	InvoicePayload             string `json:"invoice_payload"`
	SubscriptionExpirationDate int64  `json:"subscription_expiration_date"`
	IsRecurring                bool   `json:"is_recurring"`
	IsFirstRecurring           bool   `json:"is_first_recurring"`
	TelegramPaymentChargeID    string `json:"telegram_payment_charge_id"`
}

type apiResp struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

func (b *Bot) Call(ctx context.Context, method string, params any, out any) error {
	if !b.Enabled() {
		return fmt.Errorf("telegram bot disabled")
	}
	body, _ := json.Marshal(params)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+b.token+"/"+method, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var r apiResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("telegram %s: bad response", method)
	}
	if !r.OK {
		return fmt.Errorf("telegram %s: %s (%d)", method, r.Description, r.ErrorCode)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

type M map[string]any

func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string, markup any) error {
	p := M{"chat_id": chatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}
	if markup != nil {
		p["reply_markup"] = markup
	}
	return b.Call(ctx, "sendMessage", p, nil)
}

func (b *Bot) AnswerPreCheckout(ctx context.Context, id string, ok bool, errMsg string) error {
	p := M{"pre_checkout_query_id": id, "ok": ok}
	if !ok {
		p["error_message"] = errMsg
	}
	return b.Call(ctx, "answerPreCheckoutQuery", p, nil)
}

// CreateStarsInvoice creates an invoice link paid with Telegram Stars (XTR).
// subscriptionPeriod (seconds) must be 2592000 (30 days) or 0 for a one-time payment.
func (b *Bot) CreateStarsInvoice(ctx context.Context, title, desc, payload string, stars, subscriptionPeriod int) (string, error) {
	p := M{
		"title": title, "description": desc, "payload": payload, "currency": "XTR",
		"prices": []M{{"label": title, "amount": stars}},
	}
	if subscriptionPeriod > 0 {
		p["subscription_period"] = subscriptionPeriod
	}
	var link string
	err := b.Call(ctx, "createInvoiceLink", p, &link)
	return link, err
}

func (b *Bot) SetCommands(ctx context.Context) {
	_ = b.Call(ctx, "setMyCommands", M{"commands": []M{
		{"command": "start", "description": "Open Atish"},
		{"command": "help", "description": "How Atish works"},
		{"command": "verify", "description": "Verify your phone number"},
		{"command": "privacy", "description": "Privacy information"},
		{"command": "support", "description": "Contact support"},
	}}, nil)
}

func (b *Bot) SetMenuButton(ctx context.Context, url string) {
	_ = b.Call(ctx, "setChatMenuButton", M{"menu_button": M{"type": "web_app", "text": "Open Atish", "web_app": M{"url": url}}}, nil)
}

// Poll long-polls for updates until ctx is cancelled.
func (b *Bot) Poll(ctx context.Context, handle func(context.Context, Update)) {
	// Make sure no webhook is configured, otherwise getUpdates fails.
	_ = b.Call(ctx, "deleteWebhook", M{"drop_pending_updates": false}, nil)
	var offset int64
	for ctx.Err() == nil {
		var updates []Update
		err := b.Call(ctx, "getUpdates", M{"offset": offset, "timeout": 50, "allowed_updates": []string{"message", "pre_checkout_query"}}, &updates)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("bot poll error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("bot handler panic: %v", r)
					}
				}()
				handle(ctx, u)
			}()
		}
	}
}
