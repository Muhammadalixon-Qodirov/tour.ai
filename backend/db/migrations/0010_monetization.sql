-- 0010: daromad funksiyalari (docs/MONETIZATION.md): hamkorlar va takliflar, bosish/ko'rsatish/sotuv hisobi,
-- shaharlararo yo'nalishlar, safar uchun "Qanday borasiz?" (arrival) va "Safarga tayyorgarlik" (prep),
-- Lens Story va bir martalik chegirma kodlari, Safar Insights (rol, hudud, statistikadan chiqish).
--
-- Qoidalar: hamkor doim "Hamkor" deb belgilanadi (API javobida partner + sample), taklif ixtiyoriy,
-- statistikada shaxsiy ma'lumot yo'q (bosishda IP, ism, joylashuv saqlanmaydi).

-- ---------- Hamkorlar ----------
CREATE TABLE partners (
  id           text PRIMARY KEY,
  name         text NOT NULL,
  city_id      text REFERENCES cities,              -- joyga bog'langan hamkor (Lens Story chegirmasi) uchun
  lat          double precision,
  lng          double precision,
  discount_pct smallint CHECK (discount_pct BETWEEN 1 AND 50), -- story chegirmasi; NULL = story dasturida emas
  website      text,
  active       bool NOT NULL DEFAULT true,
  sample       bool NOT NULL DEFAULT false,         -- demo hamkor: UI da "Namuna hamkor"
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK ((lat IS NULL) = (lng IS NULL))
);
CREATE INDEX partners_story_gix ON partners USING gist (ll_to_earth(lat, lng)) WHERE discount_pct IS NOT NULL AND lat IS NOT NULL;

-- Hamkor API kaliti: faqat sha256 xeshi saqlanadi, kalit bir marta ko'rsatiladi.
CREATE TABLE partner_keys (
  id           uuid PRIMARY KEY,
  partner_id   text NOT NULL REFERENCES partners ON DELETE CASCADE,
  key_hash     text NOT NULL UNIQUE,
  prefix       text NOT NULL,                       -- "spk_ab12" — kabinetda qaysi kalit ekanini tanish uchun
  created_at   timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  revoked_at   timestamptz
);
CREATE INDEX partner_keys_partner_idx ON partner_keys (partner_id);

-- ---------- Takliflar ----------
-- Bitta slotga bir nechta taklif mos kelsa, eng yuqori priority dan biri kontekst bo'yicha aylantirib tanlanadi.
CREATE TABLE offers (
  id            text PRIMARY KEY,
  partner_id    text NOT NULL REFERENCES partners,
  slot          text NOT NULL CHECK (slot IN ('transport.cheapest','transport.fastest','transport.comfort','transport.bus',
                  'stay','food','event','prep.esim','prep.insurance','prep.sim')),
  city_id       text REFERENCES cities,             -- NULL = hamma shahar
  route_from    text,                               -- NULL = hamma yo'nalish; ikkala tomonga bir xil
  route_to      text,
  mode          text CHECK (mode IN ('train','afrosiyob','plane','car','bus')), -- transport sloti: qaysi transport turi
  valid_from    date,                               -- safar sanasi oynasi
  valid_to      date,
  days_min      smallint,
  days_max      smallint,
  resident      bool,                               -- NULL = hammaga; false = faqat chet ellik; true = faqat rezident
  title         jsonb NOT NULL,                     -- {"uz","ru","en"}; {d} = safar kunlari
  body          jsonb,
  cta           jsonb NOT NULL,
  price_from    bigint CHECK (price_from >= 0),     -- so'm
  price_unit    text NOT NULL DEFAULT 'trip' CHECK (price_unit IN ('trip','day')), -- day: price_from * kunlar
  link_template text,                               -- {subId} {offerId} {from} {to} {date} {days}; sample bo'lsa NULL bo'lishi mumkin
  priority      smallint NOT NULL DEFAULT 0,
  active        bool NOT NULL DEFAULT true,
  sample        bool NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK ((route_from IS NULL) = (route_to IS NULL)),
  CHECK (days_min IS NULL OR days_max IS NULL OR days_min <= days_max),
  CHECK (valid_from IS NULL OR valid_to IS NULL OR valid_from <= valid_to)
);
CREATE INDEX offers_slot_idx ON offers (slot) WHERE active;

