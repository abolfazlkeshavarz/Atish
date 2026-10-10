-- Per-account password for the browser admin panel. Only meaningful for users
-- whose role is admin or moderator; NULL means "sign in with Telegram only".
ALTER TABLE users ADD COLUMN IF NOT EXISTS admin_password_hash text;
