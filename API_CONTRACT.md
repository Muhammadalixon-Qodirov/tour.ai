# Yo'ldosh AI — API kontrakti (v1)

Bu hujjat **uch jamoa o'rtasidagi yagona kelishuv**: mobil/web (frontend), backend (Go) va AI/ML.
Frontend hozir shu kontraktga mos mock ma'lumot bilan ishlaydi (`mobile/src/mock/`), backend tayyor bo'lganda faqat `EXPO_PUBLIC_API_URL` almashtiriladi.

> Kontraktni o'zgartirish kerak bo'lsa: avval shu faylga PR, uch tomon ko'radi, keyin kod. Maydonni jimgina qayta nomlash yo'q.

Tegishli hujjatlar: [BACKEND.md](BACKEND.md) · [AI_ML.md](AI_ML.md) · [DESIGN.md](DESIGN.md) · [../main.md](../main.md)

---

## 1. Umumiy qoidalar

| Mavzu | Qoida |
|---|---|
| Base URL | `/api/v1` |
| Format | JSON, maydon nomlari **camelCase**, vaqt **RFC 3339 UTC** (`2026-11-25T09:30:00Z`) |
| ID | string (UUID v7 yoki ULID). Frontend ID ichini tahlil qilmaydi |
| Til | `Accept-Language: uz \| ru \| en`. Server **bitta tildagi tayyor matn** qaytaradi (tarjima xaritasi emas). Tarjima yo'q bo'lsa `en`, u ham bo'lmasa `uz` |
| Pul | butun son, **so'mda** (`amount: 80000`, `currency: "UZS"`). Formatlashni frontend qiladi |
| Koordinata | `{ "lat": 39.6547, "lng": 66.9758 }` (WGS84) |
| Masofa / vaqt | metr (`distanceM`) va daqiqa (`durationMin`) |
| Sahifalash | `?limit=20&cursor=...` → `{ "items": [...], "nextCursor": "..." \| null }` |
| Auth | `Authorization: Bearer <token>`; token anonim qurilma sessiyasidan olinadi (login shart emas) |
| Rasm | to'liq HTTPS URL + `blurhash`. O'lcham: `?w=800` qo'shsa server/CDN kichraytiradi |

### Xato formati (hamma endpoint uchun bir xil)

```json
{ "error": { "code": "place_not_found", "message": "Joy topilmadi", "retryable": false } }
```

| HTTP | `code` misollari | Frontend xatti-harakati |
|---|---|---|
| 400 | `validation_failed` | maydon yonida xabar |
| 401 | `unauthorized` | sessiyani qayta ochadi, so'rovni 1 marta takrorlaydi |
| 404 | `place_not_found`, `trip_not_found` | bo'sh holat ekrani |
| 422 | `lens_not_recognized`, `route_impossible` | tushunarli matn + muqobil taklif |
| 429 | `rate_limited` (`Retry-After` sarlavhasi bilan) | kutish |
| 503 | `ai_unavailable` | "Keshdagi ma'lumot ko'rsatilmoqda" banneri, `retryable: true` |

`message` doim foydalanuvchi tilida va unga ko'rsatsa bo'ladigan matn.

---

## 2. Asosiy turlar