-- Ko'rsatishlar: anonim, kunlik yig'ma (kim ko'rgani saqlanmaydi).
CREATE TABLE offer_impressions (
  offer_id text NOT NULL REFERENCES offers ON DELETE CASCADE,
  day      date NOT NULL,
  count    bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (offer_id, day)
);

-- Bosishlar. id hamkorga sub-id sifatida beriladi va sotuv xabarida (conversions.clickId) qaytadi.
CREATE TABLE offer_clicks (
  id         text PRIMARY KEY,
  offer_id   text NOT NULL REFERENCES offers ON DELETE CASCADE,
  partner_id text NOT NULL REFERENCES partners ON DELETE CASCADE,
  user_id    text REFERENCES users ON DELETE SET NULL,
  trip_id    uuid REFERENCES trips ON DELETE SET NULL,
  issued_at  timestamptz NOT NULL,                  -- taklif ko'rsatilgan vaqt (clickId ichida)
  clicked_at timestamptz NOT NULL DEFAULT now(),
  sample     bool NOT NULL DEFAULT false
);
CREATE INDEX offer_clicks_partner_idx ON offer_clicks (partner_id, clicked_at);

CREATE TABLE offer_conversions (
  id          uuid PRIMARY KEY,
  click_id    text NOT NULL REFERENCES offer_clicks ON DELETE CASCADE,
  partner_id  text NOT NULL REFERENCES partners ON DELETE CASCADE,
  amount      bigint NOT NULL CHECK (amount >= 0),
  currency    text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  external_id text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (partner_id, external_id)
);
CREATE INDEX offer_conversions_partner_idx ON offer_conversions (partner_id, created_at);

-- ---------- Shaharlararo yo'nalishlar ("Qanday borasiz?") ----------
-- Seed (seed/monetization.json) dan: har raqam manbasi va sanasi bilan; manba bo'lmasa estimate = true.
-- Har yo'nalish alohida qator (from → to): to'g'ridan-to'g'ri raqamlar ikki tomonga bir xil, Toshkent orqali
-- yo'nalishda manba tartibi yo'nalishga bog'liq.
CREATE TABLE intercity_routes (
  from_city    text NOT NULL,
  to_city      text NOT NULL,
  position     smallint NOT NULL,
  mode         text NOT NULL CHECK (mode IN ('train','afrosiyob','plane','car','bus')),
  duration_min int NOT NULL CHECK (duration_min > 0),
  price_from   bigint CHECK (price_from >= 0),      -- kishi boshiga; car uchun mashina uchun
  source       text NOT NULL,
  source_url   text,
  updated_at   date NOT NULL,
  estimate     bool NOT NULL DEFAULT false,
  via          text,
  PRIMARY KEY (from_city, to_city, mode),
  CHECK (from_city <> to_city)
);

-- ---------- Safar: arrival va prep ----------
ALTER TABLE trips ADD COLUMN arrival jsonb;           -- {from, mode, choice?, priceFrom?}
ALTER TABLE trips ADD COLUMN prep jsonb;              -- {route|transport|esim|sim|insurance|currency: done|skipped}

-- ---------- Lens Story va chegirma kodlari ----------
CREATE TABLE stories (
  share_id   text PRIMARY KEY,
  user_id    text NOT NULL REFERENCES users ON DELETE CASCADE,
  place_id   text NOT NULL REFERENCES places,
  lat        double precision,                      -- ~10 m gacha yaxlitlangan; faqat "joydami" tekshiruvi uchun
  lng        double precision,
  created_at timestamptz NOT NULL DEFAULT now(),
  shared_at  timestamptz,
  ref_count  int NOT NULL DEFAULT 0                 -- havola orqali kelganlar
);
CREATE INDEX stories_user_idx ON stories (user_id, created_at DESC);

CREATE TABLE discount_codes (
  id           text PRIMARY KEY,
  value        text NOT NULL UNIQUE,                -- SF-XXXX-XXXX
  user_id      text REFERENCES users ON DELETE SET NULL,
  partner_id   text NOT NULL REFERENCES partners ON DELETE CASCADE,
  place_id     text REFERENCES places,
  share_id     text REFERENCES stories ON DELETE SET NULL,
  discount_pct smallint NOT NULL CHECK (discount_pct BETWEEN 1 AND 50),
  issued_at    timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz NOT NULL,
  used_at      timestamptz,
  sample       bool NOT NULL DEFAULT false
);
CREATE INDEX discount_codes_user_idx ON discount_codes (user_id, partner_id, issued_at DESC);
CREATE INDEX discount_codes_partner_idx ON discount_codes (partner_id, issued_at);

-- ---------- Rollar va Insights ----------
-- users.role: tourist | partner | insights. Hamkor xodimi partner_id ga bog'lanadi; insights hududlar bilan cheklanadi.
ALTER TABLE users ADD COLUMN partner_id text REFERENCES partners ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN analytics_opt_out bool NOT NULL DEFAULT false;
UPDATE users SET role = 'tourist' WHERE role NOT IN ('tourist','partner','insights');
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('tourist','partner','insights'));

CREATE TABLE insights_regions (
  user_id text REFERENCES users ON DELETE CASCADE,
  region  text REFERENCES cities,
  PRIMARY KEY (user_id, region)
);

CREATE INDEX trips_city_start_idx ON trips (city_id, start_date);
