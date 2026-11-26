-- 0006: hisoblar (Google, email kod), refresh tokenlar.
-- Anonim qurilma sessiyasi saqlanadi; kirgandan keyin anonim ma'lumotlar hisobga o'tadi.

ALTER TABLE users ADD COLUMN email text;
ALTER TABLE users ADD COLUMN email_verified bool NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN google_sub text;
ALTER TABLE users ADD COLUMN avatar_url text;
ALTER TABLE users ADD COLUMN last_login_at timestamptz;
CREATE UNIQUE INDEX users_email_uq ON users (lower(email)) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX users_google_uq ON users (google_sub) WHERE google_sub IS NOT NULL;

-- Emailga yuborilgan bir martalik kodlar (faqat xeshi saqlanadi).
CREATE TABLE auth_codes (
  id          bigserial PRIMARY KEY,
  email       text NOT NULL,                -- kichik harfda
  code_hash   text NOT NULL,                -- sha256(email|code|pepper)
  attempts    int NOT NULL DEFAULT 0,
  expires_at  timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_codes_email_idx ON auth_codes (email, created_at DESC);

-- Refresh tokenlar: har yangilanishda almashtiriladi; qayta ishlatilsa — butun oila bekor qilinadi.
CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY,
  user_id     text NOT NULL REFERENCES users ON DELETE CASCADE,
  token_hash  text NOT NULL UNIQUE,         -- sha256
  device_id   text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  replaced_by uuid
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