```ts
type Lang = "uz" | "ru" | "en";
type LatLng = { lat: number; lng: number };
type Money = { amount: number; currency: "UZS" };
type Image = { url: string; blurhash?: string; credit?: string };

type Interest = "history" | "food" | "nature" | "craft" | "art" | "architecture" | "shopping" | "nightlife";
type Companion = "solo" | "couple" | "family" | "friends";
type Pace = "relaxed" | "balanced" | "active";
type BudgetLevel = 1 | 2 | 3;              // tejamkor | o'rtacha | qulay

type PlaceCategory =
  | "monument" | "museum" | "mosque" | "bazaar" | "workshop" | "gallery"
  | "restaurant" | "cafe" | "park" | "viewpoint" | "ecovillage";

/** Loyihaning yuragi: joy "mashhur"mi yoki "yangi/kam tanilgan"mi. */
type Novelty = "iconic" | "new" | "hidden";

type RuleCode =
  | "no_flash" | "no_photo" | "photo_fee" | "dress_code" | "shoes_off" | "quiet"
  | "no_drone" | "no_food" | "no_smoking" | "no_pets" | "ticket_required" | "prayer_time_limit";

type Rule = {
  code: RuleCode;
  severity: "info" | "warn" | "forbidden";
  text: string;                 // qisqa, 1 gap, foydalanuvchi tilida
  verifiedBy?: string;          // "Obyekt ma'muriyati"
  verifiedAt?: string;
};

type PlaceSummary = {
  id: string;
  cityId: string;
  name: string;
  tagline: string;              // 1 qator: "XV asr rasadxonasi"
  category: PlaceCategory;
  novelty: Novelty;
  openedAt?: string;            // novelty = "new" bo'lsa
  cover: Image;
  location: LatLng;
  rating: number;               // 0–5
  reviewsCount: number;
  priceLevel: 0 | 1 | 2 | 3;    // 0 = bepul
  visitDurationMin: number;
  isOpenNow?: boolean;
  crowd?: { level: "low" | "medium" | "high"; bestTime?: string };
  isPartner: boolean;           // pullik ko'rsatuv — UI da doim "Hamkor" belgisi
};

type Place = PlaceSummary & {
  description: string;
  images: Image[];
  address: string;
  openingHours: { day: 0|1|2|3|4|5|6; open: string; close: string }[];  // "09:00"
  entryFee?: { local?: Money; foreigner?: Money; note?: string };
  rules: Rule[];
  history?: { summary: string; facts: string[]; sources: Source[] };
  audioGuide?: { url: string; durationSec: number; lang: Lang };
  accessibility: { wheelchair: boolean; familyFriendly: boolean; easyWalk: boolean };
  payment: { card: boolean; cashOnly: boolean };
  geofenceRadiusM: number;
  nearbyPrices: PriceItem[];    // shu joy atrofida dolzarb narxlar (0–5 ta)
};

type Source = { title: string; url?: string };   // RAG manbasi — gallyutsinatsiyaga qarshi
```

---

## 3. Sessiya va profil

### `POST /auth/device`
```json
// so'rov
{ "deviceId": "ios-...", "platform": "ios", "lang": "uz" }
// javob 200
{ "token": "...", "userId": "u_...", "expiresAt": "..." }
```

### `GET /me` · `PATCH /me`
```json
{
  "id": "u_...", "lang": "uz",
  "interests": ["history", "food"], "companion": "couple", "pace": "balanced", "budget": 2,
  "stats": { "placesVisited": 7, "newPlacesVisited": 3, "points": 240 },
  "badges": [{ "code": "bukhara_explorer", "title": "Buxoro kashfiyotchisi", "earnedAt": "..." }]
}
```

---

## 4. Katalog

| Endpoint | Tavsif |
|---|---|
| `GET /cities` | `[{ id, name, country, cover, center: LatLng, placesCount, newPlacesCount }]` |
| `GET /places` | filtrlar: `city`, `category`, `novelty`, `interest`, `q`, `near=lat,lng`, `radiusM`, `openNow`, `wheelchair` → `PlaceSummary[]` (sahifalangan) |
| `GET /places/{id}` | to'liq `Place` |
| `GET /places/{id}/reviews` | `[{ id, author, rating, text, createdAt }]` |
| `GET /events` | filtrlar: `city`, `from`, `to`, `interest` → `Event[]` |
| `GET /stays` | filtrlar: `city`, `near=lat,lng`, `maxPrice`, `checkIn`, `checkOut` → `Stay[]` |
| `GET /geofences?city=` | qurilma ichida geofencing uchun yengil ro'yxat (pastda) |

```ts
type Event = {
  id: string; cityId: string; title: string; kind: "festival" | "concert" | "fair" | "exhibition" | "sport";
  cover: Image; startsAt: string; endsAt: string; venue: string; location: LatLng;
  price?: Money; isFree: boolean; ticketUrl?: string;
};

type Stay = {
  id: string; cityId: string; name: string; kind: "hotel" | "guesthouse" | "boutique" | "hostel";
  cover: Image; location: LatLng; rating: number; reviewsCount: number;
  pricePerNight: Money; fairPrice: boolean;      // "Adolatli narx" belgisi
  verified: boolean; distanceToRouteM?: number; amenities: string[];
};

/** Ilova shaharni ochganda bir marta yuklab oladi va OS geofencing ga beradi (oflayn ishlaydi). */
type Geofence = {
  placeId: string; name: string; center: LatLng; radiusM: number;
  headline: string;             // "Siz Ko'kaldosh madrasasidasiz"
  topRules: Rule[];             // eng muhim 2–3 tasi
  version: number;
};
```

---

## 5. Marshrut (Kashf) — asosiy funksiya

