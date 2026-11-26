# "Qanday boraman?" — Yandex orqali yo'l ko'rsatish rejasi

## Maqsad

Turist marshrutdagi joyni (yoki istalgan joy sahifasini) ochib **"Qanday boraman?"** tugmasini bosadi va usulni tanlaydi:

| Tugma | Nima bo'ladi |
|---|---|
| 🚌 Jamoat transporti | **Yandex Maps** ochiladi: qaysi avtobus/metro, qayerda tushish, avtobus hozir qayerda |
| 🚗 Mashinada | **Yandex Maps** navigator rejimida (tirbandlik bilan). Qo'shimcha: Yandex Navigator |
| 🚕 Taksi | **Yandex Go** — borish manzili oldindan to'ldirilgan |
| 🚶 Piyoda | **Yandex Maps** piyoda marshrut |
| ⋯ Boshqa | 2GIS, Google Maps |

Turist **hech narsa yozmaydi**: borish manzili — bizning bazadagi joy koordinatasi; qayerdan — turistning hozirgi joylashuvi.
Ilovamizda ham taxminiy ma'lumot qoladi: "1.2 km · piyoda 15 daq" / "taksi ≈ 12 000 so'm".

> Nega ilova ichida navigator emas: Yandex MapKit shartlari (§2.4) o'z ilovada yo'l ko'rsatuvchi navigator qurishni taqiqlaydi;
> Toshkent avtobus ma'lumotlari ochiq emas (2GIS eksklyuziv). Yandex ilovasiga yo'naltirish — qonuniy, bepul va eng sifatli
> (avtobuslar real vaqtda, tirbandlik). Batafsil: [DATA_SOURCES.md](DATA_SOURCES.md) → Navigator, Transport.

---

## Kim nima qiladi

