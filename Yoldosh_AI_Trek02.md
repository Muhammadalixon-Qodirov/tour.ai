# YO'LDOSH AI
### Sayohatchining aqlli raqamli hamrohi

**Trek 02 — Sayohatni shaxsiylashtirish va turistik marshrutlar**

> **"Mashhur joylarni hamma biladi. Yo'ldosh sizga hali hech kim bilmaganini ko'rsatadi — va yo'lda adashtirmaydi."**

---

## Mundarija

1. Loyiha haqida qisqacha
2. Trek 02 ga muvofiqlik
3. Muammolar
4. Yechim — Yo'ldosh AI modullari
5. Qo'shimcha g'oyalar (kengaytirish uchun)
6. Foydalanuvchi yo'li
7. Texnologik arxitektura
8. Maqsadli foydalanuvchilar
9. Biznes model
10. Raqobatdosh ustunlik
11. Kutilayotgan natijalar va ko'rsatkichlar
12. Xavflar va ularni kamaytirish
13. Yo'l xaritasi
14. Xulosa

> 💡 belgisi bilan belgilangan bandlar — dastlabki g'oyaga qo'shilgan **yangi takliflar**. Ularni MVP ga kiritish yoki keyingi bosqichga qoldirish jamoa ixtiyorida.

---

## 1. Loyiha haqida qisqacha

**Yo'ldosh AI** — turistga har bir qadamda hamroh bo'ladigan, sun'iy intellektga asoslangan mobil ilova. U har bir turist uchun **shaxsiy marshrut** tuzadi va unga mashhur joylar bilan birga **yangi ochilgan va kam tanilgan joylarni** ham qo'shadi. Yo'lda esa **raqamli gid** vazifasini bajaradi:

- turist turgan joyning **ichki qoidalarini oldindan ogohlantiradi**;
- har qanday obyekt **tarixini rasm orqali** aytib beradi;
- **bozor narxlarini nazorat qilib**, turistni ortiqcha to'lovdan himoya qiladi;
- marshrutga **tadbirlar va turar joyni** qo'shadi.

Loyihaning markazida — **turist va uning shaxsiy sayohat tajribasi**.

---

## 2. Trek 02 ga muvofiqlik

| Trek 02 yo'nalishi | Yo'ldosh AI dagi javobi |
|---|---|
| Sayohatni shaxsiylashtirish va turistik marshrutlar | Qiziqish, byudjet, vaqt va sayohat turiga mos shaxsiy marshrut — **Kashf** moduli |
| Tadbir turizmi | Tashrif sanasiga mos festival, sayl, konsert va ko'rgazmalar — marshrutga bir bosishda qo'shiladi |
| Mintaqalar uchun raqamli xizmatlar | Viloyatlar bo'ylab marshrutlar, oflayn xarita, joy qoidalari va adolatli narx — turist uchun |
| Turar joy yechimlari | Marshrut yaqinidagi tekshirilgan mehmonxona va mehmon uylari, adolatli narxda |

---

## 3. Muammolar

### 🔴 Asosiy muammo — Shaxsiy marshrut yo'q: hammaga bir xil ro'yxat

Turist ChatGPT, Gemini yoki boshqa AI dan *"Samarqandda 2 kunim bor, qayerga borsam bo'ladi?"* deb so'rasa, **har safar va har bir turistga bir xil mashhur joylar** (Registon, Shohi Zinda, Bibixonim, Go'ri Amir...) tavsiya qilinadi.

**Sabablari:**
- AI modellar internetda **eng ko'p yozilgan** ma'lumotdan o'rganadi — yangi joy haqida hali hech narsa yozilmagan;
- modellarning bilimi **ma'lum sanada to'xtaydi** — yangi ochilgan joylar ularga umuman ma'lum emas;
- umumiy AI turistning qiziqishi, byudjeti va vaqtini hisobga olib **haqiqiy shaxsiy marshrut** tuzmaydi.

**Oqibatlari:**
- turist **o'z qiziqishiga mos tajribani** yo'qotadi;
- yangi muzey, galereya, eko-ovul, hunarmandchilik ustaxonasi, mahalliy kafe **marshrutga tushmaydi**;
- hamma bir xil gavjum joylarda navbatda turadi, boshqa joylar bo'sh qoladi.

### 🟠 Muammo 2 — Ichki qoidalarni bilmaslik
Turist borgan masjid, muzey, qo'riqxona, mehmonxona yoki davlat binosining **ichki nizomini bilmaydi**: rasmga olish taqiqi, kiyinish talabi, dron uchirish, ovqat olib kirish, ish vaqti. Bilmasdan buzib qo'ysa — xodimlar bilan savol-javob, jarima va yomon taassurot.

