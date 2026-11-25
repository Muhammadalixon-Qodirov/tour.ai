-- 0005: tadqiqot orqali topilgan joylar uchun kontakt va tekshiruv ma'lumotlari.

ALTER TABLE places ADD COLUMN contacts jsonb NOT NULL DEFAULT '{}';   -- {"instagram":"@..","telegram":"@..","website":"https://..","phone":".."}
ALTER TABLE places ADD COLUMN research jsonb;                          -- {"confidence","coordsSource","priceNote","notes","sources":[...],"importedAt"}
