# Yo'ldosh AI: ochiq ma'lumot, deep link va hamkorlik

Qisqa javob: Yo'ldosh AI uchun **bitta "hammasini beradigan" manba yo'q**. Ilovani to'rt xil yo'l bilan to'ldirish kerak. Birinchi yo'l import: bepul va o'z bazamizda saqlash mumkin bo'lgan ma'lumot. Bularga OpenStreetMap (OSM), Wikidata, Markaziy bank kurslari va davlat reyestrlari kiradi. Ikkinchisi jonli chaqiruv: Google Places, ob-havo kabi pullik API lar. Ularning ma'lumotini bazada saqlab bo'lmaydi. Uchinchisi deep link: Yandex Navigator, Yandex Go, Booking.com, iTicket. Bunda foydalanuvchini o'sha ilovaga o'tkazamiz. To'rtinchisi hamkorlik: blogerlar, Telegram kanallar, restoranlar, mehmon uylari. Instagram va Telegram dan "yashirin joylarni" avtomatik yig'ish (scraping) ikkala platforma qoidalarida ham taqiqlangan. Qonuniy yo'l faqat bitta: muallifning roziligi bilan hamkorlik. MVP ni deyarli bepul qurish mumkin. Asosi OSM + Wikidata + o'z admin panelimizdagi qo'lda tekshirilgan joylar bo'ladi. Ustiga Travelpayouts linklari, navigatorlarga deep link, CBU kursi va OpenWeatherMap qo'shiladi. Eng katta bo'shliqlar: taom narxlari (hech qanday API yo'q), jamoat transporti jadvali (ochiq GTFS yo'q) va tadbirlar kalendari (API yo'q). Bu uch joyda ma'lumotni o'zimiz yig'amiz yoki hamkorlik orqali olamiz.

## Har bir manba to'rt yo'ldan biri orqali backendga kiradi

Har bir manbani tanlashda birinchi savol: **"Uni o'z bazamizda saqlay olamizmi?"** Javob ulanish usulini belgilaydi. Bizning Go backend da `places` va `city_services` jadvallarida allaqachon `source` maydoni bor (`manual`, `demo`, `open_data`, `partner`). Quyidagi to'rt usul shu maydonga to'g'ri keladi.

| Usul | Ma'nosi | Backend ga qanday kiradi | Misollar |
|---|---|---|---|
| **Import** | Ma'lumotni yuklab olib, PostgreSQL ga yozamiz | Go da tungi import job → `source='open_data'`, holati `pending` → admin panelda tekshirib `published` qilinadi | OSM, Wikidata, CBU kursi, davlat reyestrlari |
| **Jonli chaqiruv** | Foydalanuvchi bosganda API ga so'rov yuboramiz, natijani saqlamaymiz (yoki qisqa muddat keshlaymiz) | Backend proxy endpoint, faqat ID saqlanadi | Google Places, ob-havo |
| **Deep link** | Foydalanuvchini boshqa ilovaga yoki saytga o'tkazamiz | Faqat havola shabloni va koordinata kerak | Yandex Navigator, Yandex Go, Booking.com, iTicket |
| **Hamkorlik** | Shartnoma yoki rozilik asosida ma'lumot olamiz | Admin panel, hamkor kabineti yoki Telegram bot orqali → `source='partner'` | Blogerlar, restoranlar, mehmon uylari, Toshshahartransxizmat |

Muhim qoida: **OSM ma'lumotini o'z ma'lumotlarimizdan alohida qatlamda saqlang.** OSM ODbL litsenziyasida. Ilova ekranlarida faqat "© OpenStreetMap contributors" deb yozish kifoya. Lekin OSM asosida boyitilgan bazani ommaga tarqatsak, o'sha baza ham ODbL ostida qolishi kerak ([OSMF Licence FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)). Shuning uchun `places` jadvaliga `osm_id` ustuni qo'shish tavsiya etiladi. OSM teglari alohida jadvalda turadi. Bizning reyting, rasm va narxlar ID orqali bog'lanadi. Ichki foydalanish "ommaviy foydalanish" hisoblanmaydi ([OSMF FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)).

## Turistik joylar va ovqatlanish: OSM asos, Google faqat jonli boyitish