### `POST /trips` — marshrut yaratish
```json
{
  "cityId": "samarkand",
  "startDate": "2026-12-02",
  "days": 2,
  "interests": ["history", "food", "craft"],
  "companion": "couple",
  "pace": "balanced",
  "budget": 2,
  "startLocation": { "lat": 39.6547, "lng": 66.9758 },
  "constraints": { "wheelchair": false, "withKids": false, "vegetarian": false }
}
```
Javob `201` — to'liq `Trip`. Generatsiya **8 soniyadan** oshsa server `202 { tripId, status: "generating" }` qaytaradi va frontend `GET /trips/{id}` ni 1.5 s oraliqda so'raydi (yoki `GET /trips/{id}/stream` SSE).

```ts
type Trip = {
  id: string; cityId: string; title: string;       // "Samarqand · 2 kun"
  status: "generating" | "ready" | "failed";
  startDate: string;
  summary: string;                                  // AI yozgan 1–2 gap
  noveltyShare: number;                             // 0–1: yangi/kam tanilgan joylar ulushi
  days: TripDay[];
  totals: { places: number; newPlaces: number; walkingM: number; estimatedCost: Money };
};

type TripDay = { index: number; date: string; title: string; stops: Stop[] };

type Stop = {
  id: string;
  kind: "place" | "meal" | "event";
  place?: PlaceSummary;          // kind = place | meal
  event?: Event;                 // kind = event
  arriveAt: string;              // "09:30" mahalliy vaqt
  durationMin: number;
  reason: string;                // NEGA shu joy: "Hunarmandchilikka qiziqishingiz uchun"
  transitFromPrev?: { mode: "walk" | "taxi" | "bus" | "metro"; durationMin: number; distanceM: number; fare?: Money };
  status: "upcoming" | "current" | "visited" | "skipped";
};
```

