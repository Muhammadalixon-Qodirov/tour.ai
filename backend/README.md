# Yo'ldosh AI — backend (Go)

Reja: [../BACKEND_PLAN.md](../BACKEND_PLAN.md) · Kontrakt: [../API_CONTRACT.md](../API_CONTRACT.md)

## Ishga tushirish

```sh
cp .env.example .env        # bir marta
make db-start               # lokal PostgreSQL :5433 (Docker siz)
make seed                   # migratsiya + katalog (4 shahar, 171 joy, narx, tadbir, turar joy) + SOS demo
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
| Sessiya | `POST /auth/device` (anonim), `GET /me`, `PATCH /me` (`lang, displayName, interests, companion, pace, budget, isResident, analyticsOptOut`), `DELETE /me` (hisobni o'chirish) |
| Kirish | `POST /auth/email/start` `{email}` → 6 xonali kod emailga · `POST /auth/email/verify` `{email, code, deviceId?}` · `POST /auth/google` `{idToken, deviceId?}` · `POST /auth/refresh` `{refreshToken}` · `POST /auth/logout` `{refreshToken}` · `POST /auth/logout-all` |
| Katalog | `GET /cities`, `GET /places` (`city, category, novelty=iconic\|new\|hidden\|novel, interest, q, near=lat,lng, radiusM, openNow, wheelchair, limit, cursor`), `GET /places/{id}` (`hoursNote, entryFee{local,foreigner,note,source}, feeNote, crowd{level,bestTime}, rules[].source, images[].credit/license/sourceUrl`, `nearbyPrices`), `GET /places/{id}/reviews` |
| Narx | `GET /prices?city=&category=&q=` → `PriceItem[]`, `POST /prices/reports` (token ixtiyoriy) `{priceItemId?, cityId?, name?, category?, paid: {amount}, buyer?, location?}` → `201 {id, status: approved\|pending, priceItem?}` |
| Tadbir | `GET /events?city=&from=&to=` (o'tganlari yo'q) |
| Qoida | `GET /geofences?city=` |
| Joy sahifasi pastida | `GET /places/{id}/nearby?limit=5` — **bitta so'rovda**: yaqin turar joylar (turli narxda, foydalanuvchi byudjetiga mos), ovqatlanish (hozir ochiqlari ustun), yana nima ko'rish mumkin (kam tanilganlar ustun), eng yaqin bankomat/dorixona/valyuta; har birida `distanceM`, `walkMin`, `reason` |
| Joy qo'llanmasi | `GET /places/{id}/guide?from=lat,lng` — kartochka uchun hammasi bitta so'rovda: `summary` (sodda tushuntirish qatorlari: kirish narxi, bugungi ish vaqti, piyoda/taksi narxi yoki boshqa shahardan poyezd/avtobus, yaqin ovqat va odatiy narx, eng arzon tunash, muhim qoida), `visit` (kirish narxi, bugun ochiqmi, qoidalar, to'lov), `getThere` (`estimates`: piyoda daqiqa, taksi ≈ narx oralig'i; `intercity`; `navigation` — Yandex Maps/Go/Navi, 2GIS, Google havolalari), `food`, `foodPrices`, `stays`, `services`. `from` saqlanmaydi. `GET /places/{id}` da ham `navigation` bor |
| SOS va to'lov | `GET /cities/{id}/emergency?near=lat,lng` (raqamlar + eng yaqin shifoxona/dorixona/politsiya + hujjat yo'qolsa yo'riqnoma), `GET /services?city=&kind=hospital,pharmacy,police,embassy,atm,exchange&near=&radiusM=&openNow=` |
| Turar joy | `GET /stays?city=&kind=hotel,hostel,guesthouse&near=lat,lng&radiusM=&maxPrice=&q=` · `GET /stays/{id}` (tadqiqot yozuvlarida `ratingSource`, `pricePerNight.source`) · admin: `GET /admin/stays`, `PATCH /admin/stays/{id}` (aniq narx, kontakt, E-Mehmon, bron havolalari) |
| Oflayn | `GET /cities/{id}/offline-pack` — joylar, qoidalar, geofence, xizmatlar, SOS bitta JSON da; `ETag` → `304`, gzip |
| Marshrut | `POST /trips` (`days` 1..7, `budgetPerDay`, `isResident` (yo'q bo'lsa profildan), `groupSize`, `startLocation`, `startLabel` (≤ 80), `startKind: stay\|location\|station\|airport\|custom`), `GET /trips`, `GET /trips/{id}` (javobda `prefs`, har kunda `startLeg`), `DELETE /trips/{id}`, `POST /trips/{id}/stops` `{dayIndex, placeId, after?}`, `DELETE /trips/{id}/stops/{stopId}`, `POST /trips/{id}/stops/{stopId}/swap` (takliflar), `POST /trips/{id}/stops/{stopId}/move` `{dayIndex?, position?}` yoki `{step: -1\|1}` |
| Moslashuvchan | `PATCH /trips/{id}/stops/{stopId}` (`status: visited\|skipped\|current\|upcoming`, `rating: 1..5`, yoki `placeId` — almashtirish), `POST /trips/{id}/replan` (`reason: late+minutes \| tired \| weather \| free_time+minutes+location`, ixtiyoriy `dayIndex`) → `Trip` + `changes[]` |
| Baho | `POST /visits` (3 ta baho yig'ilgach joy reytingi haqiqiy baholardan hisoblanadi) |
| AI yordamchi | `POST /guide/chat` `{message, conversationId?, cityId?, placeId?, tripId?, location?}` — `Accept: text/event-stream` bo'lsa SSE (`delta` {text} → `places` {places: PlaceSummary[]} → `sources` {sources: {title, url?}[]} (bo'lsa) → `done` {conversationId, messageId, mode: ai\|fallback}), `?stream=false` — bitta JSON `{text, places, sources, conversationId, mode}` (15/daq); `tripId` faqat o'z marshruti (`404 trip_not_found`) · suhbatlar: `GET /guide/conversations` (eng yangisi birinchi: `title, lastText, messageCount, updatedAt`), `GET /guide/conversations/{id}` (xabarlar tartib bilan, javoblarda `places` kartochkalari), `PATCH /guide/conversations/{id}` `{title}`, `DELETE /guide/conversations/{id}` |
| Lens: joy va narx | `POST /lens/identify` multipart `image` (JPEG/PNG/WebP ≤ 2 MB), `mode=auto` (standart) \| `landmark` \| `price`, `lat?`+`lng?`, `quotedPrice?`, `placeId?` (faqat landmark, rasmsiz). **landmark:** Groq vision joy nomini taxmin qiladi, `internal/landmark` uni bazadagi joylar bilan moslaydi (uz/ru/en nomlar, taxalluslar, kategoriya; lat/lng bo'lsa 1.5 km ichida, 300 m ichidagisi ustun) → `LandmarkResult` faqat bazadan (`title`, `subtitle`, `summary`, `facts`, `sources`, `historyVerified`, `followUps`, `conversationId` = joyga bog'langan yo'riqchi suhbati, `place`). Tarixi tekshirilmagan joy: "Bu joy haqida tekshirilgan tarix yo'q" + joyning o'z manbasi (OSM/Wikidata/tadqiqot); manba ham yo'q bo'lsa `422 no_verified_info`. Mos kelmasa `200 {placeId: null, confidence < 0.5, candidates}`. **auto:** bitta vision chaqiruvi rasm turini aniqlab landmark yoki price ga o'tadi (menyu/yozuv `501`). **price:** buyum nomi (uz/ru/en), `fairRange`, `online{...}`, `verdict`, `tip`, `phrases`; 5 market (OLX, Texnomart, Asaxiy, Olcha, Idea). `translate` hali yo'q (501). Kesh: vision rasm sha256 bo'yicha, joy moslash rasm + joylashuv (taxminan 100 m) bo'yicha, bozor qidiruvi 6 soat; 10/daq. Env: `GROQ_API_KEY` (shart, vergul bilan bir nechta), `LENS_VISION_MODEL` (standart `qwen/qwen3.8-27b`), `MARKET_PROXY` (ixtiyoriy, xorijiy serverda marketlar uchun). Sinash: `GROQ_API_KEY=... go run ./cmd/lens-try rasm.jpg` (narx), `-auto rasm.jpg`, `DATABASE_URL=... ... -landmark rasm.jpg [lat lng]` |
| Ovozdan matn (STT) | `POST /speech/stt` multipart `audio` (webm/ogg/m4a/mp3/wav, ≤ 10 MB, ≤ 60 s), `lang?` (uz\|ru\|en) → `{text, lang, engine}`. O‘zbekcha — faqat **Kotib** (Apache-2.0, WER ~13 %): GPU kompyuter (`stt-relay`), u o‘chiq bo‘lsa serverning o‘zidagi Kotib (`stt-kotib`, CPU). Ruscha/inglizcha — Groq Whisper. Audio saqlanmaydi (20/daq) |
| Joy taklif qilish | `POST /places/suggest` `{name, location, category?, note?, instagram?, telegram?, website?}` → `202 {id, status: queued\|duplicate\|out_of_area, placeId?, suggestedCategory, confidence}` (5/daq) |
| Joy topish (admin) | `GET /admin/discoveries?status=queued\|published\|duplicate\|rejected\|out_of_area&source=wikidata\|osm\|user\|research&city=` · `POST /admin/discoveries/{id}/approve` `{category?, cityId?, name?}` · `POST /admin/discoveries/{id}/reject` · `POST /admin/discoveries/run` (Wikidata, fonda) |
| Saqlangan joylar | `GET /me/saved`, `PUT /me/saved/{placeId}`, `DELETE /me/saved/{placeId}` |
| Guruh safari | `POST /groups` `{name, cityId?}` → `inviteCode` · `POST /groups/join` `{inviteCode}` · `GET /groups`, `GET /groups/{id}` (a'zolar, ovozlar) · `PATCH /groups/{id}/me` `{interests, budget, pace}` · `POST /groups/{id}/votes` `{placeId, vote: 1\|-1\|0}` · `POST /groups/{id}/trip` `{startDate, days}` · `DELETE /groups/{id}/members/me` |
| Kundalik | `POST /diary/photos` (multipart `image, tripId, dayIndex, stopId?`), `DELETE /diary/photos/{id}`, `GET /trips/{id}/days/{idx}/photos`, `POST·GET /trips/{id}/days/{idx}/diary` (`tone: warm\|short\|fun\|poetic`), `GET·PATCH /diary/{entryId}`, `POST /diary/{entryId}/regenerate`, `POST /diary/{entryId}/publish` (`visibility: public\|link\|private`), `GET /diary/{entryId}/share-card?format=story\|post` (PNG), `POST /diary/{entryId}/shares` (`target`), `POST /me/push-token` |
| Lenta | `GET /feed?city=`, `GET /posts/{id}`, `POST·DELETE /posts/{id}/like`, `POST /posts/{id}/report`, `DELETE /posts/{id}`; ochiq: `GET /s/{slug}` (HTML + OpenGraph), `GET /s/{slug}/card.png`; admin: `GET /admin/posts?status=reported`, `POST /admin/posts/{id}/hide·restore` |
| Admin | `POST /admin/auth/login`, `GET /admin/me`, `GET /admin/places?status=&city=&q=`, `POST /admin/places`, `GET·PATCH /admin/places/{id}`, `POST /admin/places/{id}/submit·publish·reject·unpublish`, `PUT /admin/places/{id}/rules`, `POST /admin/places/{id}/images` (multipart `image`, `credit`), `GET /admin/analytics/novelty?days=30`, `POST·PATCH·DELETE /admin/services[/{id}]` |
| Hamkor takliflari | `GET /offers?slot=&city=&from=&to=&date=&days=&resident=&trip=` → 0 yoki 1 ta `Offer` · `POST /offers/{id}/impressions` `{count?}` · `GET /r/{clickId}` (bosish → hamkorga 302, sub-id bilan) · hamkor: `POST /partners/conversions`, `GET /partner/me/stats`, `POST /partner/codes/redeem` (`X-Partner-Key` yoki `partner` roli) |
| Yo'l va safar | `GET /routes/intercity?from=&to=` · `PATCH /trips/{id}` `{arrival?, prep?}` (yo'l narxi `totals.estimatedCost` va `totals.arrivalCost` da) |
| Lens Story | `POST /stories` `{placeId, location?}` · `POST /stories/{shareId}/shared` → `{code?, reason?}` · `GET /me/codes[?include=all]` · `GET /s/p/{placeId}?ref=` (`/api/v1` ostida) |
| Insights | `GET /insights/summary`, `GET /insights/export?format=csv` (`insights` roli, hudud bilan; K = 50) · `PATCH /me {analyticsOptOut}` |
| Admin (daromad) | faqat `admin`: `GET·POST·PATCH /admin/partners[/{id}]`, `GET·POST·DELETE /admin/partners/{id}/keys[/{keyId}]`, `GET·POST·PATCH /admin/offers[/{id}]`, `PUT /admin/users/{id\|email}/role` |
| Xizmat | `GET /healthz`, `GET /static/places/{file}.jpg` (katalog rasmlari, binar ichida), `GET /static/ph/{category\|city\|event\|stay}.svg` (rasm yo'q bo'lsa), `GET /uploads/{file}` (yuklangan rasmlar) |

Til: `Accept-Language: uz | ru | en` (yo'q bo'lsa `en`; matn topilmasa `en` → `uz`).
Katalog GET javoblarida `ETag` (`If-None-Match` → `304`). Javob shakllari: [../API_CONTRACT.md](../API_CONTRACT.md).

IP bo'yicha limitlar (`/auth/*`, oflayn paket): so'rov `TRUSTED_PROXIES` dagi manzildan kelsa `X-Forwarded-For`
o'ngdan chapga o'qiladi va birinchi ishonchsiz manzil olinadi; aks holda `RemoteAddr`.

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
seed/          katalog (build_seed.py yasaydi) + sos_*.json (SOS demo)
static/        katalog rasmlari (binar ichiga joylanadi)
scripts/catalog/build_seed.py  research/ → seed + static
```

## BACKEND.md dan farqlar (ataylab)

| BACKEND.md | Bu yerda | Nega |
|---|---|---|
| PostGIS | `cube` + `earthdistance` (GiST indeks) | har qanday PostgreSQL da bor, o'rnatish shart emas; shahar hajmi uchun yetarli. Kerak bo'lsa keyin PostGIS ga o'tish oson (`store` ichida) |
| Redis rate limit | xotiradagi token bucket | bitta server nusxasi uchun yetarli; ko'p nusxada Redis ga ko'chiriladi |
| goose + sqlc + oapi-codegen | o'z migratori + qo'lda yozilgan pgx SQL | kamroq vosita, hammasi `go build` bilan |
| MinIO / S3 | lokal disk (`UPLOAD_DIR`) | `media.Store` interfeysi orqali; S3 keyin shu interfeysga ulanadi |

## Katalog (seed) qayerdan keladi

`seed/<city>.json` (samarkand, bukhara, tashkent, namangan) va `static/places/*.jpg` **qo'lda tahrirlanmaydi**:
ular jamoaning `research/` tadqiqotidan (manbalar bilan) skript bilan yasaladi. Ilova va sayt ham aynan shu
yozuvlarni ko'radi (`research/convert.py` bilan bir xil birlashtirish qoidalari, bir xil ID lar).

```sh
# frontend repo (research/, web/public/places/) yonida; avval u yerda research/convert.py ishlatilgan bo'lsin
python3 -I scripts/catalog/build_seed.py ~/dev/tourism_hackathon
make seed        # idempotent: qayta yuklaydi
```

- Joylar `source='open_data'`; katalogda endi yo'q eski seed joylari (`demo`/`open_data`) `draft` ga o'tadi
  (o'chirilmaydi: eski marshrutlar ularga havola qiladi).
- Rasmlar binar ichiga joylanadi (`static/`, ~51 MB), bazada nisbiy `/static/places/...` saqlanadi, API
  `PUBLIC_BASE_URL` bilan to'liq manzil beradi. Litsenziyasi noma'lum rasmlar faqat demo uchun (`license` bo'sh).
- Narxlar, ish vaqtlari, "yangi" joylar ochiq manbalardan, `sampleSize = 0` (foydalanuvchi kuzatuvi yo'q).
- Ishonch darajasi: joy, tarix va qoidalar `public`; ish vaqti (`hoursTrust`), kirish narxi (`entryFee.trust`),
  narxlar (`trust`) va turar joy narxi (`pricePerNight.trust`) `sample`.
- Turar joylar (28 ta) 0008 dagi `stays` jadvaliga `source='research'` bilan yoziladi: nom 3 tilda, manzil uz,
  `priceMin` (tadqiqotdagi "dan" narx), reyting 5 ballik + `ratingSource`, qulaylik kodlari (`wifi, breakfast, ac,
  parking, transfer, courtyard, kitchen, private_bathroom, laundry, pool, spa, sauna, hammam, restaurant, bar, gym,
  room_service, tea_coffee`). Admin tahrir qilsa `source='manual'` bo'ladi va seed ustiga yozmaydi.
- Mahalliy nomlar (`aliases`) va tanaffuslar (`lunch`, `fridayPrayer`) tadqiqotda yo'q: `build_seed.py` dagi
  `ALIASES`, `LUNCH`, `FRIDAY_PRAYER` jadvallarida.
- `seed/sos_*.json` dagi shifoxona, dorixona va h.k. **o'ylab topilgan demo** (pastda). Shaharda OSM xizmatlari
  bo'lsa (`osm-import` dan keyin) seed demo xizmatlarni qayta qo'shmaydi.

## Daromad funksiyalari

To'liq kelishuv: frontend repo `docs/MONETIZATION.md`, shakllar: [../API_CONTRACT.md](../API_CONTRACT.md) §10.

- `seed/monetization.json` — namuna hamkorlar (`sample: true`, UI "Namuna hamkor"), takliflar va shaharlararo yo'nalishlar.
  Yo'nalish raqamlari sayt (`web/src/lib/monetization/intercity.ts`) bilan aynan bir xil, manba va sana bilan; mashina va
  Toshkent orqali qatorlar `internal/seed/monetization.go` da shu formula bilan hisoblanadi. Seed idempotent
  (yo'nalishlar to'liq qayta yoziladi, hamkor va takliflar id bo'yicha yangilanadi).
- Taklif tanlash: slot, shahar, yo'nalish, transport turi (eng tez = samolyot yoki Afrosiyob, qaysi tezroq), sana oynasi,
  kunlar, rezidentlik. Bir nechta mos kelsa eng yuqori `priority` dan biri kontekst bo'yicha barqaror aylantiriladi.
- Bosish: `clickId` HMAC bilan imzolangan (`TOKEN_SECRET` dan), bazaga faqat bosilganda yoziladi; IP saqlanmaydi.
  Hamkor sotuvni `clickId` = redirectdagi `sub_id` bilan xabar qiladi.
- Hamkor kaliti `spk_...`, bazada sha256 xeshi; admin `POST /admin/partners/{id}/keys` bilan beradi (bir marta ko'rinadi).
- Insights: K = 50 dan kichik har kesim `null` + `reason: "below_k"`; `analyticsOptOut` foydalanuvchilar safarlari kirmaydi.

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

## SOS ma'lumotlari haqida — muhim

`seed/` dagi shifoxona, dorixona, politsiya, bankomat yozuvlari **o'ylab topilgan demo** (nomida "(demo)",
API javobida `isDemo: true`). Favqulodda raqamlar (101, 102, 103, 104, 112) ham rasmiy manbadan tekshirilishi kerak.
Productionga chiqishdan oldin bular haqiqiy, tekshirilgan ma'lumot bilan almashtirilishi **shart** — noto'g'ri
manzil favqulodda vaziyatda turistga zarar yetkazishi mumkin. Ilova `isDemo` bo'lsa ogohlantirish ko'rsatsin.

## Kirish (auth)

- Ilova login siz ishlaydi: `POST /auth/device` → anonim token (30 kun).
- Google yoki email kod bilan kirganda javob: `{token (1 soat), refreshToken (60 kun), user, isNewUser}`.
  So'rovda anonim token `Authorization: Bearer` da yuborilsa — anonim marshrutlar, kundalik, postlar hisobga ko'chadi.
- `token` muddati tugasa → `POST /auth/refresh`; har safar **yangi** refreshToken qaytadi (eskisi bekor).
  Eski refreshToken qayta ishlatilsa (o'g'irlik belgisi) foydalanuvchining barcha sessiyalari yopiladi.
- Email kod: 10 daqiqa amal qiladi, 5 ta noto'g'ri urinishdan keyin bloklanadi, bitta emailga 10 daqiqada ≤ 3 ta kod.
- Google: frontend Google Sign-In dan **ID token** oladi va `POST /auth/google` ga yuboradi; backend uni Google ning
  ochiq kalitlari bilan tekshiradi (`aud` = `GOOGLE_CLIENT_IDS`). Faqat Google tasdiqlagan email hisobga bog'lanadi.
- Sozlanmagan bo'lsa: `503 google_unavailable` / `503 email_unavailable`.

## Ma'lumot sifati va qo'shimcha maydonlar

- `trust`: `official` (obyekt ma'muriyati, gov.uz) · `public` (ochiq manba) · `community` (foydalanuvchi) · `sample` (demo).
  Joyda, qoidada (`rules[].trust`), tarixda (`history.trust`), ish vaqtida (`hoursTrust`), kirish narxida
  (`entryFee.trust`), narxlarda (`prices[].trust`) va turar joy narxida (`pricePerNight.trust`, OSM taxmini `estimate`).
  `sample` — namuna; UI buni belgilashi kerak.
- `aliases`: mahalliy nomlar ("Chorsu", "Kalon minorasi") — `GET /places?q=` ular bo'yicha ham qidiradi.
- `openingHours[].breaks`: `[{from, to, reason: lunch|prayer|other}]` — "hozir ochiq" va rejalashtirgich tanaffusni hisobga oladi
  (masjidlarda juma 12:00–14:00).
- `isResident` (`PATCH /me`): kirish narxini (`entryFee.local` / `entryFee.foreigner`) to'g'ri ko'rsatish uchun.
- `theatre` kategoriyasi.
- Guruh safari: umumiy marshrut a'zolar istaklaridan tuziladi — qiziqishlar (ko'p uchraganlari), eng kichik byudjet,
  eng sokin temp; ko'pchilik "ha" degan joy ustun, yarmi yoki ko'pi "yo'q" degan joy chiqariladi. Marshrutni barcha a'zolar
  ko'radi va tahrirlaydi; kundalik — har kimning o'ziniki.

## AI yordamchi (`internal/assistant`)

- `ANTHROPIC_API_KEY` bo'lsa: Claude (`AI_MODEL`, standart `claude-opus-5-5`; `AI_EFFORT` standart `low`) 9 ta asbob bilan
  ishlaydi: `search_places` (shahar, kategoriya, taom so'zlari, bolalar bilan, narx darajasi, hozir ochiq, yaqin atrof),
  `get_place` (tarix va manbalar bilan), `search_stays`, `search_prices`, `get_trip` (so'rovda `tripId` bo'lsa),
  `search_events` (shahar, sana oralig'i), `get_intercity` (turlari, vaqt, narx, manba), `get_services` (eng yaqin bankomat,
  dorixona, valyuta, shifoxona, politsiya + favqulodda raqamlar), `get_weather` (Open-Meteo, 15 kungacha).
  **Har bir fakt asbob natijasidan**: bazada yo'q bo'lsa (viza qoidalari, bazada yo'q tarix ham) "Bu haqda tekshirilgan
  ma'lumot yo'q" deydi. Foydalanuvchi profili (byudjet, qiziqishlar) kontekstga qo'shiladi.
- `tripId` ("Rejam haqida so'rash"): handler marshrut egasini tekshiradi, kontekst qatoriga kunlar va to'xtashlar xulosasi
  qo'shiladi, `get_trip` to'liq rejani beradi (vaqt, yo'l, xarajat, boshlanish nuqtasi, shaharga yo'l, tayyorgarlik).
  Marshrut joylari javobda tilga olinsa kartochka bo'lib chiqadi. Chat rejani o'zgartirmaydi, "Kunni moslash" ga yo'naltiradi.
- Yozish qoidalari (CLAUDE.md): neytral ovoz, birinchi shaxs yo'q ("Bazada topildi:", "Arzon osh uchun:"), "—", "≈", "~"
  ishlatilmaydi, javob savol tilida. Oxirgi himoya `assistant.Sanitize` / `SanitizeEmit`: oqimdagi bo'laklar (bo'lak
  chegarasida bo'lingan " —" yoki "~" + "5" ham) "," / ":" va "taxminan" / "примерно" / "about" ga almashadi; saqlanadigan
  javob ham shu. `GET /places/{id}/guide` `summary` si ham shu tozalagichdan o'tadi.
- `sources` hodisasi: tilga olingan joylar tarixi manbalari, narx, yo'l, tadbir va ob-havo manbalari (ko'pi bilan 6).
- `ANTHROPIC_API_KEY` bo'lmasa, `GROQ_API_KEY` bilan Groq (`openai/gpt-oss-120b`, OpenAI-mos API): xuddi shu asboblar va qoidalar.
  Terminalda sinash: `GROQ_API_KEY=... go run ./cmd/guide-try "Samarqandda arzon osh"`.
- Kalit yo'q yoki model ishlamasa `mode: fallback`: savoldan shahar, joy turi (`discover.Classify`), "bolalar", "arzon",
  "hozir ochiq", "kam tanilgan", taom nomlari ajratiladi va bazadan qidiriladi. `tripId` bilan "ertaga", "2-kun", "bugun",
  "tushlik" kabi savollarga o'sha kun rejasi qaytadi.
- Suhbatlar `guide_conversations` (sarlavha: birinchi savol) va `guide_messages` (`seq`: qat'iy tartib) da. Har savolda
  oxirgi 20 xabar modelga beriladi; avvalgi javoblarda ko'rsatilgan joylar (id = nom) ham qo'shiladi, "birinchisi qachon
  ochiladi?" kabi davomli savollar ishlaydi. Mehmon hisobidan akkauntga kirilganda suhbatlar ko'chadi.
- `GROQ_API_KEY` da bir nechta kalit vergul bilan: so'rovlar navbat bilan taqsimlanadi, kalit limitga urilsa (429) yoki
  ishlamasa keyingisiga, hammasi band bo'lsa `gpt-oss-20b` ga o'tadi.

## Rasm va tavsif bilan boyitish (`internal/enrich`, `cmd/enrich`)

Rasmi yoki tavsifi yo'q joylar ochiq manbalardan to'ldiriladi (qo'lda qo'yilganlar tegilmaydi):

1. **Wikidata bog'langan joylar** — asosiy rasm (P18) Wikimedia Commons'dan: 960 px eskiz havolasi, muallif, litsenziya,
   Commons sahifasi (`images[0].credit/license/sourceUrl`). Faqat ochiq litsenziya: CC0, PD, CC BY, CC BY-SA
   (NC/ND va fair use rad etiladi). Tavsif — uz/ru/en Vikipediya maqolasining birinchi 2 gapi, manba
   "Wikipedia (CC BY-SA 4.0)" `sources` ga qo'shiladi.
2. **Wikidata'siz obidalar, masjidlar, muzeylar, bog'lar** — 120 m ichidagi Commons rasmlari, fayl nomi joy nomiga mos
   kelsagina (umumiy so'zlarsiz). Kafe/restoranlar uchun ochiq rasm deyarli yo'q — ularga qo'llanilmaydi.

```sh
go run ./cmd/enrich --dry-run --limit 50     # nima to'ldirilishini ko'rish
go run ./cmd/enrich                          # hammasi (serverda: bash deploy/enrich.sh)
```

## AI ma'lumot tozalovchi (`internal/cleaner`, `cmd/cleaner`)

Ichki tarmoqdagi AI server (Ollama/llama.cpp/vLLM/LM Studio — OpenAI-mos) topilgan joylarni tozalaydi: haqiqiy
turistik joymi (junk ajratish), to'g'ri kategoriya, qisqa uz/ru/en tavsif (faqat mavjud faktlardan). Fakt o'zgarmaydi.

- **Faqat chiquvchi ulanish:** `cleaner` ishchisi AI serverda turadi, asosiy serverga o'zi qo'ng'iroq qiladi
  (`GET /internal/clean/pending` → modeli bilan tozalaydi → `POST /internal/clean/results`). AI serverga port ochish shart emas.
- Himoya: `X-Cleaner-Key` = `CLEANER_KEY`; kalit o'rnatilmagan bo'lsa endpointlar o'chiq (404).
- Qaror: ishonch ≥ 0.8 va manba ishonchli (wikidata/osm/research) → avto-nashr; aks holda admin navbati. Foydalanuvchi
  taklifi hech qachon avto-nashr emas. Model tavsiyasi `discoveries.data->'ai'` da saqlanadi (admin ko'radi).
- Ishga tushirish va sozlash: [../deploy/CLEANER.md](../deploy/CLEANER.md).

## Yangi joylarni avtomatik topish (`internal/discover`)

Manbalar (hammasi qonuniy, bazada saqlash mumkin): **Wikidata** (CC0, har 24 soatda — `DISCOVERY_EVERY`),
**OpenStreetMap** (ODbL, haftalik — `deploy/osm-refresh.sh`), **foydalanuvchi takliflari** (`POST /places/suggest` —
Instagram/Telegramda ko'rgan joyini yuboradi; scraping o'rniga), **tadqiqot nomzodlari** (`seed/candidates`).

1. **Kategoriya** — `discover.Classify`: OSM teglari (`amenity=marketplace` → bazaar, `craft=*` → workshop,
   `place_of_worship+muslim` → mosque ...), Wikidata klasslari (Q33506 muzey, Q132834 madrasa ...) va nomdagi
   uz/ru/en kalit so'zlar (choyxona → cafe, kulolchilik → workshop, teatr → theatre ...). Bir nechta manba bir xil
   desa ishonch oshadi, qarama-qarshi bo'lsa kamayadi; sabablari `reasons` da (`osm:amenity=cafe`, `name:muzey`).
2. **Takror** — bir xil Wikidata/OSM id yoki 150–350 m ichida o'xshash nom (umumiy so'zlarsiz: "madrasasi",
   "Медресе" solishtirilmaydi; tahrir masofasi ≤15%). Takror topilsa mavjud joyga Wikidata id bog'lanadi.
3. **Qaror** — ishonch ≥ 0.8 va manba ishonchli (Wikidata/OSM) → avtomatik chop etiladi (`trust=public`,
   `source=wikidata`); aks holda admin navbati. Foydalanuvchi taklifi **hech qachon** tekshiruvsiz chiqmaydi;
   tasdiqlangach `trust=community`.
4. Wikidata/OSM joylari "ommaviy import" (`source=osm` filtri ikkalasini oladi): ro'yxatda jamoa tekshirganlardan
   keyin, rejalashtirgichda ×0.75. Rasmlar avtomatik olinmaydi (Commons litsenziyalari har xil).

```sh
go run ./cmd/discover wikidata --dry-run   # nima qo'shilishini ko'rish
go run ./cmd/discover research             # tadqiqot nomzodlarini tekshirib chop etish
go run ./cmd/discover stats
```

## OpenStreetMap importi

```sh
curl -LO https://download.geofabrik.de/asia/uzbekistan-latest.osm.pbf     # ~126 MB, har kuni yangilanadi
go run ./cmd/osm-extract uzbekistan-latest.osm.pbf > osm-pois.json        # ~8 soniya
go run ./cmd/osm-import osm-pois.json                                     # upsert, takroran xavfsiz
```

- Natija (2026-11-26): ~4 800 joy (restoran, kafe, muzey, obida, teatr), ~1 570 turar joy, ~6 200 xizmat
  (shifoxona, klinika, dorixona, politsiya, bankomat, valyuta, yoqilg'i, EV quvvatlash) — 17+ shahar.
- Demo SOS xizmatlari o'chiriladi va haqiqiy OSM ma'lumoti bilan almashtiriladi.
- Qo'lda yoki tadqiqot orqali kiritilgan joy yonida (150 m, o'xshash nom) takror yaratilmaydi.
- OSM joylari `source='osm'`, `trust='public'`; rejalashtirgichda jamoa tanlagan joylar ustun (ball −2, ya'ni bitta
  qiziqish mosligicha).
- Turar joy narxi OSM da yo'q — turi/yulduzi bo'yicha **taxmin** (`pricePerNight.estimated = true`, UI da "≈").
  Admin aniq narx kiritsa `priceTrust='official'` bo'ladi va keyingi import ustiga yozmaydi.
- **Litsenziya: ODbL.** Ilova va saytda "© OpenStreetMap contributors" yozuvi majburiy.
  Bazani OSM bilan birga ochiq tarqatmang (share-alike).
