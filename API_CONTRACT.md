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
type Image = {
  url: string; blurhash?: string;
  credit?: string;      // muallif yoki sayt
  license?: string;     // "CC BY-SA 4.0"; noma'lum bo'lsa yo'q
  sourceUrl?: string;   // rasm olingan sahifa (Commons yoki ochiq sahifa)
  source?: string;      // "Wikimedia Commons", "afisha.uz"
};

type Interest = "history" | "food" | "nature" | "craft" | "art" | "architecture" | "shopping" | "nightlife";
type Companion = "solo" | "couple" | "family" | "friends";
type Pace = "relaxed" | "balanced" | "active";
type BudgetLevel = 1 | 2 | 3;              // tejamkor | o'rtacha | qulay

type PlaceCategory =
  | "monument" | "museum" | "mosque" | "bazaar" | "workshop" | "gallery"
  | "restaurant" | "cafe" | "park" | "viewpoint" | "ecovillage" | "theatre";

/** Loyihaning yuragi: joy "mashhur"mi yoki "yangi/kam tanilgan"mi. */
type Novelty = "iconic" | "new" | "hidden";

type RuleCode =
  | "no_flash" | "no_photo" | "photo_fee" | "dress_code" | "shoes_off" | "quiet"
  | "no_drone" | "no_food" | "no_smoking" | "no_pets" | "ticket_required" | "prayer_time_limit";

/** Ishonch darajasi: obyekt ma'muriyati / gov.uz · ochiq manba · foydalanuvchi kuzatuvi · namuna (demo). UI sample ni belgilaydi. */
type Trust = "official" | "public" | "community" | "sample";

type Rule = {
  code: RuleCode;
  severity: "info" | "warn" | "forbidden";
  text: string;                 // qisqa, 1 gap, foydalanuvchi tilida
  trust: Trust;
  verifiedBy?: string;          // "Obyekt ma'muriyati"
  verifiedAt?: string;
  source?: string;              // URL yoki "general etiquette ..."
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
  trust: Trust;                 // tadqiqot katalogi: public
  distanceM?: number;           // near bilan so'ralganda
};

