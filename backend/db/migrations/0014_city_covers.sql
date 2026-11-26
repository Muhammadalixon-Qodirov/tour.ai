-- 0014: shahar muqovasi uchun mualliflik ma'lumotlari (rasm static/cities da, binar ichida).
-- seed/city_covers.json dan yuklanadi; OSM importi va nomzodlar importi bu ustunlarga tegmaydi.

ALTER TABLE cities ADD COLUMN cover_credit     text;  -- muallif
ALTER TABLE cities ADD COLUMN cover_license    text;  -- "CC BY-SA 4.0"
ALTER TABLE cities ADD COLUMN cover_source_url text;  -- rasm sahifasi (Wikimedia Commons)
ALTER TABLE cities ADD COLUMN cover_source     text;  -- "Wikimedia Commons"
