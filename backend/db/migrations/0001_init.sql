-- 0001: katalog, foydalanuvchi, marshrut.
-- Geo: PostGIS o'rniga standart contrib kengaytmalari (cube + earthdistance).
-- Har qanday PostgreSQL da bor; shahar hajmidagi ma'lumot uchun yetarli.

CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

CREATE TABLE cities (
  id              text PRIMARY KEY,                -- 'samarkand'
  country         text NOT NULL DEFAULT 'UZ',
  lat             double precision NOT NULL,
  lng             double precision NOT NULL,
  cover_url       text,
  cover_blurhash  text,
  timezone        text NOT NULL DEFAULT 'Asia/Tashkent',
  taxi_base_fare  bigint NOT NULL DEFAULT 10000,   -- so'm
  taxi_per_km     bigint NOT NULL DEFAULT 3000,    -- so'm
  emergency       jsonb NOT NULL DEFAULT '{}'      -- {"police":"102",...}
);

CREATE TABLE city_i18n (
  city_id text REFERENCES cities ON DELETE CASCADE,
  lang    text,
  name    text NOT NULL,
  PRIMARY KEY (city_id, lang)
);

CREATE TABLE places (
  id                 text PRIMARY KEY,
  city_id            text NOT NULL REFERENCES cities,
  category           text NOT NULL,
  novelty            text NOT NULL CHECK (novelty IN ('iconic','new','hidden')),
  opened_at          date,
  status             text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('draft','pending','published','rejected')),
  lat                double precision NOT NULL,
  lng                double precision NOT NULL,
  geofence_radius_m  int NOT NULL DEFAULT 80,
  interests          text[] NOT NULL DEFAULT '{}',
  price_level        smallint NOT NULL DEFAULT 0 CHECK (price_level BETWEEN 0 AND 3),
  visit_duration_min int NOT NULL DEFAULT 45 CHECK (visit_duration_min > 0),
  opening_hours      jsonb NOT NULL DEFAULT '[]',  -- [{day,open,close}], day 0 = yakshanba
  entry_fee          jsonb,
  wheelchair         bool NOT NULL DEFAULT false,
  family_friendly    bool NOT NULL DEFAULT true,
  easy_walk          bool NOT NULL DEFAULT true,
  pay_card           bool NOT NULL DEFAULT false,
  cash_only          bool NOT NULL DEFAULT false,
  is_partner         bool NOT NULL DEFAULT false,
  rating             numeric(2,1) NOT NULL DEFAULT 0,
  reviews_count      int NOT NULL DEFAULT 0,
  rules_version      int NOT NULL DEFAULT 1,
  images             jsonb NOT NULL DEFAULT '[]',  -- [{url,blurhash,credit}], birinchisi = cover
  source             text NOT NULL DEFAULT 'manual', -- 'manual' | 'demo' | 'open_data' | 'partner'
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  published_at       timestamptz
);
CREATE INDEX places_earth_gix ON places USING gist (ll_to_earth(lat, lng));
CREATE INDEX places_city_idx  ON places (city_id, status, novelty);
CREATE INDEX places_int_gin   ON places USING gin (interests);

CREATE TABLE place_i18n (
  place_id        text REFERENCES places ON DELETE CASCADE,
  lang            text,
  name            text NOT NULL,
  tagline         text NOT NULL,
  description     text NOT NULL DEFAULT '',
  address         text NOT NULL DEFAULT '',
  history_summary text,
  history_facts   text[] NOT NULL DEFAULT '{}',
  sources         jsonb NOT NULL DEFAULT '[]',
  PRIMARY KEY (place_id, lang)
);

CREATE TABLE place_rules (
  id          bigserial PRIMARY KEY,
  place_id    text NOT NULL REFERENCES places ON DELETE CASCADE,
  code        text NOT NULL,
  severity    text NOT NULL CHECK (severity IN ('info','warn','forbidden')),
  priority    smallint NOT NULL DEFAULT 0,
  text        jsonb NOT NULL,                      -- {"uz":"...","ru":"...","en":"..."}
  verified_by text,
  verified_at timestamptz
);
CREATE INDEX place_rules_place_idx ON place_rules (place_id);

CREATE TABLE users (
  id           text PRIMARY KEY,
  lang         text NOT NULL DEFAULT 'uz',
  role         text NOT NULL DEFAULT 'tourist',
  interests    text[] NOT NULL DEFAULT '{}',
  companion    text,
  pace         text,
  budget       smallint,
  display_name text,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE devices (
  id           text PRIMARY KEY,
  user_id      text NOT NULL REFERENCES users ON DELETE CASCADE,
  platform     text,
  push_token   text,
  timezone     text,
  last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trips (
  id            uuid PRIMARY KEY,
  user_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
  city_id       text NOT NULL REFERENCES cities,
  status        text NOT NULL CHECK (status IN ('generating','ready','failed')),
  start_date    date NOT NULL,
  prefs         jsonb NOT NULL,
  summary       text,                               -- AI yozgan bo'lsa; bo'lmasa shablon
  novelty_share real NOT NULL DEFAULT 0,
  planner       text NOT NULL DEFAULT 'fallback',   -- 'ai' | 'fallback'
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX trips_user_idx ON trips (user_id, created_at DESC);

CREATE TABLE trip_days (
  trip_id uuid REFERENCES trips ON DELETE CASCADE,
  idx     smallint,
  date    date NOT NULL,
  PRIMARY KEY (trip_id, idx)
);

CREATE TABLE stops (
  id           uuid PRIMARY KEY,
  trip_id      uuid NOT NULL,
  day_idx      smallint NOT NULL,
  position     smallint NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('place','meal')),
  place_id     text NOT NULL REFERENCES places,
  arrive_min   int NOT NULL,                        -- kun boshidan daqiqa (09:30 = 570)
  duration_min int NOT NULL,
  reason_code  text NOT NULL,                       -- shablon kaliti yoki 'ai'
  reason_arg   text NOT NULL DEFAULT '',
  reason_text  text,                                -- AI matni (reason_code = 'ai')
  transit      jsonb,                               -- {mode,durationMin,distanceM,fare}
  status       text NOT NULL DEFAULT 'upcoming'
               CHECK (status IN ('upcoming','current','visited','skipped')),
  FOREIGN KEY (trip_id, day_idx) REFERENCES trip_days ON DELETE CASCADE
);
CREATE INDEX stops_trip_idx ON stops (trip_id, day_idx, position);

CREATE TABLE visits (
  id         uuid PRIMARY KEY,
  user_id    text NOT NULL REFERENCES users ON DELETE CASCADE,
  place_id   text NOT NULL REFERENCES places,
  trip_id    uuid,
  rating     smallint CHECK (rating BETWEEN 1 AND 5),
  comment    text,
  created_at timestamptz NOT NULL DEFAULT now(),
  visit_date date NOT NULL DEFAULT current_date,
  UNIQUE (user_id, place_id, visit_date)
);
