# POI and food-place data sources for Uzbekistan (travel-assistant backend)

Research date: 2026-11-25. Primary measurements (Overpass, Wikidata SPARQL) were run live on this date. Prices are quoted as shown on pricing pages fetched on 2026-11-25 unless noted otherwise. Items marked "UNVERIFIED" come from secondary sources.

Storage classification legend used below:
- **STORE** = may copy into our own DB long-term (subject to license obligations)
- **CACHE-LIMITED / DISPLAY-ONLY** = must fetch live, store only IDs or short-lived cache

## 1. OpenStreetMap / Overpass / Geofabrik: coverage and ODbL obligations

### Takeaway
OSM is the best free, storable baseline for Uzbekistan (about 2,575 restaurants nationwide, 1,100+ cafes in Tashkent alone, measured live), but attribute coverage (hours, cuisine) is partial and there are no ratings, photos or prices. You may store it in your own DB (STORE) under ODbL: attribution is always required, and share-alike applies only if you publicly distribute a *derivative database*, not to app screens or other "produced works".

### Cited Findings
- Live Overpass count (OSM data timestamp 2026-11-25T18:41Z), whole Uzbekistan (`area["ISO3166-1"="UZ"]`): `amenity=restaurant` = **2,575** objects (2,072 nodes, 498 ways, 5 relations) — [Overpass API query, overpass-api.de](https://overpass-api.de/api/interpreter)
- Live Overpass counts, Tashkent city area (`area["name:en"="Tashkent"]["place"="city"]`), 2026-11-25: restaurant **752**; cafe **1,105**; fast_food **583**; any `tourism=*` **779**; `tourism=attraction` **49**; `tourism=museum` **44**; any `historic=*` **292**; `amenity=place_of_worship`+`religion=muslim` **192**; restaurant/cafe with `opening_hours` **484** (about 26% of 1,857); restaurant/cafe with `cuisine` **634** (about 34%); `tourism=*` with a `wikidata` tag **60** — [Overpass API query](https://overpass-api.de/api/interpreter)
- Overpass responses carry the notice "The data is made available under ODbL." — [Overpass API](https://overpass-api.de/api/interpreter)
- Geofabrik Uzbekistan extract: `uzbekistan-latest.osm.pbf` **120 MB**; shapefile zip **259 MB**; GeoPackage zip **263 MB**; refreshed daily with `.osc.gz` diff files; ODbL 1.0; user names and IDs stripped — [Geofabrik Uzbekistan](https://download.geofabrik.de/asia/uzbekistan.html)
- ODbL attribution is always required, e.g. "© OpenStreetMap contributors" linked to openstreetmap.org/copyright — [OSMF Licence & Legal FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)
- Share-alike: "a Derivative Database is created where you adapt, modify, enhance, correct or extend our data". If you publicly distribute it, "it must continue to be licensed under the ODbL" — [OSMF FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)
- Produced Works (rendered maps, screens, tile services) "can [be licensed] under any terms you like", but recipients can request the underlying derivative database — [OSMF FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)
- Internal use is not Public Use: "If you only use OSM data and any Derivative Databases privately for yourself or within your ... company then it is not public use." — [OSMF FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)
- A Collective Database (independent databases kept separate) keeps the ODbL off the non-OSM parts. The OSMF points to its Collective Database, Horizontal Map Layers, Substantial and Trivial Transformations guidelines for mixed-source cases — [OSMF FAQ](https://osmfoundation.org/wiki/Licence/Licence_and_Legal_FAQ)

### Inferences
- Coverage pattern: Tashkent alone has about 1,857 restaurant+cafe objects, while the whole country has 2,575 restaurants. So OSM food coverage is concentrated in Tashkent, and regional cities (Fergana valley, Karakalpakstan) are likely thin. Field coverage for hours and cuisine is about 25–35%, so hours need a second source.
- Recommended architecture: keep OSM in its own table or layer (osm_id, tags) and link our proprietary fields (ratings, photos, crowdsourced prices) by ID. Treat this as a collective database, so only the OSM-derived layer would be ODbL if we ever publish a dump. App screens are produced works and need attribution only.
- Use the Geofabrik PBF plus daily diffs for bulk ingest (STORE). Use public Overpass only for ad-hoc queries, because the public instances were overloaded and timed out repeatedly during this research.

### Gaps
- Samarkand, Bukhara, Khiva and Fergana city counts could not be obtained: public Overpass instances (overpass-api.de, private.coffee) timed out repeatedly on 2026-11-25. The fix is to count locally from the Geofabrik PBF with osmium or osm2pgsql.
- Nationwide counts for cafe, tourism and historic also timed out.
- No published taginfo/OSMstats breakdown for Uzbekistan was found.

## 2. Wikidata / Wikipedia / Wikimedia Commons (monuments, heritage, photos)

### Takeaway
Wikidata holds the national heritage register in structured form: 8,200 items flagged "object of tangible cultural heritage of Uzbekistan". Only about 1,472 of 8,260 heritage items have coordinates and about 714 have an image, so it is a strong ID/name backbone that needs geocoding enrichment. Wikidata is CC0 (STORE freely). Commons photos are per-file CC BY-SA/CC BY and similar (STORE with attribution and license per image).

### Cited Findings
- Live SPARQL (2026-11-25): items with `P17=Uzbekistan` and any `P1435` heritage designation = **8,260**. Of these, **1,472** have coordinates (P625) and **714** have an image (P18) — [Wikidata Query Service](https://query.wikidata.org/)
- Heritage designations breakdown: "object of tangible cultural heritage of Uzbekistan" **8,200**; Tentative World Heritage Site **34**; part of UNESCO World Heritage Site **31**; World Heritage Site **8**; Memory of the World **7**; Ramsar site **4** — [Wikidata Query Service](https://query.wikidata.org/)
- Items with `P31` (direct instance of) in Uzbekistan: mosque **1,071**; madrasa **130**; museum **53**; park **8** — [Wikidata Query Service](https://query.wikidata.org/)
- Wiki Loves Monuments has run in Uzbekistan since 2023, organised by the Uzbek Wikimedia user group with Russian Wikivoyage and the Ministry of Culture. The 2024 round had **4,096 uploads from 96 users**. The 2025 round ran 20 Sep – 31 Dec 2025 — [Commons: WLM 2024 in Uzbekistan](https://commons.wikimedia.org/wiki/Commons:Wiki_Loves_Monuments_2024_in_Uzbekistan); [Meta WLM 2025 grant](https://meta.wikimedia.org/wiki/Grants:Programs/Wikimedia_Community_Fund/Rapid_Fund/Wiki_Loves_Monuments_2025_in_Uzbekistan_(ID:_23481099)); [eco.gov.uz news](https://eco.gov.uz/en/site/news?id=3485)
- The WLM participation page links a list of cultural heritage monuments of Uzbekistan in Uzbek — [WLM 2024 Participate (mirror)](https://download.osmand.net/wiki/Commons:Wiki_Loves_Monuments_2024_in_Uzbekistan/Participate)
- A Wikidata-generated list exists for Uzbekistan's intangible heritage (shashmaqam, Navruz, tandir non). It is useful for food and culture content — [Meta: Wiki Loves Living Heritage/ICH of Uzbekistan](https://meta.wikimedia.org/wiki/Wiki_Loves_Living_Heritage/Intangible_Cultural_Heritage_of_Uzbekistan)

### Inferences
- Wikidata is CC0 (general knowledge; not re-fetched here), so it can be stored and merged with no obligations. Wikipedia text is CC BY-SA, so derived descriptions must be attributed and shared alike. Commons images must carry author and license per file, and some may be non-commercial-incompatible. Filter by license via the Commons API (extmetadata).
- Join Wikidata to OSM via OSM `wikidata=*` tags (only 60 tourism objects in Tashkent have one) and via coordinates.

### Gaps
- No machine-readable official heritage register on data.egov.uz was found (see section 10). It is unclear whether the 8,200 Wikidata items were imported from an official list, and how current that list is.
- The SPARQL count of Uzbekistan items with coordinates came back as 5,226, which looks low and may be a truncated or timeout artifact. Treat it as unreliable.

## 3. Google Places API (New): pricing, free tier, storage terms

### Takeaway
Google likely has the richest Uzbekistan restaurant data (ratings, reviews, photos, hours, price level), but it is **CACHE-LIMITED / DISPLAY-ONLY**. Only `place_id` may be stored indefinitely, lat/lng may be cached for up to 30 days, and everything else must be fetched live. Since 1 March 2025 the $200 credit has been replaced by per-SKU free caps (10k / 5k / 1k per month for Essentials / Pro / Enterprise).

### Cited Findings
- From 1 March 2025, the USD 200 monthly credit was replaced with free monthly usage thresholds per Core Services SKU: **Essentials 10,000, Pro 5,000, Enterprise 1,000** billable events per month. Volume discounts now go up to 5M+ events. Places API (Legacy) is designated Legacy, and Places API (New) is the current version — [Google: March 2025 changes](https://developers.google.com/maps/billing-and-pricing/march-2025)
- Optional subscription plans were listed: Starter USD 100/month for 50,000 calls, Essentials USD 275 for 100,000, Pro USD 1,200 for 250,000 — [Google pricing overview](https://developers.google.com/maps/billing/overview/) (this appeared in search snippets and was not re-verified on the fetched page)
- Price per 1,000 at the 0–100k tier (USD), with the free monthly cap, from the pricing page fetched 2026-11-25 — [Google Maps Platform pricing](https://developers.google.com/maps/billing-and-pricing/pricing):
  - Text Search Essentials (IDs only): unlimited free
  - Text Search Pro: $32 (5,000 free); Enterprise $35 (1,000 free); Enterprise + Atmosphere $40 (1,000 free)
  - Nearby Search Pro: $32 (5,000 free); Enterprise $35 (1,000 free)
  - Place Details Essentials: $5 (10,000 free); Pro $17 (5,000 free); Enterprise $20 (1,000 free); Enterprise + Atmosphere $25 (1,000 free)
  - Place Details Photos: $7 (1,000 free)
  - Autocomplete Requests: $2.83 (10,000 free); Autocomplete Session Usage free
- Storage: "You can therefore store place ID values indefinitely." Lat/lng may be cached "for up to 30 consecutive calendar days, after which you must delete the cached coordinates." Other content may be cached "only as expressly permitted under the Google Maps Platform Service Specific Terms". Scraping and bulk storage of "business names, addresses, or user reviews" is prohibited — [Places API policies](https://developers.google.com/maps/documentation/places/web-service/policies)
- Derivative use is prohibited, e.g. using Places lat/lng "as input for point-in-polygon analysis" — [Places API policies](https://developers.google.com/maps/documentation/places/web-service/policies)
- Attribution: use the Google Maps logo where possible. "You must always credit the author when displaying photos or reviews". End users must be able to view the source photo or review on Google Maps — [Places API policies](https://developers.google.com/maps/documentation/places/web-service/policies)

### Inferences
- Pricing tier implication: rating, hours and price level sit in the Pro/Enterprise field masks, and reviews/atmosphere fields cost $25/1k. A pattern of storing place_id from free ID-only Text Search, then calling Details Pro or Enterprise on user tap keeps costs bounded.
- Google content cannot seed our own POI DB. Use it as a live enrichment layer keyed by stored place_id.

### Gaps
- No quantitative data was found on Google's Uzbekistan listing counts or review density. Do a manual spot check in Samarkand, Bukhara and Khiva before committing.
- The Service Specific Terms text (exact caching exceptions) was not fetched.

## 4. Foursquare Places API and FSQ Open Source Places

### Takeaway
FSQ OS Places is an Apache-2.0 global dump of 100M+ POIs (STORE, with attribution). Its Uzbekistan coverage is unmeasured and likely modest. The commercial Places API has free Pro calls and then a $15 CPM.

### Cited Findings
- FSQ OS Places is licensed Apache 2.0 and released in November 2024. It has 100M+ POIs, and each record carries an ISO country code — [Foursquare blog](https://foursquare.com/resources/blog/products/foursquare-open-source-places-a-new-foundational-dataset-for-the-geospatial-community/); [Simon Willison, 2024-11-20](https://simonwillison.net/2024/Nov/20/foursquare-open-source-places); [FSQ docs](https://docs.foursquare.com/data-products/docs/fsq-places-open-source)
- The Hugging Face copy is now gated (you must share contact info). Foursquare describes additional access channels, including an Iceberg catalog — [Hugging Face](https://huggingface.co/datasets/foursquare/fsq-os-places); [Evolving FSQ OS Places](https://foursquare.com/resources/blog/data/evolving-fsq-open-source-places/)
- Places API pricing (official page via search snippet, 2025–26): **Pro** endpoints (Search, Details, Autocomplete with default fields) cost $0 for the first 500 calls (an "up to 10,000 free Pro calls" claim also appears; this is a conflict), then **$15.00 CPM** for 501–100k, **$12** for 100k–500k, **$9** for 500k+. **Premium** (tips, photos, premium fields such as rating, hours, price) costs **$18.75 CPM** for 0–100k, $15 for 100k–500k, $11.25 for 500k+ — [Foursquare pricing](https://foursquare.com/pricing/); third-party figures conflict — [AI Agent Store](https://aiagentstore.ai/services-for-ai-agents/foursquare-places)
- Customers who joined before 2 June 2025 moved to the new pricing on 1 September 2025. Legacy v2/v3 endpoints are billed at $18.75 CPM — [Foursquare upcoming changes](https://docs.foursquare.com/developer/reference/upcoming-changes)

### Inferences
- Run `SELECT count(*) FROM fsq_os_places WHERE country='UZ' GROUP BY category` locally before relying on it. Foursquare check-in usage in Uzbekistan has historically been low, so expect sparse and stale entries.
- FSQ OS has no ratings or photos (those are in the paid Premium API), so it adds little beyond OSM except as an extra source for dedupe and recall.

### Gaps
- No Uzbekistan count for FSQ OS Places was published or found.
- The free Pro allowance (500 vs 10,000) conflicts between sources, and API storage/caching terms were not retrieved.

## 5. 2GIS API

### Takeaway
2GIS is the strongest local directory for Tashkent: launched in August 2019, it has since added Samarkand and Bukhara. Its Places API is sold in RUB packages, but the two price readings below conflict. Storage terms were not found, so treat it as display/cache-limited until 2GIS confirms in writing.

### Cited Findings
- 2GIS launched in Uzbekistan (Tashkent) in August 2019 — [Daryo](https://daryo.uz/2019/08/31/ozbekistonda-2gis-dasturi-ishga-tushirildi); [Spot.uz](https://www.spot.uz/ru/2019/08/30/2gis/)
- 2GIS also acts as a guide in Samarkand and Bukhara (2024 article) and launched a Samarkand guide project — [ELLE Uzbekistan](https://elleuzbekistan.com/en/?p=16592); [kun.uz](https://kun.uz/en/61556915)
- Places API monthly packages (RUB), from the pricing page fetched 2026-11-25 — [2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing):

  | Units/month | Price |
  |---|---|
  | 10k | 6,700 ₽ |
  | 30k | 15,000 ₽ |
  | 50k | 20,000 ₽ |
  | 100k | 30,000 ₽ |
  | 300k | 46,700 ₽ |
  | 500k | 63,400 ₽ |
  | 750k | 79,200 ₽ |
  | 1M | 100,000 ₽ |
  | 1.5M | 138,400 ₽ |
  | 3M | 183,400 ₽ |
  | 5M | 250,000 ₽ |

  Yearly plans get a 10% discount. One unit is one successful request (HTTP 200/204). A demo key allows 1,000 requests in total. Rate limit is 600 units per minute. Some fields and methods (detailed contacts, floor plans, INN, building and heritage attributes, search by phone/website/INN) are billed separately via sales.
- Conflict: a search snippet of the same page read "6,700 ₽ for 100,000 units". The fetched table maps 6,700 ₽ to 10,000 units. Verify with 2GIS — [2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing)
- The pricing page contains no data-storage terms — [2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing)

### Inferences
- 2GIS is likely best-in-class for Tashkent opening hours, phones and categories. Coverage of Fergana valley cities (Fergana, Namangan, Andijan) and Khiva is unconfirmed.
- 2GIS also offers on-premise and data licensing ([on-premise API platform](https://docs.2gis.com/en/on-premise/api-platform/overview)), so a negotiated data licence may allow storage. Ask sales.

### Gaps
- The 2GIS licence agreement text (caching/storing, display requirements) was not found.
- No official current list of covered Uzbek cities or organization counts was found. Apify scrapers exist (about $1 per 1,000 results), but they violate typical ToS and are not recommended.

## 6. Yandex Maps: Geosearch ("Поиск по организациям") API

### Takeaway
Yandex Geosearch is very expensive: a minimum of 195,000 RUB/year for 1,000 requests/day under the standard licence, or 20,800 RUB/month on monthly billing. It offers a separate pricier "extended licence (with data storage)". That implies the standard licence does not permit storing results. Yandex Maps is widely used in Uzbekistan.

### Cited Findings
- Geosearch tariffs, annual, daily-metered (RUB), from the page fetched 2026-11-25 — [Yandex Geosearch tariffs](https://yandex.com/dev/tariffs/doc/ru/geosearch/prices/):

  | Requests/day | Extended licence (with data storage) | Standard licence | Overage per 1,000 requests |
  |---|---|---|---|
  | 1,000 | 226,200 | 195,000 | 390 |
  | 10,000 | 678,600 | 585,000 | 195 |
  | 25,000 | 1,281,800 | 1,105,000 | 163 |
  | 50,000 | 2,210,000 | 1,820,000 | 163 |
  | 100,000 | 2,652,000 | 2,184,000 | 163 |
  | 200,000 | 3,757,000 | 3,094,000 | 117 |
  | 300,000 | 5,226,000 | 4,329,000 | 117 |
  | 500,000 | 7,618,000 | 6,305,000 | 117 |
  | 1M | 13,481,000 | 11,050,000 | 59 |

  Above 1M/day: contact paid-api-maps@yandex-team.ru.
- Monthly standard licence: 20,800 RUB for 1,000/day; 58,500 RUB for 10,000/day — [Yandex Geosearch tariffs](https://yandex.com/dev/tariffs/doc/ru/geosearch/prices/)
- Test period: up to 100 requests/day for a maximum of 7 days, free. The minimum payment is non-refundable, and unused daily requests do not roll over — [Yandex Geosearch tariffs](https://yandex.com/dev/tariffs/doc/ru/geosearch/prices/)
- Terms are in the Maps API offer — [yandex.ru/legal/maps_api_offer](https://yandex.ru/legal/maps_api_offer)

### Inferences
- The existence of a "with data storage" licence strongly suggests the standard licence forbids storing results (CACHE-LIMITED). Historically the free Yandex Maps API tier also required display on a Yandex map. That is from training knowledge, not verified here.
- At these prices Yandex is unattractive for a startup compared with 2GIS or Google.

### Gaps
- The offer text (display-on-Yandex-map requirement, cache duration) was not fetched. Uzbekistan organization coverage stats were not found.

## 7. Tripadvisor Content API

### Takeaway
The Tripadvisor Content API gives 5,000 free calls per month and then pay-as-you-go pricing. Mandatory attribution uses the Tripadvisor logo and bubble images. It is useful for tourist-facing ratings in Samarkand, Bukhara and Khiva. Caching terms were not verified, so treat it as display-only. A new "Terra" platform is coming.

### Cited Findings
- "We offer the first 5000 API calls for free every month after you sign up." A credit card is required. Pricing is shown at checkout or on My Plans, and volume above 500k/month requires sales. A daily budget cap returns HTTP 429. Search APIs are limited to 10,000 calls/day, and limits reset at midnight UTC — [Tripadvisor Content API FAQ](https://tripadvisor-content-api.readme.io/reference/faq)
- "you must credit Tripadvisor when using our Locations API product", using the logo and bubble-rating images from the JSON results — [Tripadvisor FAQ](https://tripadvisor-content-api.readme.io/reference/faq)
- "A more modern API platform is coming soon" (Terra, docs.terra.tripadvisor.com). No date is given — [Tripadvisor FAQ](https://tripadvisor-content-api.readme.io/reference/faq)
- UNVERIFIED: the API covers location search, details, photos and reviews (commonly up to 5 reviews and 5 photos per location). Dev keys are capped at 1,000 calls/day until launch review — [Supergood report](https://supergood.ai/api-report-card/tripadvisor-api)

### Inferences
- Use it for "top attractions and restaurants" ratings in tourist cities. Expect thin coverage outside the tourist circuit.

### Gaps
- Caching terms (the Master Terms of Use at [tripadvisor-content-api.readme.io/reference/terms-of-use](https://tripadvisor-content-api.readme.io/reference/terms-of-use)) were not read. Exact per-call prices are not public.

## 8. Overture Maps Places

### Takeaway
Overture Places (about 70–75M places globally; per-source licences, mostly CDLA-Permissive-2.0, which allows storing) is a free, storable complement. Uzbekistan coverage is unmeasured. Its Meta-sourced records may cover Uzbek businesses with Facebook pages.

### Cited Findings
- Global size: about 75M places (member summit), or "more than 70 million" (BrightQuery press release, February 2026) — [Linux Foundation press](https://www.linuxfoundation.org/press/brightquery-joins-overture-maps-foundation-to-expand-open-places-data-coverage)
- The Places guide lists per-source counts and licences, e.g. BrightQuery data under CDLA-Permissive-2.0 (15.77M features) — [Overture Places Guide](https://docs.overturemaps.org/guides/places)
- The April 2026 release rolled back a confidence-normalization change on Meta-sourced places, and about 10M places got updated confidence scores. The February 2026 release refreshed Foursquare data. The September 2026 release removed `categories` in favour of `taxonomy` (a schema-breaking change) — [Overture release notes 2026-04-15](https://docs.overturemaps.org/blog/2026/04/15/release-notes/); [2026-02-18](https://docs.overturemaps.org/blog/2026/02/18/release-notes/); [blog](https://docs.overturemaps.org/blog)

### Inferences
- Pull the Uzbekistan bbox from the GeoParquet release with DuckDB (filter `addresses.country='UZ'` or bbox 55.9–73.2 E, 37.1–45.6 N) and filter on `confidence` > ~0.6. Check per-record `sources` for licences.

### Gaps
- No Uzbekistan count was found or measured: DuckDB was not available in this environment.

## 9. Menus and actual dish prices

### Takeaway
There is no public menu or price API from Uzbek delivery platforms. Uzum Tezkor, Yandex Eats and Express24 hold the menu data, but only through restaurant-partner integrations, not developer data access. Numbeo gives city-level averages (not per restaurant) for USD 260+/month. Per-restaurant dish prices will have to be crowdsourced or obtained through restaurant partnerships.

### Cited Findings
- Uzum Tezkor: partner restaurants integrate their menus into the app. Reported 2,600+ partners in 25 cities and 7,000 couriers (April 2025, secondary source). The Competition Committee named it a dominant digital platform in delivery, and Yandex Eats and Express24 were earlier deemed dominant — [Spot.uz](https://www.spot.uz/ru/2025/04/18/uzum-tezkor/); [kun.uz 2025-04-02](https://m.kun.uz/en/news/2025/04/02/uzum-tezkor-named-dominant-digital-platform-in-delivery-market); [Bento travel guide](https://www.bento.travel/uzbekistan/food-delivery)
- Yandex Eats launched in Tashkent in March 2023 with about 200 venues — [Interfax](https://interfax.com/newsroom/top-stories/88501/); [UzDaily](https://www.uzdaily.uz/en/yandex-eats-service-launched-in-uzbekistan/)
- Express24 is still active and is now in the same Yandex-controlled group. UNVERIFIED: Wolt ended Uzbekistan operations on 5 March 2026 (single travel-guide source) — [Bento travel guide](https://www.bento.travel/uzbekistan/food-delivery)
- Numbeo Data API: Basic **USD 260/month** (200k queries), Professional **USD 480** (1M), Enterprise **USD 1,250** (5M), plus VAT, cancel anytime. Includes "commercial usage rights under our Terms of Use". The restaurant price items are city averages (meal at an inexpensive restaurant, mid-range meal for two, cappuccino, beer, water) with low/avg/high values and data-point counts — [Numbeo Data API](https://www.numbeo.com/common/api.jsp); [Restaurant Prices API](https://www.numbeo.com/api/restaurant-prices-api)
- Free use of the Numbeo website data is limited to personal, academic and journalistic use — [numbeo.com](https://numbeo.com)

### Inferences
- Practical approach:
  1. In-app crowdsourcing: receipt or menu photo, OCR to dish and price in UZS, with timestamp and moderation.
  2. Restaurant self-service onboarding (menu upload).
  3. LLM-extracted menus from restaurants' own Telegram and Instagram posts, with permission.
  4. Numbeo or own averages for "typical price" fallbacks (plov, lagman, samsa, choy).
- Scraping delivery apps would likely breach their ToS and competition-sensitive terms. Delivery-app prices are also often marked up versus dine-in prices.

### Gaps
- Developer or partner programs for Uzum Tezkor, Yandex Eats and Express24 were not found.
- The Numbeo Data License (storage/redistribution) was not read, and the number of Uzbek cities with Numbeo data is unknown.

## 10. Official tourism data (uzbekistan.travel, uzbektourism.uz, data.egov.uz, heritage registry)

### Takeaway
No machine-readable official registry of tourist objects or restaurants was found on data.egov.uz. The Cultural Heritage Agency (gov.uz/madaniymeros) has "open data" sections, but they are navigation pages without visible datasets. Official portals are content sources for manual curation, not APIs.

### Cited Findings
- The Cultural Heritage Agency section on gov.uz has "Ochiq ma'lumotlar (Open data)", "Ochiq ma'lumotlar portalidagi maʼlumotlar" and "Ochiq ma'lumotlar to'plami" pages. These are navigation only, with no dataset links visible when fetched — [gov.uz/madaniymeros open data](https://gov.uz/en/madaniymeros/sections/ochiq-ma-lumotlar-open-data); [portal data page](https://gov.uz/en/madaniymeros/sections/ochiq-ma-lumotlar-portalidagi-malumotlar)
- The 2GIS paid fields include building "heritage status", which suggests 2GIS holds heritage attributes — [2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing)

### Inferences
- The national register appears to exist as about 8,200 Wikidata items (section 2), which is the most usable structured proxy. Validate against the Agency's published lists.
- uzbekistan.travel and uzbektourism.uz content (descriptions, photos) is copyrighted unless stated otherwise. Request a partnership or licence before reuse.

### Gaps
- data.egov.uz was not searched directly for tourism, restaurant or heritage datasets.
- The terms of uzbekistan.travel and uzbektourism.uz were not checked.
- These official-portal findings are the weakest area of this research and need a manual follow-up.

## Summary matrix (storage rights)

| Source | Uzbekistan coverage | Ratings/photos/hours | Storage | Cost (as of 2026-11-25) |
|---|---|---|---|---|
| OSM (Geofabrik) | Good in Tashkent; 2,575 restaurants nationwide | Hours ~26%, no ratings | STORE (ODbL: attribution; share-alike on public DB distribution) | Free |
| Wikidata | 8,260 heritage items, 1,071 mosques | 714 images | STORE (CC0) | Free |
| Commons | WLM: 4,096 photos in 2024 | Photos | STORE with per-file attribution | Free |
| FSQ OS Places | Unmeasured | No ratings | STORE (Apache 2.0) | Free |
| Overture | Unmeasured | Confidence only | STORE (per-source, mostly CDLA-P-2.0) | Free |
| Google Places (New) | Likely best (unmeasured) | Yes | place_id forever; lat/lng 30 days; rest live | $5–$40 per 1k after 1k–10k free/SKU/month |
| 2GIS | Tashkent, Samarkand, Bukhara | Likely | Unknown (assume limited) | 6,700–250,000 RUB/month packages |
| Yandex Geosearch | Widely used | Likely | Standard: no storage; extended licence = storage | From 195,000 RUB/yr or 20,800 RUB/mo |
| Tripadvisor | Tourist cities | Ratings, reviews, photos | Display with attribution; caching unverified | 5,000 free calls/month, then PAYG |
| Numbeo | City averages | Price averages | Per Data License (unread) | USD 260–1,250/month |
