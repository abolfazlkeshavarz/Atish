# 🔥 Atish — Social Connection Network

Find people who are looking for the **same kind of connection**: friends, a relationship, or both.
Atish ships as a **Telegram Mini App** today, on a backend that is built to serve web, Android and iOS clients tomorrow.

| | |
|---|---|
| **Frontend** | React 18 · TypeScript · Vite · Tailwind · Zustand · React Router |
| **Backend** | Go · Gin · PostgreSQL 16 · Redis 7 (modular monolith) |
| **Deploy** | `docker compose up -d --build` (Postgres, Redis, API, web, optional Caddy HTTPS) |
| **Admin** | Built-in admin panel at `/admin` — everything is manageable |
| **Money** | Free app + Premium ("Atish Plus") via Telegram Stars and Stripe; provider-agnostic |

---

## 1. Quick start (Docker)

You need Docker and a Telegram bot.

```bash
# 1. Create a bot with @BotFather → copy the token. Then:
cp .env.example .env
#    Fill in: POSTGRES_PASSWORD, JWT_SECRET, ENCRYPTION_KEY (openssl rand -base64 32),
#             TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME, ADMIN_PASSWORD, PUBLIC_URL

# 2. Start everything
docker compose up -d --build
#    …with automatic HTTPS (Let's Encrypt) on your domain:
docker compose --profile tls up -d --build
```

3. **Connect the bot to the app.** In @BotFather: `/mybots → your bot → Bot Settings → Menu Button → Configure` and set the URL to your `PUBLIC_URL`
   (or `/newapp` for a direct `t.me/<bot>/<app>` link). Telegram **requires HTTPS**, so use the `tls` profile or put your own proxy in front of port 8080.
4. Open the bot → `/start` → **Open Atish**. Onboarding takes about two minutes.
5. Admin panel: `https://<your-domain>/admin` — sign in with `ADMIN_USERNAME` / `ADMIN_PASSWORD`,
   or add your Telegram user id to `ADMIN_TELEGRAM_IDS` and open `/admin` from inside Telegram.

Migrations and seed data (interests, languages, quiz, plans, starter cities) are applied automatically on first start.

**Updating:** `git pull && docker compose up -d --build`. Migrations run on boot under an advisory lock.
**Backups:** `docker compose exec postgres pg_dump -U atish atish > backup.sql` and back up the `media` volume (profile photos).

## 2. Local development

```bash
# Infra only
docker run -d --name atish-pg -e POSTGRES_USER=atish -e POSTGRES_PASSWORD=atish -e POSTGRES_DB=atish -p 5432:5432 postgres:16-alpine
docker run -d --name atish-redis -p 6379:6379 redis:7-alpine

# API (hot-testable outside Telegram thanks to DEV_AUTH)
cd backend && DEV_AUTH=true ADMIN_PASSWORD=change-me-please go run ./cmd/server

# Web app — proxies /api and /media to :8080
cd frontend && npm install && npm run dev        # http://localhost:5173

# Demo people + end-to-end test
node backend/scripts/seed.mjs                    # 10 demo profiles in Genoa/Milan
node backend/scripts/smoke.mjs                   # full API flow: onboarding → match → chat → admin → delete
cd backend && go test ./...                      # init-data signature + matching rules
```

Outside Telegram the app shows a **developer sign-in** (only when `DEV_AUTH=true` / Vite dev mode). `DEV_AUTH` is refused in production.

## 3. Architecture

```
Telegram ── Bot (long polling) ─┐
   │                            │
   └── Mini App (React) ──► Go API (Gin) ──► PostgreSQL  (source of truth)
        Web / Android / iOS ──►      └──────► Redis       (rate limits, session-state cache, counters)
```

```
backend/
  cmd/server/            wiring (dependency injection by hand)
  internal/
    config/              env → typed config, production safety checks
    handlers/            thin HTTP adapters (no business rules)
    services/            ALL business logic: auth, profile, discovery, compat, chat, safety, billing, admin, bot
    repository/          ALL SQL (pgx); batch loaders avoid N+1
    models/  apperr/     shared types, typed API errors with stable codes
    middleware/          auth, role guard, Redis rate limiting, CORS, security headers
    telegram/            init-data HMAC validation + tiny Bot API client
    payments/            Provider interface + Telegram Stars + Stripe
    storage/             Storage interface (local disk; S3 can implement the same interface)
    db/migrations/       embedded SQL, applied at boot
frontend/src/
  platform/telegram.ts   the ONLY file that touches window.Telegram (swap it for other shells)
  api/                   typed fetch client, 401 → silent re-auth
  components/ pages/     UI; editors are shared between onboarding and profile editing
  admin/                 lazy-loaded admin panel (separate code-split chunk)
```

