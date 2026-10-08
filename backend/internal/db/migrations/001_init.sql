-- Atish core schema. PostgreSQL is the single source of truth.

CREATE TABLE users (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_user_id   bigint UNIQUE,                 -- permanent identity for Telegram clients (never the username)
    telegram_username  text,
    telegram_photo_url text,
    atish_username     text,
    role               text NOT NULL DEFAULT 'user'   CHECK (role IN ('user','moderator','admin')),
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','banned','deleted')),
    status_reason      text,
    telegram_verified  boolean NOT NULL DEFAULT false,
    phone_verified     boolean NOT NULL DEFAULT false,
    photo_verified     boolean NOT NULL DEFAULT false,
    identity_verified  boolean NOT NULL DEFAULT false,
    language_code      text,
    onboarded_at       timestamptz,
    last_active_at     timestamptz NOT NULL DEFAULT now(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    CONSTRAINT users_atish_prefix CHECK (atish_username IS NULL OR status = 'deleted' OR atish_username ~ '^Atish_[A-Za-z0-9_]{3,24}$')
);
CREATE UNIQUE INDEX users_atish_username_uidx ON users (lower(atish_username)) WHERE atish_username IS NOT NULL;
CREATE INDEX users_status_idx ON users (status);
CREATE INDEX users_last_active_idx ON users (last_active_at DESC);
CREATE INDEX users_created_idx ON users (created_at DESC);

CREATE TABLE locations (
    id           bigserial PRIMARY KEY,
    country_code char(2) NOT NULL,
    country      text NOT NULL,
    region       text NOT NULL DEFAULT '',
    city         text NOT NULL,
    area         text NOT NULL DEFAULT '',           -- '' = the city itself
    is_approved  boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX locations_uidx ON locations (country_code, lower(region), lower(city), lower(area));
CREATE INDEX locations_city_idx ON locations (country_code, lower(city));
CREATE INDEX locations_region_idx ON locations (country_code, lower(region));

CREATE TABLE profiles (
    user_id           uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name      text NOT NULL,
    birth_date        date NOT NULL,                  -- private; only the computed age is shown
    gender            text NOT NULL CHECK (gender IN ('male','female','non_binary','other','prefer_not_to_say')),
    bio               text NOT NULL DEFAULT '',
    location_id       bigint REFERENCES locations(id),
    occupation_status text,
    university        text,
    field_of_study    text,
    degree            text,
    profession        text,
    industry          text,
    smoking           text,
    drinking          text,
    pets              text,
    diet              text,
    sleep_schedule    text,
    social_level      text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX profiles_birth_idx ON profiles (birth_date);
CREATE INDEX profiles_location_idx ON profiles (location_id);

CREATE TABLE user_photos (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key  text NOT NULL,
    content_type text NOT NULL DEFAULT 'image/jpeg',
    position     int  NOT NULL DEFAULT 0,
    status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','removed')),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_photos_user_idx ON user_photos (user_id, position) WHERE status = 'active';

-- Reference data (all admin-manageable) ---------------------------------

CREATE TABLE connection_types (
    slug      text PRIMARY KEY,
    name      text NOT NULL,
    emoji     text NOT NULL DEFAULT '',
    position  int  NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true
);
CREATE TABLE friendship_kinds (
    slug      text PRIMARY KEY,
    name      text NOT NULL,
    emoji     text NOT NULL DEFAULT '',
    position  int  NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true
);
CREATE TABLE interests (
    id        serial PRIMARY KEY,
    slug      text NOT NULL UNIQUE,
    category  text NOT NULL,
    name      text NOT NULL,
    emoji     text NOT NULL DEFAULT '',
    position  int  NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true
);
CREATE TABLE languages (
    code        text PRIMARY KEY,
    name        text NOT NULL,
    native_name text NOT NULL DEFAULT '',
    is_active   boolean NOT NULL DEFAULT true
);
CREATE TABLE personality_questions (
    id        serial PRIMARY KEY,
    key       text NOT NULL UNIQUE,
    text      text NOT NULL,
    options   jsonb NOT NULL DEFAULT '[]',            -- [{"key":"a","label":"..."}]
    position  int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true
);

-- Per-user relations -----------------------------------------------------

CREATE TABLE user_connection_types (
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type_slug text NOT NULL REFERENCES connection_types(slug) ON UPDATE CASCADE,
    PRIMARY KEY (user_id, type_slug)
);
CREATE INDEX user_connection_types_type_idx ON user_connection_types (type_slug, user_id);

CREATE TABLE user_friendship_kinds (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind    text NOT NULL REFERENCES friendship_kinds(slug) ON UPDATE CASCADE,
    PRIMARY KEY (user_id, kind)
);
CREATE TABLE user_interests (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    interest_id int  NOT NULL REFERENCES interests(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, interest_id)
);
CREATE INDEX user_interests_interest_idx ON user_interests (interest_id);

CREATE TABLE user_languages (
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    language_code text NOT NULL REFERENCES languages(code) ON UPDATE CASCADE,
    level         text NOT NULL DEFAULT 'intermediate' CHECK (level IN ('basic','intermediate','advanced','native')),
    wants_practice boolean NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, language_code)
);

CREATE TABLE preferences (
    user_id               uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    relationship_intention text,
    preferred_genders     text[] NOT NULL DEFAULT '{everyone}',
    age_min               int NOT NULL DEFAULT 18,
    age_max               int NOT NULL DEFAULT 99,
    distance_scope        text NOT NULL DEFAULT 'same_city'
        CHECK (distance_scope IN ('same_area','same_city','nearby','same_region','same_country','anywhere')),
    CHECK (age_min <= age_max)
);

CREATE TABLE personality_answers (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id int  NOT NULL REFERENCES personality_questions(id) ON DELETE CASCADE,
    option_key  text NOT NULL,
    PRIMARY KEY (user_id, question_id)
);

CREATE TABLE availability (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day     smallint NOT NULL CHECK (day BETWEEN 0 AND 6),   -- 0 = Monday
    slot    text NOT NULL CHECK (slot IN ('morning','afternoon','evening','late_night')),
    PRIMARY KEY (user_id, day, slot)
);

CREATE TABLE privacy_settings (
    user_id          uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    show_area        boolean NOT NULL DEFAULT true,   -- false = city only
    show_gender      boolean NOT NULL DEFAULT true,
    show_education   boolean NOT NULL DEFAULT false,
    show_lifestyle   boolean NOT NULL DEFAULT false,
    discoverable     boolean NOT NULL DEFAULT true,   -- "pause my profile"
    notify_matches   boolean NOT NULL DEFAULT true,
    notify_messages  boolean NOT NULL DEFAULT true
);

-- Social graph -------------------------------------------------------------

CREATE TABLE swipes (
    from_user  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action     text NOT NULL CHECK (action IN ('like','pass')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (from_user, to_user)
);
CREATE INDEX swipes_to_idx ON swipes (to_user, action, created_at DESC);

CREATE TABLE matches (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_a           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connection_types text[] NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_message_at  timestamptz,
    unmatched_at     timestamptz,
    unmatched_by     uuid,
    CHECK (user_a < user_b),
    UNIQUE (user_a, user_b)
);
CREATE INDEX matches_a_idx ON matches (user_a) WHERE unmatched_at IS NULL;
CREATE INDEX matches_b_idx ON matches (user_b) WHERE unmatched_at IS NULL;

CREATE TABLE messages (
    id         bigserial PRIMARY KEY,
    match_id   uuid NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    sender_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at    timestamptz
);
CREATE INDEX messages_match_idx ON messages (match_id, id DESC);
CREATE INDEX messages_unread_idx ON messages (match_id, sender_id) WHERE read_at IS NULL;

-- Safety -----------------------------------------------------------------

CREATE TABLE blocks (
    blocker_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id)
);
CREATE INDEX blocks_blocked_idx ON blocks (blocked_id);

CREATE TABLE reports (
    id          bigserial PRIMARY KEY,
    reporter_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    match_id    uuid,
    reason      text NOT NULL,
    details     text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','resolved','dismissed')),
    resolution  text,
    resolved_by text,
    resolved_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_status_idx ON reports (status, created_at DESC);
CREATE INDEX reports_reported_idx ON reports (reported_id);

CREATE TABLE phone_verifications (
    user_id     uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    phone_enc   bytea NOT NULL,                      -- AES-256-GCM
    phone_hash  text  NOT NULL UNIQUE,               -- one account per phone number
    verified_at timestamptz NOT NULL DEFAULT now()
);

-- Billing (provider-agnostic) ---------------------------------------------

CREATE TABLE plans (
    id              serial PRIMARY KEY,
    code            text NOT NULL UNIQUE,
    name            text NOT NULL,
    description     text NOT NULL DEFAULT '',
    interval        text NOT NULL DEFAULT 'month' CHECK (interval IN ('month','year','lifetime')),
    price_cents     int  NOT NULL DEFAULT 0,
    currency        text NOT NULL DEFAULT 'usd',
    stars_price     int,                              -- Telegram Stars price (XTR), null = not sold with Stars
    stripe_price_id text,                             -- optional: Stripe recurring price
    features        jsonb NOT NULL DEFAULT '{}',
    position        int NOT NULL DEFAULT 0,
    is_active       boolean NOT NULL DEFAULT true
);

CREATE TABLE subscriptions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id            int  NOT NULL REFERENCES plans(id),
    provider           text NOT NULL,                 -- telegram_stars | stripe | admin
    provider_ref       text,
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','canceled','expired')),
    started_at         timestamptz NOT NULL DEFAULT now(),
    current_period_end timestamptz,                   -- null = lifetime
    canceled_at        timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subscriptions_user_idx ON subscriptions (user_id, status);
CREATE INDEX subscriptions_ref_idx ON subscriptions (provider, provider_ref);

CREATE TABLE payments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id),   -- kept (anonymized user row) for accounting
    plan_id      int  NOT NULL REFERENCES plans(id),
    provider     text NOT NULL,
    provider_ref text,
    amount_cents int  NOT NULL DEFAULT 0,
    currency     text NOT NULL DEFAULT 'usd',
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed','refunded')),
    raw          jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    paid_at      timestamptz
);
CREATE UNIQUE INDEX payments_provider_ref_uidx ON payments (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX payments_user_idx ON payments (user_id, created_at DESC);

-- Platform ---------------------------------------------------------------

CREATE TABLE app_settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id          bigserial PRIMARY KEY,
    actor       text NOT NULL,                        -- admin user id or 'root'
    action      text NOT NULL,
    target_type text,
    target_id   text,
    details     jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
