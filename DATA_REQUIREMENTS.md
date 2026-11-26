# Safar: bazani shakllantirish uchun kerakli ma'lumotlar

Bu hujjat ilova va saytning har bir funksiyasi ishlashi uchun qanday ma'lumot kerakligini sohalar bo'yicha sanab beradi. U kimga mo'ljallangan:
- **backend dasturchi:** jadvallar va maydonlar;
- **ma'lumot yig'uvchilar:** nimani, qayerdan, qanday aniqlikda yig'ish;
- **AI/ML dasturchi:** RAG korpusi va vision uchun rasmlar.

Maydon nomlari va turlari [API_CONTRACT.md](API_CONTRACT.md) bilan bir xil; kontrakt o'zgarsa, avval kontrakt, keyin shu hujjat yangilanadi. Manbalar va huquqiy chegaralar backend repodagi `DATA_SOURCES.md` da batafsil yozilgan; bu yerda faqat "nima kerak" qismi.

Belgilar: **M** majburiy (busiz funksiya ishlamaydi), **T** tavsiya (sifatni oshiradi), **K** keyinroq.

---

## 0. Hamma ma'lumot uchun umumiy qoidalar

| Qoida | Nega |
|---|---|
| Har bir fakt, narx va qoida yonida **manba** (`source`: nom + havola) va **tekshirilgan sana** (`verifiedAt`) saqlanadi | Ilova tarix yonida manbani, narx yonida kuzatuvlar sonini ko'rsatadi. Manbasiz ma'lumot UI ga chiqmaydi |
| Har bir yozuvda **ishonch darajasi**: `official` (obyekt ma'muriyati, gov.uz), `public` (ochiq manba, maqola), `community` (foydalanuvchi kuzatuvi), `sample` (namuna, demo) | Demo narx yoki ish vaqtini haqiqat sifatida ko'rsatmaslik uchun |
| Matnlar **uch tilda**: `uz`, `ru`, `en`. O'zbekcha lotin yozuvida | Interfeys uch tilda. Tarjima bo'lmasa, en ko'rsatiladi (vaqtinchalik) |
| Narxlar **so'mda** (`UZS`, butun son) | O'zbekiston qonunchiligi: narx so'mda |
| Koordinata **WGS84**, kamida 5 xona aniqlikda (taxminan 1 m) | Geofence va "Yetib keldim" 50 m radiusda ishlaydi |
| ID lar barqaror, lotin harflarida, kebab-case: `registan`, `chor-minor` | Ilova, sayt va backend seed bir xil ID ishlatadi |
| O'ylab topilgan ma'lumot yo'q. Topilmasa maydon bo'sh qoladi va `note` da sababi yoziladi | Gid "tekshirilgan ma'lumotim yo'q" deyishi kerak, to'qimasligi kerak |

---

## 1. Shaharlar

| Maydon | M/T | Izoh |
|---|---|---|
| `id`, `name` (uz/ru/en) | M | `samarkand`, `bukhara`, `tashkent`, `namangan` |
| `center` (lat, lng) | M | Marshrut shu nuqtadan 18 km radiusda tuziladi |
| `cover` rasm | M | Gorizontal, kamida 1600 px |
| `tagline` (1 qator, 3 tilda) | T | "Registon, Shohi Zinda va yangi ustaxonalar" |
| `timezone` | M | `Asia/Tashkent` (barcha shaharlar) |
| Aeroport, vokzal koordinatalari | T | Kelish nuqtasidan birinchi kunni boshlash uchun |
| Shahar bo'yicha favqulodda raqamlar, turistik politsiya | K | SOS bo'limi uchun |

Keyingi shaharlar (navbat bo'yicha): Xiva, Shahrisabz, Qo'qon, Farg'ona, Termiz, Nukus.

---

## 2. Joylar (asosiy jadval)

Joy: obida, muzey, masjid, bozor, ustaxona, galereya, restoran, kafe, bog', manzaragoh, eko-qishloq, teatr.

### 2.1. Asosiy maydonlar

| Maydon | M/T | Qanday yig'iladi |
|---|---|---|
| `id`, `cityId` | M | |
| `name` (uz/ru/en) | M | Rasmiy nom; mahalliy aholi ishlatadigan nom `aliases` ga |
| `category` | M | Kontraktdagi 12 tur. Mos kelmasa, eng yaqini va `note` |
| `novelty`: `iconic` / `hidden` / `new` | M | **Loyiha o'zagi.** `new`: so'nggi 24 oyda ochilgan yoki qayta ochilgan; `hidden`: turistik qo'llanmalarning birinchi sahifasida yo'q; qolgani `iconic` |
| `openedAt` | M (`new` uchun) | Ochilish sanasi va uning manbasi (gov.uz, rasmiy sahifa, yangilik) |
| `location` (lat, lng) | M | OSM, Yandex yoki Google xaritada **kirish eshigi** nuqtasi, binoning o'rtasi emas |
| `address` (3 tilda) | M | Ko'cha, uy raqami, mo'ljal |
| `tagline` (1 qator) | M | "XV asr rasadxonasi" |
| `description` (2 dan 4 gapgacha) | M | Turist nima ko'radi, nima qiladi |
| `interests` | M | `history`, `food`, `nature`, `craft`, `art`, `architecture`, `shopping`, `nightlife` (bir nechta) |
| `visitDurationMin` | M | O'rtacha tashrif davomiyligi. Planner shunga tayanadi |
| `rating`, `reviewsCount` | T | Google yoki Yandex xaritadan, manbasi bilan |
| `priceLevel` 0 dan 3 gacha | M | 0 = bepul |
| `crowd`: `low` / `medium` / `high` + `bestTime` | T | "Ertalab 9 gacha" |
| `accessibility`: nogironlar aravachasi, oilaga mos, oson yurish | T | Joyida tekshiriladi |
| `payment`: karta / faqat naqd | M | Ilova "naqd pul oling" ogohlantirishini shundan chiqaradi |
| `geofenceRadiusM` | M | Odatda 50–150 m; katta majmualarda (Registon, bog'lar) ko'proq |
| `isPartner` | M | Pullik ko'rsatuv. UI da doim "Hamkor" belgisi; yangilik kvotasini to'ldirmaydi |
| `phone`, `website`, `instagram`, `telegram` | T | Rasmiy kanallar |

### 2.2. Ish vaqti (hozir eng zaif joy)

| Maydon | M/T | Izoh |
|---|---|---|
| `openingHours`: hafta kunlari bo'yicha `{ day, open, close }` | M | Hozir faqat bitta oraliq bor ("09:00–18:00"), **dam olish kunlari yo'q**. Shu sababli "Hozir ochiq" yopiq kunda xato ko'rsatishi mumkin |
| Tushlik tanaffusi | T | Masalan "13:00–14:00" |
| Namoz vaqtida yopilishi | M (masjid, ziyoratgoh) | Juma kuni peshin vaqti alohida |
| Mavsumiy o'zgarish (yoz / qish) | T | Ko'p muzeylarda farq qiladi |
| Bayram kunlari | K | Navro'z, Mustaqillik kuni va h.k. |

Qayerdan: obyektning rasmiy sahifasi, telefon orqali so'rash, joyida rasm (eshikdagi jadval). Ochiq manbalar ko'pincha eskirgan.

### 2.3. Kirish narxi

| Maydon | M/T | Izoh |
|---|---|---|
| `entryFee.local` | M | O'zbekiston fuqarolari uchun |
| `entryFee.foreigner` | M | Chet elliklar uchun (ko'pincha 3–10 baravar farq qiladi) |
| Bolalar, talabalar, nafaqaxo'rlar uchun | T | Talabalar segmenti uchun muhim |
| Suratga olish uchun to'lov, gid narxi, kechki shou | T | `feeNote` da yoki alohida qator |
| `verifiedAt`, manba | M | Kassa rasmidan yoki rasmiy narx ro'yxatidan |

Bepul joyda `0` yoziladi, noma'lum joyda maydon bo'sh qoladi. "Bepul" bilan "noma'lum" aralashmasligi shart.

### 2.4. Ichki qoidalar ("Qoida" moduli)

Turist joyga kirganda geofence orqali chiqadigan qoidalar.

| Maydon | M/T | Izoh |
|---|---|---|
| `code` | M | `dress_code`, `shoes_off`, `no_photo`, `photo_fee`, `no_flash`, `quiet`, `no_drone`, `no_food`, `no_smoking`, `no_pets`, `ticket_required`, `prayer_time_limit` |
| `severity`: `info` / `warn` / `forbidden` | M | |
| `text` (1 gap, 3 tilda) | M | "Yelka va tizza yopiq bo'lsin" |
| `verifiedBy`, `verifiedAt` | M | "Obyekt ma'muriyati" bo'lsa, UI da "Ma'muriyat tasdiqlagan" belgisi chiqadi |

Har bir joyda kamida 2 ta qoida bo'lishi kerak. Masjid va ziyoratgohlarda kiyim, poyabzal, namoz vaqti majburiy.

### 2.5. Tarix va raqamli gid (RAG korpusi)

| Maydon | M/T | Izoh |
|---|---|---|
| `history.summary` (3–5 gap, 3 tilda) | M | Neytral, ensiklopedik uslubda |
| `history.facts` (3–6 ta qisqa fakt) | M | Har biri alohida tekshiriladigan gap |
| `history.sources` | M | Kamida 1 ta, yaxshisi 2 ta: Vikipediya, UNESCO, muzey sahifasi, ilmiy maqola |
| Uzun matn: me'morchilik, afsonalar, shaxslar | T | Gidning savol-javobi uchun. Afsona aniq "afsonaga ko'ra" deb belgilanadi |
| Audio gid matni va yozuvi (2–4 daqiqa) | K | `audioGuide.url`, `durationSec`, `lang` |

Qoida: manbada yo'q fakt korpusga kirmaydi. Gid javob topmasa, "tekshirilgan ma'lumotim yo'q" deydi.

### 2.6. Suratlar

| Maydon | M/T | Izoh |
|---|---|---|
| `cover` | M | Joyning umumiy ko'rinishi, gorizontal, kamida 1600 px, odamlar kam |
| `images` (3–8 ta) | T | Ichki ko'rinish, detallar, kirish eshigi |
| `credit` (muallif), `license`, `sourceUrl` | M | Hozirgi suratlarning bir qismi litsenziyasi noma'lum (faqat demo uchun). Ommaga chiqarishdan oldin almashtiriladi |
| `blurhash` | T | Yuklanish paytidagi xira fon |

Ma'qul manbalar: Wikimedia Commons (CC), obyektning o'z suratlari (yozma ruxsat bilan), o'zimiz olgan suratlar.

### 2.7. Vision uchun rasm to'plami (AI/ML)

"Rasm orqali joyni tanish" uchun har bir joyga alohida to'plam kerak (`cover` dan tashqari).

| Nima | Miqdor |
|---|---|
| Har xil burchakdan, har xil vaqtda (kunduz, kechqurun, qish, yoz) | Har bir `iconic` joy uchun 30–50 ta, boshqalar uchun 10–20 ta |
| Telefon kamerasida, turist ko'zi balandligida | To'plamning kamida yarmi |
| Bir-biriga o'xshash joylar alohida belgilanadi | Masalan, Registondagi uchta madrasa, Buxorodagi gumbazlar |
| Narx uchun mahsulot rasmlari: non, somsa, so'zana, sopol, do'ppi | Har bir mahsulot turi uchun 20–30 ta |

---

## 3. Adolatli narxlar ("Adolatli narx" moduli)

Turistga "bu narx normalmi" degan savolga javob. Har bir narx **kuzatuvlar** asosida hisoblanadi.

### 3.1. Narx bandi (`PriceItem`)

| Maydon | M/T | Izoh |
|---|---|---|
| `category` | M | `taxi`, `food`, `souvenir`, `market`, `ticket`, `transport` |
| `name` (3 tilda) | M | "Somsa (tandir, go'shtli)", "Taksi: Registon dan vokzalgacha" |
| `unit` | M | dona, kg, kishi, yo'l, soat |
| `fairRange.min`, `fairRange.max` | M | Mahalliy aholi to'laydigan oraliq |
| `touristTypical` | T | Turistga odatda aytiladigan narx (shkalada qizil belgi) |
| `sampleSize` | M | Nechta kuzatuv. **Hozir barcha 123 ta narxda 0**, ya'ni ular faqat ochiq manbalardan olingan |
| `updatedAt` | M | 90 kundan eski narx "eskirgan" deb belgilanadi |
| `placeIds` yoki `location` | T | Narx qaysi joy yoki hududga tegishli (joy sahifasida "shu yerda kerak bo'ladigan narxlar") |

### 3.2. Kuzatuv (`PriceObservation`)

Bitta xarid yozuvi. Narx oralig'i shulardan hisoblanadi.

| Maydon | Izoh |
|---|---|
| `priceItemId`, `paid` (so'm), `date` | |
| `location` (taxminiy), `vendorType` (bozor rastasi, do'kon, ko'cha) | |
| `buyerType`: mahalliy / turist | Turist narxini alohida hisoblash uchun |
| `collector`: jamoa / foydalanuvchi / hamkor | Ishonch darajasi uchun |
| `receiptPhoto` | T, moderatsiya uchun |

**Minimal maqsad:** har bir shaharda kamida 40 ta narx bandi, har birida kamida 5 ta kuzatuv. Bozorlarda (Siyob, Chorsu, Buxoro markaziy bozori) jamoa o'zi xarid qilib yozadi.

### 3.3. Narxlar ro'yxati (shahar bo'yicha)

| Guruh | Nimalar |
|---|---|
| Taksi | Yandex Go tariflari (yo'lga chiqish, km, daqiqa), mashhur yo'nalishlar: aeroport, vokzal, asosiy obidalar orasida |
| Shahar transporti | Metro (Toshkent), avtobus, Humo yoki Atto kartasi |
| Shaharlararo | "Afrosiyob", "Sharq" poyezdlari, avtobus, shaharlararo taksi (o'rindiq narxi) |
| Ovqat | Non, somsa, osh (porsiya), lag'mon, shashlik, choy, suv |
| Bozor | Meva, quruq meva, ziravor (kg) |
| Suvenir | So'zana, sopol, do'ppi, pichoq, ipak, miniatura (o'lchami bilan) |
| Chiptalar | Muzey va obidalar (2.3 bandiga bog'langan) |
| Xizmatlar | Gid (soatiga), SIM karta, valyuta ayirboshlash kursi |

### 3.4. Savdolashish iboralari va shikoyatlar

| Nima | M/T |
|---|---|
| Iboralar (uz/ru/en, talaffuz bilan): "Qancha?", "Qimmat", "Arzonroq bo'ladimi?" | T |
| Firibgarlik turlari (shahar bo'yicha), qayerga murojaat qilish | T |
| "Ishonchli sotuvchi" ro'yxati (`vendors`): nom, joy, qachondan | K |

---

## 4. Tadbirlar

| Maydon | M/T | Izoh |
|---|---|---|
| `title`, `kind` | M | `festival`, `concert`, `fair`, `exhibition`, `sport` (+ teatr, ko'rgazma) |
| `startsAt`, `endsAt` (sana + vaqt) | M | Hozirgi ba'zi yozuvlarda vaqt yo'q (00:00) |
| `venue`, `location` | M | |
| `price` yoki `isFree` | M | Narxi noma'lum bo'lsa "bepul" deb yozilmaydi |
| `ticketUrl`, manba sahifasi | M | |
| `cover` rasm | T | |
| Takrorlanadigan tadbirlar (har kuni kechki shou) | T | Jadval sifatida |

Manbalar: iTicket.uz, teatrlar sahifalari, hokimiyatlar Telegram kanallari, Uzbektourism. **Har hafta yangilanadi.** Hozir 4 shaharda jami 20 ta tadbir bor, bu kam: har bir shaharda kamida 10 ta oldinda turgan tadbir kerak.

---

## 5. Turar joy

| Maydon | M/T | Izoh |
|---|---|---|
| `name`, `kind` | M | `hotel`, `guesthouse`, `boutique`, `hostel` |
| `location`, `address` | M | |
| `pricePerNight` (2 kishi, oddiy xona) va mavsum | M | Booking/Ostrovok dagi narx, sana bilan |
| `rating` (5 ballik), `reviewsCount`, manba | M | Booking 10 ballik: 2 ga bo'linadi |
| `cover` + 3–5 rasm | M | Hozir 4 ta mehmonxonada rasm yo'q |
| `amenities` | T | Nonushta, Wi-Fi, konditsioner, hovli, transfer |
| Bog'lanish yoki bron havolasi (affiliate) | M | API emas, havola |
| `fairPrice` belgisi, `verified` | T | Jamoa tekshirgan mehmonxonalar |

Har bir shaharda kamida 15 ta turli narxdagi turar joy kerak (hostel, mehmon uyi, o'rta, qimmat). Hozir 28 ta.

---

## 6. Transport va yo'l vaqtlari

| Nima | M/T | Izoh |
|---|---|---|
| Piyoda va taksi vaqti joylar orasida | T | Hozir to'g'ri chiziq x 1.35 bilan taxminiy hisoblanadi. Haqiqiy ko'cha masofasi uchun OSRM yoki Yandex Routing |
| Metro bekatlari (Toshkent): koordinata, liniya | T | Joyga eng yaqin bekat |
| Poyezd jadvali (shaharlar orasida) | K | Jadval yopiq; faqat taxminiy vaqt va narx |
| Yo'l ko'rsatish | M | O'zimiz qilmaymiz: Yandex va Google xaritaga havola (deep link) |

---

## 7. Foydalanuvchi ma'lumotlari (backend yaratadi)

Bular yig'ilmaydi, ilova ishlaganda paydo bo'ladi. Ro'yxat jadvallarni rejalash uchun.

| Jadval | Nima saqlanadi | Shaxsiy ma'lumot |
|---|---|---|
| `users` | email yoki Google ID, til, rezident yoki chet ellik, qiziqishlar | Ha: minimal, o'chirish huquqi bilan (`DELETE /me`) |
| `devices` | anonim qurilma ID, kirishdan oldingi ma'lumot | Yo'q |
| `trips` | shahar, sanalar, qiziqishlar, byudjet, kunlar va to'xtashlar | Ha |
| `stop_events` | "Yetib keldim", "Tugatdim", vaqti | Ha (joylashuv izi). Saqlash muddati cheklanadi |
| `ratings` | tashrifdan keyingi 1–5 baho, izoh | Ha |
| `saved_places` | yurakcha bosilgan joylar | Ha |
| `groups` | guruh safari, a'zolar, har birining byudjeti, ovozlar | Ha |
| `price_reports` | foydalanuvchi kuzatuvlari (3.2) | Yo'q (anonim) |
| `scam_reports` | shikoyatlar | Yo'q (anonim) |
| `lens_queries` | qaysi joy tanildi, aniqligi (rasmning o'zi saqlanmaydi) | Yo'q |

---

## 8. Hamkorlar va moderatsiya

| Nima | M/T | Izoh |
|---|---|---|
| Hamkor obyekt profili: nom, toifa, kontakt, rekvizitlar | M | Hamkorlar kabineti (saytda bor) |
| Hamkor taklif qilgan yangi joy yoki o'zgarish | M | Moderator tasdiqlamaguncha UI ga chiqmaydi |
| Pullik ko'rsatuv muddati | M | `isPartner` shu muddat ichida |
| Moderatsiya tarixi: kim, qachon, nima o'zgardi | M | Ishonch uchun |

---

## 9. Hozirgi holat (2026-11-26, demo baza)

| Shahar | Joylar | Ulardan yangi / kam tanilgan | Ish vaqti bor | Tarix bor | Narx bandlari | Tadbirlar | Turar joy |
|---|---|---|---|---|---|---|---|
| Samarqand | 33 | 2 / 18 | 9 | 28 | 35 | 3 | 8 |
| Buxoro | 39 | 3 / 14 | 11 | 33 | 35 | 5 | 7 |
| Toshkent | 73 | 8 / 31 | 31 | 64 | 35 | 7 | 7 |
| Namangan | 26 | 1 / 17 | 5 | 23 | 18 | 2 | 6 |
| **Jami** | **171** | **14 / 80** | **56** | **148** | **123** | **17** | **28** |

Jadvaldagi tadbirlar o'tib ketganlari olib tashlangandan keyingi soni (bazada jami 20 ta).

Barcha joylarda rasm, koordinata, qoida va 3 tildagi nom bor. Asosiy bo'shliqlar:

1. **Narxlarda kuzatuv yo'q** (barchasida `sampleSize = 0`). "Adolatli narx" va'dasi uchun eng muhim ish.
2. **Ish vaqti** 171 joydan faqat 56 tasida, dam olish kunlari hech birida yo'q.
3. **Kirish narxi** ko'p joyda noma'lum; rezident va chet ellik narxlari alohida kerak.
4. **"Yangi" joylar kam** (14 ta). Loyihaning asosiy farqi shu, har shaharda kamida 10 ta bo'lishi kerak.
5. **Suratlar litsenziyasi**: bir qismi noma'lum, ommaga chiqishdan oldin almashtiriladi.
6. **Tadbirlar** kam va tez eskiradi.

Xom tadqiqot fayllari (JSON, manbalar bilan) frontend repoda emas, jamoaning `research/` papkasida; kerak bo'lsa backend seed uchun beriladi.

---

## 10. Navbat (nimadan boshlash)

| Navbat | Ish | Natija |
|---|---|---|
| 1 | 4 shahar bo'yicha narx kuzatuvlari (bozor, taksi, ovqat): har bandga 5 tadan | "Adolatli narx" haqiqiy ishlaydi |
| 2 | Ish vaqti (hafta kunlari bilan) va kirish narxlari (rezident va chet ellik): barcha `iconic` va `new` joylar | "Hozir ochiq" va xarajat hisobi to'g'ri |
| 3 | Har shaharda 10 tadan "yangi" joy, ochilish sanasi va manbasi bilan | Yangilik kvotasi to'la bajariladi |
| 4 | Joy qoidalarini obyekt ma'muriyati bilan tasdiqlash (`verifiedBy`) | "Ma'muriyat tasdiqlagan" belgisi haqiqiy |
| 5 | Vision uchun rasm to'plami (2.7) | "Joyni tanish" rasm orqali ishlaydi |
| 6 | Tadbirlarni har hafta yangilash tartibi (manba ro'yxati, mas'ul odam) | Tadbirlar bo'limi tirik |
| 7 | Suratlarni litsenziyali suratlarga almashtirish | Ommaga chiqishga tayyor |
| 8 | Yangi shaharlar (Xiva, Shahrisabz, Qo'qon) | Qamrov kengayadi |
