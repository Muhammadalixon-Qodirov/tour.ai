-- 0016: discoveries.source ga 'egov' (data.egov.uz rasmiy ochiq ma'lumot) manbasini qo'shish.
ALTER TABLE discoveries DROP CONSTRAINT IF EXISTS discoveries_source_check;
ALTER TABLE discoveries ADD CONSTRAINT discoveries_source_check
  CHECK (source IN ('wikidata','osm','user','research','egov'));
