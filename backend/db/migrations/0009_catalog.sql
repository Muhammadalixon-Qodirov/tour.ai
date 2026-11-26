-- 0009: to'liq katalog (ilova va sayt bilan bir xil), narxlar, tadbirlar, to'xtash bahosi.
-- Turar joy jadvali (stays) va users.is_resident 0007/0008 da yaratilgan; bu yerda faqat qo'shimcha ustunlar.

-- Joy: olomon darajasi; matnlar (eng yaxshi vaqt, ish vaqti va narx izohi) tilga bog'liq.
ALTER TABLE places ADD COLUMN crowd text CHECK (crowd IN ('low','medium','high'));
ALTER TABLE place_i18n ADD COLUMN best_time  text;
ALTER TABLE place_i18n ADD COLUMN hours_note text;
ALTER TABLE place_i18n ADD COLUMN fee_note   text;
-- Qoida manbasi: URL yoki "general etiquette ..." (verified_by esa obyekt ma'muriyati tasdiqlaganda).
ALTER TABLE place_rules ADD COLUMN source text;

-- To'xtash: tashrifdan keyingi baho (1-5). Baho visits jadvaliga ham yoziladi.
ALTER TABLE stops ADD COLUMN rating smallint CHECK (rating BETWEEN 1 AND 5);

-- ---------- Narxlar ("Adolatli narx") ----------
-- base_min/base_max — tadqiqot (ochiq manba) oralig'i. Kuzatuvlar yetarli bo'lsa oraliq ulardan hisoblanadi.
CREATE TABLE price_items (
  id          text PRIMARY KEY,
  city_id     text NOT NULL REFERENCES cities,
  category    text NOT NULL CHECK (category IN ('taxi','food','souvenir','market','ticket','transport')),
  name        jsonb NOT NULL,                 -- {"uz","ru","en"}
  unit        jsonb NOT NULL,                 -- {"uz","ru","en"}
  base_min    bigint NOT NULL CHECK (base_min >= 0),
  base_max    bigint NOT NULL CHECK (base_max >= base_min),
  tourist     bigint,                         -- turistga odatda aytiladigan narx
  place_ids   text[] NOT NULL DEFAULT '{}',   -- qaysi joyga tegishli (kirish chiptasi)
  source_url  text,
  source_year smallint,
  note        text,
  base_updated_at timestamptz NOT NULL DEFAULT now(),
  source      text NOT NULL DEFAULT 'open_data'
);
CREATE INDEX price_items_city_idx ON price_items (city_id, category);

-- Foydalanuvchi kuzatuvi. Joylashuv ~100 m gacha yaxlitlanib saqlanadi; foydalanuvchi o'chirilsa kuzatuv qoladi (anonim).
CREATE TABLE price_reports (
  id            uuid PRIMARY KEY,
  price_item_id text REFERENCES price_items ON DELETE SET NULL,
  city_id       text REFERENCES cities,
  user_id       text REFERENCES users ON DELETE SET NULL,
  name          text NOT NULL,
  category      text NOT NULL CHECK (category IN ('taxi','food','souvenir','market','ticket','transport')),
  paid          bigint NOT NULL CHECK (paid > 0),
  buyer         text CHECK (buyer IN ('local','tourist')),
  lat           double precision,
  lng           double precision,
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX price_reports_item_idx ON price_reports (price_item_id, status);

-- ---------- Tadbirlar ----------
CREATE TABLE events (
  id         text PRIMARY KEY,
  city_id    text NOT NULL REFERENCES cities,
  kind       text NOT NULL CHECK (kind IN ('festival','concert','fair','exhibition','sport')),
  title      jsonb NOT NULL,
  venue      jsonb NOT NULL,
  starts_at  timestamptz NOT NULL,
  ends_at    timestamptz NOT NULL,
  lat        double precision NOT NULL,
  lng        double precision NOT NULL,
  price      bigint,                          -- NULL: narx noma'lum yoki bepul (is_free)
  is_free    bool NOT NULL DEFAULT false,
  ticket_url text,
  source_url text,
  recurring  text,
  image      jsonb,                           -- {url, credit, license, sourceUrl, source}
  status     text NOT NULL DEFAULT 'published' CHECK (status IN ('draft','published')),
  source     text NOT NULL DEFAULT 'open_data',
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX events_city_idx ON events (city_id, starts_at);

-- ---------- Turar joylar (0008 dagi stays ga qo'shimcha) ----------
-- Tadqiqot yozuvlari (source = research): narx va reyting qayerdan olingani. Reyting 5 ballik
-- (Booking kabi 10 ballik manbada 2 ga bo'lingan).
ALTER TABLE stays ADD COLUMN price_source  text;
ALTER TABLE stays ADD COLUMN rating_source text;

-- ---------- Ishonch darajasi (0007 dagi trust ga qo'shimcha) ----------
-- Ish vaqti alohida: tadqiqotda joy ma'lumoti ochiq manbadan (public), ish vaqti esa namuna (sample) bo'lishi mumkin.
ALTER TABLE places ADD COLUMN hours_trust text CHECK (hours_trust IN ('official','public','community','sample'));
UPDATE places SET hours_trust = trust;
-- Narx bazasi (base_min/base_max) ning ishonch darajasi; kuzatuvlar yetarli bo'lsa javobda community.
ALTER TABLE price_items ADD COLUMN trust text NOT NULL DEFAULT 'sample'
  CHECK (trust IN ('official','public','community','sample'));
