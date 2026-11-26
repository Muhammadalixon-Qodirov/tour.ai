-- 0015: Lens "bu qanday joy?" suhbati: rasmdan tanilgan joy bilan boshlanadigan yo'riqchi suhbati shu joyga bog'lanadi
-- (keyingi savollarda placeId yuborilmasa ham joy konteksti saqlanadi).
ALTER TABLE guide_conversations ADD COLUMN place_id text;
