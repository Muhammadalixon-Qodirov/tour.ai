# Sayohat videobloggerlari va videodan joy olish

Qisqa va halol javob: **O'zbekiston bo'yicha katta, sof sayohat-fokusli mahalliy YouTuber yo'q** — mahalliy qamrov o'rta hajmli rus/o'zbek kanallarga (Umid Gafurov, Central Asia Travel, Kurbonali) va mikro-ijodkorlarga bo'linib ketgan; eng ishonchli "mashhur" kontent xalqaro YouTuberlardan keladi — **Vagabrothers** (2019, hukumat taklifi bilan, ~1.1M), **Drew Binsky** (~7M, Samarqand/Buxoro), **Bald and Bankrupt** (~3M, Sokh anklavi) va **David's Been Here** (2019) ([Kun.uz](https://kun.uz/en/77492296); [SponsorRadar](https://sponsorradar.com/channels/vagabrothers)). Instagram tarafida eng kuchli joy-kashf lead'i — **@sanjiksancho** (~61.5K, yashirin joylar) va **@axmedov_dilmurad** (~28.1K, Orol dengizi road-trip'lari); TikTok'da esa **@side_quest_project** ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)). Endi bitta qattiq haqiqat: **ixtiyoriy uchinchi tomon videosidan ma'lumotni avtomatik, qonuniy tarzda chiqarib bo'lmaydi.** Videoning ovozini yoki kadrlarini tahlil qilish uchun avval videoni yuklab olish kerak, bu esa YouTube/TikTok/Instagram shartlarini (ToS) buzadi — to'siq texnik emas, **huquqiy**. Yagona qonuniy yo'l: **inson videoni ko'rib, joy nomini yozib oladi → yozuvni ochiq ma'lumotdan (Wikidata/OSM) quramiz → video faqat embed/havola sifatida ko'rsatiladi.** Faqat YouTube Data API orqali olingan **matn** maydonlari (sarlavha, tavsif, teglar, boblar) avtomatik va legal — ularni NER/LLM bilan o'qib, joy nomi ishoralarini chiqarish mumkin.

## 1-BO'LIM — Ijodkorlar ro'yxati (platforma bo'yicha)

> **Eslatma barcha jadvallar uchun:** obunachi raqamlari TAXMINIY, uchinchi tomon agregatorlaridan (vidIQ, Social Blade, Yoloco, russia-promo, russia aggregatorlari) olingan — ular kechikadi va bir-biriga zid keladi. Handle'lar platformada bevosita tekshirilmagan. Qaror qabul qilishdan oldin jonli profilda qayta tasdiqlang.

### 1.1 YouTube — mahalliy (O'zbek / rus tilida)

| Kanal / handle | Taxminiy hajm | Til / fokus | Izoh |
|---|---|---|---|
| **Umid Gafurov** (@umidgafurov) | ~44.8–45.3K | O'zbek; sayohat + ijtimoiy mavzular | Asosan **Troll.uz** hazil-blogi muallifi sifatida tanilgan — sof travel emas, aralash kontent ([Yoloco](https://yoloco.io/top-youtubers-travel-uzbekistan); [Spot.uz](https://www.spot.uz/ru/2025/01/18/troll-uz/)) |
| **Central Asia Travel** (@centralasiatravel) | ~22.6K, ~8.13M ko'rish | Rus; tog'/dron/sarguzasht | Kattaroq va faolroq; Lenin cho'qqisi dron videolari, 3,600 m baza lager ([vidIQ](https://vidiq.com/youtube-stats/channel/@centralasiatravel/)) |
| **Central Asia Travel** (@thecatravel) | ~5.75K, ~2.49M ko'rish | Rus | **Boshqa** kanal, 2011-da ochilgan ([vidIQ](https://vidiq.com/youtube-stats/channel/@thecatravel/)) |
| **Kurbonali** | ~25.9K | O'zbek; sayohat | Yoloco reytingida ([Yoloco](https://yoloco.io/top-youtubers-travel-uzbekistan)) |
| **Traveling with Poly Kostina** (Polina Kostina) | ~138K (IG ~12.8K) | Rus; mashhur + yashirin yo'nalishlar | Asli Andijondan, Toshkentda yashaydi; handle imlosi nomuvofiq — tekshirish kerak ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **Durdona Safarova** (@yourgraced) | YouTube ~129K; IG ~270K; TikTok ~48.2K | O'zbek/rus; hayot + safar | Styuardessa; Toshkent joylari ham ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **«Nastoyashchiy Uzbekskiy»** | aniq emas | Rus; O'zbekiston madaniyat/tarix/oshxona/infratuzilma sharhlari | Ishonchli obunachi soni topilmadi ([ThoughtLeaders](https://app.thoughtleaders.io/youtube/nastoiashchii-uzbekskii)) |

