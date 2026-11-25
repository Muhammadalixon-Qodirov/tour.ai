-- 0002: admin foydalanuvchilar, audit, moderatsiya maydonlari.

CREATE TABLE admin_users (
  id            text PRIMARY KEY,
  email         text NOT NULL UNIQUE,
  name          text NOT NULL DEFAULT '',
  password_hash text NOT NULL,                     -- argon2id
  role          text NOT NULL CHECK (role IN ('moderator','admin')),
  disabled      bool NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz
);

CREATE TABLE admin_audit (
  id        bigserial PRIMARY KEY,
  admin_id  text NOT NULL REFERENCES admin_users,
  action    text NOT NULL,                         -- place.create | place.update | place.publish | place.reject | place.rules | place.image
  entity    text NOT NULL,
  entity_id text NOT NULL,
  before    jsonb,
  after     jsonb,
  at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_entity_idx ON admin_audit (entity, entity_id, at DESC);

ALTER TABLE places ADD COLUMN reject_reason text;
ALTER TABLE places ADD COLUMN created_by text;