OSM O'zbekiston uchun eng yaxshi bepul va saqlanadigan asos. 2026-yil 25-noyabrdagi jonli so'rov natijalari: butun mamlakatda **2,575 ta restoran**. Faqat Toshkentda **752 restoran, 1,105 kafe va 583 fast-food** bor ([Overpass API](https://overpass-api.de/api/interpreter)). Lekin ma'lumot to'liq emas. Toshkentdagi restoran va kafelarning faqat **~26% ida ish vaqti** va **~34% ida oshxona turi** ko'rsatilgan. Reyting, rasm va narx umuman yo'q ([Overpass API](https://overpass-api.de/api/interpreter)). Butun mamlakat fayli bor-yo'g'i **120 MB** va har kuni yangilanadi ([Geofabrik](https://download.geofabrik.de/asia/uzbekistan.html)).

Tarixiy obidalar uchun Wikidata kuchli. U yerda O'zbekistonning **8,260 ta meros obyekti** bor. Lekin ulardan faqat **1,472 tasida koordinata** va **714 tasida rasm** bor ([Wikidata Query Service](https://query.wikidata.org/)). Wikidata CC0, ya'ni hech qanday shartsiz saqlash mumkin. Wikimedia Commons rasmlari esa har biri o'z litsenziyasi bilan keladi, muallifini yozish shart. 2024-yilgi Wiki Loves Monuments tanlovida O'zbekistondan **4,096 ta rasm** yuklangan ([Commons WLM 2024](https://commons.wikimedia.org/wiki/Commons:Wiki_Loves_Monuments_2024_in_Uzbekistan)).

Google Places eng boy ma'lumotga ega bo'lsa kerak: reyting, sharh, rasm, ish vaqti. Ammo uni **seed sifatida ishlatib bo'lmaydi**. Faqat `place_id` ni abadiy saqlash mumkin. Koordinatani 30 kungacha keshlash mumkin. Qolganini har safar jonli olish kerak. Nom, manzil va sharhlarni ommaviy saqlash taqiqlangan ([Places API policies](https://developers.google.com/maps/documentation/places/web-service/policies)). 2025-yil 1-martdan $200 kredit o'rniga har SKU uchun oylik bepul limit kiritildi: Essentials 10,000, Pro 5,000, Enterprise 1,000 ([Google March 2025](https://developers.google.com/maps/billing-and-pricing/march-2025)).

| Manba | Nima beradi | Narx | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **OSM (Geofabrik)** | Joy nomi, koordinata, kategoriya, qisman ish vaqti | Bepul | **Ha** (ODbL: atribusiya; ochiq bazani tarqatsak share-alike) | Toshkentda yaxshi, viloyatlarda siyrak | **MVP asosi. Import** |
| **Wikidata** | Meros obyektlari ro'yxati, ID lar, ko'p tilli nomlar | Bepul | **Ha** (CC0) | 8,260 obyekt, koordinatasi kam | **MVP. Import** (obidalar) |
| **Wikimedia Commons** | Obidalar rasmlari | Bepul | Ha, har rasmga muallif va litsenziya yoziladi | WLM: 2024-yilda 4,096 rasm | **MVP. Import** (litsenziya filtri bilan) |
| **Google Places (New)** | Reyting, sharh, rasm, ish vaqti, narx darajasi | 1k ga $5–40, oyiga 1k–10k bepul ([Google pricing](https://developers.google.com/maps/billing-and-pricing/pricing)) | **Yo'q**, faqat `place_id` | Ehtimol eng yaxshi, lekin o'lchanmagan | 2-bosqich. **Jonli chaqiruv** |
| **2GIS Places API** | Toshkent tashkilotlari, telefon, ish vaqti | 10k so'rov 6,700 ₽/oy dan ([2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing)) | Noma'lum, cheklangan deb hisoblang | Toshkent (2019 dan), Samarqand, Buxoro ([Daryo](https://daryo.uz/2019/08/31/ozbekistonda-2gis-dasturi-ishga-tushirildi); [kun.uz](https://kun.uz/en/61556915)) | 2-bosqich. Ma'lumot litsenziyasi bo'yicha **hamkorlik** so'rang |
| **Yandex Geosearch** | Tashkilotlar qidiruvi | Yiliga 195,000 ₽ dan yoki oyiga 20,800 ₽ ([Yandex tariffs](https://yandex.com/dev/tariffs/doc/ru/geosearch/prices/)) | Standart litsenziyada yo'q. "Saqlash bilan" litsenziya qimmatroq | Keng ishlatiladi | **Tavsiya etilmaydi** (qimmat) |
| **Tripadvisor Content API** | Turistik reyting, sharh, rasm | Oyiga 5,000 chaqiruv bepul ([Tripadvisor FAQ](https://tripadvisor-content-api.readme.io/reference/faq)) | Faqat ko'rsatish, logotip bilan | Samarqand, Buxoro, Xiva | 2-bosqich. **Jonli chaqiruv** |
| **Foursquare OS Places** | Global POI dump | Bepul | Ha (Apache 2.0) ([Foursquare](https://foursquare.com/resources/blog/products/foursquare-open-source-places-a-new-foundational-dataset-for-the-geospatial-community/)) | O'lchanmagan, ehtimol kam | Ixtiyoriy. Dublikat tekshirish uchun |
| **Overture Places** | Global POI (Meta, Foursquare va boshqalar) | Bepul | Ha, ko'pi CDLA-Permissive-2.0 ([Overture](https://docs.overturemaps.org/guides/places)) | O'lchanmagan | Ixtiyoriy. Import qilib sinab ko'ring |
| **Turizm qo'mitasi reyestrlari** | Turistik obyektlar yagona reyestri, 13 viloyat bo'yicha 1,056 yozuv ([gov.uz/uzbektourism](https://gov.uz/en/uzbektourism/sections/turistik-yo-nalish-marshrut-dagi-obyektlarining-yagona-reyestri)) | Bepul | Litsenziya noma'lum | Butun mamlakat | **Hamkorlik**: rasmiy eksport so'rang |

**Taom narxlari eng katta bo'shliq.** Uzum Tezkor, Yandex Eats va Express24 da menyular bor. Lekin ular dasturchilarga ma'lumot bermaydi, faqat restoranlar bilan integratsiya qiladi ([Spot.uz](https://www.spot.uz/ru/2025/04/18/uzum-tezkor/)). Numbeo faqat shahar bo'yicha o'rtacha narxlarni beradi va oyiga **$260 dan** turadi ([Numbeo API](https://www.numbeo.com/common/api.jsp)). Demak, har bir restoran narxini o'zimiz yig'amiz. Variantlar: foydalanuvchi chek yoki menyu rasmini yuklaydi. Restoran o'z menyusini admin panel orqali kiritadi. Yoki restoran ruxsati bilan uning Telegram postlaridan AI narxlarni ajratib oladi. Bizning `places.price_level` (0–3) maydoni boshlanishi uchun yetadi. Keyinchalik "osh ~X so'm" kabi namunaviy narxlar uchun alohida jadval kerak bo'ladi.

## Instagram va Telegram: faqat rozilik bilan, scraping yo'q

Bu bo'lim eng muhim. Sababi, ilovaning asosiy farqi aynan "kam tanilgan local joylar". Ular esa ko'pincha faqat Instagram va Telegramda bor. Bir sayohat blogida yozilganidek, ko'plab yashirin joylarning sayti yo'q, faqat Telegram kanali bor ([Minzifa Travel](https://minzifatravel.com/en/adventures/uzbekistan-blog/travel-like-a-local-using-digital-tools-to-experience-authentic-uzbekistan)).

**Instagram.** Joy yoki kalit so'z bo'yicha ommaviy postlarni ko'rib chiqadigan rasmiy API yo'q. Mavjud yo'llar uchta. Birinchisi Hashtag Search. U App Review talab qiladi va bitta akkaunt uchun **7 kunda atigi 30 ta hashtag** ruxsat beradi ([Meta Hashtag Search](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/hashtag-search)). Ikkinchisi Business Discovery. U username ma'lum bo'lgan biznes akkauntning ommaviy postlarini o'qiydi. Uchinchisi oEmbed. 2026-yil iyundan u tokensiz ishlaydi, lekin faqat postni ko'rsatish uchun, ma'lumot yig'ish uchun emas ([Meta blog](https://developers.facebook.com/blog/post/2026/06/15/tokenless-access-to-meta-oembed-apis/)). Basic Display API 2024-yil 4-dekabrda yopilgan ([Meta blog](https://developers.facebook.com/blog/post/2024/09/04/update-on-instagram-basic-display-api)). Meta qoidasi aniq: avtomatik ma'lumot yig'ish faqat API orqali yoki yozma ruxsat bilan mumkin ([Meta Automated Data Collection](https://developers.facebook.com/docs/development/terms-and-policies/automated-data-collection/)).

**Telegram.** Bot qoidalari §4.3 ommaviy kanal va guruhlarni scraping qilishni va katta dataset yoki AI mahsulot yaratish uchun ma'lumot yig'ishni **to'g'ridan-to'g'ri taqiqlaydi** ([Telegram Bot ToS](https://telegram.org/tos/bot-developers?setln=en)). Client API (MTProto) qoidalari ham Telegram ma'lumotini AI uchun yig'ishni taqiqlaydi. Buzilsa, 10 kunlik ogohlantirishdan keyin API yopiladi ([Telegram API ToS](https://core.telegram.org/api/terms)). Lekin foydalanuvchi botga o'zi yuborgan ma'lumotni ishlatish mumkin. Buning sharti: aniq, faol va qaytarib olinadigan rozilik ([Telegram Bot ToS](https://telegram.org/tos/bot-developers?setln=en)).

AQSh sud amaliyoti ham ogohlantiradi. Meta v. Bright Data (2024) ishida tizimga kirmasdan ommaviy sahifalarni yig'ish qoidabuzarlik deb topilmadi. Lekin bu faqat tuman sudi qarori, apellyatsiya bo'lmagan ([TechCrunch](https://techcrunch.com/2024/01/24/court-rules-in-favor-of-a-web-scraper-bright-data-which-meta-had-used-and-then-sued)). hiQ v. LinkedIn ishida esa soxta akkauntlar bilan scraping uchun **$500,000** to'lov, taqiq va ma'lumotlarni yo'q qilish belgilandi ([Proskauer](https://newmedialaw.proskauer.com/2022/12/08/hiq-and-linkedin-reach-proposed-settlement-in-landmark-scraping-case/)). Bizning amaliy xavf sud emas, balki akkaunt, API va App Review dan mahrum bo'lish. Bundan tashqari postlardagi yuzlar va ismlar shaxsiy ma'lumot, rasmlar esa muallifning mulki.

| Manba / yo'l | Nima beradi | Narx | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **Telegram bot (hamkor kanal)** | Hamkor botni kanalga admin qiladi yoki postlarni botga forward qiladi | Bepul | Ha, hamkor yozma ruxsati bilan: faqat joy faktlari + asl postga havola | Yuqori | **MVP. Hamkorlik** |
| **Telegram bot (foydalanuvchi yuboradi)** | Rasm, lokatsiya, tavsif | Bepul | Ha, rozilik va maxfiylik siyosati bilan | Yuqori | **MVP. Hamkorlik/UGC** |
| **Instagram Login (biznes o'zi ulaydi)** | Biznes o'z akkauntini ulaydi, postlari sinxronlanadi | Bepul (App Review kerak) | Ha, egasi roziligi bilan ([Meta blog](https://developers.facebook.com/blog/post/2024/07/30/instagram-api-with-instagram-login/)) | Kafelar, mehmon uylari | 2-bosqich. **Hamkorlik** |
| **Instagram oEmbed** | Asl postni joy kartochkasida ko'rsatish | Bepul | Yo'q, faqat embed | — | **MVP. Ko'rsatish uchun** |
| **Instagram Business Discovery** | Ma'lum biznes akkauntning ommaviy postlari | Bepul (App Review) | Meta Platform Terms ga bog'liq (o'qilmagan) | — | 2-bosqich |
| **Instagram Hashtag Search** | 7 kunda 30 hashtag | Bepul (App Review) | Cheklangan | — | Faqat monitoring uchun |
| **Scraping (Apify, userbot va h.k.)** | Hamma narsa | Arzon | **Yo'q** | — | **Qat'iyan tavsiya etilmaydi** |

Hamkor bo'lishi mumkin bo'lgan kanallar: rasmiy **@uzbekistan_travel** (~15.6k obunachi) ([t.me/uzbekistan_travel](https://t.me/uzbekistan_travel)) va kam tanilgan joylarni ko'rsatadigan **Sanjiksancho** (~112k). Yana Kompas (~33.5k, eski raqam) va Mandarin Tour (~44.5k) bor ([Russia-Promo](https://russia-promo.com/blog/leading-influencers-uzbekistan)). TGStat dagi "Travel" toifasida ko'pincha turagentliklar va ish e'lonlari kanallari ustun ([TGStat](https://uz.tgstat.com/en/travels)). Shuning uchun har bir kanalni qo'lda tekshiring.

Boshqa ilovalar ham shunday qiladi. Atlas Obscura foydalanuvchi yuborgan har bir joyni muharrir orqali o'tkazadi ([Atlas Obscura FAQ](https://www.atlasobscura.com/faq)). Spotted by Locals har shaharda tanlangan mahalliy "Spotter" larga tayanadi ([Spotted by Locals](https://spottedbylocals.com/about-us)). Google Local Guides esa ball tizimi va kuchli moderatsiyadan foydalanadi ([Google Local Guides](https://support.google.com/maps/answer/6225851?hl=en)). Bizning `places.novelty='hidden'` va `status='pending'` maydonlari shu modelga mos. Ish tartibi bunday: AI hamkor postidan JSON chiqaradi (nom, kategoriya, shahar, manzil, ish vaqti, narx darajasi, asl post ID). Yozuv `pending` holatida admin panelga tushadi. Muharrir tasdiqlagach `published` bo'ladi. O'zbekistonda bunday rozilikka asoslangan hamkorlik tarmog'iga ega raqobatchi topilmadi.

## Turar joy: API emas, affiliate link va o'z reyestrimiz

Booking.com Demand API faqat shartnoma imzolagan "Managed Affiliate Partner" larga ochiq ([Booking.com Developers](https://developers.booking.com/demand/docs/getting-started/prerequisites)). 2025-yil iyunda Booking.com minglab kichik affiliatelar bilan shartnomani bekor qildi va ularni tarmoqlarga yo'naltirdi ([Skift](https://skift.com/2025/05/30/why-booking-com-cut-thousands-of-affiliate-partners-and-what-comes-next/)). Startap uchun real yo'l **Travelpayouts**. Bir marta ro'yxatdan o'tiladi va Booking.com, Agoda, Trip.com, Hostelworld linklari bilan ishlanadi. Booking.com bo'yicha bronlash qiymatining **maksimum 4%** to'lanadi. Faqat link, banner va widget beriladi, **API yo'q** ([Travelpayouts Booking.com](https://www.travelpayouts.com/en/offers/bookingcom-affiliate-program)). Airbnb da ochiq API ham, affiliate dastur ham yo'q ([Airbnb API Terms](https://www.airbnb.com/terms/api)).

Booking.com da O'zbekiston bo'yicha **~3,000 ta joy** bor, shundan Toshkentda ~1,600 ([Booking.com](https://www.booking.com/country/uz.html)). Turizm qo'mitasi "Joylashtirish vositalari reyestri" ni yuritadi: nomi, egasi, manzili, kontakti, sertifikat raqami ([gov.uz](https://gov.uz/oz/uzbektourism/pages/joylashtirish-vositalarining-reyestri)). Bu "litsenziyalangan mehmon uyi" qatlami uchun yaxshi asos. Lekin narx va bo'sh joy ma'lumoti unda yo'q. E-Mehmon esa yopiq davlat tizimi, ochiq ma'lumot emas ([UZINFOCOM](https://uzinfocom.uz/uz/projects/e-mehmon-uz-latin-11)).

| Manba | Nima beradi | Narx / daromad | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **Travelpayouts** (Booking, Agoda, Trip.com, Hostelworld) | Link, widget, komissiya | Bepul. Booking.com da 4% gacha, Agoda da ~5–6%, Hostelworld da depozitning ~40% ([Travelpayouts](https://travelpayouts.com/blog/?p=19263)) | Narx va rasmni saqlash **yo'q** | Booking ~3,000 joy | **MVP. Deep link** |
| **Turizm qo'mitasi reyestri** | Litsenziyalangan joylar ro'yxati | Bepul | Litsenziya noma'lum | Butun mamlakat | **MVP. Import** (rasmiy so'rov bilan) |
| **OSM** `tourism=hotel/hostel/guest_house` | Nomi va koordinatasi | Bepul | Ha (ODbL) | Qisman | **MVP. Import** |
| **Mehmon uylari bilan to'g'ridan-to'g'ri** | Narx, rasm, bo'sh joy | Komissiya kelishuvi | Ha, shartnoma bilan | Xiva, Buxoro, Samarqandda kichik mehmon uylari ko'p | **MVP. Hamkorlik** (admin panel orqali) |
| **Agoda Affiliate Lite API** | Jonli narx | Komissiya | Kunlik yangilash tavsiya etiladi ([Agoda](https://partners.agoda.com/Content/Documents/AffiliateLiteApi/Affiliate_Lite_API_V2.0.pdf)) | Noma'lum | 2-bosqich |
| **Hostelworld API** | Hostel ma'lumoti | Individual ([Hostelworld](https://www.partners.hostelworld.com)) | Noma'lum | Noma'lum | 2-bosqich |
| **Booking Demand, Expedia Rapid, Hotelbeds, RateHawk** | To'liq inventar va narx | Shartnoma, sertifikatsiya, ko'pincha depozit ([Expedia](https://developers.expediagroup.com/rapid/setup/launch-requirements/b2c-standalone)) | Shartnomaga ko'ra | Katta | 3-bosqich |

Tipik narxlar (ilovada "taxminiy narx" ko'rsatish uchun). Hostel yotoq joyi **$9–15** ([Lonely Planet](https://www.lonelyplanet.com/articles/uzbekistan-on-a-budget)). Hostel yoki mehmon uyida xona o'rtacha **~$14** ([BudgetYourTrip](https://www.budgetyourtrip.com/hostels/uzbekistan-UZ)). Ipak yo'li shaharlarida nonushta bilan o'rta darajadagi xona **$19–42** ([Journal of Nomads](https://www.journalofnomads.com/uzbekistan-travel-costs-budget-guide/)). Toshkentda Booking.com dagi mehmonxonalar **~$46–57 dan** ([Champion Traveler](https://championtraveler.com/price/cost-of-a-trip-to-tashkent-uz)). Daromad kichik: 3 kecha $60–120 lik bronlash 4–6% komissiya bilan atigi **$3–7** keltiradi. Demak, turar joy MVP da asosiy daromad manbai emas.

## Transport: narxlar ochiq, jadvallar yopiq

O'zbekistonning hech bir shahri uchun ochiq **GTFS** (jamoat transporti jadvali formati) topilmadi. Toshshahartransxizmat 2020-yilda jonli avtobus ma'lumotlari bo'yicha **2GIS bilan eksklyuziv** shartnoma tuzgan ([Podrobno.uz](https://podrobno.uz/cat/tehnp/toshshakhartranskhizmat-i-2gis-podpisali-soglashenie-o-monitoringe-obshchestvennogo-transporta-/)). Yandex Maps ham Toshkentda ~230 yo'nalishni jonli ko'rsatadi ([Kun.uz](https://kun.uz/en/news/2019/09/19/yandex-launches-real-time-bus-movement-tracking-service-in-tashkent)). Shuning uchun MVP da "qanday borish mumkin" tugmasi Yandex Maps yoki 2GIS ga deep link bo'lgani ma'qul. Toshkent metrosida **50 bekat** bor ([Daryo](https://daryo.uz/en/2026/11/04/tashkent-metro-plans-to-introduce-electronic-displays-showing-train-arrival-times/)). Bu kichik son, ularni qo'lda kiritish mumkin.

Narxlar davlat qarorlari bilan e'lon qilinadi. Bu ommaviy fakt, manba va sana bilan saqlash mumkin. Ammo ular **yiliga kamida bir marta o'zgaradi**. Shuning uchun kodga yozib qo'ymang, admin panelda "sanasi bilan" saqlang.

| Narx (2026) | Qiymat | Manba |
|---|---|---|
| Toshkent metro/avtobus (1-oktabr 2026 dan) | Karta 2,500 so'm, naqd 3,000 so'm, 1 kunlik 13,500 so'm | [UzDaily](https://www.uzdaily.uz/en/tashkent-approves-new-public-transport-fares-from-1-october/) |
| Samarqand avtobus/tramvay (1-may 2026 dan) | Karta 2,000 so'm, naqd 4,000 so'm | [Kun.uz](https://kun.uz/en/news/2026/05/01/samarkand-raises-public-transport-fares-plans-to-phase-out-cash) |
| Buxoro (1-aprel 2026 dan) | Faqat elektron to'lov | [Mintrans](https://gov.uz/ru/mintrans/news/view/124612) |
| Afrosiyob Toshkent–Samarqand | Ekonom 311,000 / Biznes 455,000 / VIP 709,000 so'm | [Kun.uz](https://kun.uz/en/news/2026/01/16/uzbekistan-railways-raises-train-fares-again-from-february-1) |
| Afrosiyob Toshkent–Buxoro | Ekonom 509,000 so'm | [Kun.uz](https://kun.uz/en/news/2026/01/16/uzbekistan-railways-raises-train-fares-again-from-february-1) |
| Yandex Go "Start", Toshkent | Min 4,600 so'm + 1,050 so'm/km + 500 so'm/daqiqa (uchinchi tomon o'qishi) | [Voyage.uz](https://voyage.uz/guides/transport-yandex-go/) |
| Uzbekistan Airways Toshkent–Buxoro | 276,741 so'm dan | [uzairways.com](https://uzairways.com/en/press-center/news/uzbekistan-airways-fare-policy-update) |

Eslatma: bizning `cities` jadvalida taksi uchun standart qiymatlar 10,000 so'm boshlang'ich va 3,000 so'm/km. Bu Yandex Go "Start" e'lon qilingan tarifidan ancha yuqori. Admin panelda shahar bo'yicha yangilab qo'ying.

| Manba | Nima beradi | Narx | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **Yandex Go deep link** | Ilovani manzil va tarif bilan ochadi ([Yandex docs](https://yandex.com/support/taxi-distr/en/api/deeplinks)) | Bepul | — | Toshkent, Xiva, Urganch va boshqalar | **MVP. Deep link** |
| **Yandex Trip info API** | Taksi narxi va vaqti bahosi ([Yandex](https://yandex.com/support/taxi-distr/en/api/trip-info)) | Hamkorlik dasturi orqali | Yo'q | O'zbekiston zonalarida ishlashi tekshirilmagan | 2-bosqich |
| **MyTaxi** | Faqat do'kon havolasi | — | — | Faqat Toshkent ([Times of Central Asia](https://timesca.com/yandex-go-outage-boosts-rival-uzbekistan/)) | Hamkorlik bo'lmaguncha store link |
| **eticket.railway.uz** | Rasmiy poyezd chiptasi | — | API yo'q | Barcha poyezdlar ([dgov.uz](https://dgov.uz/en/solution-detail/109)) | **MVP. Deep link** |
| **12Go (Travelpayouts orqali)** | Poyezd/avtobus chiptasi, komissiya | 12Go komissiyasining 50% gacha, ~$3/bron ([Travelpayouts](https://www.travelpayouts.com/en/offers/12go-affiliate-program)) | Yo'q | Afrosiyob sinf tanlovi yo'q | **MVP. Deep link** (daromad uchun) |
| **avtoticket.uz, UzBus, ATTO** | Shaharlararo avtobus chiptasi | — | API yo'q ([Trend.az](https://en.trend.az/casia/uzbekistan/3703244.html)) | Toshkent avtovokzalidan | **MVP. Deep link** |
| **Aviasales Data API** | Keshlangan aviabilet narxlari | Bepul. Jonli Search API faqat 50k+ MAU uchun ([Travelpayouts](https://support.travelpayouts.com/hc/en-us/articles/203956173)) | Travelpayouts shartlariga ko'ra | Ichki reyslar kam bo'lishi mumkin | MVP/2-bosqich |
| **2GIS Public Transport API** | Avtobus marshruti | Pullik, narxi tekshirilmagan ([2GIS docs](https://docs.2gis.com/en/api/navigation/public-transport/examples)) | Yo'q | Toshkent jonli | 2-bosqich. **Hamkorlik** |
| **Amadeus Self-Service** | — | 2026-yil 17-iyulda yopilgan ([Amadeus](https://amadeus.com/en/industry-messaging/statement-regarding-amadeus-for-developers-portal)) | — | — | **Ishlatib bo'lmaydi** |

## Navigator: xaritani o'zimiz ko'rsatamiz, yo'l ko'rsatishni Yandexga topshiramiz

Toshkentda tirbandlik ma'lumotida Yandex aniq yetakchi. 2019-yildan tirbandlik balli, svetofor taymerlari va 2026-yil sentabrdan Toshkent, Andijon va Namangan uchun qator darajasidagi xaritalar bor ([spot.uz](https://www.spot.uz/ru/2026/09/23/yandex-maps/)). Ammo Yandex MapKit shartlari §2.4 SDK asosida **avtomatik navigatsiya yaratishni taqiqlaydi** ([Yandex MapKit terms](https://yandex.com/dev/tariffs/doc/en/mapkit/terms/index.md)). Shuning uchun eng to'g'ri yo'l: xaritani o'zimiz ko'rsatamiz, "Yo'lga chiqish" tugmasi Yandex Navigator, 2GIS yoki Google Maps ga deep link qiladi.

Expo ilovasi uchun eng arzon xarita to'plami: **@maplibre/maplibre-react-native + OpenFreeMap**. OpenFreeMap bepul, API kalitsiz, tijoriy foydalanishga ruxsat beradi va so'rovlar soniga cheklov qo'ymaydi. Faqat atribusiya kerak ([openfreemap.org](https://openfreemap.org/)). MapLibre native modul. Shuning uchun Expo Go da ishlamaydi, development build (EAS) kerak bo'ladi. Bu tekshirilmagan, lekin umumiy qoida.

| Manba | Nima beradi | Narx | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **OpenFreeMap + MapLibre RN** | Vektor xarita | Bepul ([openfreemap.org](https://openfreemap.org/)) | Offline: bir yuklashda ≤50 MB / 3,000 tile | OSM sifatiga teng | **MVP. Xarita** |
| **O'zimiz host qilgan PMTiles** | Butun mamlakat offline xaritasi | VPS/CDN narxi | Ha (ODbL) | OSM | 2-bosqich (offline rejim) |
| **Valhalla / GraphHopper (o'z serverimizda)** | Marshrut ko'rinishi, ETA, piyoda yo'l | Bepul (MIT / Apache 2.0), kichik VPS ([Valhalla](https://github.com/Valhalla/valhalla)) | Ha | OSM sifatiga teng, tirbandlik yo'q | 2-bosqich. MVP da Mapbox/Google yetadi |
| **Mapbox Directions** | Marshrut | Oyiga 100,000 so'rov bepul ([Mapbox pricing](https://www.mapbox.com/pricing)) | Shartlariga ko'ra | O'rtacha | MVP alternativ. **Jonli chaqiruv** |
| **Google Routes API** | Marshrut | Oyiga 10,000 bepul, keyin 1k ga $5 ([Google pricing](https://developers.google.com/maps/billing-and-pricing/pricing)) | Yo'q | Tirbandlik sifati tasdiqlanmagan | MVP alternativ |
| **Yandex Navigator / 2GIS / Google Maps deep link** | To'liq navigatsiya, tirbandlik, kameralar | Bepul ([2GIS help](https://help.2gis.com/question/developers-launching-2gis-navigation-using-deeplink)) | — | Yandex eng yaxshi | **MVP. Deep link** |
| **Yandex MapKit** | Mahalliy xarita | ≤1,000 DAU bepul ([Yandex MapKit prices](https://yandex.ru/dev/tariffs/doc/en/mapkit/prices/)) | Geocoder/Router: 30 kungacha | Eng yaxshi | Ixtiyoriy. Navigatsiya taqiqlangan |
| **Google Navigation SDK (RN)** | Ilova ichida yo'l ko'rsatish | Oyiga 1,000 manzil bepul ([Google](https://developers.google.com/maps/documentation/navigation/android-sdk/pricing)) | Yo'q | Tasdiqlanmagan | 3-bosqich (beta) |
| **OSM `amenity=fuel`, `charging_station`** | Yoqilg'i (metan, propan) va EV zaryad shoxobchalari | Bepul | Ha | Soni o'lchanmagan | **MVP. Import** |
| **Open Charge Map** | EV zaryad | Bepul kalit kerak (2026-11-25 da tekshirildi) | Litsenziyani tekshiring | O'lchanmagan | MVP/2-bosqich |
| **Energetika vazirligi Avto24 / U-Gas** | Jonli metan mavjudligi, 1,553 ta CNG shoxobchasi ([gov.uz](https://gov.uz/en/cirns/news/view/195943); [kun.uz](https://www.kun.uz/en/news/2024/11/20/uzbekistan-introduces-real-time-fuel-and-gas-station-tracking-app)) | — | Ochiq API yo'q | Butun mamlakat | 2-bosqich. **Hamkorlik** |

Mashinadagilar uchun muhim faktlar. EV zaryad shoxobchalarining ~70% i Toshkent shahri va viloyatida ([Daryo](https://daryo.uz/en/2025/05/04/uzbekistan-sees-2-5-fold-surge-in-ev-charging-stations-1216442/)). Shuning uchun shaharlararo EV safarida masofani rejalashtirish ogohlantirishi kerak. 2026-yil 30-oktabrdan ko'chma tezlik kameralari uchun ogohlantirish belgisi talab qilinmaydi ([Daryo](https://daryo.uz/en/2026/11/02/uzbekistan-removes-mandatory-warning-signs-for-mobile-speed-cameras/)). Bu ham navigatsiyani Yandexga topshirish uchun qo'shimcha sabab. Chet elliklarga milliy guvohnoma va xalqaro haydovchilik guvohnomasi (IDP) kerak ([GOV.UK](https://www.gov.uk/guidance/living-in-uzbekistan)).

## Ob-havo, valyuta, SOS va tadbirlar: uchtasi oson, tadbirlar qo'lda

**Valyuta eng oson qism.** Markaziy bankning JSON endpointi kalitsiz ishlaydi. 2026-11-25 da USD = **11,990.54 so'm** qaytardi ([CBU JSON](https://cbu.uz/uz/arkhiv-kursov-valyut/json/)). Kuniga bir marta olib, saqlang va "CBU rasmiy kursi" deb belgilang. Bu bank sotib olish/sotish kursi emas. Apple Pay va Google Pay 2026-yil 27-oktabrdan ishga tushdi ([Euronews](https://www.euronews.com/2026/10/30/apple-pay-and-google-pay-launch-in-uzbekistan-amid-wider-fintech-reforms)). Uzcard va Humo ni Apple Pay ga qo'shish 17-noyabrdan cheklangan holda boshlandi ([CBU](https://cbu.uz/en/press_center/releases/4929729/)). Bankomatlar uchun ochiq dataset yo'q, OSM `amenity=atm` dan foydalaning.

**Ob-havo.** Open-Meteo ning bepul tarifi **faqat notijoriy** foydalanish uchun. Reklama yoki obuna bo'lgan ilova tijoriy hisoblanadi ([Open-Meteo Terms](https://open-meteo.com/en/terms)). OpenWeatherMap bepul tarifi daqiqasiga 60 va oyiga 1,000,000 chaqiruvgacha. Pullik rejasi oyiga 30 GBP dan ([OpenWeatherMap](https://openweathermap.org/price)). O'zgidromet API bermaydi ([FAO](https://agritechobservatory.review.fao.org/ru/hydrometuz)).

**SOS.** Raqamlar tasdiqlangan: **112** (2025-yil martdan butun mamlakatda) ([Kun.uz](https://kun.uz/en/news/2025/03/25/uzbekistans-unified-112-emergency-system-now-fully-operational-nationwide)), 101 o't o'chirish, 102 militsiya, 103 tez yordam, **1173** turistlar uchun ishonch telefoni (24/7, ingliz, o'zbek, rus tillarida) ([GOV.UK](https://www.gov.uk/foreign-travel-advice/uzbekistan/getting-help)). Yana 104 gaz xizmati ([my.gov.uz](https://oldmy.gov.uz/ru/life-situations/14)) va 1050 Favqulodda vaziyatlar vazirligi ([OSAC](https://www.osac.gov/Content/Report/b6b69dde-af36-471e-a427-1c2097fde1f4)). Bular `cities.emergency` jsonb maydoniga to'g'ridan-to'g'ri kiradi. Shifoxona va elchixonalar `city_services` jadvaliga tushadi.

**Tadbirlar uchun API yo'q.** uzbekistan.travel faqat HTML sahifa va afisha.uz ga havola beradi ([uzbekistan.travel](https://uzbekistan.travel/en/o/events-festivals/)). iTicket.uz yirik konsertlarning rasmiy sotuvchisi, lekin ochiq API yoki affiliate dasturi topilmadi ([gov.uz](https://gov.uz/en/uzbektourism/news/view/60460)). 2023-yilda Humo Arenadagi soxta chipta janjali ([Daryo](https://daryo.uz/en/2023/11/27/ticket-scam-unleashes-chaos-at-tashkents-humo-arena-during-jah-khalibs-concert/)) shuni ko'rsatadi: ilova faqat rasmiy sotuvchilarga havola berishi kerak.

| Manba | Nima beradi | Narx | Saqlash mumkinmi | O'zbekistonda qamrov | Tavsiya |
|---|---|---|---|---|---|
| **CBU JSON** | Rasmiy kurslar | Bepul | Ha (ommaviy fakt; foydalanish shartlari o'qilmagan) | To'liq | **MVP. Import** (kunlik) |
| **OpenWeatherMap** | Ob-havo va prognoz | Oyiga 1M chaqiruv bepul | Litsenziya ODbL deb ko'rsatilgan | Global | **MVP. Jonli chaqiruv + 1 soatlik kesh** |
| **Open-Meteo** | Ob-havo | Notijoriy bepul, tijoriy narx e'lon qilinmagan ([Open-Meteo](https://open-meteo.com/en/pricing)) | CC BY 4.0 | Global | Faqat ilova notijoriy bo'lsa |
| **SOS raqamlari** | 112, 101–104, 1050, 1173 | Bepul | Ha | To'liq | **MVP. Admin panelda qo'lda** |
| **Turizm qo'mitasi tibbiyot muassasalari reyestri** | Klinikalar ([gov.uz](https://gov.uz/en/uzbektourism/sections/turistik-yo-nalish-marshrut-dagi-obyektlarining-yagona-reyestri)) | Bepul | Litsenziya noma'lum | Butun mamlakat | **MVP. Import/qo'lda** |
| **OSM** `amenity=hospital/pharmacy/atm/police` | Koordinatalar | Bepul | Ha (ODbL) | Qisman | **MVP. Import** → `city_services` |
| **Elchixonalar ro'yxati** | Manzil va telefon ([travel.gc.ca](https://travel.gc.ca/assistance/embassies-consulates/uzbekistan)) | Bepul | Ha (fakt) | Toshkent | **MVP. Qo'lda** |
| **uzbekistan.travel, afisha.uz, gov.uz yangiliklari** | Tadbirlar ro'yxati | Bepul | Faktni qayta yozish mumkin; matn va rasm mualliflik huquqida | Yirik tadbirlar | **MVP. Qo'lda kiritish** |
| **iTicket.uz, Ticketon** | Chipta sotish ([Ticketon](https://ticketon.kz/en/tashkent/event/original-enigma-voices-uz)) | Kelishuv | — | Toshkent konsertlari | **MVP. Deep link**, keyin komissiya bo'yicha hamkorlik |
| **data.egov.uz** | Davlat datasetlari (CSV/JSON/API, id.egov.uz tokeni bilan) ([user guide](https://data.egov.uz/user_en.pdf)) | Bepul | Litsenziya topilmadi | Turlicha | Qidirib ko'rish kerak |

Tadbirlar uchun bizning bazada hozircha jadval yo'q. MVP uchun oddiy `events` jadvali kerak: nom, sana, joy, `place_id`, chipta havolasi, manba, `verified_at`. Navro'z (21-mart) kabi sanalari o'zgarmas bayramlarni bir marta kiritish kifoya. Boysun bahori (2026-yil 1–7-may) ([Advantour](https://www.advantour.com/uzbekistan/festivals.htm)) kabi festivallarni har yili tekshirish kerak.

## Huquqiy chegaralar: narx so'mda, shaxsiy ma'lumot va turizm bildirishnomasi

Uchta qonun ilovaga bevosita ta'sir qiladi. **Birinchi: narxlar so'mda ko'rsatilishi kerak.** Reklama to'g'risidagi qonun (ZRU-776) reklamada xorijiy valyutadagi narxni taqiqlaydi. Valyuta qonuni esa narxlarni, jumladan e-commerce da, faqat milliy valyutada belgilashni talab qiladi ([Kun.uz 2022](https://kun.uz/en/news/2022/09/09/law-on-advertising-comes-into-force-basic-rules-and-restrictions-announced); [Kun.uz 2019](https://kun.uz/en/news/2019/10/25/linking-prices-to-foreign-currency-and-contingent-units-banned-in-uzbekistan)). Ilovada asosiy narx so'mda bo'lsin. Dollar ekvivalenti faqat ma'lumot uchun, "CBU kursi, sana" belgisi bilan. Buning chet elliklar uchun ruxsat etilishini yurist bilan tasdiqlang.

**Ikkinchi: shaxsiy ma'lumot.** 2026-yil martdagi 1125-son qonun 2021-yildagi "hamma narsani O'zbekistonda saqlash" talabini toraytirdi. Endi majburiy lokalizatsiya faqat biometrik, genetik va telekom abonent ma'lumotlariga tegishli ([Kun.uz](https://kun.uz/en/news/2026/03/27/uzbekistan-amends-personal-data-law-to-facilitate-global-payment-systems)). Boshqa ma'lumotlarni AQSh, Germaniya, Buyuk Britaniya kabi **49 ta "adekvat" davlatga** o'tkazish mumkin. Chegaradan o'tkazishda buzilish bo'lsa, 24 soat ichida xabar berish kerak ([Kun.uz](https://kun.uz/en/news/2026/08/04/uzbekistan-approves-49-countries-for-simplified-personal-data-transfers)). Foydalanuvchi ma'lumotlari uchun eng xavfsiz yo'l hali ham O'zbekistondagi hosting. Selfie orqali yuz tekshiruvi qilsangiz, u albatta O'zbekistonda saqlanishi shart.

**Uchinchi: turizm faoliyati.** Litsenziya 2024-yil 1-martdan bildirishnoma bilan almashtirildi ([Kun.uz](https://kun.uz/en/news/2025/04/30/no-more-licenses-travel-agencies-in-uzbekistan-to-operate-under-new-rules)). Faqat ma'lumot beradigan va link orqali yo'naltiradigan ilovaga bildirishnoma kerak bo'lmasa kerak. Tur paketlarini o'zi sotsa, kerak bo'ladi. Xususiy uyda turgan chet ellik 3 kun ichida E-Mehmon orqali ro'yxatdan o'tishi kerak ([GOV.UK](https://www.gov.uk/foreign-travel-advice/uzbekistan/entry-requirements)). Ilova bu haqda eslatma bera oladi.

## MVP uchun tavsiya etilgan to'plam

Bu to'plamning to'g'ridan-to'g'ri API xarajati deyarli nol. Asosiy xarajat: server, bitta-ikkita muharrir vaqti va hamkorlar bilan ishlash.

| Kategoriya | Manba | Ulanish usuli | Backend ga qayerga tushadi | Oylik xarajat |
|---|---|---|---|---|
| Joylar (asos) | OSM (Geofabrik) + Wikidata + Commons | Import (haftalik) | `places` (`source='open_data'`, `status='pending'`), `osm_id`, `wikidata_id` | $0 |
| Mashhur joylar | Mavjud demo seed + admin panel + Turizm qo'mitasi reyestri | Qo'lda / import | `places` (`novelty='iconic'`) | $0 |
| Yashirin joylar | Telegram bot (hamkor kanallar + foydalanuvchilar) + AI ajratish + muharrir | Hamkorlik | `places` (`novelty='hidden'`, `source='partner'`), Instagram oEmbed havolasi | LLM narxi |
| Ovqat narxi | `price_level` + restoran o'zi kiritadi + foydalanuvchi chek rasmi | Hamkorlik / UGC | `places.price_level`, keyinroq taomlar jadvali | $0 |
| Turar joy | Travelpayouts (Booking, Agoda, Hostelworld) + reyestr + to'g'ridan-to'g'ri mehmon uylari | Deep link + import + hamkorlik | `places` (`category='stay'`), affiliate havola | $0 (daromad keltiradi) |
| Shahar transporti | Narxlar admin panelda (sana bilan) + Yandex Maps/2GIS deep link | Qo'lda + deep link | `cities` (yangi tarif maydonlari) | $0 |
| Taksi | Yandex Go deep link + mahalliy narx bahosi | Deep link | `cities.taxi_base_fare`, `taxi_per_km` | $0 |
| Shaharlararo | eticket.railway.uz, avtoticket.uz, 12Go link + Afrosiyob narxlari | Deep link | Statik jadval | $0 |
| Xarita | MapLibre RN + OpenFreeMap | Jonli (bepul) | Mijozda | $0 |
| Marshrut ko'rinishi | Mapbox Directions yoki Google Routes Essentials | Jonli chaqiruv | Backend proxy | Bepul limit ichida $0 |
| Navigatsiya | Yandex Navigator / 2GIS / Google Maps | Deep link | Faqat koordinata | $0 |
| Yoqilg'i/EV | OSM import | Import | `city_services` (yangi `fuel`, `ev` turlari) | $0 |
| Tadbirlar | Qo'lda (uzbekistan.travel, afisha.uz, gov.uz) + iTicket link | Qo'lda + deep link | Yangi `events` jadvali | $0 |
| Ob-havo | OpenWeatherMap bepul tarif | Jonli + 1 soatlik kesh | Backend kesh | $0 |
| Valyuta | CBU JSON | Import (kunlik) | Yangi `fx_rates` jadvali | $0 |
| SOS | 112, 101–104, 1050, 1173 + OSM shifoxona/dorixona + elchixonalar | Qo'lda + import | `cities.emergency`, `city_services` | $0 |

## Nimadan boshlash kerak

Quyidagi qadamlar tartib bilan berilgan. Har biri oldingisiga tayanadi.

1. **Bazani tayyorlang (1-hafta).** `places` jadvaliga `osm_id`, `wikidata_id`, `google_place_id`, `license`, `attribution` ustunlarini qo'shing. OSM teglari uchun alohida jadval yarating. `city_services.kind` ga `fuel`, `ev_charging` qo'shing. `events` va `fx_rates` jadvallarini yarating.
2. **Tez natija beradiganlarni ulang (1-hafta).** CBU kursini kunlik cron bilan oling. SOS raqamlarini har bir shaharning `emergency` maydoniga yozing. OpenWeatherMap ni backend orqali keshlab ulang.
3. **OSM importini yozing (2–3-hafta).** Go da Geofabrik PBF ni o'qing (masalan osmium yoki osm2pgsql bilan). Turistik joylar, restoran, kafe, mehmonxona, shifoxona, dorixona, bankomat, yoqilg'i shoxobchalarini `pending` holatida yuklang. Ommaviy Overpass serverlariga tayanmang, ular tadqiqot paytida doim timeout bergan. Ilova ekranida "© OpenStreetMap contributors" yozuvini qo'shing.
4. **Obidalarni Wikidata bilan boyiting (3-hafta).** 8,260 ta meros obyektini OSM `wikidata` tegi va koordinata orqali bog'lang. Commons rasmlarini faqat CC BY / CC BY-SA / CC0 litsenziyalari bilan oling va `images[].credit` ga muallifni yozing.
5. **Admin panelda tekshiring (doimiy).** Samarqand, Buxoro, Xiva va Toshkentdagi eng muhim 100–200 joyni muharrir qo'lda tasdiqlasin. Ish vaqti, kirish narxi, `price_level` va qoidalarni to'ldirsin. Qolgan joylar `pending` da qolsin.
6. **Deep linklarni qo'shing (3–4-hafta).** Yandex Navigator, 2GIS, Google Maps, Yandex Go, eticket.railway.uz, avtoticket.uz, iTicket uchun havola shablonlarini kiriting. Travelpayouts da ro'yxatdan o'ting. Ro'yxatdan oldin mobil ilova va O'zbekiston trafigi ruxsat etilishini so'rang.
7. **Xaritani ulang (4-hafta).** Expo da development build qilib, MapLibre RN + OpenFreeMap ni ulang. Marshrut ko'rinishi uchun Mapbox yoki Google bepul limitidan foydalaning.
8. **Transport narxlarini admin panelga kiriting (4-hafta).** Toshkent, Samarqand, Afrosiyob, Yandex Go tariflarini "amal qilish sanasi" bilan saqlang. Har chorakda tekshirish eslatmasini qo'ying.
9. **Hamkorlik tarmog'ini boshlang (5–8-hafta).** 10–30 ta bloger va Telegram kanal bilan yozma kelishuv tuzing: kim, qanday huquq, qanday haq. Hamkorlar Telegram botni kanalga admin qiladi yoki postlarni forward qiladi. Faqat joy faktlari va asl postga havolani saqlang. Rasmni litsenziyasiz qayta joylamang.
10. **AI ajratish quvurini yozing (6–8-hafta).** Hamkor posti → LLM → qat'iy JSON → geokodlash → OSM bilan dublikat tekshiruvi → `pending` → muharrir → `published`. Telegram ma'lumoti bilan model o'qitmang.
11. **Rasmiy so'rovlar yuboring (parallel).** Turizm qo'mitasiga reyestrlar eksportini so'rang. Energetika vazirligiga Avto24/U-Gas ma'lumoti bo'yicha murojaat qiling. Toshshahartransxizmat yoki 2GIS bilan transport ma'lumoti haqida gaplashing. iTicket.uz bilan komissiya bo'yicha muzokara qiling.
12. **Huquqiy tekshiruv (ishga tushirishdan oldin).** Mahalliy yurist bilan 1125-son qonun, so'mda narx ko'rsatish va maxfiylik siyosatini tekshiring. Telegram bot uchun maxfiylik siyosatini @BotFather da ko'rsating.
13. **2-bosqich (foydalanuvchilar paydo bo'lgach).** Google Places ni faqat foydalanuvchi joyni ochganda jonli chaqiring va faqat `place_id` ni saqlang. Tripadvisor reytinglari, Agoda API, Instagram Login orqali biznes ulanishi va offline xarita (PMTiles) shu bosqichda qo'shiladi.

## Diqqat: tekshirilmagan / xavfli

**Xavfli, qilmang:**

| Nima | Nega xavfli |
|---|---|
| Instagram yoki Telegram ni scraping qilish (Apify, userbot, soxta akkaunt) | Meta va Telegram qoidalarida taqiqlangan ([Meta](https://developers.facebook.com/docs/development/terms-and-policies/automated-data-collection/); [Telegram Bot ToS](https://telegram.org/tos/bot-developers?setln=en)). Akkaunt va API bloklanadi. Soxta akkaunt bilan scraping hiQ ishida $500k ga tushgan. Rasmlar mualliflik huquqida, yuzlar shaxsiy ma'lumot |
| Google Places ma'lumotini bazaga yozish yoki seed qilish | Taqiqlangan, faqat `place_id` saqlanadi ([Google policies](https://developers.google.com/maps/documentation/places/web-service/policies)) |
| Booking.com, Airbnb, 2GIS, yetkazib berish ilovalarini scraping qilish | Foydalanish shartlariga zid |
| Yandex MapKit asosida ilova ichida navigatsiya qurish | MapKit shartlari §2.4 da taqiqlangan ([Yandex terms](https://yandex.com/dev/tariffs/doc/en/mapkit/terms/index.md)) |
| Reklama/obunali ilovada Open-Meteo bepul tarifi | Faqat notijoriy foydalanish uchun ([Open-Meteo Terms](https://open-meteo.com/en/terms)) |
| Narxni faqat dollarda ko'rsatish | Qonunga zid ([Kun.uz](https://kun.uz/en/news/2019/10/25/linking-prices-to-foreign-currency-and-contingent-units-banned-in-uzbekistan)) |
| Norasmiy chipta sotuvchilarga havola | Soxta chipta xavfi ([Daryo](https://daryo.uz/en/2023/11/27/ticket-scam-unleashes-chaos-at-tashkents-humo-arena-during-jah-khalibs-concert/)) |
| OSM asosidagi boyitilgan bazani ochiq tarqatish | Butun baza ODbL ostiga tushishi mumkin |

**Tekshirilmagan, qaror qabul qilishdan oldin aniqlang:**

| Nima | Holat |
|---|---|
| Travelpayouts Booking.com: mobil ilova va O'zbekiston trafigiga ruxsat | Shartlarda "faqat kontent saytlar" va "ruxsat etilgan davlatlar" deyilgan. Ro'yxat ko'rilmagan |
| Komissiya foizlari | Booking: sarlavhada 5%, shartlarda maksimum 4%. Agoda: 4.8–6%. Hostelworld: 18–40%. Manbalar bir-biriga zid |
| 2GIS narxi | 6,700 ₽ = 10k birlikmi yoki 100k birlikmi, ikki o'qish bor. Ma'lumot saqlash shartlari topilmadi |
| Foursquare bepul limiti | 500 yoki 10,000 bepul chaqiruv, manbalar zid |
| Google, Foursquare, Overture, Agoda, Hostelworld ning O'zbekistondagi qamrovi | O'lchanmagan. Samarqand, Buxoro, Xivada qo'lda tekshiring |
| OSM ning viloyat shaharlaridagi qamrovi va yoqilg'i shoxobchalari soni | Overpass timeout berdi. Geofabrik faylidan lokal hisoblang |
| data.egov.uz va Turizm qo'mitasi reyestrlari litsenziyasi | Tijoriy qayta foydalanish shartlari topilmadi |
| CBU JSON ning sana bo'yicha filtrlangan variantlari va foydalanish shartlari | Faqat asosiy endpoint tekshirildi |
| Yandex Navigator deep link uchun imzolangan kalit talabi | Faqat uchinchi tomon manbasida aytilgan |
| Yandex Go tariflari | Rasmiy sahifa emas, Voyage.uz o'qishi |
| Yandex Trip info API ning O'zbekistonda ishlashi | Kalit bilan sinab ko'rilmagan |
| Meta Platform Terms dagi saqlash va kesh qoidalari | O'qilmagan |
| Telegram "Content Licensing and AI Scraping" shartlari | O'qilmagan. Hamkor roziligi bilan LLM ishlatish ham "kulrang zona" |
| 1125-son qonun matni va "abonent ma'lumotlari" doirasi | lex.uz da to'liq o'qilmagan. Yurist kerak |
| Chet elliklar uchun dollar ekvivalentini ko'rsatish ruxsati | Yuridik tasdiq kerak |
| Ro'yxatdan o'tishdagi 3 kunga dam olish kunlari kiradimi | GOV.UK va O'zbekiston manbalari bir-biriga zid |
| OpenWeatherMap atribusiya talabi, Open-Meteo tijoriy narxi | Topilmadi |
| Uzum Tezkor, Yandex Eats, Express24 ning hamkor dasturlari | Topilmadi |
| Bloger litsenziyasining O'zbekistondagi odatiy narxi | Ma'lumot yo'q |

## Xulosa

Asosiy xulosa shu: Yo'ldosh AI ning raqobatdagi ustunligi API dan emas, **tahrir qilingan va rozilik bilan yig'ilgan mahalliy ma'lumotdan** keladi. Ochiq manbalar (OSM, Wikidata) faqat "skelet" beradi: nom va koordinata. Google va Yandex boyroq, lekin ularning ma'lumotini saqlab bo'lmaydi. Demak, o'z bazasini qurgan har kim ularga qaram bo'lib qolmaydi. Taom narxlari, yashirin joylar va tadbirlar hech qayerda API sifatida yo'q. Aynan shu bo'shliq imkoniyat. Admin panel va `pending → published` oqimi allaqachon bor. Endi unga hamkor Telegram boti va AI ajratish quvurini ulash kerak. Shunda ilova "Google Maps dan nusxa" emas, alohida mahsulot bo'ladi.

Ikkinchi xulosa: MVP da navigatsiya, taksi, turar joy va chiptalarni **o'zimiz qurmasdan, deep link bilan topshirish** to'g'ri qaror. Bu bepul. Bundan tashqari, Toshkentda Yandex ning tirbandlik va kamera ma'lumotlaridan yaxshirog'ini o'zimiz qura olmaymiz. Pul va kuchni hamkorlik tarmog'iga va muharrirlarga sarflang. Pullik API larni (Google Places, Agoda, 2GIS) faqat foydalanuvchilar paydo bo'lgach, aniq so'rovlar soni ma'lum bo'lganda yoqing. O'zbekistonda narxlar va qoidalar tez o'zgaradi: transport tariflari 2026-yilda ikki marta, to'lov va shaxsiy ma'lumot qoidalari bir yilda tubdan o'zgardi. Shuning uchun har bir ma'lumot `source`, `verified_at` va "amal qilish sanasi" bilan saqlansin.
