# Yo'ldosh AI — Backend qo'llanmasi (Go)

Tegishli hujjatlar: [API_CONTRACT.md](API_CONTRACT.md) (yagona kontrakt) · [AI_ML.md](AI_ML.md) · [DESIGN.md](DESIGN.md) · [../main.md](../main.md) (mahsulot g'oyasi)

---

## 1. Loyiha va backendning roli

**Yo'ldosh AI** — turist uchun AI hamroh: qiziqish, byudjet va vaqtga qarab shaxsiy marshrut tuzadi va unga mashhur joylar bilan birga **yangi ochilgan / kam tanilgan joylarni majburan qo'shadi** ("yangilik kvotasi"). Yo'lda obyekt qoidalari haqida ogohlantiradi (geofencing), rasmdan tarixni aytadi (Lens), bozor narxini tekshiradi (Adolatli narx). MVP: Samarqand va Buxoro, 3 til (uz/ru/en). Mijozlar: mobil ilova (Expo/React Native) va web platforma (Next.js, shu jumladan admin/moderatsiya).

**Mas'uliyat taqsimoti:**

| Qatlam | Egasi | Nima qiladi |
|---|---|---|
| **Go API** (siz) | ma'lumot egasi | auth, katalog, geo so'rovlar, marshrut **orkestratsiyasi va invariantlari**, narx hisobi, geofence/oflayn paket, admin/moderatsiya, fayl ombori, rate limit |
| **AI xizmati** (Python) | AI/ML dasturchi | reyting/tartiblash, RAG javob, vision tanish, embedding indeks, TTS/STT. Faqat ichki tarmoqda, Go orqali chaqiriladi |
| **Mijozlar** | frontend | faqat Go API bilan gaplashadi; AI xizmatini to'g'ridan-to'g'ri ko'rmaydi |

Tamoyil: **AI maslahat beradi, Go qaror qiladi.** Biznes qoidalar (kvota, hamkor shaffofligi, manbasiz javob yo'q) Go tomonida tekshiriladi — model xato qilsa ham mijozga noto'g'ri javob ketmaydi.

Tashqi API shakllari, xato formati, kechikish byudjeti — hammasi [API_CONTRACT.md](API_CONTRACT.md) da. Bu hujjat uni takrorlamaydi; **qanday qurish**ni aytadi. Kontraktni o'zgartirish kerak bo'lsa — avval o'sha faylga PR.

---

## 2. Tavsiya etilgan stack

| Vazifa | Tanlov | Nega |
|---|---|---|
| Til | **Go 1.26** | jamoa tanlovi; `net/http` pattern routing, `slog`, `iter` tayyor |
| HTTP | stdlib `net/http` (`mux.HandleFunc("GET /api/v1/places/{id}", …)`) yoki `chi` | middleware kam, bog'liqlik kam. `chi` — guruhlash qulayroq bo'lsa |
| Kontrakt | OpenAPI 3 fayl + `oapi-codegen` (server interfeysi va tiplar) | kontraktdagi TS tiplar bilan bir manbadan; qo'lda DTO yozilmaydi |
| DB | **PostgreSQL + PostGIS + pgvector** | geo (`ST_DWithin`, KNN), vektor qidiruv bir bazada |
| DB kirish | `pgx v5` + `sqlc` | tip-xavfsiz SQL, ORM sehrisiz; PostGIS uchun xom SQL baribir kerak |
| Migratsiya | `goose` (SQL fayllar) | sodda, `sqlc` bilan bir xil sxemadan o'qiydi |
| Kesh / limit / navbat | Redis | katalog keshi, rate limit, marshrut generatsiya holati (202 + polling) |
| Fayllar | S3-mos ombor (lokalda MinIO) | lens rasmlari, audio-gid, joy rasmlari |
| Kuzatuv | `log/slog` (JSON) + OpenTelemetry | AI chaqiruvlari kechikishini trace da ko'rish shart |
| Konfiguratsiya | env (`envconfig` yoki qo'lda) | 12-factor |
| Lokal | Docker Compose | bitta buyruq bilan butun stack |

> Versiyalar ataylab yozilmagan (Go dan tashqari) — o'rnatish paytida har birining oxirgi barqaror relizini oling va `go.mod` / compose faylida qotiring.

**Hackathon uchun soddalashtirish:** gRPC, Kafka, mikroservislar kerak emas. Bitta Go binar + bitta Python xizmat + Postgres + Redis + MinIO.

---

## 3. Papka tuzilmasi

```
backend/
├── cmd/
│   ├── api/main.go            # HTTP server
│   └── seed/main.go           # seed ma'lumot yuklash
├── api/openapi.yaml           # API_CONTRACT.md ning mashina o'qiydigan nusxasi
├── internal/
│   ├── http/                  # router, middleware (auth, lang, ratelimit, recover), handlerlar
│   │   └── gen/               # oapi-codegen chiqishi
│   ├── domain/                # sof tiplar va invariantlar (Place, Trip, Stop, Rule, Money) — bog'liqliksiz
│   ├── store/                 # sqlc chiqishi + repository o'ramlari
│   ├── catalog/               # places, cities, events, stays
│   ├── trip/                  # nomzod tanlash, AI chaqiruv, kvota tekshiruvi, fallback rejalashtirgich
│   ├── lens/                  # multipart, S3, AI proksi
│   ├── guide/                 # SSE chat proksi
│   ├── price/                 # fairRange hisobi, verdict
│   ├── geo/                   # geofence, offline-pack, masofa/tranzit bahosi
│   ├── ai/                    # AI xizmati mijozi (timeout, retry, breaker)
│   ├── admin/                 # moderatsiya, RBAC
│   ├── auth/                  # qurilma tokeni
│   ├── i18n/                  # Accept-Language tahlili, fallback
│   └── platform/              # config, logger, otel, redis, s3
├── db/
│   ├── migrations/            # goose: 0001_init.sql …
│   └── queries/               # sqlc: places.sql, trips.sql …
├── seed/                      # samarkand.json, bukhara.json, prices.json
├── sqlc.yaml · Makefile · docker-compose.yml · Dockerfile
```

Qoida: `domain` hech narsani import qilmaydi; `http` → servis paketlari → `store`/`ai`. Handlerda biznes mantiq yo'q.

---

## 4. Ma'lumotlar modeli

### 4.1. ID va i18n

- **ID — `text`.** Kontrakt bo'yicha frontend ID ichini tahlil qilmaydi. Seed joylar va shaharlar uchun o'qiladigan slug (`samarkand`, `registan`), foydalanuvchi yaratadigan yozuvlar uchun UUID v7 (`trips`, `stops`, `visits`…).
- **i18n — alohida jadval** (`*_i18n`, kalit `(entity_id, lang)`). JSONB ustun emas: moderatsiya, to'liq matnli qidiruv va "qaysi tilga tarjima yo'q" hisoboti osonroq.
- **Fallback** (kontraktga mos): so'ralgan til → `en` → `uz`. Bitta so'rovda hal qilinadi:

```sql
-- name: GetPlaceText :one
SELECT name, tagline, description, address
FROM place_i18n
WHERE place_id = $1 AND lang = ANY($2::text[])        -- $2 = {ru,en,uz}
ORDER BY array_position($2::text[], lang)
LIMIT 1;
```

Ro'yxat so'rovlarida `DISTINCT ON (place_id) … ORDER BY place_id, array_position(...)` yoki `LATERAL` ishlating. `lang` middleware `Accept-Language` dan oladi (`uz|ru|en`, boshqasi → `en`).

### 4.2. DDL eskizi

```sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE cities (
  id text PRIMARY KEY,                       -- 'samarkand'
  country text NOT NULL DEFAULT 'UZ',
  center geography(Point,4326) NOT NULL,
  cover_url text, cover_blurhash text,
  timezone text NOT NULL DEFAULT 'Asia/Tashkent'
);
CREATE TABLE city_i18n (city_id text REFERENCES cities, lang text, name text NOT NULL, PRIMARY KEY (city_id, lang));

CREATE TABLE places (
  id text PRIMARY KEY,
  city_id text NOT NULL REFERENCES cities,
  category text NOT NULL,                    -- PlaceCategory
  novelty text NOT NULL CHECK (novelty IN ('iconic','new','hidden')),
  opened_at date,                            -- novelty='new' uchun
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('draft','pending','published','rejected')),
  location geography(Point,4326) NOT NULL,
  geofence_radius_m int NOT NULL DEFAULT 80,
  interests text[] NOT NULL DEFAULT '{}',    -- Interest[]
  price_level smallint NOT NULL DEFAULT 0,
  visit_duration_min int NOT NULL DEFAULT 45,
  opening_hours jsonb NOT NULL DEFAULT '[]', -- [{day,open,close}]
  entry_fee jsonb,
  wheelchair bool NOT NULL DEFAULT false, family_friendly bool NOT NULL DEFAULT true, easy_walk bool NOT NULL DEFAULT true,
  pay_card bool NOT NULL DEFAULT false, cash_only bool NOT NULL DEFAULT false,
  is_partner bool NOT NULL DEFAULT false,
  rating numeric(2,1) NOT NULL DEFAULT 0, reviews_count int NOT NULL DEFAULT 0,
  rules_version int NOT NULL DEFAULT 1,      -- Geofence.version
  images jsonb NOT NULL DEFAULT '[]',        -- [{url,blurhash,credit}], birinchisi = cover
  audio jsonb,                               -- {uz:{url,durationSec}, ...}
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);
CREATE INDEX places_loc_gix   ON places USING gist (location);
CREATE INDEX places_city_idx  ON places (city_id, status, novelty);
CREATE INDEX places_int_gin   ON places USING gin (interests);

CREATE TABLE place_i18n (
  place_id text REFERENCES places ON DELETE CASCADE, lang text,
  name text NOT NULL, tagline text NOT NULL, description text NOT NULL DEFAULT '', address text NOT NULL DEFAULT '',
  history_summary text, history_facts text[], sources jsonb NOT NULL DEFAULT '[]',
  PRIMARY KEY (place_id, lang)
);

CREATE TABLE place_rules (
  id bigserial PRIMARY KEY,
  place_id text NOT NULL REFERENCES places ON DELETE CASCADE,
  code text NOT NULL,                        -- RuleCode
  severity text NOT NULL CHECK (severity IN ('info','warn','forbidden')),
  priority smallint NOT NULL DEFAULT 0,      -- topRules tanlash uchun
  text jsonb NOT NULL,                       -- {uz,ru,en} — qisqa, 1 gap
  verified_by text, verified_at timestamptz
);

CREATE TABLE events (
  id text PRIMARY KEY, city_id text NOT NULL REFERENCES cities, kind text NOT NULL,
  title jsonb NOT NULL, venue jsonb NOT NULL, location geography(Point,4326) NOT NULL,
  starts_at timestamptz NOT NULL, ends_at timestamptz NOT NULL,
  price_amount bigint, is_free bool NOT NULL DEFAULT false, ticket_url text, cover jsonb, interests text[] DEFAULT '{}'
);
CREATE INDEX events_city_time ON events (city_id, starts_at, ends_at);

CREATE TABLE stays (
  id text PRIMARY KEY, city_id text NOT NULL REFERENCES cities, kind text NOT NULL, name text NOT NULL,
  location geography(Point,4326) NOT NULL, price_per_night bigint NOT NULL,
  fair_price bool NOT NULL DEFAULT false, verified bool NOT NULL DEFAULT false,
  rating numeric(2,1), reviews_count int DEFAULT 0, amenities text[] DEFAULT '{}', cover jsonb
);
CREATE INDEX stays_loc_gix ON stays USING gist (location);

CREATE TABLE users (
  id text PRIMARY KEY, lang text NOT NULL DEFAULT 'uz', role text NOT NULL DEFAULT 'tourist',
  interests text[] DEFAULT '{}', companion text, pace text, budget smallint,
  points int NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE devices (id text PRIMARY KEY, user_id text NOT NULL REFERENCES users, platform text, last_seen_at timestamptz);

CREATE TABLE trips (
  id uuid PRIMARY KEY, user_id text NOT NULL REFERENCES users, city_id text NOT NULL REFERENCES cities,
  status text NOT NULL CHECK (status IN ('generating','ready','failed')),
  start_date date NOT NULL, prefs jsonb NOT NULL,            -- POST /trips tanasi
  title text, summary text, novelty_share real, planner text, -- planner: 'ai' | 'fallback'
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE trip_days (trip_id uuid REFERENCES trips ON DELETE CASCADE, idx smallint, date date, title text, PRIMARY KEY (trip_id, idx));
CREATE TABLE stops (
  id uuid PRIMARY KEY, trip_id uuid NOT NULL, day_idx smallint NOT NULL, position smallint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('place','meal','event')),
  place_id text REFERENCES places, event_id text REFERENCES events,
  arrive_at time NOT NULL, duration_min int NOT NULL, reason text NOT NULL CHECK (reason <> ''),
  transit jsonb, status text NOT NULL DEFAULT 'upcoming',
  FOREIGN KEY (trip_id, day_idx) REFERENCES trip_days ON DELETE CASCADE
);

CREATE TABLE price_items (
  id text PRIMARY KEY, city_id text NOT NULL REFERENCES cities, category text NOT NULL,
  name jsonb NOT NULL, unit text NOT NULL,
  fair_min bigint, fair_max bigint, tourist_typical bigint,
  sample_size int NOT NULL DEFAULT 0, updated_at timestamptz,
  location geography(Point,4326)             -- ixtiyoriy: joyga bog'liq narx
);
CREATE TABLE price_observations (
  id bigserial PRIMARY KEY, price_item_id text REFERENCES price_items, city_id text NOT NULL,
  category text NOT NULL, name text, paid bigint NOT NULL,
  source text NOT NULL,                      -- 'user' | 'partner' | 'moderator' | 'open_data'
  geocell text,                              -- umumlashtirilgan joy (aniq nuqta emas)
  observed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE visits (
  id uuid PRIMARY KEY, user_id text NOT NULL REFERENCES users, place_id text NOT NULL REFERENCES places,
  trip_id uuid, rating smallint, comment text, created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, place_id, (created_at::date))
);
CREATE TABLE vendors (id text PRIMARY KEY, city_id text, name text, category text, location geography(Point,4326), trusted_since date);
CREATE TABLE scam_reports (id bigserial PRIMARY KEY, city_id text, category text, description text, geocell text, created_at timestamptz DEFAULT now());
-- scam_reports da user_id YO'Q — shikoyat chindan anonim.
```

### 4.3. Geo so'rovlar

```sql
-- name: PlacesNear :many  (near=lat,lng&radiusM=)
SELECT p.id, ST_Distance(p.location, ST_MakePoint(@lng, @lat)::geography) AS distance_m
FROM places p
WHERE p.city_id = @city AND p.status = 'published'
  AND ST_DWithin(p.location, ST_MakePoint(@lng, @lat)::geography, @radius_m)
ORDER BY p.location <-> ST_MakePoint(@lng, @lat)::geography
LIMIT @lim;
```

Diqqat: `ST_MakePoint(lng, lat)` — **avval uzunlik**. `geography` turi metrda ishlaydi, GIST indeksni ishlatadi. `isOpenNow` — `opening_hours` va shahar `timezone` bo'yicha Go da hisoblanadi (so'rovda emas).

---

## 5. Marshrut yaratish (`POST /trips`) — eng muhim qism

```
so'rov → [1] nomzodlar (Go, SQL) → [2] reyting/tartib (AI) → [3] invariant tekshiruvi (Go)
       → [4] tuzatish / qayta so'rash / fallback → [5] vaqt jadvali + tranzit (Go) → saqlash → Trip
```

**[1] Nomzodlar (Go).** `status='published'`, shahar, `interests && $prefs`, cheklovlar (`wheelchair`, `withKids` → `family_friendly`), sana kunida ochiq. Ikki hovuz alohida olinadi: `iconic` va `new|hidden` — AI ga yuboriladigan ro'yxatda yangi joylar yetarli bo'lishi kafolatlanadi (masalan 40 nomzoddan kamida 15 tasi). Ovqat joylari (`restaurant|cafe`) alohida hovuz (`meal` to'xtashlari uchun). Shu sanalardagi `events` ham qo'shiladi.

**[2] AI.** `POST /internal/route/rank` (6-bo'lim): nomzodlar + prefs → kunlarga bo'lingan tartib va har bir to'xtash uchun `reason`. Timeout **6 s**.

**[3] Invariantlar (Go, har doim, AI javobiga ishonmasdan):**

```go
// internal/domain/trip_invariants.go
const MinNoveltyShare = 0.25

type Violation struct{ Day int; Code string }

func (t *Trip) Validate() []Violation {
	var v []Violation
	total, novel := 0, 0
	for _, d := range t.Days {
		dayNovel := 0
		var prevEnd time.Duration
		for i, s := range d.Stops {
			if s.Reason == "" {
				v = append(v, Violation{d.Index, "empty_reason"})
			}
			if i > 0 && s.ArriveAt < prevEnd+s.Transit.Duration() {
				v = append(v, Violation{d.Index, "overlap"})
			}
			prevEnd = s.ArriveAt + s.Duration
			if s.Kind != KindPlace || s.Place == nil {
				continue
			}
			total++
			// Hamkor joy kvotani TO'LDIRMAYDI — shaffoflik tamoyili.
			if s.Place.Novelty != NoveltyIconic && !s.Place.IsPartner {
				novel++
				dayNovel++
			}
		}
		if dayNovel == 0 {
			v = append(v, Violation{d.Index, "day_without_novelty"})
		}
	}
	if total > 0 && float64(novel)/float64(total) < MinNoveltyShare {
		v = append(v, Violation{-1, "novelty_share_low"})
	}
	return v
}
```

`noveltyShare` javobda ham shu formula bilan hisoblanadi (hamkor bo'lmagan `new|hidden` / barcha `place` to'xtashlari).

**[4] Buzilsa:**
1. `day_without_novelty` / `novelty_share_low` → **Go o'zi tuzatadi**: o'sha kunning eng past ball olgan `iconic` to'xtashini eng yaqin (PostGIS KNN) mos `new|hidden` nomzodga almashtiradi; `reason` shablondan ("Yaqinda ochilgan, hali kam odam biladi"). Qayta `Validate()`.
2. `overlap` → [5] bosqich vaqtlarni baribir qayta hisoblaydi.
3. `empty_reason` → shablon matn (qiziqishga qarab, i18n).
4. AI xato/timeout/noto'g'ri JSON → **deterministik fallback**, AI ni qayta chaqirmasdan.

**Fallback rejalashtirgich** (`internal/trip/fallback.go`) — demo hech qachon yiqilmasin:
- ball = qiziqish mosligi × reyting × (yangi joyga bonus); kuniga `pace` bo'yicha 4/5/6 to'xtash;
- har kunga avval 1–2 ta `new|hidden` "langar" qo'yiladi, qolgani eng yuqori balllilar;
- tartib: `startLocation` dan boshlab greedy nearest-neighbour; tushlik (12:30–14:00) va kechki ovqat oralig'iga `meal` to'xtashi;
- `trips.planner = 'fallback'` deb yoziladi (kuzatuv uchun; mijozga farqi ko'rinmaydi).

**[5] Jadval va tranzit (Go).** Kun 09:00 dan (yoki birinchi joy ochilishidan). `transitFromPrev`: masofa PostGIS dan; < 1.2 km → `walk` (≈ 80 m/daq, to'g'ri chiziq × 1.3), aks holda `taxi` (o'rtacha shahar tezligi) va `fare` — `price_items` dagi taksi tarifidan. Tashqi routing API MVP da shart emas.

**8 soniya qoidasi.** Handler ishni goroutine da boshlaydi va `select` bilan kutadi: 8 s ichida tugasa `201 + Trip`; aks holda trip `generating` holatida saqlanib `202 { tripId, status }` qaytadi, ish fonda davom etadi (`context.WithoutCancel`), natija DB ga yoziladi; mijoz `GET /trips/{id}` ni so'raydi. Fon ishi 30 s dan oshsa — fallback natijasi yoziladi, `failed` faqat nomzod umuman bo'lmaganda (`422 route_impossible`).

`/stops` qo'shish/o'chirish va `/replan` ham shu quvurdan o'tadi: o'zgartirish → [5] → `Validate()` → butun `Trip` qaytadi. Oxirgi `new|hidden` to'xtashni o'chirishga ruxsat (foydalanuvchi erki — agency), lekin `swap` takliflarida yangi joylar birinchi turadi.

---

## 6. Go ↔ AI ichki kontrakt

Ichki tarmoq, JSON over HTTP, sarlavhalar: `X-Internal-Key`, `X-Request-Id`, `X-Lang`. Batafsil model tomoni — [AI_ML.md](AI_ML.md) (endpoint nomlari aynan shu).

| Endpoint | Timeout | So'rov → Javob |
|---|---|---|
| `POST /internal/route/rank` | 6 s | `{ prefs, days, startLocation, candidates:[{id, category, novelty, isPartner, interests, location, rating, visitDurationMin, openingHours, tagline}], events:[…] }` → `{ days:[{ title, stops:[{ placeId \| eventId, kind, reason, score }] }], summary }` |
| `POST /internal/vision/identify` | 4 s | `{ imageUrl, mode, location?, quotedPrice?, lang, nearbyPlaceIds:[…] }` → `landmark`: `{ confidence, placeId?, title, subtitle, summary, facts, sources, followUps }` · `price`: `{ confidence, item, category, priceItemId? }` · `translate`: `{ detectedLang, blocks }` |
| `POST /internal/rag/answer` | birinchi token 1.5 s | `{ conversationId, placeId?, message, lang, history:[…] }` → **SSE**: `delta`, `sources`, `done` |
| `POST /internal/index/place` | 10 s (asinxron bo'lsa ham bo'ladi) | `{ placeId, texts:{uz,ru,en}, images:[url], sources }` → `{ indexed: true }` — admin `publish` da chaqiriladi |
| `POST /internal/tts` | 8 s | `{ text, lang, voice? }` → `{ audioUrl, durationSec }` (S3 ga AI yozadi yoki baytlarni qaytaradi — AI_ML.md bilan kelishiladi) |

Muhim bo'linish: **narx raqamlarini AI qaytarmaydi.** `vision/identify` (`mode=price`) faqat buyumni taniydi (`item`, `category`, `priceItemId?`); `fairRange`, `verdict`, `overpayRatio`, `sampleSize` ni Go bazadan hisoblaydi (8-bo'lim). Model narxni to'qib chiqara olmaydi.

**Chidamlilik (`internal/ai`):**
- har chaqiruvga `context` timeout; retry faqat idempotent va faqat ulanish xatosi/`503` da, 1 marta, jitter bilan;
- circuit breaker (masalan `sony/gobreaker`): ketma-ket 5 xato → 30 s ochiq; ochiq paytda `route/rank` → fallback, lens/chat → `503 ai_unavailable` (`retryable: true`);
- AI javobi qat'iy struct ga decode qilinadi (`DisallowUnknownFields` emas, lekin majburiy maydonlar tekshiriladi); bazada yo'q `placeId` qaytsa — tashlab yuboriladi;
- har chaqiruv uchun OTel span + metrika (`ai_request_duration_seconds{endpoint,outcome}`).

---

## 7. Lens va chat

**`POST /lens/identify`:**
1. `http.MaxBytesReader(w, r.Body, 3<<20)`; multipart dan `image`, `mode`, `lat/lng`, `quotedPrice`.
2. Validatsiya: `http.DetectContentType` → faqat `image/jpeg|png|webp`; `image.DecodeConfig` bilan o'lcham (uzun tomon ≤ 2000 px); EXIF olib tashlanadi (GPS sizib chiqmasin).
3. S3 ga `lens/{yyyy-mm-dd}/{uuid}.jpg`, **lifecycle 24–72 soat** (rasmlar doimiy saqlanmaydi); AI ga presigned URL.
4. `lat/lng` bo'lsa 300 m ichidagi joylar ID lari `nearbyPlaceIds` sifatida yuboriladi — tanishni keskin yaxshilaydi.
5. Javobni to'ldirish: `landmark` → `placeId` bo'lsa `audioUrl`, `conversationId` (Redis, TTL 1 soat); `price` → 8-bo'lim; `translate` → o'tkazib yuboriladi.
6. **`landmark` da `sources` bo'sh yoki `confidence < 0.3` → `422 lens_not_recognized`.** Manbasiz tarix mijozga ketmaydi.

**`POST /guide/chat` (SSE proksi):**

```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("X-Accel-Buffering", "no")          // nginx buferlamasin
rc := http.NewResponseController(w)

req, _ := http.NewRequestWithContext(r.Context(), "POST", aiURL+"/internal/rag/answer", body)
resp, err := aiClient.Do(req)                       // mijoz uzilsa r.Context() bekor bo'ladi → AI ham to'xtaydi
// ... xato → oqim boshlanmagan bo'lsa oddiy JSON xato (503)
sc := bufio.NewScanner(resp.Body)
for sc.Scan() {
	fmt.Fprintf(w, "%s\n", sc.Text())
	if sc.Text() == "" { rc.Flush() }               // hodisa chegarasida flush
}
```

Server `WriteTimeout` bu yo'l uchun o'chiriladi (`rc.SetWriteDeadline`). Suhbat tarixi Redis da (`conv:{id}`, oxirgi 10 xabar).

---

## 8. Narx moduli

`fairRange` kuzatuvlardan hisoblanadi, qo'lda yozilmaydi (seed bundan mustasno):

1. Oyna: oxirgi **90 kun**; manba og'irligi: `moderator`/`partner` > `open_data` > `user`.
2. Outlierlarni kesish: `Q1 − 1.5·IQR … Q3 + 1.5·IQR` dan tashqarisi tashlanadi (turistga aytilgan 5× narxlar medianani buzmasin).
3. `fair_min = P25`, `fair_max = P75` (yaxlitlash: 1 000 so'mgacha), `sample_size = n`, `updated_at = now()`.
4. `n < 5` bo'lsa avvalgi/seed qiymat saqlanadi — kichik tanlanma bilan diapazon o'zgartirilmaydi.
5. Qayta hisob: `POST /prices/reports` dan keyin shu item uchun (debounce, Redis) yoki soatlik job.

```sql
SELECT percentile_cont(0.25) WITHIN GROUP (ORDER BY paid) AS p25,
       percentile_cont(0.75) WITHIN GROUP (ORDER BY paid) AS p75, count(*) AS n
FROM price_observations
WHERE price_item_id = $1 AND observed_at > now() - interval '90 days';
```

**Verdict** (`quotedPrice` berilgan bo'lsa), `ratio = quoted / fair_max`:

| Shart | `verdict` |
|---|---|
| `ratio ≤ 1.3` | `fair` |
| `1.3 < ratio ≤ 2.0` | `high` |
| `ratio > 2.0` | `very_high` |

`overpayRatio = round(ratio, 1)`. `tip` va `phrases` — i18n shablonlar (savdolashish iboralari statik lug'at, `audioUrl` oldindan generatsiya qilingan). Foydalanuvchi kuzatuvlariga himoya: qurilma boshiga kuniga limit, `paid` mantiqiy chegarada (item medianasining 0.1×–20×).

---

## 9. Geofence va oflayn paket

- `GET /geofences?city=` — yengil ro'yxat; `topRules` = `place_rules` dan `severity` (`forbidden` > `warn` > `info`), keyin `priority` bo'yicha birinchi 3 tasi. `version = places.rules_version`.
- `PUT /admin/places/{id}/rules` → tranzaksiyada qoidalar almashtiriladi va `rules_version + 1`. Mijoz versiyalar farqi bo'yicha yangilaydi.
- Geofencing **qurilmada** ishlaydi (OS API); server turistning kirish-chiqishini bilmaydi va kuzatmaydi.
- `GET /cities/{id}/offline-pack` — bitta JSON (joylar to'liq, qoidalar, geofence, narxlar, rasm URL ro'yxati), til bo'yicha. `ETag = sha256(max(updated_at), count, lang)`; `If-None-Match` → `304`. Javob Redis/CDN da keshlanadi, `publish`/qoida o'zgarishida invalidatsiya. `gzip` shart.
- Katalog keshi: `Cache-Control: public, max-age=60` + `ETag`; `Vary: Accept-Language`.

---

## 10. Xavfsizlik va maxfiylik

| Mavzu | Yechim |
|---|---|
| Sessiya | `POST /auth/device` → imzolangan token (JWT HS256 yoki PASETO v4.local), `sub=userId`, `role`, muddat 30 kun. Login yo'q; `deviceId` — mijoz yaratgan tasodifiy qiymat |
| Rate limit | Redis token bucket: umumiy 120/daq; `lens/identify` 20/daq; `POST /trips` 6/daq; `auth/device` IP bo'yicha. Oshsa `429` + `Retry-After` |
| Joylashuv | so'rovdagi `lat/lng` **saqlanmaydi** (faqat so'rov davomida). Statistika uchun ~500 m katakka (`geocell`, masalan geohash-6) umumlashtiriladi. Loglarga koordinata yozilmaydi |
| Anonim shikoyat | `scam_reports` da `user_id`/`device_id` yo'q; rate limit faqat Redis da, vaqtinchalik |
| Admin | `role ∈ {moderator, admin}` middleware; `publish`, qoida va narx tahriri audit log ga (`admin_audit`: kim, nima, eski/yangi) |
| Yuklash | 7-bo'limdagi tekshiruvlar; fayl nomi mijozdan olinmaydi; S3 bucket ochiq emas, presigned URL |
| Kirish ma'lumoti | hamma tashqi kirish validatsiyadan o'tadi (`400 validation_failed`); SQL faqat parametrli (sqlc) |
| Sirlar | faqat env; repoga `.env.example` |
| CORS | web platforma domenlari ro'yxati; `*` emas |
| Hamkor shaffofligi | `isPartner` har doim javobda; saralashda hamkorlik ball qo'shmaydi; kvotaga kirmaydi (5-bo'lim) |

---

## 11. Hackathon rejasi (ustuvorlik tartibida)

| Bosqich | Natija | Mobil ilova nima oladi |
|---|---|---|
| **1** | repo, compose, migratsiyalar, seed; `POST /auth/device`, `GET /cities`, `GET /places`, `GET /places/{id}`, `GET /geofences`; i18n fallback; xato formati | Kashf ro'yxati, joy sahifasi, qoidalar real ma'lumotdan |
| **2** | `POST /trips` **fallback rejalashtirgich bilan** (AI siz ham ishlaydi), `GET /trips/{id}`, `/stops` qo'shish/o'chirish, `Validate()` + testlar; keyin AI `route/rank` ulanadi | asosiy demo oqimi: marshrut yaratish |
| **3** | `POST /lens/identify` proksi, `GET /prices`, verdict hisobi, `POST /guide/chat` SSE; `GET /events`, `GET /stays` | Lens (tarix + narx), savol-javob |
| **4** | admin: moderatsiya navbati, joy qo'shish, `publish` → `index/place`; `offline-pack`; `POST /visits` (ball) | "yangi joy qo'shildi → marshrutda paydo bo'ldi" jonli demosi |

**Definition of done:** mobil ilovada `EXPO_PUBLIC_API_URL` ni backend manziliga qo'yganda mock o'chadi va onboarding → marshrut → joy sahifasi → Lens oqimi **kod o'zgarishisiz** ishlaydi. Tekshiruv: har bir endpoint javobi kontraktdagi shaklga mos (OpenAPI validatsiya testi), `uz/ru/en` uchalasi matn qaytaradi, AI xizmati o'chirilganda ham `POST /trips` `201` beradi.

Eng kuchli demo lahzasi — 4-bosqichdagi jonli ko'rsatuv: adminda yangi joy chop etiladi va bir necha soniyadan keyin yangi marshrutda "Yangi" belgisi bilan chiqadi. Vaqt qisilsa 3-bosqichdagi `events`/`stays` ni qurbon qiling, buni emas.

---

## 12. Seed ma'lumot

- Ikki shahar: `samarkand`, `bukhara`. Har birida kamida **15–20 joy**: ~60% `iconic`, ~40% `new|hidden` (kvota bajarilishi uchun yangi joylar yetarli bo'lsin), 3–4 ta ovqat joyi, 3–5 tadbir, 4–6 turar joy, 15+ narx elementi (taksi, do'ppi, non, osh, suvenir, chipta).
- **ID/sluglar mobil mock bilan bir xil bo'lishi shart** — manba: `mobile/src/mock/`. Avval o'sha fayllardan ID larni oling (`registan`, `shah-i-zinda`, …), o'zingiz yangi nom o'ylab topmang; farq bo'lsa mockni emas, kelishuvni tuzating.
- Format: `seed/*.json` (kontrakt shakliga yaqin, matnlar `{uz,ru,en}`), `cmd/seed` idempotent `UPSERT` qiladi.
- Tarix matnlari va `sources` AI/ML dasturchi bilan birga tayyorlanadi (RAG korpusi bilan bir manba). Qoidalar va narxlar — **namuna** deb belgilanadi (`verified_by = 'demo'`), haqiqiy ma'lumot pilotda yig'iladi.
- Rasmlar: litsenziyasi ochiq manbalar (Wikimedia Commons), `credit` to'ldiriladi.

---

## 13. Lokal ishga tushirish va testlar

```yaml
# docker-compose.yml (qisqartirilgan)
services:
  db:
    image: postgis/postgis            # pgvector qo'shilgan image yoki o'z Dockerfile
    environment: { POSTGRES_DB: yoldosh, POSTGRES_USER: yoldosh, POSTGRES_PASSWORD: yoldosh }
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
  redis: { image: redis, ports: ["6379:6379"] }
  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports: ["9000:9000", "9001:9001"]
  api:
    build: .
    env_file: .env
    depends_on: [db, redis, minio]
    ports: ["8080:8080"]
volumes: { pgdata: {} }
```

```makefile
up:        ; docker compose up -d db redis minio
migrate:   ; goose -dir db/migrations postgres "$$DATABASE_URL" up
gen:       ; sqlc generate && oapi-codegen -config api/oapi.yaml api/openapi.yaml
seed:      ; go run ./cmd/seed
run:       ; go run ./cmd/api
test:      ; go test ./... -race
lint:      ; golangci-lint run
```

Asosiy env: `DATABASE_URL`, `REDIS_URL`, `S3_ENDPOINT/KEY/SECRET/BUCKET`, `AI_BASE_URL`, `AI_INTERNAL_KEY`, `TOKEN_SECRET`, `CORS_ORIGINS`. `AI_BASE_URL` bo'sh bo'lsa — fallback rejimi (marshrut ishlaydi, lens/chat `503`).

**Test strategiyasi:**
- **Invariantlar — table-driven unit testlar** (`domain`): yangi joysiz kun, ulush 0.24/0.25 chegarasi, faqat hamkor yangi joy (kvota bajarilmagan bo'lishi kerak), ustma-ust vaqt, bo'sh `reason`.
- **Rejalashtirgich — property uslubida:** tasodifiy prefs × seed ma'lumot → fallback natijasi doim `Validate() == nil`. AI mijozi uchun fake: yaxshi javob, kvotani buzgan javob, timeout, buzuq JSON — to'rttasida ham yakuniy `Trip` yaroqli.
- **Store — testcontainers** (haqiqiy PostGIS): `ST_DWithin`, i18n fallback tartibi, percentil so'rovi.
- **HTTP — kontrakt testlari:** har endpoint javobi OpenAPI sxemaga validatsiya qilinadi; xato formati; `Accept-Language` uch til.
- **Narx:** verdict chegaralari (1.3 / 2.0), outlier kesish, `n < 5`.
- CI: `lint` + `test -race` + `sqlc diff` (generatsiya yangilanganini tekshirish).