| Ish | Kim |
|---|---|
| Har bir joy va marshrut to'xtashi uchun tayyor havolalar (`navigation`) API javobida | **Backend (men)** |
| `GET /places/{id}/directions?from=lat,lng` — masofa, vaqt, taksi narxi + havolalar | **Backend (men)** |
| `GET /transit/nearby?near=lat,lng` — eng yaqin metro bekati (2-bosqich) | **Backend (men)** |
| Joy qidirish (`GET /places?q=Registon`) | ✅ allaqachon bor |
| "Qanday boraman?" tugmasi, tanlov oynasi (bottom sheet), ikonkalar | **Frontend** |
| Joylashuvga ruxsat so'rash va olish (`expo-location`) | **Frontend** |
| Havolani ochish: ilova o'rnatilgan bo'lsa ilovada, bo'lmasa brauzerda | **Frontend** |
| iOS / Android sozlamasi (URL scheme ro'yxati) | **Frontend** |
| Telefonda sinash (Android + iPhone, Yandex o'rnatilgan / o'rnatilmagan) | **Frontend + men** |

---

## Backend: API (men qilaman)

### 1. Joy va to'xtash javobiga `navigation` qo'shiladi

`GET /places/{id}`, `GET /trips/{id}` dagi har bir `stop.place` va lentadagi joylarda:

```json
"navigation": {
  "destination": { "lat": 39.6547, "lng": 66.9758, "name": "Registon maydoni" },
  "options": [
    { "mode": "transit", "provider": "yandex_maps",
      "app": "yandexmaps://maps.yandex.ru/?rtext=~39.6547,66.9758&rtt=mt",
      "web": "https://yandex.uz/maps/?rtext=~39.6547,66.9758&rtt=mt" },
    { "mode": "car",     "provider": "yandex_maps",   "app": "...rtt=auto", "web": "...rtt=auto" },
    { "mode": "walk",    "provider": "yandex_maps",   "app": "...rtt=pd",   "web": "...rtt=pd" },
    { "mode": "taxi",    "provider": "yandex_go",
      "app": null,
      "web": "https://3.redirect.appmetrica.yandex.com/route?end-lat=39.6547&end-lon=66.9758&tariffClass=econom&ref=safar&appmetrica_tracking_id=25395763362139037&lang=uz" },
    { "mode": "car",     "provider": "yandex_navi",   "app": "yandexnavi://build_route_on_map?lat_to=39.6547&lon_to=66.9758", "web": null },
    { "mode": "transit", "provider": "2gis",          "app": "dgis://2gis.ru/routeSearch/rsType/bus/to/66.9758,39.6547", "web": "https://2gis.uz/routeSearch/rsType/bus/to/66.9758,39.6547" },
    { "mode": "transit", "provider": "google_maps",   "app": null, "web": "https://www.google.com/maps/dir/?api=1&destination=39.6547,66.9758&travelmode=transit" }
  ]
}
```

**Muhim:** "qayerdan" qismi bo'sh (`rtext=~lat,lng`, 2GIS da `from` yo'q, Yandex Go da `start` yo'q) — bu holda
**Yandex/2GIS ilovasining o'zi turistning hozirgi joylashuvini oladi**. Ya'ni havola joylashuv ruxsatisiz ham ishlaydi;
bizning ilova joylashuvni faqat o'z taxminini ("15 daqiqa", "≈12 000 so'm") ko'rsatish uchun so'raydi.

### 2. `GET /places/{id}/directions?from=lat,lng`
Javob: `navigation` + bizning taxmin:
```json
{ "distanceM": 1240,
  "estimates": [ {"mode":"walk","durationMin":15}, {"mode":"taxi","durationMin":7,"fare":{"amount":12000,"currency":"UZS"}} ],
  "navigation": { ... } }
```
`from` koordinatasi **saqlanmaydi va logga yozilmaydi** (maxfiylik qoidasi).

### 3. (2-bosqich) `GET /transit/nearby?near=lat,lng`
Eng yaqin metro bekatlari (OpenStreetMap dan, 50 ta bekat) + metro narxi (2 500 so'm karta / 3 000 naqd, 2026-10-01 dan).

---

## Frontend: nima qilish kerak

1. **Tugma va tanlov oynasi.** Joy kartasida "Qanday boraman?" → bottom sheet: 🚌 🚗 🚕 🚶 + "Boshqa ilovalar".
   Har tugmada bizning taxmin (agar joylashuv bor bo'lsa): "piyoda 15 daq", "taksi ≈ 12 000 so'm".
2. **Joylashuv.** `expo-location` → `requestForegroundPermissionsAsync()`. Rad etilsa ham tugmalar ishlaydi (yuqoridagi "Muhim" ga qarang),
   faqat taxmin ko'rsatilmaydi.
3. **Havolani ochish:**
   ```ts
   async function openNav(o: { app: string | null; web: string | null }) {
     if (o.app && (await Linking.canOpenURL(o.app))) return Linking.openURL(o.app);
     if (o.web) return Linking.openURL(o.web); // ilova yo'q — brauzer yoki do'konga
   }
   ```
4. **iOS** (`app.json` → `ios.infoPlist.LSApplicationQueriesSchemes`): `["yandexmaps", "yandexnavi", "dgis", "comgooglemaps"]` —
   busiz `canOpenURL` doim `false` qaytaradi.
5. **Android 11+** (`app.json` → `android` yoki config plugin, `<queries>`): xuddi shu scheme lar uchun `intent` yozuvlari — busiz ilova "ko'rinmaydi".
6. **Web (Vercel)** versiyada: faqat `web` havolalar (yangi oynada).

---

## Bosqichlar

| # | Ish | Kim | Muddat |
|---|---|---|---|
| 1 | `navigation` maydoni + `/directions` endpoint + testlar, deploy | Backend | 1 kun |
| 2 | Tugma, bottom sheet, `openNav`, iOS/Android sozlamasi | Frontend | 1–2 kun |
| 3 | Telefonda sinov (pastdagi ro'yxat), havola formatlarini tuzatish | Ikkalamiz | 0.5 kun |
| 4 | Metro: eng yaqin bekat, narxlar (`/transit/nearby`) | Backend + Frontend | 2–3 kun |
| 5 | Ilova ichida xarita (MapLibre + OpenFreeMap): marshrut nuqtalari, metro liniyalari | Frontend (+ backend ma'lumot) | 3–5 kun |

## Sinov ro'yxati (3-bosqich)

- [ ] Android, Yandex Maps o'rnatilgan: 🚌 avtobus marshruti ochiladi, borish manzili to'g'ri
- [ ] Android, Yandex Maps **o'rnatilmagan**: brauzerda yandex.uz/maps ochiladi
- [ ] iPhone: xuddi shu ikki holat
- [ ] 🚕 Yandex Go: manzil to'ldirilgan holda ochiladi (o'rnatilmagan bo'lsa — do'kon/sayt)
- [ ] 🚗 Yandex Navigator: koordinata bilan ochiladimi? (Ochilmasa — tugma olib tashlanadi, Yandex Maps 🚗 qoladi)
- [ ] Joylashuvga ruxsat berilmagan holatda ham tugmalar ishlaydi
- [ ] Samarqand va Buxoroda ham jamoat transporti ko'rinadimi (Yandex qamrovi)

## Cheklovlar va xavflar

- **Yandex Navigator** havolasi koordinata uchun imzolangan kalit talab qilishi mumkin (tasdiqlanmagan) → asosiy 🚗 tugma Yandex Maps.
- **Yandex Go** statistikasi (bizdan nechta buyurtma) uchun Yandex bilan hamkorlik kerak (`taxi-api@yandex-team.ru`); hozir umumiy `appmetrica_tracking_id` ishlatiladi — havola ishlaydi, statistika yo'q.
- **MyTaxi** ning ochiq havolasi yo'q — qo'shilmaydi.
- Havola formatlari Yandex tomonidan o'zgartirilishi mumkin — formatlar backendda bitta joyda, frontendni qayta chiqarmasdan tuzatiladi.
- Shahar avtobuslari faqat Yandex/2GIS ilovasida; ilovamiz ichida avtobus marshruti (3-daraja) — pullik 2GIS API yoki Yandex MapKit shartnomasi bilan, keyinroq.
