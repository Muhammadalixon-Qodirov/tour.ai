# Yo'ldosh AI — backend (Go)

Reja: [../BACKEND_PLAN.md](../BACKEND_PLAN.md) · Kontrakt: [../API_CONTRACT.md](../API_CONTRACT.md)

## Ishga tushirish

```sh
cp .env.example .env        # bir marta
make db-start               # lokal PostgreSQL :5433 (Docker siz)
make seed                   # migratsiya + demo ma'lumot (Samarqand, Buxoro)
make run                    # API → http://localhost:8080
make test                   # hamma testlar (unit + haqiqiy bazada API testlari)
```

Admin hisobi (parol kamida 10 belgi, so'raladi):

```sh
make admin EMAIL=siz@example.com ROLE=admin
```

Docker bo'lsa: `docker compose up -d --build`, keyin
`docker compose run --rm --entrypoint /app/seed api /app/seed`.

Talablar: Go 1.26, PostgreSQL 14+ (contrib: `cube`, `earthdistance` — standart paketda bor). PostGIS **kerak emas**.

## Tezkor tekshiruv

```sh
B=http://localhost:8080/api/v1
TOK=$(curl -s -X POST $B/auth/device -H 'Content-Type: application/json' \
  -d '{"deviceId":"my-test-device","platform":"android","lang":"uz"}' | jq -r .token)

curl -s -X POST $B/trips -H "Authorization: Bearer $TOK" -H 'Accept-Language: uz' \
  -H 'Content-Type: application/json' \
  -d '{"cityId":"bukhara","startDate":"2026-12-02","days":2,"interests":["history","craft"],"pace":"balanced"}' | jq
```

## Hozir ishlaydigan endpointlar (`/api/v1`)

| Guruh | Endpoint |
|---|---|
| Sessiya | `POST /auth/device`, `GET /me`, `PATCH /me` |
| Katalog | `GET /cities`, `GET /places` (`city, category, novelty=iconic\|new\|hidden\|novel, interest, q, near=lat,lng, radiusM, openNow, wheelchair, limit, cursor`), `GET /places/{id}`, `GET /places/{id}/reviews` |
| Qoida | `GET /geofences?city=` |
| Marshrut | `POST /trips`, `GET /trips`, `GET /trips/{id}`, `POST /trips/{id}/stops`, `DELETE /trips/{id}/stops/{stopId}`, `POST /trips/{id}/stops/{stopId}/swap` |
| Moslashuvchan | `PATCH /trips/{id}/stops/{stopId}` (`status: visited\|skipped\|current\|upcoming`), `POST /trips/{id}/replan` (`reason: late+minutes \| tired \| weather \| free_time+minutes+location`, ixtiyoriy `dayIndex`) → `Trip` + `changes[]` |
| Baho | `POST /visits` |
| Kundalik | `POST /diary/photos` (multipart `image, tripId, dayIndex, stopId?`), `DELETE /diary/photos/{id}`, `GET /trips/{id}/days/{idx}/photos`, `POST·GET /trips/{id}/days/{idx}/diary` (`tone: warm\|short\|fun\|poetic`), `GET·PATCH /diary/{entryId}`, `POST /diary/{entryId}/regenerate`, `POST /diary/{entryId}/publish` (`visibility: public\|link\|private`), `GET /diary/{entryId}/share-card?format=story\|post` (PNG), `POST /diary/{entryId}/shares` (`target`), `POST /me/push-token` |
| Lenta | `GET /feed?city=`, `GET /posts/{id}`, `POST·DELETE /posts/{id}/like`, `POST /posts/{id}/report`, `DELETE /posts/{id}`; ochiq: `GET /s/{slug}` (HTML + OpenGraph), `GET /s/{slug}/card.png`; admin: `GET /admin/posts?status=reported`, `POST /admin/posts/{id}/hide·restore` |
| Admin | `POST /admin/auth/login`, `GET /admin/me`, `GET /admin/places?status=&city=&q=`, `POST /admin/places`, `GET·PATCH /admin/places/{id}`, `POST /admin/places/{id}/submit·publish·reject·unpublish`, `PUT /admin/places/{id}/rules`, `POST /admin/places/{id}/images` (multipart `image`, `credit`), `GET /admin/analytics/novelty?days=30` |
| Xizmat | `GET /healthz`, `GET /static/ph/{category}.svg` (rasm yo'q joylar uchun vaqtinchalik rasm), `GET /uploads/{file}` (yuklangan rasmlar) |

Til: `Accept-Language: uz | ru | en` (yo'q bo'lsa `en`; matn topilmasa `en` → `uz`).

## Tuzilma

```
cmd/api        HTTP server (ishga tushganda migratsiyalar avtomatik)
cmd/seed       seed/*.json → baza
cmd/admin      admin/moderator hisobini yaratish
db/migrations  SQL migratsiyalar (binar ichiga joylanadi)
internal/
  domain       sof tiplar, ish vaqti, masofa, marshrut invariantlari (Validate)
  planner      marshrut rejalashtirgichi: ball → kunlar → tartib → jadval → kvota
  store        PostgreSQL so'rovlari (pgx, parametrli SQL)
  api          router, middleware, handlerlar, DTO
  auth         JWT (HS256) sessiya tokenlari, argon2id parollar
  media        rasm yuklash: tekshirish, kichraytirish, EXIF/GPS ni o'chirish, saqlash
  diary        kun hikoyasi (shablon; AI keyin shu interfeysga) va ulashish kartasi (PNG)
  weather      Open-Meteo kunlik prognozi (kalitsiz, 1 soat kesh)
  i18n         tillar va foydalanuvchiga ko'rinadigan matnlar
  seed         seed yuklovchi
  platform     konfiguratsiya, DB ulanishi, migrator
seed/          demo ma'lumot (source='demo')
```

## BACKEND.md dan farqlar (ataylab)

| BACKEND.md | Bu yerda | Nega |
|---|---|---|
| PostGIS | `cube` + `earthdistance` (GiST indeks) | har qanday PostgreSQL da bor, o'rnatish shart emas; shahar hajmi uchun yetarli. Kerak bo'lsa keyin PostGIS ga o'tish oson (`store` ichida) |
| Redis rate limit | xotiradagi token bucket | bitta server nusxasi uchun yetarli; ko'p nusxada Redis ga ko'chiriladi |
| goose + sqlc + oapi-codegen | o'z migratori + qo'lda yozilgan pgx SQL | kamroq vosita, hammasi `go build` bilan |
| MinIO / S3 | lokal disk (`UPLOAD_DIR`) | `media.Store` interfeysi orqali; S3 keyin shu interfeysga ulanadi |

## Demo ma'lumot haqida

`seed/` dagi yozuvlar **namuna**: "(demo)" nomli joylar o'ylab topilgan, mashhur joylarning koordinatasi,
ish vaqti, reytingi va qoidalari taxminiy (`source='demo'`, qoidalarda `verified_by='demo'`).
Haqiqiy ma'lumot yig'ilganda shu fayllar almashtiriladi yoki admin panel orqali kiritiladi.

## Admin: joy hayot sikli

```
draft ──submit──▶ pending ──publish──▶ published ──unpublish──▶ draft
  │                  │                                   
  └──────reject──────┴──▶ rejected ──submit/publish──▶ …
```

- `publish` dan oldin joy to'liq tekshiriladi (uz nom va tagline, koordinata, ish vaqti va h.k.).
- Chop etilgan joy **darhol** keyingi marshrutlarga tushadi (kesh yo'q).
- `PUT /rules` → `rulesVersion` oshadi, ilova geofence ni yangilaydi.
- Har bir o'zgarish `admin_audit` jadvaliga yoziladi (kim, nima, oldin/keyin).
- Yuklangan rasm qayta kodlanadi: EXIF/GPS o'chadi, uzun tomoni 1600 px gacha kichrayadi.

## Qayta rejalash (`/replan`)

| `reason` | Nima bo'ladi |
|---|---|
| `late` + `minutes` | qolgan joylar suriladi; ish vaqti yoki 21:00 ga sig'maganlari olib tashlanadi (avval mashhur, eng past balli; kunning yagona yangi joyi oxirgi navbatda) |
| `tired` | qolgan joylarning yarmi qoladi — eng yaxshilari, yangi joy saqlanadi |
| `weather` | Open-Meteo: ≥ 35°C yoki yomg'ir bo'lsa park/manzara/eko-ovul → yaqin muzey/galereya/ustaxona (yangi joy o'rniga yangi joy). Ob-havo yaxshi bo'lsa o'zgarishsiz |
| `free_time` + `minutes` + `location` | 1.5 km ichida vaqtga sig'adigan ochiq joylar hozirdan boshlab qo'shiladi, qolgan reja keyinga suriladi |

"Bordim" / "o'tkazib yubordim" deb belgilangan va vaqti o'tgan to'xtashlar o'zgarmaydi.
Ob-havo servisiga faqat shahar koordinatasi (~1 km gacha yaxlitlangan) yuboriladi.

## Sayohat kundaligi va lenta

```
kun davomida: PATCH stop {status: visited} + POST /diary/photos
20:00 (mahalliy): push "Bugungi hikoyangiz tayyor ✨" — hikoya oldindan yoziladi (EXPO_PUSH_ENABLED=true)
POST /trips/{id}/days/{idx}/diary → hikoya (sarlavha, matn, lahzalar, heshteglar, caption, joylar, rasmlar)
PATCH /diary/{id} (tahrir) · POST /regenerate (ohang, 5 marta)
POST /publish → lentaga post + ochiq havola /s/{slug}  ·  GET /share-card → Instagram/Telegram uchun PNG
```

- Hikoya **faqat** borilgan joylar va bazadagi tasdiqlangan faktlardan yoziladi; AI ulanganda ham
  `diary.CheckGrounded` ro'yxatda yo'q joy nomini rad etadi va shablonga qaytadi.
- Rasmlardan EXIF/GPS olib tashlanadi; postda faqat shahar va sana ko'rinadi; muallif — taxallus.
- Postda havola (`http`, `t.me/` …) bo'lsa rad etiladi. 3 ta turli shikoyat → post avtomatik yashiriladi,
  admin `restore` qiladi.
- Push: kuniga bitta, faqat kamida 2 ta joyga "bordim" belgisi qo'yilgan bo'lsa.
- Geofence orqali "Rasmga oling" eslatmasi — ilova tomonida (lokal bildirishnoma).
