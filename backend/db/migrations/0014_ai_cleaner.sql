-- 0014: AI "ma'lumot tozalovchi" — ichki tarmoqdagi AI server topilmalarni tozalaydi (kategoriya, tavsif, junk ajratish).
-- ai_cleaned_at: tozalovchi ko'rib chiqqan vaqt (NULL = navbatda, hali ko'rilmagan). Tavsiyalar data->'ai' da.
ALTER TABLE discoveries ADD COLUMN ai_cleaned_at timestamptz;
CREATE INDEX discoveries_ai_pending_idx ON discoveries (id) WHERE status = 'queued' AND ai_cleaned_at IS NULL;