**Majburiy biznes qoidalar (backend + AI kafolatlaydi, frontend ko'rsatadi):**
1. **Yangilik kvotasi:** har bir kunda kamida **1 ta** `novelty ∈ {new, hidden}` to'xtash; butun marshrutda `noveltyShare ≥ 0.25`.
2. `isPartner: true` joy yangilik kvotasini **to'ldirmaydi** va reytingni sun'iy ko'tarmaydi.
3. Har bir `Stop.reason` bo'sh bo'lmaydi — shaxsiylashtirish ko'rinib turishi kerak.
4. To'xtashlar ish vaqtiga va `transitFromPrev` ga mos tartiblangan (ustma-ust tushmaydi).

### Boshqa marshrut endpointlari
| Endpoint | Tavsif |
|---|---|
| `GET /trips` · `GET /trips/{id}` | ro'yxat / bitta marshrut |
| `POST /trips/{id}/stops` | `{ dayIndex, placeId \| eventId, after?: stopId }` — bir bosishda qo'shish; server vaqtlarni qayta hisoblaydi va **butun `Trip`** qaytaradi |
| `DELETE /trips/{id}/stops/{stopId}` | olib tashlash → yangilangan `Trip` |
| `POST /trips/{id}/stops/{stopId}/swap` | shunga o'xshash boshqa joy taklifi → `{ alternatives: PlaceSummary[] }` |
| `POST /trips/{id}/replan` | `{ reason: "late" \| "tired" \| "weather" \| "free_time", minutes?: number, location?: LatLng }` → yangilangan `Trip` + `changes: string[]` |
| `POST /visits` | `{ placeId, tripId?, rating?: 1–5, comment? }` → `{ pointsEarned, badge? }` |

---

## 6. Lens (Raqamli gid + Adolatli narx + Tarjima)

### `POST /lens/identify` — `multipart/form-data`
| maydon | tur | izoh |
|---|---|---|
| `image` | fayl (JPEG, ≤ 2 MB, uzun tomoni ≤ 1600 px — frontend siqadi) | |
| `mode` | `landmark \| price \| translate` | |
| `lat`, `lng` | son, ixtiyoriy | tanishni keskin aniqlashtiradi |
| `quotedPrice` | son, ixtiyoriy (`mode=price`) | sotuvchi aytgan narx |

Javob — `mode` ga qarab uchta shakldan biri (`kind` diskriminator):

```ts
type LensResult = LandmarkResult | PriceResult | TranslateResult;

type LandmarkResult = {
  kind: "landmark";
  confidence: number;            // 0–1; < 0.5 bo'lsa UI "Aniq emas" deb ko'rsatadi
  placeId?: string;              // bazadagi joy bilan mos kelsa
  title: string;
  subtitle: string;              // "1417–1420 · Me'mor: ..."
  summary: string;               // 2–3 gap
  facts: string[];
  sources: Source[];             // BO'SH BO'LMAYDI. Manba yo'q bo'lsa server 422 qaytaradi
  audioUrl?: string;
  followUps: string[];           // "Nega minora qiyshaygan?"
  conversationId: string;        // savol-javobni davom ettirish uchun
};

type PriceResult = {
  kind: "price";
  confidence: number;
  item: string;                  // "Do'ppi (qo'lda tikilgan)"
  category: PriceCategory;
  fairRange: { min: Money; max: Money; unit: string };   // unit: "dona", "kg", "km"
  quoted?: Money;
  verdict?: "fair" | "high" | "very_high";               // quoted berilgan bo'lsa
  overpayRatio?: number;                                 // 3.3 = 3.3 barobar
  tip: string;                                           // "Savdolashing, 100 000 dan boshlang"
  phrases: { text: string; transliteration?: string; meaning: string; audioUrl?: string }[];
  updatedAt: string;             // narx ma'lumoti qachon yangilangan
  sampleSize: number;            // nechta kuzatuv asosida
};

type TranslateResult = {
  kind: "translate";
  detectedLang: string;
  blocks: { original: string; translated: string; note?: string }[];   // note: taom tarkibi, allergen
};
```

### `POST /guide/chat` — savol-javob (SSE oqim)
```json
{ "conversationId": "c_...", "placeId": "registan", "message": "Nega minora qiyshaygan?" }
```
`text/event-stream`:
```
event: delta   data: {"text":"Ulug'bek madrasasining "}
event: delta   data: {"text":"minorasi..."}
event: sources data: {"sources":[{"title":"...","url":"..."}]}
event: done    data: {"messageId":"m_..."}
```
Bazada javob bo'lmasa model to'qimaydi: `event: done` oldidan `{"text":"Bu haqda tekshirilgan ma'lumotim yo'q."}`.

### `POST /speech/tts` → `{ audioUrl, durationSec }` · `POST /speech/stt` (multipart `audio`) → `{ text, lang }`

---

## 7. Narxlar

```ts
type PriceCategory = "taxi" | "food" | "souvenir" | "market" | "ticket" | "transport";

type PriceItem = {
  id: string; cityId: string; category: PriceCategory;
  name: string;                  // "Taksi: Registon → Vokzal"
  fairRange: { min: Money; max: Money; unit: string };
  touristTypical?: Money;        // turistlarga odatda aytiladigan narx
  updatedAt: string; sampleSize: number;
};
```

| Endpoint | Tavsif |
|---|---|
| `GET /prices?city=&category=&q=` | `PriceItem[]` |
| `POST /prices/reports` | `{ priceItemId? , name, category, paid: Money, location?: LatLng }` — foydalanuvchi kuzatuvi (anonim) |
| `POST /reports/scam` | `{ location, category, description }` — anonim shikoyat → `202` |
| `GET /vendors?city=&near=` | "Ishonchli sotuvchi" belgili joylar: `[{ id, name, category, location, trustedSince }]` |

---

## 8. Admin (web platforma uchun, `role: moderator | admin`)

| Endpoint | Tavsif |
|---|---|
| `GET /admin/places?status=pending` | moderatsiya navbati |
| `POST /admin/places` · `PATCH /admin/places/{id}` | joy qo'shish / tahrirlash (hamma tilda matn: `{ uz, ru, en }`) |
| `POST /admin/places/{id}/publish` | chop etish → AI xizmatiga qayta indekslash signali |
| `PUT /admin/places/{id}/rules` | qoidalarni yangilash → `Geofence.version` oshadi |
| `GET /admin/analytics/novelty` | KPI: yangi joylarga tashrif ulushi, shahar bo'yicha |

---

## 9. Kelishuvlar

- **Kechikish byudjeti:** katalog `p95 < 300 ms`; `lens/identify` `p95 < 4 s`; `POST /trips` `p95 < 8 s` (oshsa 202 + polling); chat birinchi token `< 1.5 s`.
- **Oflayn:** `GET /cities/{id}/offline-pack` → shahar bo'yicha bitta JSON (joylar, qoidalar, geofence, narxlar) + rasm URL ro'yxati; `ETag` bilan.
- **Mock:** frontend `EXPO_PUBLIC_API_URL` bo'sh bo'lsa mock rejimda ishlaydi — shu kontraktdagi shakllar bilan aynan bir xil.
- **Versiya:** buzuvchi o'zgarish = `/api/v2`. Yangi ixtiyoriy maydon qo'shish buzuvchi emas.
