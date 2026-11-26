-- 0007: rezident belgisi, joy nomlari (aliases), ishonch darajasi, saqlangan joylar, guruh safari.

ALTER TABLE users ADD COLUMN is_resident bool;          -- null = aytilmagan; kirish narxini to'g'ri ko'rsatish uchun

ALTER TABLE places ADD COLUMN aliases text[] NOT NULL DEFAULT '{}';   -- mahalliy nomlar: "Chorsu", "Eski shahar bozori"
CREATE INDEX places_aliases_gin ON places USING gin (aliases);

-- Ishonch darajasi: official (obyekt ma'muriyati, gov.uz) · public (ochiq manba) · community (foydalanuvchi) · sample (demo)
ALTER TABLE places ADD COLUMN trust text NOT NULL DEFAULT 'public'
  CHECK (trust IN ('official','public','community','sample'));
ALTER TABLE places ADD COLUMN verified_at timestamptz;
UPDATE places SET trust = 'sample' WHERE source = 'demo';

ALTER TABLE place_rules ADD COLUMN trust text NOT NULL DEFAULT 'public'
  CHECK (trust IN ('official','public','community','sample'));
UPDATE place_rules SET trust = CASE WHEN verified_by = 'demo' THEN 'sample'
                                    WHEN verified_by IS NOT NULL THEN 'official' ELSE 'public' END;

ALTER TABLE place_i18n ADD COLUMN history_trust text
  CHECK (history_trust IN ('official','public','community','sample'));
UPDATE place_i18n i SET history_trust = CASE WHEN p.source = 'demo' THEN 'sample' ELSE 'public' END
  FROM places p WHERE p.id = i.place_id AND i.history_summary IS NOT NULL;

-- Saqlangan joylar (yurakcha)
CREATE TABLE saved_places (
  user_id    text REFERENCES users ON DELETE CASCADE,
  place_id   text REFERENCES places ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, place_id)
);

-- Guruh safari
CREATE TABLE groups (
  id          uuid PRIMARY KEY,
  owner_id    text NOT NULL REFERENCES users ON DELETE CASCADE,
  name        text NOT NULL,
  city_id     text REFERENCES cities,
  invite_code text NOT NULL UNIQUE,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE group_members (
  group_id  uuid REFERENCES groups ON DELETE CASCADE,
  user_id   text REFERENCES users ON DELETE CASCADE,
  role      text NOT NULL CHECK (role IN ('owner','member')),
  budget    smallint CHECK (budget BETWEEN 1 AND 3),
  interests text[] NOT NULL DEFAULT '{}',
  pace      text CHECK (pace IN ('relaxed','balanced','active')),
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id);

CREATE TABLE group_votes (
  group_id   uuid REFERENCES groups ON DELETE CASCADE,
  user_id    text REFERENCES users ON DELETE CASCADE,
  place_id   text REFERENCES places ON DELETE CASCADE,
  vote       smallint NOT NULL CHECK (vote IN (-1, 1)),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id, place_id)
);

ALTER TABLE trips ADD COLUMN group_id uuid REFERENCES groups ON DELETE SET NULL;
CREATE INDEX trips_group_idx ON trips (group_id) WHERE group_id IS NOT NULL;
