-- 0004: SOS va to'lov xizmatlari (shifoxona, dorixona, politsiya, elchixona, bankomat, valyuta ayirboshlash).

CREATE TABLE city_services (
  id         text PRIMARY KEY,
  city_id    text NOT NULL REFERENCES cities,
  kind       text NOT NULL CHECK (kind IN ('hospital','pharmacy','police','embassy','atm','exchange')),
  name       jsonb NOT NULL,                        -- {"uz":"...","ru":"...","en":"..."}
  address    jsonb NOT NULL DEFAULT '{}',
  lat        double precision NOT NULL,
  lng        double precision NOT NULL,
  phone      text,
  hours      jsonb NOT NULL DEFAULT '[]',           -- [{day,open,close}]; bo'sh = noma'lum
  open_24h   bool NOT NULL DEFAULT false,
  accepts    text[] NOT NULL DEFAULT '{}',          -- bankomat: {"uzcard","humo","visa","mastercard"}
  source     text NOT NULL DEFAULT 'manual',        -- manual | demo | open_data | partner
  verified_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX city_services_earth_gix ON city_services USING gist (ll_to_earth(lat, lng));
CREATE INDEX city_services_city_kind ON city_services (city_id, kind);