type Place = PlaceSummary & {
  description: string;
  aliases: string[];            // mahalliy nomlar ("Kalon minorasi", "Siyob"); GET /places?q= ular bo'yicha ham qidiradi
  verifiedAt?: string;
  images: Image[];
  address: string;
  openingHours: {
    day: 0|1|2|3|4|5|6; open: string; close: string;                    // "09:00"
    breaks?: { from: string; to: string; reason?: "lunch" | "prayer" | "other" }[];  // shu vaqtda yopiq (juma namozi)
  }[];
  hoursTrust: Trust;            // ish vaqti ishonchi (tadqiqotda sample)
  hoursNote?: string;           // dam olish kuni, seanslar, ta'mir
  entryFee?: { local?: Money; foreigner?: Money; note?: string; source?: Source; trust: Trust };  // noma'lum tarif yo'q; 0 = bepul
  feeNote?: string;             // narx nimaga bog'liq (yosh, gid, shou); entryFee.note bilan bir xil
  rules: Rule[];
  history?: { summary: string; facts: string[]; sources: Source[]; trust: Trust };
  audioGuide?: { url: string; durationSec: number; lang: Lang };
  accessibility: { wheelchair: boolean; familyFriendly: boolean; easyWalk: boolean };
  payment: { card: boolean; cashOnly: boolean };
  geofenceRadiusM: number;
  nearbyPrices: PriceItem[];    // 0–5 ta: shu joyning o'z chiptasi birinchi, keyin joy turiga mos narxlar
                                // (bozor: market/souvenir/food; ustaxona, galereya: souvenir; restoran, kafe: food;
                                //  qolganlari: taxi). Boshqa obidalarning chiptalari kirmaydi.
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
  "isResident": true, // O'zbekiston rezidenti: kirish narxi mahalliy tarifda; null = aytilmagan. PATCH /me da ham
  "analyticsOptOut": false, // true: safarlari Safar Insights statistikasiga kirmaydi. PATCH /me { analyticsOptOut } (§10)
  "role": "tourist",        // tourist | partner | insights (admin beradi, §10); partner bo'lsa "partnerId" ham
  "stats": { "placesVisited": 7, "newPlacesVisited": 3, "points": 240 },
  "badges": [{ "code": "bukhara_explorer", "title": "Buxoro kashfiyotchisi", "earnedAt": "..." }]
}
```

---

## 4. Katalog

| Endpoint | Tavsif |
|---|---|
| `GET /cities` | `[{ id, name, country, cover, center: LatLng, placesCount, newPlacesCount }]` |
| `GET /places` | filtrlar: `city`, `category`, `novelty`, `interest`, `q`, `near=lat,lng`, `radiusM`, `openNow`, `wheelchair`, `source=curated|osm` → `PlaceSummary[]` (sahifalangan; jamoa tekshirgan joylar har doim OSM importidan oldin; har bir joyda `source`: `osm` = OpenStreetMap ommaviy importi) |
| `GET /places/{id}` | to'liq `Place` |
| `GET /places/{id}/reviews` | `[{ id, author, rating, text, createdAt }]` |
| `GET /events` | filtrlar: `city`, `from`, `to` (`YYYY-MM-DD` shahar vaqti yoki RFC 3339) → `Event[]`, `startsAt` bo'yicha. O'tib ketgan (`endsAt < hozir`) tadbirlar hech qachon qaytmaydi. `interest` hali yo'q |
| `GET /stays` | filtrlar: `city` yoki `near=lat,lng` (shart), `radiusM` (near bilan, standart 3000, ≤ 50000), `kind=hotel,hostel,...`, `maxPrice` (so'm, `priceMin` bo'yicha), `q`, `limit` (≤ 50), `cursor` → `{ items: Stay[], nextCursor }`. `near` bo'lsa masofa bo'yicha, aks holda tekshirilgan / aniq narxli / reytingli birinchi |
| `GET /stays/{id}` | bitta `Stay`; topilmasa `404 stay_not_found` |

Katalog GET javoblari (`/cities`, `/places`, `/places/{id}`, `/geofences`, `/prices`, `/events`, `/stays`) `ETag` beradi;
`If-None-Match` mos kelsa `304`. ETag tilga bog'liq (`Vary: Accept-Language`).
| `GET /geofences?city=` | qurilma ichida geofencing uchun yengil ro'yxat (pastda) |

```ts
type Event = {
  id: string; cityId: string; title: string; kind: "festival" | "concert" | "fair" | "exhibition" | "sport";
  cover: Image; startsAt: string; endsAt: string; venue: string; location: LatLng;
  price?: Money; isFree: boolean; ticketUrl?: string;
  source?: Source;               // tadbir e'lon qilingan sahifa
  recurring?: "annual";          // sana o'tgan yilgi tadbirdan ko'chirilgan
};

