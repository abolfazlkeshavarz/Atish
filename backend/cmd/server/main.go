package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"atish/internal/config"
	"atish/internal/db"
	"atish/internal/handlers"
	"atish/internal/payments"
	"atish/internal/repository"
	"atish/internal/router"
	"atish/internal/services"
	"atish/internal/storage"
	"atish/internal/telegram"

	"github.com/redis/go-redis/v9"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ropts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(ropts)
	defer rdb.Close()
	for i := 0; i < 20; i++ {
		if err = rdb.Ping(ctx).Err(); err == nil {
			break
		}
		log.Printf("waiting for redis: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("redis: %v", err)
	}

	store, err := storage.NewLocal(cfg.MediaDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	bot := telegram.NewBot(cfg.TelegramBotToken)

	repo := repository.New(pool)
	settings := services.NewSettings(repo)
	catalog := services.NewCatalog(repo)
	auth := services.NewAuth(cfg, repo, rdb, settings)
	signer := services.NewMediaSigner(cfg.MediaKey, cfg.PublicURL)
	presenter := services.NewPresenter(signer)
	billing := services.NewBilling(cfg, repo, settings, payments.NewStars(bot), payments.NewStripe(cfg.StripeSecretKey))
	profile := services.NewProfile(cfg, repo, auth, settings, catalog, store, signer, billing)
	notifier := services.NewNotifier(cfg, bot, repo, rdb)
	discovery := services.NewDiscovery(repo, settings, billing, presenter, notifier, rdb)
	chat := services.NewChat(repo, settings, discovery, notifier)
	safety := services.NewSafety(repo, presenter)
	admin := services.NewAdmin(repo, auth, profile, billing, settings, catalog, bot, store)
	botSvc := services.NewBotService(cfg, bot, profile, billing, settings)

	go botSvc.Start(ctx)
	billing.StartExpiryLoop(ctx)

	h := &handlers.Handlers{Cfg: cfg, Auth: auth, Profile: profile, Discovery: discovery, Chat: chat, Safety: safety,
		Billing: billing, Admin: admin, Settings: settings, Catalog: catalog}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.New(cfg, h, repo, rdb),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Printf("Atish API listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
