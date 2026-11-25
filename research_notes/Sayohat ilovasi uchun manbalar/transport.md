# Transport data sources for an Uzbekistan travel-assistant app (city and intercity, with prices) — notes as of 2026-11-25

## 1. GTFS / open transit data for Tashkent and other Uzbek cities; transit apps; metro data

### Takeaway
I found no public GTFS feed for Tashkent or any other Uzbek city on Mobility Database, Transitland or government sites. Live bus data goes to 2GIS (exclusive data partnership since 2020) and Yandex Maps. For an app, the realistic options are: deep-link to Yandex Maps or 2GIS, use 2GIS's commercial Public Transport API, or build your own data from OSM or Wikiroutes (check licensing).

### Cited Findings
- Searches of Mobility Database and Transitland turned up no GTFS feed for Tashkent or Uzbekistan — [search result set](https://www.gtssacademy.org/explore/country/Uzbekistan/Uzbekistan+-+Tashkent/survey-data/) (irrelevant hits only; this is a negative finding).
- A World Bank publication (Jan 2026), written at the Government of Uzbekistan's request, on a roadmap for Tashkent public transport links to a "Data Set". I could not confirm whether it is GTFS — [World Bank OKR](https://openknowledge.worldbank.org/entities/publication/52649dc7-954c-4bfe-9c1c-a86cfeff9d67)
- Toshshahartransxizmat (the Tashkent city bus operator) runs 165 routes with about 1,700 buses on the road daily — [Tashkent Times](https://www.tashkenttimes.uz/national/12400-tashkent-public-bus-journeys-increase-by-1-5-times-in-2023)
- 2020: Toshshahartransxizmat signed an agreement making 2GIS its exclusive partner for public transport monitoring data (route diagrams, timetables, actual vehicle positions) — [Podrobno.uz](https://podrobno.uz/cat/tehnp/toshshakhartranskhizmat-i-2gis-podpisali-soglashenie-o-monitoringe-obshchestvennogo-transporta-/); [Spot.uz 2020](https://www.spot.uz/ru/2020/08/18/2gis/)
- 2GIS has shown real-time Tashkent buses since 2019, updating within about 30 s — [Spot.uz 2019](https://www.spot.uz/ru/2019/11/27/2gis/). Minibus (marshrutka) tracking was added in Feb 2021 — [Spot.uz 2021](https://www.spot.uz/ru/2021/08/05/app/)
- 2GIS sells a developer Public Transport routing API (docs.2gis.com, navigation/public-transport) — [2GIS docs](https://docs.2gis.com/en/api/navigation/public-transport/examples). Commercial key; I did not check pricing or Uzbekistan coverage terms.
- Yandex Maps added real-time Tashkent bus tracking in Sept 2019, using route data from the Tashkent administration and Toshshahartransxizmat. It updates every 30 s and covers about 230 routes — [Kun.uz](https://kun.uz/en/news/2019/09/19/yandex-launches-real-time-bus-movement-tracking-service-in-tashkent)
- Real-time bus tracking is reported in 13 Uzbek cities — [UzDaily](https://www.uzdaily.uz/en/real-time-bus-tracking-now-available-in-13-cities-across-uzbekistan/) (snippet only; I did not check the date or which app).
- Wikiroutes.info lists Tashkent routes, stops and payment info — [Wikiroutes](https://wikiroutes.info/en/payment/236). Golden Pages also has bus route listings — [goldenpages.uz](https://www.goldenpages.uz/en/overal_t/bus_t/)
- A 2025 MDPI *Data* paper builds a GTFS dataset from raw GPS for 3 high-frequency routes. The snippet does not name the city — [IDEAS/RePEc](https://ideas.repec.org/a/gam/jdataj/v10y2025i8p119-d1707743.html)
- Tashkent Metro: 50 stations, about 71 km (Nov 2026). 21 stations were opened over the last 7 years — [Daryo, 2026-11-04](https://daryo.uz/en/2026/11/04/tashkent-metro-plans-to-introduce-electronic-displays-showing-train-arrival-times/). Wikipedia gives 4 lines, 70.4 km, 50 stations — [Wikipedia](https://en.wikipedia.org/wiki/Tashkent_Metro)
- Metro plans: 79 stations and 103 km by 2030. Trainsets to go from 40 to 87 by end-2026 — [Kun.uz 2026-05-14](https://kun.uz/en/news/2026/05/14/president-mirziyoyev-approves-major-expansion-of-tashkent-metro-through-2035). New 12 km Mingurik–Ippodrom line is in design — [Kun.uz 2026-08-24](https://kun.uz/en/news/2026/08/24/tashkent-to-build-12km-metro-line-from-mingurik-to-ippodrom-to-ease-pressure-on-chilonzor-line). Arrival-time displays are planned — [Daryo 2026-11-04](https://daryo.uz/en/2026/11/04/tashkent-metro-plans-to-introduce-electronic-displays-showing-train-arrival-times/)
- ATTO (the Tashkent transport card/app) added intercity bus ticket booking in Dec 2025 — [atto.uz](https://atto.uz/en/news/services)

### Inferences
- With no GTFS, the cheapest MVP is to deep-link "how do I get there" to Yandex Maps or 2GIS rather than run your own router. Metro stations (about 50) are few enough to curate by hand. OSM data is ODbL (attribution plus share-alike for derived databases). I did not check how complete the OSM transit mapping is for Tashkent.
- Because 2GIS has the exclusive live-data deal, real-time arrivals from Toshshahartransxizmat would most likely need a 2GIS commercial agreement or a direct deal with the operator.

### Gaps
- I did not verify OSM wiki or Overpass coverage of Tashkent metro and bus route relations.
- I did not open the World Bank dataset format.
- No GTFS was found for Samarkand or Bukhara. Their electronic-payment systems are presumably operator-run, but I found no developer API.
- No public developer API was found for ATTO.

## 2. Current fares (2025-2026): city transport, trains, taxis, intercity bus, shared taxi, domestic flights

### Takeaway
Tashkent bus and metro: 2,500 UZS electronic and 3,000 UZS cash since 1 Oct 2026. Samarkand: 2,000 UZS electronic and 4,000 UZS cash since 1 May 2026. Afrosiyob economy (from 1 Feb 2026): Tashkent–Samarkand 311,000 UZS, Tashkent–Bukhara 509,000 UZS. Yandex Go "Start" class in Tashkent: about 4,600 UZS minimum plus 1,050 UZS/km and 500 UZS/min (published caps).

### Cited Findings
**Tashkent metro and bus (from 1 Oct 2026, City Council decision of 25 Sep 2026)** — [UzDaily](https://www.uzdaily.uz/en/tashkent-approves-new-public-transport-fares-from-1-october/); [Mintrans gov.uz](https://gov.uz/ru/mintrans/news/view/225119); [Daryo](https://daryo.uz/en/2026/09/28/tashkent-to-raise-metro-and-bus-fares-for-card-users-from-october-1/)
- Electronic (card): 2,500 UZS, up from 1,700 (+47%). Cash: 3,000 UZS (unchanged). Baggage: 4,000 UZS.
- One-hour transfer chain: 1st trip 2,500. 2nd bus 1,500. 3rd and later buses free. Metro inside the chain 1,500. Metro can be used only once per 60-min window.
- Bus + metro passes: 13,500 (1 day), 73,000 (1 week), 345,000 (1 month), 945,000 (3 months), 1.62M (6 months), 2.7M (1 year). Bus-only passes: 11,500 / 54,000 / 250,000 / 682,500 / 1.17M / 1.95M.
- Trip packs: 10 trips 19,000; 20 trips 38,500; 30 trips 57,500.
- 50% off tariff plans for schoolchildren, students, pensioners and disabled people.
- Distance-based metro fares are planned from 1 Jan 2027, with rates not yet set — [Tashkent Times](https://www.tashkenttimes.uz/national/17692-distance-fares-commuter-parking-and-retail-spaces-set-for-metro-network)

**Samarkand city transport (from 1 May 2026)** — [Kun.uz](https://kun.uz/en/news/2026/05/01/samarkand-raises-public-transport-fares-plans-to-phase-out-cash)
- Bus, e-bus and tram: cash (QR ticket) 4,000 UZS (+1,000). Transport or bank card 2,000 UZS (unchanged). Mobile apps and infokiosks are accepted. Cash is to be abolished.

**Bukhara city transport**
- Cash is no longer accepted on the remaining routes from 1 Apr 2026 (electronic only). The notice gives no fare figure — [Mintrans](https://gov.uz/ru/mintrans/news/view/124612)

**Trains (Uzbekistan Railways, +15% from 1 Feb 2026)** — [Kun.uz 2026-01-16](https://kun.uz/en/news/2026/01/16/uzbekistan-railways-raises-train-fares-again-from-february-1); [Kursiv](https://uz.kursiv.media/en/2026-01-17/afrosiyob-ticket-prices-in-uzbekistan-to-rise-15-from-february-2026/)

| Afrosiyob route | Economy | Business | VIP |
|---|---|---|---|
| Tashkent–Samarkand | 311,000 | 455,000 | 709,000 |
| Tashkent–Karshi | 429,000 | 642,000 | 976,000 |
| Tashkent–Bukhara | 509,000 | 760,000 | 1,138,000 |

- Sharq, Nasaf and "Uzbekistan" trains also rose 15%, but the article gives no per-route figures.
- Fares also rose earlier: Jan 2025 — [Kun.uz](https://www.kun.uz/en/news/2025/01/06/uzbekistan-to-raise-train-ticket-prices-again-causing-public-concerns). In Nov 2025 the railway said actual costs are about 2x the fares — [Kun.uz](https://m.kun.uz/en/news/2025/11/29/cheaper-than-in-china-uzbekistan-railways-defends-rising-fares-says-real-costs-are-twice-as-high)
- Aggregators list Tashkent–Samarkand train fares from about 8 to 51 USD across all train types. Rome2rio gives about 2h20m and 26–29 USD for the fast train — [12go](https://12go.asia/en/train/Tashkent/Samarkand/?z=15648926); [Rome2rio](https://www.rome2rio.com/s/Tashkent/Samarkand)

**Taxis**
- Yandex Go Tashkent caps (before surge), snapshot of 20 Oct 2026 — [Voyage.uz](https://voyage.uz/guides/transport-yandex-go/); also 12 Mar 2026 — [Voyage.uz DE](https://voyage.uz/de/yandex-go-usbekistan)
  - Start: minimum 4,600 UZS, then 1,050 UZS/km + 500 UZS/min
  - Comfort: minimum 5,400, then 1,400/km
  - Comfort+: minimum 8,900, then 2,250/km
  - Business: minimum 10,800, then 2,850/km
  - These are a third-party reading of the tariff page. Official live rates are shown in the app or site — [Yandex Go terms](https://yandex.com/legal/yandexgo_termsofuse/16032026/)
- Yandex Go has closed its "Standart" tariff — [Anhor](https://anhor.uz/news/standart/). It is present in Urgench and Khiva (since June 2024) — [Kun.uz](https://kun.uz/en/news/2024/06/27/yandex-go-service-launched-in-urgench-and-khiva)
- The ride-hailing market is getting crowded: Yandex Go, MyTaxi and new entrants (Aug 2026) — [Kursiv](https://uz.kursiv.media/en/2026-08-03/more-than-pricing-uzbekistans-ride-hailing-market-gets-crowded/amp/)

**Intercity bus and shared taxi (aggregator estimates, low confidence)**
- Tashkent–Samarkand bus: about 7 USD, roughly hourly, about 4h. Taxi: about 26–37 USD — [Rome2rio](https://www.rome2rio.com/s/Tashkent/Samarkand). Bookaway lists the cheapest bus at 23 USD (different operator or class) — [Bookaway](https://www.bookaway.com/routes/uzbekistan/tashkent-to-samarkand)
- Private 3-seat transfer via 12go: about 227–231 USD — [12go](https://12go.asia/en/travel/Tashkent/Samarkand/?z=15846451)
- Samarkand–Bukhara: cheapest about 16–17 USD (train) — [12go](https://12go.asia/en/travel/Samarkand/Bukhara/?z=15846451)
- I found no dated 2026 shared-taxi price from a primary source.

**Domestic flights**
- Uzbekistan Airways branded fares from 1 Apr 2026 (announced 17 Mar 2026): Tashkent–Bukhara from 276,741 UZS, Tashkent–Urgench from 484,298 UZS — [uzairways.com](https://uzairways.com/en/press-center/news/uzbekistan-airways-fare-policy-update)
- Silk Avia (low-cost, part of the Uzbekistan Airways group since Oct 2023) raised fares about 7–7.5% on 14 domestic routes from Feb 2026 — [Kursiv](https://uz.kursiv.media/en/2026-01-17/silk-avia-raises-domestic-air-fares-from-february-2026/amp/). Its new Bukhara–Sariosiyo route started 10 Jul 2026 — [UzDaily](https://www.uzdaily.uz/en/silk-avia-to-launch-new-bukhara-sariosiyo-flight-route/). It suspended some routes for 2 months in Aug 2025 — [Kun.uz](https://kun.uz/en/news/2025/08/26/silk-avia-suspends-several-domestic-flights-for-two-months)
- Humo Air (private low-cost, A320) suspended domestic flights from 10 Mar 2024 — [Daryo](https://daryo.uz/en/2024/03/01/uzbekistans-airline-humo-air-temporarily-suspends-domestic-flights/). I did not confirm whether it has resumed.
- Uzbekistan plans to grow regional flights that bypass Tashkent from 624 (2026) to 1,560 (2030) — [Daryo](https://daryo.uz/2026/01/14/uzbekistan-to-more-than-double-regional-flights-bypassing-tashkent-by-2030/)

### Inferences
- Fares change at least once a year: trains in Jan 2024, Jan 2025 and Feb 2026; Tashkent city transport in Oct 2026. The app should keep fares in an editable config with an "as of" date, not hardcoded.
- Approximate USD (at roughly 12,000 UZS/USD; I did not verify the rate): Tashkent metro about 0.21 USD; Afrosiyob Tashkent–Samarkand economy about 26 USD.

### Gaps
- No per-route Sharq or Nasaf fares for 2026.
- No Bukhara city bus fare.
- No Tashkent–Bukhara shared-taxi price.
- No official Yandex Go Uzbekistan tariff page fetched; no MyTaxi tariffs.

## 3. Taxi app integration: Yandex Go API/deep links, MyTaxi, others

### Takeaway
Yandex Go has a documented, public deep-link format that opens the app with pickup, destination and tariff pre-filled. It also has partner "trip info" and "zone info" APIs (for price and ETA estimates) that you get through the Yandex taxi distribution or partner programme. MyTaxi has no public deep-link or API documentation.

### Cited Findings
- Yandex Go deep link: `https://3.redirect.appmetrica.yandex.com/route?start-lat=..&start-lon=..&end-lat=..&end-lon=..&tariffClass=econom&ref=<partner_id_or_site>&appmetrica_tracking_id=<id>&lang=..`. app_code 3 means Yandex Go. The tracking id 1178268795219780156 falls back to the App Store and 25395763362139037 to the website. The link opens the app if it is installed. Points are optional; use the Geocoder to turn addresses into coordinates. Order statistics come via taxi-api@yandex-team.ru — [Yandex docs](https://yandex.com/support/taxi-distr/en/api/deeplinks); [dev doc](https://yandex.com/dev/taxi/doc/dg/concepts/deeplinks.html)
- Yandex "Trip information API" (price and time estimate) — [Yandex](https://yandex.com/support/taxi-distr/en/api/trip-info). "Zone info API" (available tariffs in a region) — [Yandex RU](https://yandex.com/support/taxi-distr/ru/api/zone-info)
- MyTaxi: operated by MYTAXI OPS LLC; developer listed as Newmax Technologies; bundle id com.uznewmax.mytaxi; site company.mytaxi.uz — [App Store](https://apps.apple.com/uz/app/mytaxi-uz/id865012817); [AppGoblin](https://appgoblin.info/apps/com.uznewmax.mytaxi); [Golden Pages](https://www.goldenpages.uz/en/company/?Id=80434). It is reported to operate only in Tashkent — [Times of Central Asia](https://timesca.com/yandex-go-outage-boosts-rival-uzbekistan/). It switched card acquiring to ATMOS to restore Visa/Mastercard (Jan 2026 case study) — [ATMOS](https://atmos.uz/en/blog/case-mytaxi)
- A Yandex Go outage gave rivals a boost — [Times of Central Asia](https://timesca.com/yandex-go-outage-boosts-rival-uzbekistan/)

### Inferences
- MVP: a "Book in Yandex Go" button using the deep link, plus a fare estimate computed locally from the published caps. For MyTaxi, a store link is the only safe option unless the company agrees a partnership.
- I did not confirm that the Trip info API works for Uzbekistan zones. Test with a key.

### Gaps
- Full list of tariffClass values for Uzbekistan (e.g. econom / comfort / comfortplus / business).
- MyTaxi URL scheme; other apps (e.g. Ways.uz — [App Store](https://apps.apple.com/uz/app/id1580534890)) not researched.

## 4. Uzbekistan Railways booking: official channels, API, resellers / affiliates

### Takeaway
The official channels are eticket.railway.uz and the "Uzrailways tickets" app (One-ID integration, loyalty points). I found no public or partner API. Third-party sale with affiliate commission is available through 12Go (direct or via Travelpayouts), which lists Uzbekistan Railways as an operator.

### Cited Findings
- eticket.railway.uz sells tickets for all passenger trains, is integrated with One-ID, and has about 1.5M users — [dgov.uz](https://dgov.uz/en/solution-detail/109). It has a loyalty programme — [eticket.railway.uz](https://eticket.railway.uz/en/pages/loyalty-instruction). App support: e-ticket@railway.uz — [App Store](https://apps.apple.com/app/id1497456207)
- The railway charges its own e-ticket service fee: 16,000 UZS for Afrosiyob and other domestic trains, from an older (about 2020) article — [Kun.uz](https://kun.uz/en/news/2020/02/22/uzbekistan-railways-reduces-commission-rate-for-e-tickets). The current fee is unverified.
- Tickets for special trains are sold electronically and accept international payment systems — [railway.uz](https://www.railway.uz/en/informatsionnaya_sluzhba/novosti/21871/?VOICE=Y)
- Offline or agency resellers (e.g. OrexCA) sell tickets for a commission — [OrexCA](https://www.orexca.com/uzbekistan/trains/useful-information.htm)
- 12Go lists Uzbekistan Railways — [12go](https://12go.asia/ko/operator/uzbekistan-railways). Its affiliate programme (direct at agent.12go.asia or via Travelpayouts) pays up to 50% of 12Go's commission, with a 30-day cookie and an average of about 3 USD per booking. It does not offer class choice (Economy/Business/VIP) for Afrosiyob — [agent.12go.asia](https://agent.12go.asia/); [Travelpayouts 12go offer](https://www.travelpayouts.com/en/offers/12go-affiliate-program); [12go blog](https://blog.12go.asia/?p=219)
- uztrans.uz (government) aggregates air, rail and bus ticket viewing, but buying goes through the other systems — [gov.uz Mintrans](https://gov.uz/en/mintrans/pages/interaktiv-xizmatlar-ro-yxati)

### Inferences
- For schedules, the app would need to scrape eticket.railway.uz (check its ToS) or pull 12Go's widget or API data. Ask O'zbekiston temir yo'llari directly about B2B access.

### Gaps
- Whether Tutu.ru (also on Travelpayouts) sells Uzbek domestic trains: not verified.
- 12Go's per-booking commission specifically for UZ trains: not published.

## 5. Intercity buses

### Takeaway
There are official online booking channels: avtoticket.uz (Mintrans, Dec 2025, departures from Tashkent bus station), the UzBus app (Mintrans digital transport centre, Payme) and ATTO (Dec 2025). Aggregators are Busbud, 12Go, CheckMyBus and Infobus. I found no public API.

### Cited Findings
- avtoticket.uz launched Dec 2025 — [Trend.az](https://en.trend.az/casia/uzbekistan/3703244.html). UzBus launched Jan 2023, with seat choice, Payme and cancellation — [Spot.uz](https://www.spot.uz/ru/2023/01/31/uzbus/). ATTO intercity bus booking — [atto.uz](https://atto.uz/en/news/services). Busbud covers UZ — [Busbud](https://www.busbud.com/en-za/country/uz)
- Online intercity bus tickets have existed since 2019 — [Kun.uz](https://kun.uz/en/news/2019/08/29/purchase-of-tickets-for-intercity-bus-services-becomes-available-online)

### Gaps
- Which regional bus stations are on avtoticket.uz or UzBus as of 2026; any partner API.

## 6. Flight APIs: Travelpayouts/Aviasales, Amadeus Self-Service, Kiwi Tequila, Skyscanner

### Takeaway
Amadeus Self-Service is gone: registrations closed in Feb 2026 and keys were disabled on 17 Jul 2026, leaving Enterprise only. Kiwi Tequila is no longer self-serve and is partner-approval only (secondary sources). Travelpayouts/Aviasales is the most accessible option: the free Data API serves cached prices, and the real-time Search API needs at least 50k MAU.

### Cited Findings
- Amadeus: decommissioning was announced Feb 2026. New registrations were paused, and on 17 Jul 2026 all self-service keys were disabled and the portal closed. Enterprise APIs remain — [Amadeus statement](https://amadeus.com/en/industry-messaging/statement-regarding-amadeus-for-developers-portal); [PhocusWire](https://www.phocuswire.com/amadeus-shut-down-self-service-apis-portal-developers). The GitHub amadeus4dev org was archived on 17 Jul 2026 — [GitHub](https://github.com/amadeus4dev). Amadeus later announced work with Anthropic on Claude access — [Skift 2026-09-03](https://skift.com/2026/09/03/amadeus-brings-travel-tech-into-claude/)
- Travelpayouts/Aviasales: the Data API (cached prices) is for smaller projects. The real-time Flight Search API is only for projects with 50,000+ MAU. The old Search API was shut down on 15 Jun 2026; the new version has been available since 1 Nov 2025 — [Travelpayouts support](https://support.travelpayouts.com/hc/en-us/articles/203956173); [new API](https://support.travelpayouts.com/hc/en-us/articles/30565016140434); [Data API](https://support.travelpayouts.com/hc/en-us/articles/203956163); [GraphQL](https://support.travelpayouts.com/hc/en-us/articles/4417975783314)
- Aviasales commission is about 1.1–1.5% of booking value (third-party estimate). Travelpayouts passes on 50–70% of brand commission (third party) — [track360](https://track360.io/blog/travelpayouts-affiliate-network-operator-teardown-2026); [partnerkin](https://partnerkin.com/blog/articles/affiliate-programs-aviasales). There is a list of brands offering API or feeds to partners — [Travelpayouts](https://support.travelpayouts.com/hc/en-us/articles/20384016664594). Hotellook has been closed (relevant to Travelpayouts hotel data) — [Travelpayouts FAQ](https://support.travelpayouts.com/hc/en-us/articles/29534131568530-FAQ-on-the-closure-of-Hotellook)
- Kiwi Tequila: closed to public self-registration (reportedly in 2024) and now partnership-only with volume thresholds, per a competitor's page — [Ignav](https://ignav.com/docs/kiwi-tequila-alternative). Kiwi describes a "new approach to partnerships" — [Kiwi media](https://media.kiwi.com/articles-and-interviews/better-for-business-kiwi-com-takes-a-new-approach-to-partnerships/). Integrators say access needs approval (AffilID plus key) — [PHPTRAVELS](https://phptravels.com/es/integrations/kiwi-api-integration)

### Inferences
- Domestic Uzbekistan Airways and Silk Avia fares can show up in Aviasales cached data, but coverage of low-cost domestic routes may be thin. Link to uzairways.com as a fallback.

### Gaps
- Skyscanner partner API status 2025–26 not researched (historically approval-only).
- Exact Aviasales payout percentage in 2026 not confirmed from an official source.
- Uzbekistan Airways has no public API or affiliate programme found.

## 7. Licensing and storage of schedule and fare data

### Takeaway
No open-licensed official transit data was found. Each source has its own terms.

### Cited Findings
- 2GIS holds exclusive rights to Tashkent bus monitoring data — [Podrobno.uz](https://podrobno.uz/cat/tehnp/toshshakhartranskhizmat-i-2gis-podpisali-soglashenie-o-monitoringe-obshchestvennogo-transporta-/)
- Travelpayouts terms govern any use of its data — [Travelpayouts terms](https://support.travelpayouts.com/hc/en-us/articles/360004162111)

### Inferences
- OSM (ODbL) is the only clearly reusable route and stop source. Fares come from government decisions (public facts) and can be stored with source and date. Do not bulk-store or redistribute Yandex, 2GIS or Aviasales data beyond their caching terms. Scraping eticket.railway.uz carries ToS risk.

### Gaps
- I did not read the ToS of eticket.railway.uz, 2GIS API, Yandex Maps, 12Go or Travelpayouts for caching limits.