type Stay = {
  id: string; cityId: string; name: string;
  kind: "hotel" | "boutique" | "guesthouse" | "hostel" | "motel" | "apartment";
  cover: Image; images: Image[];                 // rasm yo'q bo'lsa cover = /static/ph/<kind>.svg
  location: LatLng; address: string; stars: number | null;
  rating: number | null;                         // 5 ballik (10 ballik manba 2 ga bo'lingan)
  ratingSource?: Source;                         // tadqiqot yozuvlarida
  reviewsCount: number;
  pricePerNight: {                               // so'm, 2 kishi, oddiy xona
    min: Money | null; max: Money | null;
    estimated: boolean;                          // true: OSM turi/yulduzi bo'yicha taxmin, UI da "taxminan"
    trust: Trust | "estimate" | null; note?: string; updatedAt?: string;
    source?: Source;                             // tadqiqot yozuvlarida narx qayerdan
  };
  fairPrice: boolean; verified: boolean; isPartner: boolean;
  amenities: string[];   // kodlar: wifi, breakfast, ac, parking, transfer, courtyard, kitchen, private_bathroom, laundry,
                         // pool, spa, sauna, hammam, restaurant, bar, gym, room_service, tea_coffee
  contacts: { phone?: string; website?: string; telegram?: string; instagram?: string; email?: string };
  checkIn?: string; checkOut?: string; wheelchair: boolean | null;
  emehmon: boolean | null;                       // chet ellikni E-Mehmon orqali ro'yxatga oladimi
  bookingUrl?: string; agodaUrl?: string;        // hamkor havolalar: faqat web saytda ko'rsatiladi
  trust: Trust; distanceM?: number;
};
// Manbalar: OSM importi (narx taxmin, estimated = true) va jamoa tadqiqoti (28 ta: priceMin = tadqiqotdagi "dan" narx,
// trust sample, ratingSource / pricePerNight.source bilan). Admin aniq narx kiritsa trust official.

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
  "budgetPerDay": 300000,
  "isResident": true,
  "groupSize": 3,
  "startLocation": { "lat": 39.6547, "lng": 66.9758 },
  "constraints": { "wheelchair": false, "withKids": false, "vegetarian": false }
}
```
- `days`: 1..7 (ilova va sayt hozir 1..3 taklif qiladi).
- `budgetPerDay` — **kishi boshiga, kuniga, so'mda** (yo'q yoki `null` = cheklovsiz). Kun xarajati (kirish + tushlik 60 000 + taksi ulushi) shu chegaraga sig'guncha **faqat mashhur joylar** (eng qimmati birinchi) olib tashlanadi; yangi/kam tanilgan joylar kvota uchun saqlanadi.
- `isResident` — `true`: kirish narxi mahalliy tarifda. Yuborilmasa profildagi `isResident` (`PATCH /me`) olinadi.
- `groupSize` — taksi shu songa bo'linadi; yo'q bo'lsa `companion` dan (solo 1, couple 2, family 3, friends 4).

Javob `201` — to'liq `Trip`. Generatsiya **8 soniyadan** oshsa server `202 { tripId, status: "generating" }` qaytaradi va frontend `GET /trips/{id}` ni 1.5 s oraliqda so'raydi (yoki `GET /trips/{id}/stream` SSE).

```ts
type Trip = {
  id: string; cityId: string; title: string;       // "Samarqand · 2 kun"
  status: "generating" | "ready" | "failed";
  startDate: string;
  summary: string;                                  // AI yozgan 1–2 gap
  noveltyShare: number;                             // 0–1: yangi/kam tanilgan joylar ulushi
  days: TripDay[];
  totals: { places: number; newPlaces: number; walkingM: number; estimatedCost: Money };  // estimatedCost — kishi boshiga
  prefs: TripRequest;                               // POST /trips sozlamalari (ko'rsatish va tahrirlash uchun)
};

