# Atish REST API

Base URL: `https://<host>` · JSON everywhere · `Authorization: Bearer <jwt>` for all `/api/*` routes except those marked **public**.
Timestamps are RFC 3339 (UTC). IDs are UUIDs (`id`) except reports (int) and catalog rows.

## Errors

```json
{ "error": { "code": "daily_limit", "message": "You've used all your likes for today", "data": { "limit": 60 } } }
```

`code` is stable and machine-readable; `message` is English and for display only. Common codes:
`unauthorized` (401) · `account_suspended` / `account_banned` (403, `data.reason`) · `not_found` (404) · `rate_limited` (429) ·
`daily_limit` / `premium_required` (402) · `not_compatible` (403) · `onboarding_incomplete` (400, `data.missing[]`) ·
`username_taken` (409) · `too_young` (400, `data.min_age`) · `maintenance` (503).

## Authentication

| Method | Path | Notes |
|---|---|---|
| POST | `/api/auth/telegram` **public** | `{ "init_data": "<Telegram.WebApp.initData>" }` → `{ token, expires_at, is_new }`. Signature (HMAC with the bot token) and `auth_date` are verified server-side. |
| POST | `/api/auth/dev` **public** | Only when `DEV_AUTH=true`. `{ telegram_id, name }`. |
| POST | `/api/admin/login` **public** | `{ username, password }` for the admin panel. |

Tokens last `JWT_TTL_HOURS` (24 by default). Telegram clients simply re-authenticate with fresh init data on `401`.
Add your own auth endpoint for other clients and issue the same JWT (`AuthService.Issue`).

## Account & profile

| Method | Path | |
|---|---|---|
| GET | `/api/catalog` **public** | `{ catalog: { connection_types, friendship_kinds, interests, languages, personality_questions }, app: {…public settings} }` |
| GET | `/api/me` | Full owner view: profile, photos (signed URLs), preferences, privacy, `onboarding {complete, missing[]}`, `completeness`, `suggestions[]`, `entitlements`. |
| PATCH | `/api/me/profile` | Partial: `display_name, birth_date (YYYY-MM-DD), gender, bio, location_id, occupation{}, lifestyle{}, interests[], languages[{code,level,wants_practice}], connection_types[]` |
| GET/PATCH | `/api/me/preferences` | `friendship_kinds[], preferences{relationship_intention, preferred_genders[], age_min, age_max, distance_scope}, personality{key:option}, availability[{day 0=Mon,slot}], privacy{…}` |
| GET | `/api/username/check?suffix=alex` | 200 if available, else error with reason. |
| PUT | `/api/me/username` | `{ suffix }` — the part after `Atish_`. |
| POST | `/api/me/onboarding/complete` | Validates minimum profile, opens discovery. |
| POST | `/api/me/photos` | `multipart/form-data`, field `photo` (JPEG/PNG/WebP ≤ 8 MB). Re-encoded server-side. |
| POST | `/api/me/photos/telegram` | Imports the Telegram profile photo. |
| PUT / DELETE | `/api/me/photos/order` · `/api/me/photos/:id` | Reorder (`{ids:[…]}`) / remove. |
| DELETE | `/api/me/phone` | Remove verified phone (verification itself happens in the bot via shared contact). |
| GET | `/api/me/blocks` | Blocked users. |
| DELETE | `/api/me` | `{ "confirm": "DELETE" }` — irreversible erasure. |
| GET | `/api/locations/cities?country=IT&q=gen` | Area-based hierarchy only. |
| GET | `/api/locations/areas?country=IT&city=Genoa` | |
| POST | `/api/locations` | `{ country_code, country, region?, city, area? }` — add a missing place. |

## Discovery, matches, chat

| Method | Path | |
|---|---|---|
| GET | `/api/discover?type=&min_age=&max_age=&interests=a,b&language=&city=&limit=` | `{ profiles: PublicProfile[], likes_left }`. Each profile has `compat { strength, types[], reasons[{type, items[]}] }` — render reasons in your own language. |
| POST | `/api/discover/:id/like` | `{ matched, match_id?, user?, likes_left }` · `402 daily_limit` for free users at their cap. |
| POST | `/api/discover/:id/pass` | 204 |
| POST | `/api/discover/rewind` | Plus. Restores the last pass. |
| GET | `/api/likes/received` | `{ total, locked, profiles[] }` — `locked` for free users. |
| GET | `/api/matches` · DELETE `/api/matches/:id` · GET `/api/users/:id` (matched users only) | |
| GET | `/api/chats` | `[{ match, last_message, unread }]` |
| GET | `/api/chats/:id/messages?after=<id>&before=<id>&limit=` | Cursor pagination; poll with `after` (or move to WebSocket later). |
| POST | `/api/chats/:id/messages` | `{ body }` (1–2000 chars) |
| POST | `/api/chats/:id/read` · GET `/api/chats/unread` | |

## Safety

`POST /api/users/:id/block` · `DELETE /api/users/:id/block` · `POST /api/users/:id/report {reason, details}`
(reasons: `fake_profile, inappropriate_photo, harassment, spam_scam, underage, hate_speech, other`).

## Billing

| Method | Path | |
|---|---|---|
| GET | `/api/billing/plans` | Plans with the `providers[]` that can sell each, plus current `entitlements`. |
| POST | `/api/billing/checkout` | `{ plan, provider }` → `{ payment_id, provider, url? , invoice_link? }` — open `url` in a browser (Stripe) or pass `invoice_link` to `Telegram.WebApp.openInvoice` (Stars). |
| POST | `/api/billing/webhook/stripe` **public** | Verified with `Stripe-Signature`. |

## Media

Photo URLs are `GET /media/:id?e=<expiry>&s=<signature>` and expire; always use the URL from the latest API response.

## Admin (`role` = `admin` or `moderator`)

`/api/admin/…`: `whoami, stats, users[/:id], users/:id/profile, photos, reports[/:id], audit` (moderator + admin) ·
`users/:id DELETE, users/:id/premium, catalog/:resource (interests|languages|connection-types|friendship-kinds|personality-questions|locations|plans), settings[/:key], payments, subscriptions, broadcast` (admin only).