**Bozor konteksti:** O'zbekistonning eng yirik YouTube kanallari — sayohat emas, yangilik/ko'ngilochar; masalan Kun.uz 2024-dekabrda 3M obunachidan oshdi ([Kun.uz](https://ad.kun.uz/en/news/2024/12/21/kunuz-youtube-channel-surpasses-3-million-subscribers)). Ya'ni mahalliy sof-travel segment hali kichik va bo'lingan.

### 1.2 YouTube — xalqaro (O'zbekiston kontenti TASDIQLANGAN)

| Kanal | Taxminiy hajm | O'zbekiston kontenti |
|---|---|---|
| **Vagabrothers** (Marco & Alex Ayling) | ~1.1M | 2019-da Davlat turizm qo'mitasi taklifi bilan; Toshkent, Samarqand, Buxoro, Xiva, Orol dengizi; "Why should you visit Uzbekistan now?" ([Kun.uz](https://kun.uz/en/77492296); [SponsorRadar](https://sponsorradar.com/channels/vagabrothers)) |
| **Drew Binsky** | ~6.8–7.5M | Samarqand va Buxoroda ~5 kun; "one of my favorite countries"; shaharlarga diqqat tavsiya qilgan ([LADbible](https://www.ladbible.com/news/world-news/worlds-cheapest-country-uzbekistan-travel-drew-binsky-288491-20230723); [youtubers.me](https://us.youtubers.me/drew-binsky/youtuber-stats)) |
| **Bald and Bankrupt** (Benjamin Rich) | ~3M+ | 15 sobiq Sovet respublikasini reytinglash (2026-may); **Sokh anklavi** epizodlari; arzon Ipak yo'li + plov ([Recapio](https://recapio.com/digest/avoid-this-soviet-exclave-by-bald-and-bankrupt); [Radio Prague](https://english.radio.cz/node/8691809)) |
| **David's Been Here** (David Hoffmann) | — | 2019; Buxoro, Samarqand, Shahrisabz; mahalliy taom fokusi ([davidsbeenhere.com](https://davidsbeenhere.com/2019/12/02/video-first-impressions-of-bukhara-ancient-unesco-silk-road-city-uzbekistan)) |
| **Pete R. / Bucketlistly** | — | "Uzbekistan Odyssey" — Buxoro orqali bekpeker film, DJI dron ([bucketlistly.blog](https://www.bucketlistly.blog/videos/uzbekistan-odyssey-travel-video)) |
| **Sojourner** | — | 2025-noyabr; Toshkent–Xiva–Buxoro–Samarqand poyezd marshruti ([blacktraveltube.com](https://www.blacktraveltube.com/videos/i-train-traveled-around-uzbekistan-🇺🇿-tashkent-khiva-bukhara-and-samarkand/)) |
| **Euronews — Silk Road seriyasi** | media | 2025–2026 davomiy seriya: Toshkent metrosi → Samarqand, Xiva, bozorlar, gilamlar ([Euronews](https://www.euronews.com/2026/11/11/exploring-uzbekistans-silk-road-soul-from-tashkent-metro-to-samarkand)) |

**TEKSHIRILMAGAN / EHTIMOL YO'Q (ro'yxatga kiritishdan oldin kanalni to'g'ridan-to'g'ri tekshiring):**

| Kanal | Holat |
|---|---|
| **Indigo Traveller** (Nick Fisher) | Wikipedia davlatlar ro'yxatida Qozog'iston bor, O'zbekiston YO'Q; tasdiqlanmagan ([Wikipedia](https://en.wikipedia.org/wiki/Indigo_Traveller)) |
| **Eva zu Beck** | O'zbekistonda bo'lganini ko'rsatuvchi manba topilmadi ([Wikipedia](https://en.wikipedia.org/wiki/Eva_zu_Beck)) |
| **Kara and Nate** | O'zbekiston videosi tasdiqlanmadi |

### 1.3 Instagram

| Handle | Taxminiy hajm | Fokus | Izoh |
|---|---|---|---|
| **@sanjiksancho** | IG ~61.5K; TG ~41.6K | **Yashirin joylar (hidden gems)** + layfhaklar | **Eng kuchli joy-kashf lead'i** ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **@axmedov_dilmurad** (Dilmurad Akhmedov) | IG ~28.1K | Orol dengizi, road-trip | Barsakelmes, Ustyurt, Mo'ynoq (2024-noyabr) ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **@thecolorsofsam** (Samantha Russell) | ~240K+ | Amerikalik oilaviy travel | Uzbektourism tomonidan featured; Toshkent, Samarqand (Registon, Shohi Zinda), Buxoro, Xiva; 7–10 kunlik marshrut ([gov.uz](https://gov.uz/en/uzbektourism/news/view/236637)) |
| **@darsik** (Daria Sirotina) | tasdiqlanmagan | O'zbekiston joylari, `#в_узбекистан_за_счастьем` | Obunachi soni ochiq manbada yo'q ([WE Project](https://weproject.media/articles/detail/chto-vy-ne-znali-o-tsentralnoy-azii-9-poleznykh-travel-blogov/)) |
| **@klavdinka_malinka** (Klavdia) | tasdiqlanmagan | "O'zbekistonning eng go'zal joylari" | Ko'p tilda; obunachi yo'q ([WE Project](https://weproject.media/articles/detail/chto-vy-ne-znali-o-tsentralnoy-azii-9-poleznykh-travel-blogov/)) |
| **@journeywithoutend** (Ludovico Alkorta) | tasdiqlanmagan | Butun Markaziy Osiyo, joy hikoyalari | Obunachi yo'q ([WE Project](https://weproject.media/articles/detail/chto-vy-ne-znali-o-tsentralnoy-azii-9-poleznykh-travel-blogov/)) |
| **@amirzaev_** (Amirbek Mirzaev) | IG ~13.8K | Samarqanddan hujjatli fotografiya | So'nggi safarlar Qozog'iston/Tailand ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **@alena.volchica** (Alena Aitova) | IG ~12.6K | Tog' treklari, ekspeditsiyalar | Wolf Travel asoschisi ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)) |
| **@uzbekistan.travel** / **@gotouzbekistan** | tasdiqlanmagan | Rasmiy / yarim-rasmiy joy-promo | Obunachi soni tasdiqlanmadi ([belgium.mfa.uz](https://belgium.mfa.uz/news/7224)) |

Diqqat: yirik "O'zbek influencerlar" (@leo_17s ~1.3M, @iren.uz ~243K, @yourgraced ~270K) asosan **chet el safari/lifestyle** qiladi — O'zbekiston joylari ularda ikkilamchi, shuning uchun place-discovery uchun kamroq foydali ([russia-promo.com](https://russia-promo.com/blog/leading-influencers-uzbekistan)).

### 1.4 TikTok

| Handle | Fokus | Izoh |
|---|---|---|
| **@side_quest_project** (Justin, Kanadalik) | **Eng aniq travel-fokusli TikToker** | O'zbekiston/Qozog'iston/Qirg'iziston; "insider" joy va tur-kompaniya ma'lumoti; obunachi topilmadi ([Caspian Policy Center](https://www.caspianpolicy.org/research/economy/beyond-the-silk-road-how-central-asia-is-reinventing-itself-for-the-tik-tok-generation-of-travelers)) |
| Littos (~6M), Nadir Sailov, Kamilla | **Ko'ngilochar — travel emas** | O'zbekistonning top-TikTokerlari, lekin joy-kontent emas ([Heepsy](https://www.heepsy.com/en/find-influencers/tiktok/uzbekistan)) |
| **@khojiakbar_karim** | **TEKSHIRILMAGAN** | Ochiq manbada tasdiq topilmadi |

Umumiy manzara: O'zbekiston "joy" kontenti bo'yicha TikTok maydoni Instagram'ga nisbatan kamroq hujjatlashtirilgan; mahalliy yirik TikTokerlar komediya/musiqa yo'nalishida — Samarqand/Toshkent faqat "yashash joyi" sifatida tilga olinadi.

### 1.5 Hukumat blogger-kampaniyalari (kontekst)

Turizm qo'mitasi 2019-yildan beri xorijiy blogerlarni faol taklif qiladi: ilk "World Influencers Congress" 40+ davlatdan ~100 blogerni Xiva, Buxoro, Samarqandga olib keldi ([Kun.uz](https://kun.uz/en/news/2019/09/05/world-influencers-congress-tour-to-ancient-cities-presidential-selfie-a-week-with-international-bloggers)). 2024-iyunda ~**$5M global kampaniya** va har biri **10M+ obunachili 50+ blogger** uchun info-tur e'lon qilindi ([daryo.uz](https://daryo.uz/uz/2024/06/04/uzbekistan-announces-5mn-global-tourism-promotion-campaign/); [Kun.uz](https://kun.uz/en/news/2024/06/03/world-famous-travel-bloggers-to-be-invited-to-uzbekistan)). Rasmiy hashteglar: **#UzbekistanImpressions** va **#MeningSayohatim** ([belgium.mfa.uz](https://belgium.mfa.uz/news/7224)). Mustaqil eslatma: RFE/RL hukumatning xorijiy blogerlarni qo'llashi bilan ichki ijtimoiy-media erkinligidagi zaif natijasi o'rtasidagi ziddiyatni qayd etadi ([RFE/RL](https://rferl.org/a/uzbekistan-tourism-foreign-social-media-stars-to-boost-tourism/30176880.html)).

## 2-BO'LIM — Videodan joy ma'lumotini qanday olish (asosiy texnik/huquqiy javob)

Bu bo'lim har bir usulni **qonuniylik tartibida** beradi — eng xavfsizdan eng xavfligacha. Har bir usul uchun qaror qoidasi oddiy: **(a)** faqat rasmiy API matn maydonlaridan foydalanadimi (keshlash chegarasi ichida)? → **legal.** **(b)** video faylini / ovozini / kadrlarini olishni talab qiladimi? → bu **yuklab olish** → ToS buzilishi (+ mualliflik huquqi xavfi) → mahsulotni bunga qurmang.

### 2.1 Legal va avtomatlashtiriladigan: faqat YouTube Data API MATNI

YouTube Data API ixtiyoriy videoning **sarlavha, tavsif, teglar, boblar (chapters) va pinned matnini** yuklab olmasdan beradi — bu yagona ishonchli legal, to'liq avtomatik matn kanali. Joy nomlari ko'pincha aynan shu maydonlarda bo'ladi (masalan sarlavhada "Registon, Samarqand"), shuning uchun bu eng qimmatli legal signal. Ushbu matnga NER yoki LLM qo'llab, "we spent the morning at the Registan in Samarkand" → `{place: "Registan", city: "Samarqand"}` ga aylantirish mumkin — bu sof matn-ishlash qadami, qo'shimcha ToS xavfisiz.

**Qattiq kesh chegarasi:** saqlangan API ma'lumoti **30 kalendar kundan** ortiq saqlanmasligi kerak (§III.E.4), joriy YouTube holatini aks ettirishi va foydalanuvchi so'rasa 7 kun ichida o'chirilishi shart ([YouTube API Developer Policies](https://developers.google.com/youtube/terms/developer-policies)).

**Muhim cheklov:** `captions.download` (transkript matni) faqat **video egasiga** ruxsat etilgan — u videoni tahrir qila olish huquqini talab qiladi va yetarli ruxsat bo'lmasa `403 forbidden` qaytaradi. `captions.list` esa faqat metadata beradi, transkript matnini emas. Ya'ni **boshqa ijodkorning transkriptini rasmiy API orqali OLA OLMAYSIZ** ([YouTube captions.download](https://developers.google.com/youtube/data/reference/rest/v3/captions)).

### 2.2 Texnik oson, lekin uchinchi tomon videosi uchun HUQUQIY BLOKLANGAN

Bu usullar texnik jihatdan juda samarali va ko'pincha aniq joy nomini berardi — lekin **hammasi video faylini talab qiladi, demak yuklab olishni, demak ToS buzilishini.** To'siq huquqiy, texnik emas.

- **Ovozni matnga (speech-to-text):** Whisper/Groq arzon va aniq — Whisper Large v3 ~**10.3% so'z xatosi (WER)**, soatiga **$0.111** (Turbo $0.04, Distil $0.02) ([Groq docs](https://console.groq.com/docs/speech-to-text)). ~10% WER "Registon", "Shohi Zinda", "Chimyon" kabi atoqli otni chiqarishga yetarli. **Ammo:** ovoz olish uchun yuklab olish kerak; YouTube §III.E.1 kontentni "download, import, backup, cache, or store" qilishni, §III.I.7 esa ovoz va videoni ajratishni taqiqlaydi ([YouTube Policies](https://developers.google.com/youtube/terms/developer-policies)). Uchinchi tomon "youtube-transcript" kutubxonalari ham rasmiy API'dan tashqari ishlaydi va o'zlari ToS/mualliflik huquqiga bo'ysunishini tan oladi ([youtube-transcript.io](https://www.youtube-transcript.io/terms-of-service)).
- **Ekrandagi matn (OCR):** ijodkor "📍 Registon maydoni" deb yozganda, OCR eng aniq avtomatik signal bo'lardi (bu ijodkorning o'z yorlig'i) — lekin bu kadrlarni, ya'ni video faylini talab qiladi.
- **AI landmark/ko'rish tanib olish:** Google Cloud Vision landmark detection **nom + lat/long koordinata + Knowledge Graph ID** qaytaradi — bu Wikidata/OSM'ga eng toza mos tushadigan natija, eng yaxshi aniqlik. Mashhur O'zbek yodgorliklari (Registon, Bibi-Xonim, Ichan Qal'a) uchun ehtimoli yuqori; kichik kafe/manzaralar uchun zaif ([Cloud Vision](https://docs.cloud.google.com/vision/docs/landmark-tutorial)). **Ammo** u ham kadrlarni = yuklab olishni talab qiladi.

Xulosa: OCR va ko'rish tanib olish — "eng yaxshi aniqlik, eng yomon qonuniylik"; ularni faqat **o'z** yoki **ochiq litsenziyali** footage uchun saqlang.

### 2.3 Instagram / TikTok: sanktsiyalangan yo'l yo'q

- **Instagram:** ixtiyoriy jamoat Reels'ining sarlavhasi/transkripti uchun sanktsiyalangan endpoint YO'Q. Graph API faqat siz boshqaradigan Business/Creator akkauntlari ma'lumotini beradi; Basic Display API 2024-dekabrda butunlay o'chirilgan ([IG changelog](https://developers.facebook.com/docs/instagram-platform/changelog)).
- **TikTok:** `voice_to_text` transkript maydoni aynan kerakli signal — lekin u faqat **Research API**'da mavjud, u esa faqat **AQSh/Yevropadagi notijorat universitet akademiklari** uchun ochiq. Tijorat ilovasi uni qonuniy ololmaydi ([TikTok Research API codebook](https://developers.tiktok.com/doc/research-api-codebook); [query videos](https://developers.tiktok.com/docs/en/research-api-specs-query-videos)).

### 2.4 Yagona istisno: YouTube Creative Commons (CC BY)

YouTube ijodkorlari videoni **CC BY** deb belgilashi mumkin — bu qayta ishlatishga (shu jumladan tijorat, atributsiya bilan) ruxsat beradi, ya'ni ovozni transkripsiya qilish va transkript/ma'lumotni qayta ishlatish mumkin — to'liq avtomatik quvur uchun eng toza huquqiy yo'l. YouTube ~**4 million CC litsenziyali video** kutubxonasini targ'ib qilgan ([License types](https://support.google.com/youtube/answer/2797468); [YouTube blog](https://blog.youtube/news-and-events/heres-your-invite-to-reuse-and-remix-4/)). **Lekin** korpus yupqa, ayniqsa O'zbekiston uchun, va CC litsenziyasi mualliflik to'sig'ini olib tashlasa-da, YouTube'ning API siyosati (scraping/download yo'q) texnik kirish usuliga baribir tegishli — nusxani ruxsat etilgan yo'l bilan oling va ijodkorga atributsiya bering. Instagram/TikTok'da bunga ekvivalent yo'q.

### 2.5 TAVSIYA ETILGAN ISH OQIMI (aniq)

1. **Inson kurator** ijodkorlarning videolarini oddiy ko'rish tarzida tomosha qiladi (yuklab olmasdan) va joy nomlarini yozib oladi — tomosha qilish ToS buzilishi emas.
2. Ixtiyoriy avto-yordam: rasmiy YouTube API orqali **legal tavsif/bob matnini** oling va LLM bilan nomzod joy nomlarini chiqarib, kurator mehnatini kamaytiring (≤30 kun kesh).
3. Har bir nomni **Wikidata/OSM**'ga solib hal qiling va yozuvni **ochiq ma'lumotdan** quring — koordinata, kanonik nom, QID/ID shu yerdan keladi.
4. Videoni faqat **embed/havola** orqali (YouTube iframe, oEmbed) ko'rsating — hech qachon nusxasini saqlamang.

Bu oqim bizning mavjud tizimimizga to'g'ridan-to'g'ri ulanadi: kurator chiqargan joy nomi `discover` quvuriga yoki admin review navbatiga tushadi, u yerda toifalanadi, dedupe qilinadi va ishonch darajasiga qarab avto-nashr yoki qo'lda tasdiq bo'ladi. Foydalanuvchilar ham `POST /places/suggest` orqali joy taklif qilishi mumkin (hech qachon avto-nashr emas). Ya'ni video — faqat yangi frontend ishora manbasi; baza mantig'i o'zgarmaydi.

## Xulosa: ustuvor harakatlar va ochiq savollar

**Birinchi kuratsiya qilinadigan ijodkorlar (ustuvorlik bo'yicha):** (1) **@sanjiksancho** va **@axmedov_dilmurad** Instagram'da — yashirin joylar va aniq nomlangan Orol/road-trip nuqtalari joy-kashf uchun eng zich; (2) **Vagabrothers**, **Drew Binsky**, **David's Been Here** YouTube'da — bu videolarning tavsiflari odatda bob/joy nomlari va havolalar bilan boy, ya'ni 2.1-usuldagi "tavsif → joy-nom" avto-yordamiga eng mos; (3) **@side_quest_project** TikTok'da. Keyingi amaliy qaror: **kichik "YouTube tavsif → joy-nom ishoralari" yordamchisini** qurish arziydimi — u rasmiy API'dan tavsif/boblarni oladi, LLM bilan nomzod nomlarni chiqaradi va to'g'ridan-to'g'ri admin navbatini oziqlantiradi. Bu kam xavfli, legal va mavjud quvurga ulanadi; MVP sifatida 10–20 ta eng yaxshi O'zbekiston videosida sinab ko'rish mumkin. To'liq avtomatik "video → transkript → joy" quvuriga **sarmoya kiritmang** — u faqat CC BY korpusida qonuniy, u esa O'zbekiston uchun juda yupqa.

**Ochiq savollar / tekshirish kerak:** (a) ko'p Instagram/TikTok handle va obunachi raqamlari tasdiqlanmagan (@darsik, @klavdinka_malinka, @journeywithoutend, @khojiakbar_karim, @side_quest_project obunachisi, @uzbekistan.travel/@gotouzbekistan) — platformada jonli ochib tasdiqlang; (b) flaglangan har bir xalqaro ijodkor (Indigo Traveller, Eva zu Beck, Kara & Nate) haqiqatan O'zbekiston videosi borligini kanalni to'g'ridan-to'g'ri ochib tekshiring — hozirgi dalil "yo'q"ga ishora qiladi; (c) 2026-yil 1-iyun YouTube siyosat reviziyasining ("derived metrics va data storage bo'yicha qo'shimcha siyosatlar") to'liq matni olinmagan — kesh chegaralari o'zgargan bo'lishi mumkin, nashrdan oldin aniq matnni tasdiqlang; (d) Meta oEmbed Read Instagram sarlavha matnini qaytaradimi — tasdiqlanmagan (tarixan faqat muallif/thumbnail/embed HTML beradi, to'liq sarlavha emas).
