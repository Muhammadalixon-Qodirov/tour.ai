# Nomzod joylar (tadqiqot natijasi)

Bu papkadagi fayllar — ochiq internetdagi manbalardan qo'lda o'rganib topilgan **nomzod** joylar.
Ular bazaga `status='pending'`, `source='research'` bilan tushadi va admin tekshirib `publish` qilmaguncha turistga ko'rinmaydi.

## Qoidalar (majburiy)

1. **To'qima yo'q.** Har bir joy haqiqatda mavjudligi kamida bitta ochiq manba bilan tasdiqlangan bo'lsin (yaxshisi 2 ta). Ishonch past bo'lsa `confidence: "low"` qo'ying yoki umuman qo'shmang.
2. **Nusxa yo'q.** Nom, tagline va tavsif — o'z so'zingiz bilan qisqa (manba matnini ko'chirmang). Rasm yuklamang, rasm URL ham yozmang.
3. **Scraping yo'q.** Instagram/Telegram dan avtomatik yig'ish taqiqlangan. Ruxsat: qidiruv tizimi natijalari, yangiliklar saytlari, bloglar, rasmiy saytlar, joyning o'z sayti; Telegram kanalning ochiq veb-ko'rinishi (`https://t.me/s/<kanal>`) dan faqat bir nechta postni **ko'rib chiqish** (topish uchun), keyin boshqa manba bilan tasdiqlash.
4. **Koordinata faqat ishonchli manbadan**: OpenStreetMap (nominatim.openstreetmap.org yoki openstreetmap.org dagi obyekt), Wikipedia/Wikidata, joyning rasmiy sayti, manbadagi xarita havolasi. Taxmin qilmang. Topilmasa `lat`/`lng` = `null`, `coordsSource: "none"`.
5. Shaxsiy ma'lumot yozmang (uy egasining shaxsiy telefoni va h.k.). Biznesning ommaviy Instagram/Telegram akkaunti — mumkin.

## Fayl formati (`<hudud>.json`)

```json
{
  "region": "samarkand",
  "cities": [
    {"id": "samarkand", "name": {"uz": "Samarqand", "ru": "Самарканд", "en": "Samarkand"},
     "center": {"lat": 39.6542, "lng": 66.9597}}
  ],
  "places": [
    {
      "id": "lotin-kichik-harf-slug",
      "cityId": "samarkand",
      "category": "cafe",
      "novelty": "new",
      "openedAt": "2025-06",
      "name": {"uz": "...", "ru": "...", "en": "..."},
      "tagline": {"uz": "...", "ru": "...", "en": "..."},
      "description": {"uz": "...", "ru": "...", "en": "..."},
      "address": "ko'cha, mo'ljal",
      "lat": 39.65, "lng": 66.97, "coordsSource": "osm",
      "interests": ["food"],
      "priceLevel": 2,
      "priceNote": "o'rtacha chek 80–120 ming so'm (manba, sana)",
      "hours": "09:00-22:00",
      "instagram": "@handle", "telegram": "@handle", "website": "https://...",
      "sources": [{"title": "Kun.uz: ...", "url": "https://...", "date": "2025-07-01"}],
      "confidence": "high",
      "notes": "nimani tekshirish kerak"
    }
  ]
}
```

- `category`: `monument | museum | mosque | bazaar | workshop | gallery | restaurant | cafe | park | viewpoint | ecovillage`
- `novelty`: `new` (so'nggi ~2 yilda ochilgan) · `hidden` (kam tanilgan, mahalliylar biladi) · `iconic` (hamma biladi — bu tadqiqotda kam bo'lsin)
- `interests`: `history | food | nature | craft | art | architecture | shopping | nightlife`
- `priceLevel`: 0 bepul · 1 arzon · 2 o'rtacha · 3 qimmat · `null` noma'lum
- `hours`: har kuni bir xil bo'lsa `"HH:MM-HH:MM"`, noma'lum bo'lsa `null`
- `id`: lotin kichik harf, raqam, chiziqcha; hudud prefiksi bilan (`tash-`, `sam-`, `bux-`, `xiva-`, `fer-`, `nat-`)
