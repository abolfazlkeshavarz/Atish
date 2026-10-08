package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"atish/internal/config"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/telegram"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Presenter converts internal profiles into the privacy-filtered public shape.
// It is the only code path that produces data about one user for another.
type Presenter struct{ signer *MediaSigner }

func NewPresenter(signer *MediaSigner) *Presenter { return &Presenter{signer: signer} }

func (p *Presenter) Public(pr *models.Profile, compat *models.Compat, likedYou bool) *models.PublicProfile {
	pub := &models.PublicProfile{
		ID:              pr.ID,
		DisplayName:     pr.DisplayName,
		Age:             pr.Age,
		Bio:             pr.Bio,
		Photos:          make([]models.Photo, len(pr.Photos)),
		Interests:       pr.Interests,
		Languages:       pr.Languages,
		ConnectionTypes: pr.ConnectionTypes,
		FriendshipKinds: pr.FriendshipKinds,
		Compat:          compat,
		LikedYou:        likedYou,
		Badges: models.Badges{Telegram: pr.TelegramVerified, Phone: pr.PhoneVerified, Photo: pr.PhotoVerified,
			Identity: pr.IdentityVerified, Premium: pr.Premium},
	}
	if pr.AtishUsername != nil {
		pub.AtishUsername = *pr.AtishUsername
	}
	for i, ph := range pr.Photos {
		pub.Photos[i] = models.Photo{ID: ph.ID, Position: ph.Position, URL: p.signer.URL(ph.ID)}
	}
	if pr.Location != nil {
		pub.City, pub.CountryCode = pr.Location.City, pr.Location.CountryCode
		if pr.Privacy.ShowArea {
			pub.Area = pr.Location.Area
		}
	}
	if pr.Privacy.ShowGender {
		pub.Gender = pr.Gender
	}
	if pr.Privacy.ShowEducation {
		o := pr.Occupation
		pub.Occupation = &o
	}
	if pr.Privacy.ShowLifestyle {
		l := pr.Lifestyle
		pub.Lifestyle = &l
	}
	return pub
}

// ---- notifications (Telegram bot messages) ----

type Notifier struct {
	cfg  *config.Config
	bot  *telegram.Bot
	repo *repository.Repo
	rdb  *redis.Client
}

func NewNotifier(cfg *config.Config, bot *telegram.Bot, repo *repository.Repo, rdb *redis.Client) *Notifier {
	return &Notifier{cfg: cfg, bot: bot, repo: repo, rdb: rdb}
}

func (n *Notifier) openButton(label, path string) map[string]any {
	return map[string]any{"inline_keyboard": [][]map[string]any{{
		{"text": label, "web_app": map[string]string{"url": n.cfg.MiniAppURL + "/?go=" + path}},
	}}}
}

func (n *Notifier) send(uid uuid.UUID, text string, markup any) {
	if !n.bot.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tg, err := n.repo.TelegramIDOf(ctx, uid)
		if err != nil {
			return
		}
		if err := n.bot.SendMessage(ctx, tg, text, markup); err != nil {
			log.Printf("notify %s: %v", uid, err)
		}
	}()
}

func (n *Notifier) NewMatch(ctx context.Context, to uuid.UUID, otherName string, matchID uuid.UUID) {
	if m, _ := n.repo.NotifyPrefs(ctx, to); !m {
		return
	}
	n.send(to, fmt.Sprintf("🔥 <b>It's a match!</b>\nYou and <b>%s</b> like each other. Say hi!", escapeHTML(otherName)),
		n.openButton("Open chat", "/chats/"+matchID.String()))
}

func (n *Notifier) NewMessage(ctx context.Context, to uuid.UUID, fromName string, matchID uuid.UUID) {
	_, m := n.repo.NotifyPrefs(ctx, to)
	if !m {
		return
	}
	// At most one message notification per conversation every 10 minutes.
	key := "notif:msg:" + matchID.String() + ":" + to.String()
	if ok, err := n.rdb.SetNX(ctx, key, 1, 10*time.Minute).Result(); err != nil || !ok {
		return
	}
	n.send(to, fmt.Sprintf("💬 New message from <b>%s</b>", escapeHTML(fromName)), n.openButton("Reply", "/chats/"+matchID.String()))
}

func escapeHTML(s string) string {
	r := make([]rune, 0, len(s))
	for _, c := range s {
		switch c {
		case '<':
			r = append(r, []rune("&lt;")...)
		case '>':
			r = append(r, []rune("&gt;")...)
		case '&':
			r = append(r, []rune("&amp;")...)
		default:
			r = append(r, c)
		}
	}
	return string(r)
}
