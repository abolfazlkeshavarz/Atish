package services

import (
	"context"
	"log"
	"strings"

	"atish/internal/apperr"
	"atish/internal/config"
	"atish/internal/telegram"
)

// BotService handles Telegram bot updates. The bot is only an entry point:
// it launches the Mini App, confirms phone verification and receives payments.
type BotService struct {
	cfg      *config.Config
	bot      *telegram.Bot
	profile  *ProfileService
	billing  *BillingService
	settings *Settings
}

func NewBotService(cfg *config.Config, bot *telegram.Bot, profile *ProfileService, billing *BillingService, settings *Settings) *BotService {
	return &BotService{cfg: cfg, bot: bot, profile: profile, billing: billing, settings: settings}
}

func (b *BotService) openMarkup() map[string]any {
	return map[string]any{"inline_keyboard": [][]map[string]any{{
		{"text": "Open Atish", "web_app": map[string]string{"url": b.cfg.MiniAppURL}},
	}}}
}

// Start configures the bot menu and begins long polling (blocks until ctx is done).
func (b *BotService) Start(ctx context.Context) {
	if !b.bot.Enabled() || !b.cfg.EnableBotPolling {
		log.Println("telegram bot polling disabled")
		return
	}
	b.bot.SetCommands(ctx)
	b.bot.SetMenuButton(ctx, b.cfg.MiniAppURL)
	log.Println("telegram bot polling started")
	b.bot.Poll(ctx, b.Handle)
}

func (b *BotService) Handle(ctx context.Context, u telegram.Update) {
	switch {
	case u.PreCheckoutQuery != nil:
		ok, msg := b.billing.ValidatePreCheckout(ctx, u.PreCheckoutQuery)
		if err := b.bot.AnswerPreCheckout(ctx, u.PreCheckoutQuery.ID, ok, msg); err != nil {
			log.Printf("answer pre-checkout: %v", err)
		}
	case u.Message != nil:
		b.handleMessage(ctx, u.Message)
	}
}

func (b *BotService) handleMessage(ctx context.Context, m *telegram.Message) {
	if m.Chat.Type != "private" {
		return
	}
	reply := func(text string, markup any) {
		if err := b.bot.SendMessage(ctx, m.Chat.ID, text, markup); err != nil {
			log.Printf("bot reply: %v", err)
		}
	}
	switch {
	case m.SuccessfulPayment != nil:
		if err := b.billing.HandleStarsPayment(ctx, m.SuccessfulPayment); err != nil {
			log.Printf("stars payment: %v", err)
			reply("We received your payment but could not activate it automatically. Please contact support.", nil)
			return
		}
		reply("✨ Thank you! Atish Plus is now active.", b.openMarkup())
	case m.Contact != nil:
		// Only accept the sender's own number (Telegram guarantees contact.user_id for shared contacts).
		if m.From == nil || m.Contact.UserID != m.From.ID {
			reply("Please share your own contact to verify your number.", nil)
			return
		}
		err := b.profile.VerifyPhoneFromTelegram(ctx, m.From.ID, m.Contact.PhoneNumber)
		remove := map[string]any{"remove_keyboard": true}
		if e, ok := apperr.As(err); ok {
			reply("⚠️ "+e.Message, remove)
			return
		}
		if err != nil {
			log.Printf("verify phone: %v", err)
			reply("Something went wrong. Please try again.", remove)
			return
		}
		reply("✅ Your phone number is verified. It will never be shown to other users.", remove)
		reply("Back to Atish:", b.openMarkup())
	default:
		cmd := strings.ToLower(strings.TrimSpace(strings.SplitN(m.Text, " ", 2)[0]))
		cmd = strings.SplitN(cmd, "@", 2)[0]
		switch cmd {
		case "/start":
			reply("Welcome to Atish 👋\n\nFind people who are looking for the same kind of connection — friends, a relationship, or both.", b.openMarkup())
		case "/help":
			reply("<b>How Atish works</b>\n1. Open the app and set up your profile in about 2 minutes.\n2. Tell us if you want friends, a relationship or both.\n3. Tap ❤️ if you're interested, ✕ if not.\n4. When it's mutual, you match and can chat.", b.openMarkup())
		case "/verify":
			reply("Tap the button below to share your phone number. It is only used to verify your account and is never shown to anyone.",
				map[string]any{"keyboard": [][]map[string]any{{{"text": "📱 Share my number", "request_contact": true}}},
					"resize_keyboard": true, "one_time_keyboard": true})
		case "/privacy":
			reply("<b>Your privacy</b>\n• We never show your exact location — only your city/area.\n• Your phone number and Telegram ID are never visible to others.\n• You can pause, hide, or delete your account anytime in Settings.", nil)
		case "/support":
			sup := b.settings.String(ctx, "support_username", "")
			if sup == "" {
				reply("Contact us from Atish → Settings → Support.", b.openMarkup())
			} else {
				reply("Need help? Write to @"+strings.TrimPrefix(sup, "@"), nil)
			}
		}
	}
}
