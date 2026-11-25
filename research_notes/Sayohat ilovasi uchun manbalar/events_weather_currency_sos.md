# Data sources for events, weather, currency, SOS, open data and legal aspects (Uzbekistan travel app)

Research date: 2026-11-25. Anything marked [UNVERIFIED] comes from background knowledge and was not confirmed by a fetched source in this session.

## (a) Events: festivals, concerts, ticketing platforms, official calendars

### Takeaway
No official machine-readable event calendar or ticketing API exists for Uzbekistan. uzbekistan.travel has an HTML events page that links out to afisha.uz and a Telegram channel. iTicket.uz is the main ticket seller for large concerts, and Ticketon (Kazakhstan) also sells in Tashkent. Neither publishes a public API or affiliate program, so event data has to be curated by hand, scraped (legal risk), or obtained through partnership deals.

### Cited Findings
- uzbekistan.travel's events page (https://uzbekistan.travel/en/o/events-festivals/) is HTML only, with no calendar, RSS feed or API. It lists, for example, a John Legend concert (June 8, Registan, Samarkand), "The World of Hans Zimmer" (March 29, Alisher Navoi Cinema Palace), Muynak Rally 2026 (May 27-28, 2026) and a Van Gogh multimedia exhibition (until June 9, 2026, Tashkent City Mall). It links to afisha.uz ("MORE EVENTS"), a regional "Catalogue of entertainment events" for April-June 2026, and Telegram t.me/uzbekistan_travel. — [uzbekistan.travel](https://uzbekistan.travel/en/o/events-festivals/)
- Boysun Bahori 2026: a Tourism Committee news item (April 20, 2026) says the festival is returning, with more than 100 folklore groups from over 40 countries. Advantour lists it for May 1-7, 2026 in Boysun, Surxondaryo. — [gov.uz/uzbektourism](https://gov.uz/en/uzbektourism/news/view/155666); [Advantour](https://www.advantour.com/uzbekistan/festivals.htm)
- Sharq Taronalari is held every two years in Samarkand. A government-linked document of planned international cultural events lists it for August 2026, and Eurasia.Travel gives late August in general. No confirmed 2026 day range was found. — [Advantour](https://www.advantour.com/uzbekistan/festivals.htm); [Eurasia.Travel](https://eurasia.travel/uzbekistan/holidays-festivals/)
- Silk and Spices (Bukhara): no confirmed 2026 dates. Older listings and the Eurasia.Travel 2025 guide give June 6-8. — [Eurasia.Travel 2025](https://eurasia.travel/?p=43685); [Advantour](https://www.advantour.com/uzbekistan/festivals.htm)
- Other 2026 dates from Advantour (an aggregator, not an official source): International Jazz Festival, April 28 to May 12 (Tashkent); Dishes of the East, March 23-24 (Samarkand). — [Advantour](https://www.advantour.com/uzbekistan/festivals.htm)
- iTicket.uz was the official ticket operator for Jennifer Lopez's August 2025 concert in Tashkent. — [gov.uz/uzbektourism](https://gov.uz/en/uzbektourism/news/view/60460); [Kursiv](https://uz.kursiv.media/en/2025-06-26/j-lo-to-perform-in-astana-tashkent-and-almaty-where-are-tickets-more-expensive/)
- The legal entity behind iTicket.uz is "ITICKET INGURUZ" LLC. It runs a promoter sales-report app ("iReports Uzbekistan"), so a B2B promoter channel exists, but no public API or affiliate program was found. — [App Store: iReports](https://apps.apple.com/us/app/id1521266484); [App Store: iTicket.UZ](https://apps.apple.com/app/id1460557783)
- iTicket.uz has run co-marketing promotions with Mastercard (rules PDF names iTicket.uz as the organizer). — [Mastercard rules PDF](https://www.mastercard.com/content/dam/mccom/eeu/uzbekistan/for-you/documents/rules-iticket-ru.pdf)
- Ticketon (Kazakhstan) opened a representative office in Uzbekistan in 2018 and sells Tashkent events under ticketon.kz/en/tashkent/... — [UzDaily](https://uzdaily.ru/en/ticketon-opens-its-representative-office-in-uzbekistan); [Ticketon](https://ticketon.kz/en/tashkent/event/original-enigma-voices-uz)
- Afisha.uz runs an events listing (concerts at https://www.afisha.uz/concerts/). — [afisha.uz](https://www.afisha.uz/concerts/)
- Fake tickets are a real problem: the 2023 Jah Khalib concert at Humo Arena descended into chaos because of a ticket scam, and authorities told buyers to use only official sellers. — [Daryo](https://daryo.uz/en/2023/11/27/ticket-scam-unleashes-chaos-at-tashkents-humo-arena-during-jah-khalibs-concert/)

### Inferences
- The most practical approach is a curated in-app event database, seeded from uzbekistan.travel, the gov.uz/uzbektourism news feed, afisha.uz and organizers. For ticket links, use deep links to iTicket.uz or Ticketon pages rather than an API, and negotiate a referral or commission deal directly with ITICKET INGURUZ LLC.
- Navruz is fixed on March 21 (a public holiday), so it needs no source feed. The Melon festival and other regional festivals vary each year and must be checked annually.
- Because of the fake-ticket history, the app should link only to official sellers.

### Gaps
- Exact 2026 dates for Silk and Spices and Sharq Taronalari, and the 2027 calendar (Sharq Taronalari next in 2028 if the biennial cycle holds).
- Melon festival (Qovun sayli) dates were not found.
- Whether iTicket.uz, Ticketon or afisha.uz offer an API, affiliate program or commission terms; they must be contacted directly.
- Whether a "Kassir"-type platform operates in Uzbekistan was not confirmed.

## (b) Weather: Open-Meteo, OpenWeatherMap, Uzhydromet

### Takeaway
Open-Meteo's free tier is non-commercial only, so a commercial app (subscriptions or ads) needs a paid Open-Meteo plan, whose prices are not published on the pricing page. OpenWeatherMap's free tier (60 calls/min, 1M calls/month) and its One Call API (1,000 free calls/day, then 0.0012 GBP per call) are cheaper to plan around. Uzhydromet offers no public API.

### Cited Findings
- Open-Meteo free/open-access tier: under 10,000 calls/day, 5,000/hour, 600/minute and 300,000/month, for non-commercial use only. Apps with subscriptions or advertising count as commercial. Data is licensed CC BY 4.0. — [Open-Meteo Terms](https://open-meteo.com/en/terms)
- Open-Meteo paid plans: API Standard (1M calls/month), Professional (5M/month) and Enterprise (more than 50M/month) all allow commercial use. Standard does not include the historical, climate, ensemble and satellite APIs; Professional does. Prices are described only as a "fixed monthly price" and are not shown (customer portal: dashboard.open-meteo.com). Self-hosting the open-source server is suggested for very large volumes. — [Open-Meteo Pricing](https://open-meteo.com/en/pricing)
- OpenWeatherMap: the free plan gives 60 calls/min and 1,000,000 calls/month. The One Call API gives 1,000 calls/day free, then 0.0012 GBP per call. Monthly plans: Startup 30 GBP (600/min, 10M/month), Developer 140 GBP (3,000/min, 100M), Professional 370 GBP (30,000/min, 1B), Expert 1,200 GBP (100,000/min, 3B). The license is listed as ODbL. — [OpenWeatherMap Pricing](https://openweathermap.org/price)
- Uzhydromet (hydromet.uz, gov.uz/hydromet) offers real-time weather, 3-day forecasts and agro-meteorological bulletins on its website, and archive data on request. FAO describes the site as free. — [FAO Agritech Observatory](https://agritechobservatory.review.fao.org/ru/hydrometuz)
- A Single Weather Observation Portal at data.meteo.uz provides automatic-station data and forecasts, with login through a unified government ID. No API is documented. — [dgov.uz](https://dgov.uz/en/solution-detail/27)
- An older Uzhydromet report says meteorological data is free for a listed set of government bodies and paid for everyone else. — [JMA country report PDF](https://www.jma.go.jp/jma/en/Activities/qmws_2010/CountryReport/CS405_Merkushkin_Aleksandr(Uzbekistan).pdf)

### Inferences
- MVP: Open-Meteo free tier only while the app is non-commercial (no ads or subscriptions), then upgrade to API Standard, or use OpenWeatherMap's free or One Call tier from the start. Show CC BY 4.0 attribution ("Weather data by Open-Meteo.com") either way. The exact attribution wording is [UNVERIFIED]; check open-meteo.com/en/licence.
- Uzhydromet is the official source for severe-weather warnings, but it has no API, so integration would need a formal agreement.

### Gaps
- Open-Meteo commercial plan prices (not on the public page; background knowledge suggests roughly EUR 29+/month for Standard [UNVERIFIED]).
- OpenWeatherMap attribution requirements under ODbL were not confirmed.
- Whether data.meteo.uz offers API access or an official severe-weather alert feed.

## (c) Currency: CBU official rates, bank rates, cards, ATMs

### Takeaway
The Central Bank's JSON endpoint is live and verified: `https://cbu.uz/uz/arkhiv-kursov-valyut/json/` returns an array of all currencies, updated daily, with no key required. In card payments, Apple Pay and Google Pay launched in Uzbekistan on October 27, 2026, and Uzcard/Humo support in Apple Pay began in a limited first phase on November 17, 2026.

### Cited Findings
- `GET https://cbu.uz/uz/arkhiv-kursov-valyut/json/` returns a JSON array. Fields: `id`, `Code` (ISO numeric), `Ccy` (ISO alpha), `CcyNm_RU`, `CcyNm_UZ`, `CcyNm_UZC`, `CcyNm_EN`, `Nominal`, `Rate`, `Diff`, `Date` (DD.MM.YYYY). Rates and nominals are strings. The fetch on 2026-11-25 returned USD 11990.54 (Diff -12.39), EUR 13136.84 and RUB 146.24, all dated 25.11.2026. No authentication is required. — [CBU JSON endpoint](https://cbu.uz/uz/arkhiv-kursov-valyut/json/)
- [UNVERIFIED, background knowledge] Filtered variants follow the same pattern: `/uz/arkhiv-kursov-valyut/json/{CCY}/{YYYY-MM-DD}/` and `/uz/arkhiv-kursov-valyut/json/all/{YYYY-MM-DD}/`. Test them before relying on them. Other language prefixes (`/ru/`, `/en/`) are assumed to work.
- Apple Pay and Google Pay began rolling out on October 27, 2026 for eligible Visa and Mastercard cards issued by local banks. Some banks are still testing. — [Times of Central Asia](https://timesca.com/apple-pay-and-google-pay-uzbekistan/); [Euronews](https://www.euronews.com/2026/10/30/apple-pay-and-google-pay-launch-in-uzbekistan-amid-wider-fintech-reforms)
- Apple's list names eight local banks for Visa/Mastercard; Uzum Bank and Hamkorbank support Visa only. — [Kun.uz](https://kun.uz/en/29855597)
- On November 17, 2026 the CBU announced that Uzcard and Humo cards can be added to Apple Pay in an initial phase: Uzcard from Xalq Bank and Humo from Octobank only. — [CBU press release](https://cbu.uz/en/press_center/releases/4929729/); [Spot.uz](https://www.spot.uz/en/2026/11/17/apple-pay-cards/)
- Alipay+ went live on Humo in August 2026, and a UPI link for Indian visitors is planned. The Tourism Committee says the aim is for visitors (especially from China and India) to use familiar payment methods. — [gov.uz/uzbektourism](https://gov.uz/en/uzbektourism/news/view/236472)
- Foreign tourists with Apple Pay or Google Pay have been able to pay at Humo terminals since 2019. Before 2026, the data localization law was the stated obstacle to a domestic launch. — [Kun.uz 2024](https://kun.uz/en/news/2024/12/26/uzbekistans-data-localization-law-hinders-entry-of-apple-pay-and-google-pay)
- Russian-issued Visa and Mastercard cards do not work in Uzbekistan; carry a foreign card and cash (single travel-blog source). — [travelto.ru](https://travelto.ru/blog/kak-platit-v-uzbekistane/)

### Inferences
- Cache the CBU feed once a day (the CBU sets rates daily; the exact publish time was not verified). Label it as the "official CBU rate," not a buy/sell rate.
- Commercial bank buy/sell rates must come from bank websites or aggregators (for example bank.uz or kurs.uz [UNVERIFIED]); none was verified to have an official API.
- Card guidance in the app should say: Visa and Mastercard are accepted at many terminals, local cards are Uzcard and Humo, and contactless phone payment has become widely available since late October 2026.

### Gaps
- Commercial bank rate aggregators: names, APIs and terms were not researched successfully.
- ATM location datasets: no open dataset found. Options are bank websites, OpenStreetMap (`amenity=atm`, ODbL) or Google/Yandex Places (paid).
- The CBU's daily update time and any terms of use for the JSON feed.

## (d) SOS: emergency numbers, tourist police, hospitals, embassies, pharmacies

### Takeaway
Verified numbers: 112 (unified, nationwide since March 2025), 101 fire, 102 police, 103 ambulance, 104 gas emergency, 1050 Ministry of Emergency Situations, and 1173 tourist hotline (24/7, English, Uzbek and Russian). The UK FCDO confirms the old numbers still work alongside 112.

### Cited Findings
- 112 timeline: in July 2023 the President ordered the Interior Ministry and the Digital Technologies Ministry to set up the service. A Cabinet resolution in May 2024 laid out a phased rollout, starting with Tashkent city and region on June 1, 2024. On March 25, 2025 the system was reported fully operational nationwide, ahead of the end-2024 target that had slipped. Integrated services: Interior Ministry, Ministry of Emergency Situations, Ministry of Health, National Agency for Social Protection, and Hududgaztaminot JSC. — [Kun.uz 25.03.2025](https://kun.uz/en/news/2025/03/25/uzbekistans-unified-112-emergency-system-now-fully-operational-nationwide); [Kun.uz 31.05.2024](https://kun.uz/en/news/2024/05/31/unified-dispatch-service-112-being-launched-in-uzbekistan); [gov.uz (government decision)](https://gov.uz/oz/digital/news/view/12837)
- Xs.uz dates the nationwide launch announcement to March 27, 2025 (Kun.uz says March 25). — [Xs.uz](https://xs.uz/en/post/112-unified-dispatch-service-now-fully-operational-across-uzbekistan)
- UK FCDO: ambulance 103, fire 101, police 102. The unified number 112 was introduced in 2024 to cover all emergency calls, and the previous numbers "are still in operation." The Call Centre for Safe Tourism is on 1173, 24/7, with operators speaking English, Uzbek and Russian. — [GOV.UK](https://www.gov.uk/foreign-travel-advice/uzbekistan/getting-help)
- 104 is the emergency gas dispatch service (Uzbek government life-situations page, Russian). — [my.gov.uz](https://oldmy.gov.uz/ru/life-situations/14)
- 1050 reaches the Ministry of Emergency Situations (US OSAC). — [OSAC](https://www.osac.gov/Content/Report/b6b69dde-af36-471e-a427-1c2097fde1f4)
- The Interior Ministry's contact page lists all its numbers, including 1102 for complaints about police misconduct. — [gov.uz/iiv contacts](https://gov.uz/en/iiv/pages/contacts)
- The 1173 tourist call centre was launched in February 2021; operators can transfer calls to the ambulance service. — [Kursiv 2021](https://uz.kursiv.media/2021-02-05/v-uzbekistane-zapustili-call-centr-dlya-turistov/); [Advantour](https://www.advantour.com/uzbekistan/medical-care.htm)
- The Tourism Committee publishes a "Register of medical and health institutions" and a "Register of accommodation facilities" on gov.uz/uzbektourism. — [gov.uz/uzbektourism registers](https://gov.uz/en/uzbektourism/sections/turistik-yo-nalish-marshrut-dagi-obyektlarining-yagona-reyestri)
- Lists of foreign embassies in Tashkent are published by the embassies' own governments (for example Canada). The mfa.uz list was not fetched. — [travel.gc.ca](https://travel.gc.ca/assistance/embassies-consulates/uzbekistan)

### Inferences
- In the SOS screen, make 112 the main button and list 101, 102, 103, 104, 1050 and 1173 as secondary. Show 1173 prominently for foreigners because its operators speak English.
- For clinics that accept foreigners, combine the Tourism Committee medical register, embassy-published lists of doctors (US and UK embassies publish these), and OSM/Google Places. The data is not available in a structured form.

### Gaps
- No official "tourist police" short number was found apart from the 1173 tourist call centre. The Tourism Security Department of the Interior Ministry exists, but no number was verified.
- No structured dataset of hospitals and clinics that accept foreigners.
- The mfa.uz page listing diplomatic missions was not fetched (URL [UNVERIFIED]: mfa.uz/en/pages/diplomatic-missions).
- Pharmacy data: no open dataset confirmed (see section e).

## (e) data.egov.uz open data portal

### Takeaway
data.egov.uz publishes each dataset in XML, CSV, JSON, XLS and RDF, plus an auto-generated JSON web-service (API) endpoint that requires registering on id.egov.uz and generating a token. It has Tourism/sport and Healthcare categories, but no pharmacy dataset was confirmed, and the license terms were not found.

### Cited Findings
- Each dataset page has Passport, Structure, Table, Web service (API) and Versions sections. Tables download as XML, CSV, JSON, XLS or RDF; structure as JSON or CSV. The Web service section gives a JSON API URL for integration. — [data.egov.uz user guide (PDF)](https://data.egov.uz/user_en.pdf)
- API access requires an account on the unified identification system (id.egov.uz) / auth.egov.uz and generating a token. — [data.egov.uz user guide](https://data.egov.uz/user_en.pdf)
- The legacy portal (olddata.gov.uz / data.gov.uz) documented `/api/v1/json/` endpoints (list, dataset, version, structure, organization, category). These may be outdated. — [olddata.gov.uz docs](https://olddata.gov.uz/en/docs/do/view/usage)
- The data.egov.uz catalog has spheres such as Tourism and sport and Healthcare, filterable by organization (example: https://data.egov.uz/eng/spheres/607feffa7b6428eee08802b7?orgId=84). — [data.egov.uz sphere page](https://data.egov.uz/eng/spheres/607feffa7b6428eee08802b7?orgId=84)
- The legacy portal listed a "Hotels" dataset from the State Statistics Committee. The Tourism Committee keeps its own registers of accommodation facilities, tourist-route objects, and medical and health institutions on gov.uz/uzbektourism. — [olddata.gov.uz sphere 7](https://olddata.gov.uz/en/sphere/7); [gov.uz/uzbektourism](https://gov.uz/en/uzbektourism/sections/turistik-yo-nalish-marshrut-dagi-obyektlarining-yagona-reyestri)
- The Tourism Committee also publishes statistics and "socially significant data" sections. — [gov.uz/uzbektourism statistics](https://gov.uz/en/uzbektourism/sections/statistika)

### Inferences
- Datasets are often static XLS/CSV snapshots with irregular updates. Plan for ETL with deduplication and geocoding, since coordinates are often missing.
- Tourism Committee registers (hotels, guesthouses, guides, tour operators) are likely more current for tourism than data.egov.uz copies.

### Gaps
- The license of data.egov.uz data (whether it allows commercial reuse and requires attribution) was not found. Check the portal's terms page or the regulation on open data.
- A confirmed list of current datasets for pharmacies (dorixona), medical institutions with coordinates, and transport (bus routes, GTFS) was not obtained. Search the catalog in Uzbek directly.
- Exact current API endpoint syntax and rate limits.

## (f) Legal aspects: personal data, tourism licensing, foreigner registration (e-mehmon), price display

### Takeaway
(1) Law No. ZRU-1125 of March 26, 2026 narrowed the 2021 rule that all citizens' personal data be stored in Uzbekistan. Localization now covers a narrower set of categories (reported as biometric, genetic and telecom data), and transfers abroad are allowed to "adequate" countries (49 approved in July/August 2026). (2) Tourism licensing was replaced by notification for most activities in 2024-2025. (3) Foreigners must be registered within 3 days, by the hotel or by a private host via e-mehmon, and online registration carries a per-day tourist tax. (4) Prices must be shown in soums; showing prices in foreign currency in ads is illegal.

### Cited Findings
- Personal data: under the 2021 rule, all personal data of Uzbek citizens had to be stored in Uzbekistan. Law No. 1125 (signed March 26, 2026) limits localization to a narrower set of categories and allows cross-border processing under conditions such as an adequacy list kept by the Cabinet of Ministers. The reform was driven by the need to let international payment systems enter the market. — [Kun.uz 28.03.2026](https://kun.uz/en/news/2026/03/28/uzbekistan-amends-personal-data-law-to-facilitate-global-payment-systems); [Daryo 29.03.2026](https://daryo.uz/en/2026/03/29/uzbekistan-amends-personal-data-law-for-domestic-storage-and-regulated-foreign-processing/); [Legal500](https://www.legal500.com/developments/thought-leadership/legislative-updates-in-the-field-of-artificial-intelligence-and-personal-data-regulation-uzbekistan/)
- A government resolution (July 29, 2026) approved 49 countries for simplified personal data transfers. — [Kun.uz 04.08.2026](https://kun.uz/en/news/2026/08/04/uzbekistan-approves-49-countries-for-simplified-personal-data-transfers)
- In July 2025 experts called for revising the localization rules because they hindered innovation. — [Kun.uz 22.07.2025](https://kun.uz/en/news/2025/07/22/experts-urge-uzbekistan-to-revise-data-localization-rules-to-support-innovation)
- Tourism licensing: the 2019 Law "On Tourism" (signed July 2019) required licenses for tour operators and travel agents, and certificates for guides. A presidential decree abolished the tourism activity license from March 1, 2024 and introduced notification. In April 2025 the Legislative Chamber passed a law moving travel agency activity from licensing to notification; at that point Senate approval, the President's signature and publication were still pending. — [Kun.uz 2019](https://kun.uz/en/news/2019/07/20/shavkat-mirziyoyev-signs-bill-on-tourism-into-law); [Kun.uz 30.04.2025](https://kun.uz/en/news/2025/04/30/no-more-licenses-travel-agencies-in-uzbekistan-to-operate-under-new-rules); [gov.uz/uzbektourism licensing](https://gov.uz/en/uzbektourism/pages/litsenziyalash)
- Umrah/hajj trip organizers need a separate license (2024). — [Kun.uz 26.03.2024](https://kun.uz/en/news/2024/03/26/companies-providing-services-for-organizing-umrah-pilgrimage-events-to-be-licensed-separately)
- Foreigner registration: register within 3 days of arrival. Hotels register their guests. Private hosts register guests at the district office or through the e-mehmon system. Campers register themselves on e-mehmon, and online registration requires paying the tourist tax for each day. The UK says weekends count toward the 3 days; Uzbek legal sources (Cabinet of Ministers Resolution No. 593 of 28.09.2020) say Sundays and holidays are excluded. This conflict is unresolved. — [GOV.UK entry requirements](https://www.gov.uk/foreign-travel-advice/uzbekistan/entry-requirements); [advice.uz / adliya](https://gendermadad.adliya.uz/en/document/1827); [Kun.uz 2023](https://kun.uz/en/news/2023/02/01/uzbekistan-simplifies-requirements-for-temporary-registration-of-foreigners)
- Breaking registration rules is fined under Article 225 of the Administrative Code. — [advice.uz](https://advice.uz/en/document/1827)
- A draft migration law would increase liability for failing to enter foreigners' data into E-mehmon and align legislation with the registration law of July 10, 2025. Its enactment status is unconfirmed. — [gov.uz/migration](https://gov.uz/en/migration/news/view/199572)
- Prices: Law "On Advertising" No. ZRU-776 (signed June 7, 2022, in force September 2022) bans prices or tariffs in foreign currency in ads. The Currency Regulation Law requires prices for goods and services in Uzbekistan, including on e-commerce platforms, to be set only in national currency, and bans linking prices to foreign currency or "conventional units" (since 2019). The Competition Committee treats foreign-currency prices in ads as a violation. — [Kun.uz 2022](https://kun.uz/en/news/2022/09/09/law-on-advertising-comes-into-force-basic-rules-and-restrictions-announced); [Kun.uz 2019](https://kun.uz/en/news/2019/10/25/linking-prices-to-foreign-currency-and-contingent-units-banned-in-uzbekistan); [anhor.uz](https://anhor.uz/news/ads/)

### Inferences
- Showing prices to Uzbek users: give the soum (UZS) price as the primary one. A USD or EUR conversion may be shown as informational, labeled with the CBU rate and its date. Whether that conversion is allowed for a foreign-facing app needs legal confirmation.
- If the app stores Uzbek citizens' data, hosting in Uzbekistan is the safe choice until the exact list of localized categories under ZRU-1125 is checked on lex.uz. Data of foreign tourists was never covered by the citizen localization rule (as the rule is described).
- An app that only gives information or refers users to bookings probably does not need a tourism notification. If it sells tour packages, it likely needs to file a tour operator or travel agent notification.
- E-mehmon: the app can remind users who stay in private accommodation that the host must register them within 3 days. The app cannot register guests itself unless it integrates with e-mehmon as an accommodation provider.

### Gaps
- The full text and categories of ZRU-1125 on lex.uz were not fetched. Daryo returned 403.
- Final adoption date of the 2025 notification law for travel agencies.
- Whether the July 10, 2025 registration law or the migration amendments changed the 3-day rule or the e-mehmon process.
- Fine amounts for showing prices in foreign currency.
- Current tourist tax rates per day.