**Design rules followed:** no business logic in handlers or React components · Telegram numeric id (never the username) is the identity ·
Telegram data is verified server-side with the bot-token HMAC · PostgreSQL is the only source of truth · Redis holds only disposable data ·
one deployable (modular monolith), no premature microservices.

## 4. What users get

* **Onboarding in ~2 minutes**, 6 steps with progress, every step saved immediately (resume anywhere):
  you → photo (one-tap *Use my Telegram photo*) → **Friends / Relationship / Both** → city & area → interests → optional bio/languages.
* **Atish username** — always `Atish_…`. Generated from the Telegram username (`Atish_alex_rossi`, `…2`, `…24` if taken); users without a username choose one live-validated
  (uniqueness, charset, length, reserved and blocked words).
* **Discovery** with swipe or **✕ / ❤️** buttons, photo carousel, profile sheet, filters (type, age, distance, language, interests), undo last pass (Plus).
* **Qualitative compatibility** — never a percentage: *Strong match · You both like Gaming · You're both in Genoa · You both want friendship*.
* **Matches, chat** (polling now, WebSocket-ready), Telegram bot notifications for matches and messages (throttled).
* **Safety:** report, block, unmatch, pause profile, hide fields, delete account, rate limiting, moderation records.
* Optional extras: 12-question quiz (≈30 s), availability grid, languages with level + "want to practice", work/education, lifestyle — all skippable.

### Matching rules (all server-side — `services/compat.go`)
* Two users can connect only if their **connection types intersect** (friends↔friends, relationship↔relationship, both↔either). Relationship-only never meets friends-only.
* **Age ranges and distance scopes must hold in both directions.** Distance is hierarchical (area → city → region → country → anywhere); no coordinates exist anywhere.
* **Gender preferences apply only to romantic connections** — a mismatch downgrades the pair to friends instead of hiding them.
* Internal score (never shown): intent (dominant) + shared interests, languages, language-exchange fit, location, friendship kinds, relationship intention,
  personality answers, availability overlap, "already likes you". Weights are editable in the admin panel.
* A mutual like creates the match with the *shared* types; the match is created in one transaction.

## 5. Admin panel (`/admin`)

Dashboard & 14-day trends · **Users** (search, filter, suspend/ban with reason, verification badges, role, edit name/bio, delete, grant/revoke Plus, activity stats) ·
**Reports** (queue, conversation context — access is audit-logged, one-click suspend/ban) · **Photo moderation** · **Catalogs**: interests, languages,
connection types, friendship kinds, quiz questions, locations, **plans** · **Payments & subscriptions** · **Settings** (premium on/off, free like limit, min age,
photo count, registration, maintenance mode, banner, blocked words, matching weights) · **Broadcast** via the bot · **Audit log** of every admin action.
Roles: `admin` (everything) and `moderator` (users, reports, photos, audit). Deactivating a catalog item hides it from pickers without breaking existing profiles.

## 6. Premium & payments (app is free)

Free users get everything except: daily like cap (`free_daily_likes`, default 60), *see who liked you*, *rewind*, Plus badge.
Set **`premium_enabled = false`** in Settings for a free-launch mode in which every feature is unlocked for everyone.

* **Telegram Stars** — works with just the bot token; monthly plans are real recurring Star subscriptions.
* **Stripe** — set `STRIPE_SECRET_KEY` + `STRIPE_WEBHOOK_SECRET`, webhook → `/api/billing/webhook/stripe`
  (`checkout.session.completed`, `invoice.paid`, `customer.subscription.deleted`). Per-plan `stripe_price_id` for native Stripe subscriptions.
* **Adding a method** (PayPal, ZarinPal, crypto, in-app purchase…): implement `payments.Provider` (`Name`, `Supports`, `Checkout`), register it in `cmd/server/main.go`,
  and have its webhook call `BillingService.Activate…`. Plans, entitlements, admin screens and the paywall UI need no changes.
* Entitlements come from the plan's `features` JSON, so new perks are data, not migrations.

## 7. Privacy & data model

* **Public:** username, name, age, photos, city (+area if the user allows), languages, interests, bio, connection intent, verification badges.
* **Matching-only:** quiz answers, preferred age/gender, distance, availability, friendship/relationship preferences.
* **Private, never exposed:** Telegram ID, phone number (AES-256-GCM encrypted, hash-unique), birth date, internal ids, auth data, moderation/reports.
* Photos are re-encoded (EXIF/GPS stripped), resized, stored privately and served only through **HMAC-signed, expiring URLs**.
* **Account deletion** (typed confirmation) erases profile, photos (files too), interests, preferences, likes/passes, matches **and all messages**, blocks, phone data, and the Telegram link;
  the Atish username is freed; subscriptions stop. Only an anonymized tombstone row remains so payment records and moderation reports stay consistent.

