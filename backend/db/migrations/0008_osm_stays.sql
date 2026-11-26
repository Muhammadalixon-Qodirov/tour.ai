-- 0008: OpenStreetMap importi (osm_id), turar joy moduli, xizmat turlari kengaytirildi.

ALTER TABLE places ADD COLUMN osm_id text;
CREATE UNIQUE INDEX places_osm_uq ON places (osm_id) WHERE osm_id IS NOT NULL;

ALTER TABLE city_services ADD COLUMN osm_id text;
CREATE UNIQUE INDEX city_services_osm_uq ON city_services (osm_id) WHERE osm_id IS NOT NULL;
ALTER TABLE city_services DROP CONSTRAINT city_services_kind_check;
ALTER TABLE city_services ADD CONSTRAINT city_services_kind_check
  CHECK (kind IN ('hospital','clinic','pharmacy','police','embassy','atm','exchange','fuel','ev_charging'));

ALTER TABLE cities ADD COLUMN radius_km real NOT NULL DEFAULT 12;   -- import: shu radiusdagi obyektlar shu shaharga

CREATE TABLE stays (
  id            text PRIMARY KEY,
  city_id       text NOT NULL REFERENCES cities,
  kind          text NOT NULL CHECK (kind IN ('hotel','boutique','guesthouse','hostel','motel','apartment')),
  status        text NOT NULL DEFAULT 'published' CHECK (status IN ('draft','pending','published','rejected')),
  name          jsonb NOT NULL,                         -- {"uz":..,"ru":..,"en":..}
  lat           double precision NOT NULL,
  lng           double precision NOT NULL,
  address       text NOT NULL DEFAULT '',
  stars         smallint CHECK (stars BETWEEN 1 AND 5),
  price_min     bigint,                                 -- so'm / kecha, 2 kishi, oddiy xona
  price_max     bigint,
  price_trust   text CHECK (price_trust IN ('official','public','community','sample','estimate')),
  price_note    text,
  price_updated_at timestamptz,
  amenities     text[] NOT NULL DEFAULT '{}',           -- wifi, breakfast, ac, parking, transfer, courtyard
  contacts      jsonb NOT NULL DEFAULT '{}',            -- phone, website, telegram, instagram, email
  check_in      text,
  check_out     text,
  wheelchair    bool,
  emehmon       bool,                                   -- chet ellikni E-Mehmon orqali ro'yxatga oladimi
  rating        numeric(2,1),
  reviews_count int NOT NULL DEFAULT 0,
  images        jsonb NOT NULL DEFAULT '[]',
  booking_url   text,                                   -- hamkor havola (faqat web saytda ko'rsatiladi)
  agoda_url     text,
  is_partner    bool NOT NULL DEFAULT false,
  fair_price    bool NOT NULL DEFAULT false,            -- jamoa tekshirgan adolatli narx
  verified      bool NOT NULL DEFAULT false,
  trust         text NOT NULL DEFAULT 'public' CHECK (trust IN ('official','public','community','sample')),
  source        text NOT NULL DEFAULT 'manual',         -- manual | open_data | partner
  osm_id        text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stays_earth_gix ON stays USING gist (ll_to_earth(lat, lng));
CREATE INDEX stays_city_idx ON stays (city_id, status, kind);
CREATE UNIQUE INDEX stays_osm_uq ON stays (osm_id) WHERE osm_id IS NOT NULL;