type TripDay = { index: number; date: string; title: string; stops: Stop[]; estimatedCost: Money };

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
  rating?: number;               // 1–5, tashrifdan keyingi baho
};
```

**Rejalashtirgich (backend = ilova/sayt planner.ts):** shahar markazidan 18 km ichidagi joylar; ball = qiziqish mosligi×2 +
(reyting−4)×2 (reyting yo'q = 4.5) + yangi/kam tanilgan 0.6 + olomon (kam +0.3, ko'p −0.3) − markazdan har km 0.35;
avval yangilik kvotasi (kuniga ≥ 1, jami ≥ 1/3), qolgani mashhur joylar (+2.5); eng yaqin qo'shni zanjiri kunlarga bo'linadi;
ikkinchi joydan keyin eng yaqin ovqatlanish joyi (restoran, kafe, bozor); 09:00 dan 20:00 gacha, ish vaqti va tanaffuslar
(tushlik, juma namozi) hisobga olinadi, kelish vaqti 5 daqiqaga yaxlitlanadi; `budgetPerDay` ga sig'maguncha mashhur joylar
(eng qimmati birinchi) tushadi; yo'l: ko'cha = to'g'ri chiziq × 1.35, 1.3 km gacha piyoda (75 m/daq), aks holda shahar
taksi tarifi. OSM importidagi joylar jamoa tanlaganlaridan past (ball −2). Guruh marshrutida (`POST /groups/{id}/trip`)
ko'pchilik "ha" degan joy ball oladi (+4 × ulush), ko'pchilik "yo'q" degani chiqariladi.

**Majburiy biznes qoidalar (backend + AI kafolatlaydi, frontend ko'rsatadi):**
1. **Yangilik kvotasi:** har bir kunda kamida **1 ta** `novelty ∈ {new, hidden}` to'xtash; butun marshrutda `noveltyShare ≥ 0.25`.
2. `isPartner: true` joy yangilik kvotasini **to'ldirmaydi** va reytingni sun'iy ko'tarmaydi.
3. Har bir `Stop.reason` bo'sh bo'lmaydi — shaxsiylashtirish ko'rinib turishi kerak.
4. To'xtashlar ish vaqtiga va `transitFromPrev` ga mos tartiblangan (ustma-ust tushmaydi).

### Boshqa marshrut endpointlari
| Endpoint | Tavsif |
|---|---|
| `GET /trips` · `GET /trips/{id}` | ro'yxat / bitta marshrut |
| `DELETE /trips/{id}` | marshrutni o'chirish (kundalik rasmlari va hikoyalari ham) → `204` |
| `POST /trips/{id}/stops` | `{ dayIndex, placeId \| eventId, after?: stopId }` — bir bosishda qo'shish; server vaqtlarni qayta hisoblaydi va **butun `Trip`** qaytaradi |
| `DELETE /trips/{id}/stops/{stopId}` | olib tashlash → yangilangan `Trip` |
| `POST /trips/{id}/stops/{stopId}/swap` | shunga o'xshash boshqa joy taklifi → `{ alternatives: PlaceSummary[] }` (o'zgartirmaydi) |
| `PATCH /trips/{id}/stops/{stopId}` | `{ status?, rating?: 1–5 }` yoki `{ placeId }` (joyni almashtirish: o'rin va stop `id` saqlanadi, holat `upcoming`) → `Trip`. `rating` `visits` ga ham yoziladi (joy reytingi shundan). Band joy → `409 place_already_in_trip` |
| `POST /trips/{id}/stops/{stopId}/move` | `{ dayIndex?, position? }` — boshqa kunga yoki kun ichida `position` ga (0 dan; yo'q = oxiriga); yoki `{ step: -1 \| 1 }` — bir pog'ona yuqoriga/pastga (chetda bo'lsa `400`). Boshqa kunga ko'chgan to'xtash `upcoming` bo'ladi. Ikkala kun vaqtlari qayta hisoblanadi → `Trip` |
| `PATCH /trips/{id}` | `{ arrival?: Arrival \| null, prep?: PrepState \| null }` — "Qanday borasiz?" va "Safarga tayyorgarlik" (§10.3). `null` tozalaydi, `prep` to'liq almashtiriladi → `Trip` |
| `POST /trips/{id}/replan` | `{ reason: "late" \| "tired" \| "weather" \| "free_time", minutes?: number, location?: LatLng }` → yangilangan `Trip` + `changes: string[]` |
| `POST /visits` | `{ placeId, tripId?, rating?: 1–5, comment? }` → `{ pointsEarned, badge? }` |

### Saqlangan joylar va guruh safari
| Endpoint | Tavsif |
|---|---|
| `GET /me/saved` · `PUT /me/saved/{placeId}` · `DELETE /me/saved/{placeId}` | "yurakcha": `PlaceSummary[]`; `PUT` takrorlansa xatosiz, yo'q joy `404` |
| `POST /groups` `{ name, cityId? }` | guruh va `inviteCode`; `POST /groups/join` `{ inviteCode }` |
| `GET /groups` · `GET /groups/{id}` | a'zolar, istaklar, ovozlar; a'zo bo'lmasa `404` |
| `PATCH /groups/{id}/me` | `{ interests, budget, pace }` — a'zoning istaklari |
| `POST /groups/{id}/votes` | `{ placeId, vote: 1 \| -1 \| 0 }` |
| `POST /groups/{id}/trip` | `{ startDate, days, cityId?, startLocation? }` → `201 Trip` (barcha a'zolar ko'radi va tahrirlaydi). Qiziqishlar ko'p uchraganlari, byudjet eng kichigi, temp eng sokini |
| `DELETE /groups/{id}/members/me` | chiqish (egasi chiqsa guruh o'chadi, marshrut yaratuvchida qoladi) |

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
  updatedAt: string;             // tadqiqot sanasi yoki oxirgi tasdiqlangan kuzatuv
  sampleSize: number;            // tasdiqlangan foydalanuvchi kuzatuvlari (tadqiqot manbasi kirmaydi)
  trust: Trust;                  // tadqiqot oralig'i sample; oraliq kuzatuvlardan hisoblansa community
  source?: Source;               // ochiq manba (sayt, yil)
  placeIds?: string[];           // kirish chiptasi qaysi joyga tegishli
};
// fairRange: kuzatuv 5 tadan kam bo'lsa tadqiqot oralig'i; ko'p bo'lsa kuzatuvlar + tadqiqot chegaralarining 20–80 protsentili.
```