## 8. Building Android / iOS / Web clients

The API is plain REST + JSON with bearer tokens — nothing depends on Telegram except *how a session is obtained*:

1. Add an auth endpoint for the new client next to `POST /api/auth/telegram` (e.g. email/OTP, Apple, Google). `users.telegram_user_id` is nullable for exactly this reason;
   issue the same JWT with `AuthService.Issue`.
2. Everything else (`/api/me`, `/api/discover`, `/api/chats`, …) is client-agnostic. Errors always look like `{"error":{"code","message","data"}}` with stable codes.
3. See **[docs/API.md](docs/API.md)** for the endpoint reference. Photos are plain `https://` URLs; compatibility reasons are structured so each client localises its own text.
4. Browser shells: `frontend/src/platform/telegram.ts` is the single platform adapter — provide the same functions (haptics, open link, invoice…) for web/React Native.

## 9. Security checklist for production

- `APP_ENV=production` (set by default in compose) enforces: strong `JWT_SECRET`, `ENCRYPTION_KEY`, ≥12-char `ADMIN_PASSWORD`, `DEV_AUTH` off.
- Serve over HTTPS (`--profile tls`). Keep Postgres/Redis unexposed (compose publishes only the web port).
- Rate limits are per user/IP in Redis (auth, likes, messages, reports, uploads, admin login). Banned/suspended users are cut off within ~30 s.
- Rotate `JWT_SECRET` to invalidate all sessions. **Never rotate `ENCRYPTION_KEY`** without re-encrypting stored phone numbers.
- Restrict `CORS_ORIGINS` if you serve native/web clients from other origins.

## 10. Roadmap hooks already in place

WebSocket chat (polling endpoint already cursor-based) · S3 storage (`storage.Storage`) · city-adjacency for "nearby cities" ·
photo / identity verification (flags + badges exist) · image & voice messages · boosts (entitlement features are data-driven).

---

## 11. Operations cheat-sheet (Makefile)

`make help` lists everything. Typical flows:

```bash
# Develop
make dev-up && make run              # Postgres+Redis in Docker, API on :8080 (DEV_AUTH on)
make run-frontend                    # Vite on :5173            ·  make demo   → 10 demo profiles
make test-all                        # unit tests + typecheck + full end-to-end smoke test

# Deploy on a fresh Ubuntu/Debian VPS (installs Docker, generates secrets, builds, nginx + Let's Encrypt)
DOMAIN=atish.example.com LETSENCRYPT_EMAIL=me@example.com ./scripts/bootstrap-vps.sh
# …or on a small server: build here, ship, load there
make images-bundle   →  scp dist/atish-images.tar.gz  →  (server) make load-images && make up-prebuilt
make deploy-check                    # pre-flight: secrets, build, repo hygiene
make health / make deploy-health HOST=https://atish.example.com
make backup                          # database dump + photos archive in backups/
make update                          # git pull + rebuild
```

**Run the platform from the terminal** (works on the server through the container, or on your machine against the dev database;
`U` accepts an Atish username, name, Telegram id, `@telegram_username` or UUID):

| Command | What it does |
|---|---|
| `make admin-stats` · `admin-users Q=alex` · `admin-user U=Atish_alex` | counters, search, full user detail |
| `make admin-promote TG=123456789 [ROLE=moderator]` · `admin-demote U=…` | give / remove staff access |
| `make admin-ban U=… REASON=…` · `admin-suspend` · `admin-activate` | moderation — takes effect within seconds |
| `make admin-verify U=… FLAG=photo` | verification badges |
| `make admin-premium U=… DAYS=30` · `admin-unpremium U=…` · `admin-plans` | grant / revoke Atish Plus |
| `make admin-reports` · `admin-resolve ID=3 ACTION=ban REASON=scam` | moderation queue |
| `make admin-set KEY=free_daily_likes VALUE=100` · `admin-settings` | any platform setting |
| `make admin-maintenance ON=1` · `admin-registration OPEN=0` · `admin-premium-mode ON=0` · `admin-banner TEXT="…"` | quick switches |
| `make admin-broadcast TEXT="…"` · `admin-audit` · `admin-delete-user U=…` | announcements, audit trail, erasure |
| `make admin-set-password U=@name [PASSWORD=…]` · `admin-clear-password U=…` | give an admin/moderator (promoted ones too) their own `/admin` password — generated if omitted, stored hashed, sign in with their Atish or Telegram username |
| `make admin-password` | rotate the shared root password (`ADMIN_PASSWORD` in `.env`) and restart the API |

Every change made through the CLI is written to the same audit log as the admin panel (actor `cli`).