### 🟡 Muammo 3 — Gidga qaramlik
Gid **qimmat**, har doim **kerakli tilda topilmaydi** va turistni o'z jadvaliga bog'laydi. Gidsiz esa turist obyekt oldida turib, uning tarixi va ahamiyatini bilmay o'tib ketadi.

### 🟢 Muammo 4 — "Turist narxi"
Ba'zi sotuvchilar turist mahalliy narxni bilmaydi deb **narxni bir necha barobar oshirib aytadi** (taksi, bozor, suvenir, ovqat). Bu turistning ishonchini va mamlakat imijini pasaytiradi.

### 🔵 Muammo 5 — Tadbir va turar joy ma'lumotining tarqoqligi
Mintaqaviy festival, sayl va konsertlar haqida ma'lumot turli kanallarda tarqoq — turist ularni bilmay qoladi. Marshrutga mos va adolatli narxdagi turar joyni topish ham qiyin.

### 💡 Muammo 6 — Til to'sig'i va madaniy farqlar
Xorijiy turist mahalliy aholi bilan gaplasha olmaydi, menyu va yozuvlarni o'qiy olmaydi, mahalliy odob-axloq qoidalarini (salomlashish, mehmondorchilik, kiyinish) bilmaydi.

### 💡 Muammo 7 — Shahar ichida harakatlanish va to'lov
Qaysi avtobus yoki metro kerakligi, taksi narxi, qayerda karta qabul qilinishi, qayerda faqat naqd pul kerakligi noaniq.

### 💡 Muammo 8 — Favqulodda vaziyatlar
Kasal bo'lib qolsa, hujjatini yo'qotsa yoki adashsa — turist qayerga murojaat qilishni bilmaydi.

---

## 4. Yechim — Yo'ldosh AI modullari

### 🧭 4.1. Kashf — shaxsiy marshrut: mashhur + yangi (asosiy yechim)

Oddiy AI dan farqli, Yo'ldosh **o'zining doim yangilanib turadigan bazasiga** tayanadi:

- **Jonli baza:** yangi ochilgan joylar bazaga doimiy qo'shib boriladi, moderatsiyadan so'ng turist ularni marshrutida darhol ko'radi;
- **"Yangilik kvotasi":** har bir marshrutda mashhur joylar bilan birga kamida **1–2 ta yangi yoki kam tanilgan joy** bo'ladi;
- **Shaxsiylashtirish:** qiziqish (tarix, gastronomiya, tabiat, hunarmandchilik, san'at), byudjet, vaqt, sayohat turi (oila, yolg'iz, juftlik, guruh), jismoniy faollik darajasi;
- **Gavjumlikni hisobga olish:** gavjum joylarga muqobil, yaqin va qiziqarli joylarni taklif qiladi;
- **RAG texnologiyasi:** til modeli javob berishdan oldin Yo'ldosh bazasidan eng so'nggi ma'lumotni oladi, shuning uchun "eskirgan bilim" muammosi yo'q.

> **Namuna — Buxoro, 1-kun:** Ark qal'asi *(mashhur)* → Hunarmandlar ustaxonasi *(yangi)* → Labi Hovuz *(mashhur)* → Zamonaviy san'at galereyasi *(yangi)* → Oilaviy milliy taomlar oshxonasi *(yangi)*

### 📍 4.2. Qoida — joylashuvga asoslangan ogohlantirish

- Turist obyekt hududiga kirishi bilan (**geofencing**) telefoniga qisqa bildirishnoma keladi:
  > ⚠️ *"Siz Ko'kaldosh madrasasidasiz. Ichkarida chaqnoqli rasmga olish mumkin emas, juma namozi vaqtida tashrif cheklangan."* *(namuna)*
- Qoidalarni obyekt ma'muriyati kiritadi va tasdiqlaydi;
- Ikonkalar bilan tushunarli ko'rinish: 📷🚫 🚭 👗 🐕 🚁;
- Turistning o'z tilida ko'rsatiladi.

### 📸 4.3. Raqamli gid — rasmga ol va bil

- Turist bino, yodgorlik, buyum, taom yoki yozuvni **rasmga oladi**, Yo'ldosh uni taniydi (**computer vision**) va:
  - tarixini, qurilgan davrini, me'morini, qiziqarli faktlarni aytib beradi;
  - **audio-gid** rejimida ovoz chiqarib hikoya qiladi;
  - savol-javobni davom ettiradi: *"Nega minora qiyshaygan?"*;