| Endpoint | Tavsif |
|---|---|
| `GET /prices?city=&category=&q=` | `PriceItem[]` |
| `POST /prices/reports` | `{ priceItemId?, cityId?, name?, category?, paid: Money, buyer?: "local" \| "tourist", location?: LatLng }` — kuzatuv (token ixtiyoriy, daqiqasiga 10 ta). `priceItemId` bo'lsa `name`/`category` undan. → `201 { id, status: "approved" \| "pending", priceItem?: PriceItem }`. Tadqiqot oralig'ining yarmidan 2 baravarigacha bo'lgan narx darhol tasdiqlanadi va oraliqqa kiradi; qolgani moderatsiyaga. Joylashuv ~100 m gacha yaxlitlanadi. Noma'lum band → `404 price_item_not_found` |
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

---

## 10. Daromad funksiyalari (to'liq kelishuv: frontend repo `docs/MONETIZATION.md`)

Qat'iy qoidalar: hamkor doim `partner` + `sample` bilan qaytadi (UI "Hamkor" / "Namuna hamkor" deb yozadi), taklif ixtiyoriy va
faqat tanlovga mos kelganda chiqadi (har slotga 0 yoki 1 ta), statistika anonim, `K = 50` dan kichik kesim ko'rsatilmaydi,
foydalanuvchi statistikadan chiqa oladi. Demo hamkorlar va takliflar `sample: true` (seed: `seed/monetization.json`).

### 10.1 Takliflar va bosishlar

```ts
type OfferSlot = "transport.cheapest" | "transport.fastest" | "transport.comfort" | "transport.bus"
  | "stay" | "food" | "event" | "prep.esim" | "prep.insurance" | "prep.sim";
type Offer = { id; partner: { id; name; logo? }; slot; title; body?; cta; url /* {PUBLIC_BASE_URL}/r/{clickId} */; priceFrom?: Money; sample: boolean };
```

