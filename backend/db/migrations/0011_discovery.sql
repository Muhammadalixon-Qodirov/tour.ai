-- 0011: yangi joylarni avtomatik topish (Wikidata, OSM, foydalanuvchi takliflari, tadqiqot nomzodlari).
-- Har bir topilma bitta qator: kategoriya va ishonch avtomatik hisoblanadi; ishonch yuqori bo'lsa places ga
-- chop etiladi (place_id), past bo'lsa admin navbatida turadi. Takrorlar bazadagi joyga bog'lanadi.

ALTER TABLE places ADD COLUMN wikidata text;            -- Q123 (OSM wikidata tegi yoki Wikidata importi)
CREATE UNIQUE INDEX places_wikidata_uq ON places (wikidata) WHERE wikidata IS NOT NULL;

CREATE TABLE discoveries (
  id           bigserial PRIMARY KEY,
  source       text NOT NULL CHECK (source IN ('wikidata','osm','user','research')),
  ref          text NOT NULL,                          -- Q123 | node/1 | foydalanuvchi taklifi uuid | tadqiqot id
  name         jsonb NOT NULL,                         -- {"uz","ru","en"}
  lat          double precision NOT NULL,
  lng          double precision NOT NULL,
  city_id      text REFERENCES cities,
  category     text,
  confidence   real NOT NULL DEFAULT 0,
  novelty      text,
  interests    text[] NOT NULL DEFAULT '{}',
  reasons      text[] NOT NULL DEFAULT '{}',           -- kategoriya qarori sabablari: "osm:amenity=cafe", "name:muzey" ...
  data         jsonb NOT NULL DEFAULT '{}',            -- xom signallar: teglar, klasslar, tavsif, rasm, izoh, kontaktlar
  status       text NOT NULL DEFAULT 'queued'
               CHECK (status IN ('queued','published','duplicate','rejected','out_of_area')),
  place_id     text REFERENCES places ON DELETE SET NULL,
  submitted_by text REFERENCES users ON DELETE SET NULL,
  first_seen   timestamptz NOT NULL DEFAULT now(),
  last_seen    timestamptz NOT NULL DEFAULT now(),
  reviewed_by  text,
  reviewed_at  timestamptz,
  UNIQUE (source, ref)
);
CREATE INDEX discoveries_status_idx ON discoveries (status, confidence DESC, id);

CREATE TABLE discovery_runs (
  id          bigserial PRIMARY KEY,
  source      text NOT NULL,
  started_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  report      jsonb,
  error       text
);