- 10+ tilda (o'zbek, rus, ingliz, xitoy, turk, arab, koreys, nemis, fransuz, yapon);
- **Oflayn rejim:** hudud xaritasi va asosiy ma'lumotlar oldindan yuklanadi.

### 💰 4.4. Adolatli narx — bozor holatini nazorat qilish

- **Narx ma'lumotnomasi:** taksi, bozor mahsulotlari, suvenir, ovqat, kirish chiptalari uchun o'rtacha mahalliy narx diapazoni;
- **Rasmga ol — narxni bil:** mahsulot yoki menyuni rasmga olsa, adolatli narx oralig'i chiqadi:
  > 💡 *"Bu do'ppi uchun odatiy narx: 80 000 – 120 000 so'm. Sizga 400 000 so'm aytilgan bo'lsa, savdolashing."* *(namuna)*
- **Savdolashish yordamchisi:** mahalliy tilda tayyor iboralar va ovozli tarjima;
- **"Ishonchli sotuvchi" belgisi** xaritada;
- **Anonim shikoyat** — firibgarlik holatini xabar qilish;
- Narxlar ochiq manbalar, hamkorlar va foydalanuvchilar orqali yangilanadi.

### 🎉 4.5. Tadbirlar
- Mintaqaviy tadbirlar kalendari — tashrif sanasiga mos festival va sayllar avtomatik tavsiya qilinadi;
- Tadbir marshrutga bir bosishda qo'shiladi, vaqt va yo'l avtomatik qayta hisoblanadi;
- Chiptani ilova ichida xarid qilish.

### 🏨 4.6. Turar joy
- Marshrut yaqinidagi tekshirilgan mehmonxona va mehmon uylari;
- Adolatli narx belgisi va haqiqiy foydalanuvchi baholari;
- Marshrutning ertangi boshlanish nuqtasiga qarab eng qulay joyni tavsiya qilish.

---

## 5. 💡 Qo'shimcha g'oyalar (kengaytirish uchun)

Quyidagilar — dastlabki g'oyaga qo'shilgan takliflar. Ular Trek 02 doirasida qoladi, chunki hammasi **turistning shaxsiy sayohat tajribasini** yaxshilaydi.

### 💡 5.1. Jonli tarjimon va madaniy etiket
- **Ovozli tarjima:** turist gapiradi — Yo'ldosh mahalliy tilda ovoz chiqarib aytadi va javobni tarjima qiladi;
- **Menyu va yozuv tarjimasi** kamera orqali;
- **"Madaniy maslahat" kartochkalari:** salomlashish, mehmonga borish, choy tortish, bozor odobi haqida qisqa maslahatlar.

### 💡 5.2. Moslashuvchan marshrut
- Turist kechiksa, charchasa yoki ob-havo o'zgarsa, marshrut **avtomatik qayta tuziladi**;
- **Ob-havo va mavsumga mos tavsiya:** issiq kunda yopiq joylar, yomg'irda muzeylar, kechki salqinda ochiq maydonlar;
- **"Bo'sh 2 soatim bor"** rejimi — yaqin atrofdagi qisqa marshrut.

### 💡 5.3. Real vaqtdagi gavjumlik
- Foydalanuvchilar oqimi asosida joyning hozirgi gavjumligini taxmin qilish: *"Registon hozir gavjum, 1 soatdan keyin keling yoki shu vaqtda yaqin atrofdagi yangi galereyani ko'ring."*

### 💡 5.4. "Kashfiyotchi" — o'yinlashtirish
- Yangi va kam tanilgan joylarga borgani uchun **ball va nishonlar** ("Buxoro kashfiyotchisi", "Hunarmand do'sti");
- Ballarni hamkor joylarda chegirmaga almashtirish;
- Bu turistni yangi joylarga borishga qo'shimcha rag'batlantiradi — asosiy muammoni yechishga yordam beradi.

### 💡 5.5. Inklyuziv sayohat
- Filtrlar: **nogironlar aravachasi uchun qulaylik**, keksalar uchun yengil marshrut, bolali oilalar uchun joylar;
- **Ovqat filtrlari:** vegetarian, allergiyalar, bolalar menyusi.

### 💡 5.6. Transport va to'lov yordamchisi
- Shahar ichidagi transport bo'yicha yo'nalish (piyoda, avtobus, metro, taksi) va taxminiy narx;
- Xaritada **"karta qabul qilinadi / faqat naqd"** belgisi, eng yaqin bankomat va valyuta ayirboshlash joylari.

### 💡 5.7. Xavfsizlik va SOS
- Bir tugma bilan **favqulodda xizmatlar** raqamlari, eng yaqin shifoxona va dorixona;
- Turistning joylashuvini ishonchli shaxsga yuborish;
- Hujjat yo'qolganda nima qilish kerakligi bo'yicha qisqa yo'riqnoma.

### 💡 5.8. Avtomatik sayohat kundaligi
- Kun oxirida Yo'ldosh turist borgan joylar, rasmlar va qisqa tarixiy ma'lumotlardan **chiroyli sayohat hikoyasi** tayyorlaydi;
- Uni ijtimoiy tarmoqqa ulashish mumkin — bu yangi joylar haqida tabiiy reklama.

### 💡 5.9. Mahalliy hamroh (Local friend)
- Tasdiqlangan mahalliy yoshlar yoki talabalar bilan qisqa uchrashuv: bozor aylanish, milliy taom tayyorlash, mahalla hayoti bilan tanishish;
- Turist uchun haqiqiy mahalliy tajriba, mahalliy yoshlar uchun qo'shimcha daromad.

### 💡 5.10. Telegram bot
- O'zbekistonda Telegram juda keng tarqalgan — ilova yuklamay turib asosiy funksiyalarni (marshrut, narx tekshirish, tarjima) Telegram bot orqali ham taqdim etish.

---

## 6. Foydalanuvchi yo'li — turistning bir kuni Yo'ldosh bilan

| Qadam | Nima bo'ladi |
|---|---|
| 1 · Kirish | Til va qiziqishlarni tanlaydi — 30 soniya |
| 2 · Marshrut | Mashhur + yangi joylardan iborat shaxsiy reja oladi |
| 3 · Yo'lda | Har bir obyektda qoidalar haqida ogohlantirish oladi |
| 4 · Obyektda | Rasmga oladi — tarix va audio-hikoya eshitadi |
| 5 · Bozorda | Narxni tekshiradi, tayyor iboralar bilan savdolashadi |
| 6 · Kechqurun | Yaqin atrofdagi tadbir va turar joy tavsiyasini oladi |
| 7 · Baho | Joyni baholaydi — yangi joylarning reytingi oshadi |
| 💡 8 · Kundalik | Kun hikoyasini oladi va do'stlari bilan ulashadi |

---

## 7. Texnologik arxitektura

| Qatlam | Texnologiya |
|---|---|
| Mobil ilova | React Native (iOS + Android), oflayn kesh |
| Backend | Python (FastAPI), PostgreSQL + PostGIS (geo-ma'lumotlar) |
| AI yadrosi | LLM + RAG (vektor bazasi), ko'p tilli |
| Tavsiya tizimi | Shaxsiy marshrut va "yangilik kvotasi" algoritmi |
| Tasvirni tanish | Computer vision (obyekt va mahsulotni aniqlash), OCR |
| Joylashuv | GPS, geofencing |
| Ovoz | Speech-to-text va text-to-speech |
| Narx tahlili | Narx ma'lumotlari bazasi + anomaliyani aniqlash |
| 💡 Kontekst | Ob-havo API, gavjumlik bashorati |
| 💡 Kanal | Telegram bot |

---

## 8. Maqsadli foydalanuvchilar

- **Xorijiy turistlar** — til to'sig'i, qoidalarni bilmaslik, narx muammosi;
- **Ichki turistlar** — yangi joylarni kashf etish, dam olish kunlari marshrutlari;
- **Mustaqil sayohatchilar (backpackerlar)** — gidsiz, erkin va tejamkor sayohat;
- **Oilaviy va yoshlar sayohati** — byudjet va vaqtga mos marshrutlar, tadbirlar.

---

## 9. Biznes model

| Daromad manbai | Tavsif |
|---|---|
| Freemium | Asosiy funksiyalar bepul; Premium — oflayn xaritalar, cheksiz audio-gid, reklamasiz |
| Bron komissiyasi | Marshrut ichida turar joy bron qilishdan 5–10% |
| Chiptalar | Tadbir, muzey va ekskursiya chiptalarini ilova orqali sotishdan komissiya |
| Hamkor joylar | Marshrutda belgilangan "Hamkor" ko'rsatuvi |
| 💡 Mahalliy hamroh | "Local friend" uchrashuvlaridan komissiya |

> ⚖️ **Shaffoflik tamoyili:** pullik ko'rsatish doim "Hamkor" deb belgilanadi va "yangilik kvotasi"ni bosib keta olmaydi — tavsiyalar adolatli bo'lib qoladi.

---

## 10. Raqobatdosh ustunlik

| Imkoniyat | Umumiy AI chatbotlar | Xarita ilovalari | Gid ilovalari | **Yo'ldosh AI** |
|---|:-:|:-:|:-:|:-:|
| Yangi ochilgan joylar | Yo'q | Qisman | Yo'q | **Ha** |
| Shaxsiy marshrut + "yangilik kvotasi" | Qisman | Yo'q | Yo'q | **Ha** |
| Joylashuvga qarab qoidalar | Yo'q | Yo'q | Yo'q | **Ha** |
| Rasm orqali tarix | Qisman | Yo'q | Qisman | **Ha** |
| Adolatli narx nazorati | Yo'q | Yo'q | Yo'q | **Ha** |
| Mahalliy tadbirlar | Yo'q | Qisman | Yo'q | **Ha** |
| Oflayn va ko'p tilli | Yo'q | Qisman | Qisman | **Ha** |

---

## 11. Kutilayotgan natijalar va ko'rsatkichlar

**Natijalar:**
- Turist uchun shaxsiy, qiziqishiga mos sayohat tajribasi;
- Yangi va kam tanilgan joylarga turist oqimining oshishi;
- Qoida buzilishi bilan bog'liq noxush holatlarning kamayishi;
- Ortiqcha to'lovlarning kamayishi va mamlakatga ishonchning oshishi;
- Turistlarning viloyatlarga ham ko'proq chiqishi.

**O'lchanadigan ko'rsatkichlar (KPI)** — qiymatlar maqsad sifatida belgilangan, pilotda aniqlashtiriladi:

| Ko'rsatkich | Maqsad |
|---|---|
| Marshrutlardagi yangi joylarga tashrif ulushi | 20–30% |
| Turist qoniqishi (ilova bahosi) | 4.5 / 5 dan yuqori |
| Bir sayohatda ilovadan foydalangan kunlar | o'rtacha 3+ kun |
| Narx tekshiruvidan keyin savdolashgan foydalanuvchilar | kuzatib boriladi |
| Qoida ogohlantirishini o'qiganlar ulushi | kuzatib boriladi |

---

## 12. Xavflar va ularni kamaytirish

| Xavf | Yechim |
|---|---|
| Yangi joylar haqida ma'lumot noto'g'ri yoki sifatsiz | Moderatsiya, foydalanuvchi baholari, past reytingli joylarni avtomatik pasaytirish |
| AI noto'g'ri tarixiy ma'lumot aytishi ("gallyutsinatsiya") | Javoblar faqat tekshirilgan bazaga tayanadi (RAG), manba ko'rsatiladi |
| Narx ma'lumotlari eskirishi | Doimiy yangilash, foydalanuvchi xabarlari, narx diapazoni sifatida ko'rsatish |
| Sotuvchilar noroziligi | Ijobiy yondashuv: "Ishonchli sotuvchi" belgisi orqali halol sotuvchilarni rag'batlantirish |
| Shaxsiy ma'lumotlar xavfsizligi | Joylashuv faqat rozilik bilan, ma'lumotlar shifrlangan, anonim statistika |
| Internet yo'qligi | Oflayn rejim, oldindan yuklangan hududlar |

---

## 13. Yo'l xaritasi

| Bosqich | Muddat | Natija |
|---|---|---|
| **MVP** | 0–3 oy | Samarqand va Buxoro: Kashf + Qoida + Raqamli gid (3 til) |
| **Pilot** | 3–6 oy | Adolatli narx moduli, 1 000+ obyekt, 💡 Telegram bot |
| **Kengayish** | 6–12 oy | Xiva, Toshkent, Farg'ona vodiysi; tadbirlar va bron; 10 til; 💡 o'yinlashtirish va sayohat kundaligi |
| **Masshtab** | 12+ oy | Butun O'zbekiston, Markaziy Osiyo davlatlari; 💡 mahalliy hamroh xizmati |

---

## 14. Xulosa

Yo'ldosh AI — **har bir turistga o'z sayohati**. U Trek 02 ning barcha yo'nalishlarini bitta ilovada birlashtiradi:

- **Shaxsiy marshrut** — mashhur va yangi joylar muvozanatda;
- **Tadbirlar va turar joy** — marshrut ichida, bitta ilovada;
- **Raqamli gid** — qoidalar, tarix va adolatli narx har qadamda.

**Yo'ldosh AI — har bir yo'lda ishonchli hamroh.**

---

*Eslatma: hujjatdagi narxlar, joy qoidalari va marshrut namunalari tushuntirish uchun berilgan; haqiqiy ma'lumotlar pilot bosqichida yig'iladi.*