| Endpoint | Tavsif |
|---|---|
| `GET /offers?slot=&city=&from=&to=&date=&days=&resident=&trip=` | token ixtiyoriy → `Offer[]` (0 yoki 1). `transport.*`: `to` bo'lmasa `city`; `from`/`to` bo'yicha tanlangan variantning transport turi aniqlanadi (cheapest = poyezd, fastest = samolyot yoki Afrosiyob, qaysi tezroq, comfort = mashina, bus = avtobus) va faqat shu turdagi hamkor chiqadi; yo'nalish noma'lum bo'lsa `[]`. `date` standart bugun, `days` 1–60 (standart 1), `resident` standart `false`. `title`/`cta` dagi `{d}` = `days`; kunlik narx (sug'urta) `days` ga ko'paytiriladi. Bir nechta mos kelsa eng yuqori `priority` dan biri kontekst (slot, shahar, yo'nalish, sana) bo'yicha barqaror aylantiriladi. `trip` — bosishni safarga bog'lash uchun |
| `POST /offers/{id}/impressions` | `{ count?: 1–50 }` (tana bo'sh yoki `{}` = 1) → `204`. Anonim kunlik yig'ma, daqiqasiga 60 ta. Yo'q taklif `404 offer_not_found` |
| `GET /r/{clickId}` (ildizda, `/api/v1/r/...` ham) | `clickId` imzolangan (HMAC): taklif, foydalanuvchi, safar, vaqt; 30 kun amal qiladi. Bosish yoziladi va hamkor havolasiga `302` (`{subId}` o'rniga yoki `sub_id=` bilan, `utm_source=safar`). Namuna yoki havolasiz taklif → `{WEB_BASE_URL}/{lang}/partner?sample=1&offer={id}`; buzilgan/eskirgan → `{WEB_BASE_URL}/{lang}` |
| `POST /partners/conversions` (faqat hamkor kaliti) | `{ clickId /* sub_id */, amount, currency?: "UZS", externalId }` → `201`; shu `externalId` qayta kelsa `200 { …, duplicate: true }` (ikki marta sanalmaydi). Boshqa hamkorning bosishi `404 click_not_found` |
| `GET /partner/me/stats?from=&to=` (kalit yoki `partner` roli) | standart oxirgi 30 kun, eng ko'pi 366 → `{ partner: { id, name, sample }, from, to, impressions, clicks, conversions, amount: Money /* UZS */, amounts: Money[], codesIssued, codesRedeemed, months: [{ month: "YYYY-MM", …shu maydonlar }], offers: [{ offerId, … }] }` |

**Hamkor autentifikatsiyasi:** `X-Partner-Key: spk_...` yoki `Authorization: Bearer spk_...` (kalit bazada faqat sha256 xeshi),
yoki `partner` rolidagi foydalanuvchi tokeni (hamkor kabineti). Hamkor yoki kalit o'chirilsa `401`.

### 10.2 Shaharlararo yo'nalishlar

`GET /routes/intercity?from=&to=` → `IntercityRoute[]` (shu tartibda; bir xil shahar yoki noma'lum juft `[]`):

```ts
type IntercityRoute = { mode: "train" | "afrosiyob" | "plane" | "car" | "bus"; durationMin: number;
  priceFrom?: Money /* kishi boshiga; car — mashina uchun; manba narx bermasa yo'q */; source: string; sourceUrl?: string;
  updatedAt: "YYYY-MM-DD"; estimate?: true /* Safar hisobi, e'lon qilingan raqam emas */; via?: "tashkent" };
```

Raqamlar sayt `web/src/lib/monetization/intercity.ts` bilan aynan bir xil (manba va sana bilan); mashina: yo'l masofasi,
~75 km/soat, ~1 200 so'm/km; to'g'ridan-to'g'ri yo'nalish bo'lmasa Toshkent orqali (vaqtlar + 60 daqiqa, narx ikkala bo'lak ma'lum bo'lsa).

### 10.3 Safar: `arrival` va `prep`

`Trip` ga qo'shiladi: `arrival: { from, mode, choice?: "cheapest" | "fastest" | "comfort" | "bus", priceFrom?: number } | null`,
`prep: { [route | transport | esim | sim | insurance | currency]: "done" | "skipped" }` (bo'sh = `{}`).
Yo'l narxi (bir tomonga, kishi boshiga; mashina `groupSize` yoki `companion` bo'yicha odamlar soniga bo'linib 1 000 so'mgacha
yaxlitlanadi) `totals.estimatedCost` ga qo'shiladi va alohida `totals.arrivalCost` da. Narx serverdagi yo'nalishdan olinadi;
`arrival.priceFrom` — mijozning oflayn nusxasi, saqlanadi va qaytadi. `from` safar shahri bo'lsa yoki bu juftda shunday transport bo'lmasa `400`.

### 10.4 Lens Story va chegirma kodlari

| Endpoint | Tavsif |
|---|---|
| `POST /stories` | `{ placeId, location?: LatLng }` → `201 { shareId, shareUrl }`; `shareUrl = {WEB_BASE_URL}/s/p/{placeId}?ref={shareId}`. Joylashuv ~10 m gacha yaxlitlanadi |
| `POST /stories/{shareId}/shared` | tana ixtiyoriy `{ location? }` → `{ code?: DiscountCode, reason?: "not_on_site" \| "no_partner" \| "recent" }`. Joyda = joylashuv joydan 300 m ichida yoki bugungi marshrutda shu joy `current`/`visited`. Joydan 1.5 km ichida story hamkori bo'lishi kerak. Bir hamkor bir foydalanuvchiga 7 kunda 1 ta kod (`recent` — o'sha kod qaytadi), kod 7 kun amal qiladi. Boshqaning story si `404 story_not_found` |
| `GET /me/codes` | amaldagi kodlar (ishlatilmagan, muddati o'tmagan); `?include=all` — oxirgi 30 kundagi hammasi (`usedAt` bilan) |
| `POST /partner/codes/redeem` (kalit yoki `partner` roli) | `{ value }` (yozilgan kod, QR matni, kichik harf yoki bo'sh joy bilan ham) → `{ status: "ok" \| "used" \| "expired" \| "not_found", discountPct?, partner?, sample? }`. Hamkor faqat o'z kodini ko'radi, boshqasiniki `not_found` |
| `GET /api/v1/s/p/{placeId}?ref=` | story havolasi hisobi: `ref` sanaladi va `{WEB_BASE_URL}/{lang}/places/{placeId}?ref=` ga `302` (sayt `/s/p/...` shu yerga so'raydi). Ildizdagi `/s/p/` yo'q: `/s/{slug}/card.png` bilan to'qnashadi |

```ts
type DiscountCode = { id; value: "SF-XXXX-XXXX"; partner: { id; name }; discountPct; expiresAt; qrPayload /* {WEB_BASE_URL}/c/{value} */;
  placeId?; issuedAt; usedAt?; sample };
```

### 10.5 Safar Insights (`role: insights`, hududlar bilan cheklangan)

| Endpoint | Tavsif |
|---|---|
| `GET /insights/summary?region=&from=&to=` | `region` — ruxsat etilgan shahar (bittasi bo'lsa ixtiyoriy; boshqasi `403 insights_region_forbidden`); standart oxirgi 30 kun, eng ko'pi 366 |
| `GET /insights/export?region=&from=&to=&format=csv` | saytdagi `toCsv` bilan bir xil qatorlar (`section,key,label,count,share`), yashirilgan katak bo'sh |

```ts
type Share = { key; count: number | null; share: number | null; reason?: "below_k"; label?: string /* joy nomi */ };
type InsightsSummary = { region; from; to; k: 50; sample: false; trips; tripsPrev; change /* 0.12 = +12 % */;
  avgBudgetPerPerson /* so'm, kishi boshiga kuniga */; avgDays; topPlaces: Share[] /* 8 tagacha */; transport: Share[];
  companion: Share[]; interests: Share[] /* 6 tagacha */; residency: Share[];
  gapNoStay: { plans; share; cheapestStay; reason? }; withheld: { [field]: "below_k" } };
```

Manba: `status = ready` safarlar, `start_date` oralig'ida; `analyticsOptOut = true` foydalanuvchilarniki kirmaydi (anonim
qurilmalar ham sanaladi). Faqat shahar, sana, kunlar, odamlar, `budgetPerDay`, qiziqishlar, `companion`, `isResident`,
`arrival.mode` va rejadagi joylar o'qiladi. Har kesim: jami yoki kesim `< 50` bo'lsa `null` + `reason: "below_k"`.
"Talab bor, taklif yo'q": 2+ kunlik byudjetli rejalar, eng arzon turar joy (2 kishilik, odam soniga 2 tagacha bo'linadi)
kunlik byudjetning 40% iga sig'maydi; bunday rejalar `0 < n < 50` bo'lsa `plans` ham `share` ham `null`.

### 10.6 Admin (faqat `role: admin`, moderator `403`)

| Endpoint | Tavsif |
|---|---|
| `GET /admin/partners` · `POST /admin/partners` · `PATCH /admin/partners/{id}` | `{ id?, name, cityId?, location?, discountPct? /* story chegirmasi, location bilan */, website?, active, sample }` |
| `GET` · `POST /admin/partners/{id}/keys` · `DELETE /admin/partners/{id}/keys/{keyId}` | API kaliti: `POST` → `201 { id, key /* faqat shu javobda */, prefix, createdAt }` |
| `GET /admin/offers?partner=` · `POST /admin/offers` · `PATCH /admin/offers/{id}` | `{ id?, partnerId, slot, cityId?, routeFrom?, routeTo?, mode?, validFrom?, validTo?, daysMin?, daysMax?, resident?, title, body?, cta /* {uz,ru,en} */, priceFrom?, priceUnit: "trip" \| "day", linkTemplate? /* {subId} {offerId} */, priority, active, sample }`. Jonli (sample emas) taklifga `linkTemplate` majburiy. `PATCH` da `null` maydonni tozalaydi |
| `PUT /admin/users/{id \| email}/role` | `{ role: "tourist" \| "partner" \| "insights", partnerId?, regions?: string[] }` |
