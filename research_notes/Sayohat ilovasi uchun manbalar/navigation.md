# Navigation, routing, maps, traffic and fuel/charging for a car-travel app in Uzbekistan

Research date: 2026-11-25. Prices are as shown on vendor pages on that date unless stated otherwise. Items marked "(prior knowledge, unverified)" are not backed by a source fetched in this session.

## 1. Self-hosted open-source routing on OSM data (OSRM, Valhalla, GraphHopper)

### Takeaway
The Uzbekistan OSM extract is small (120 MB PBF), so any of the three engines runs comfortably on a single small VPS. Valhalla (MIT) or GraphHopper (Apache-2.0) are the safest licence choices for car, foot and bike routing with turn-by-turn instructions. None of them provides live traffic.

### Cited Findings
- Geofabrik `uzbekistan-latest.osm.pbf` is 120 MB, last updated about 2026-11-24 (file `uzbekistan-261124.osm.pbf`). Other downloads: Shapefile 259 MB, GeoPackage 263 MB, experimental Shortbread vector tiles `uzbekistan-shortbread-1.0.mbtiles` 307 MB. Daily .osc.gz diffs are also offered. — [Geofabrik Uzbekistan](https://download.geofabrik.de/asia/uzbekistan.html)
- GraphHopper is licensed Apache 2.0. Valhalla and all projects in the Valhalla organisation are MIT. — [pistack.xyz 2026 comparison](https://www.pistack.xyz/posts/2026-04-25-graphhopper-vs-osrm-vs-valhalla-self-hosted-routing-engines-guide-2026/); [Valhalla GitHub](https://github.com/Valhalla/valhalla)
- Published memory figures are informal and disagree by several times. Examples: GraphHopper planet 8–16 GB heap and OSRM planet about 55 GB RAM; Valhalla "comfortably on 4 to 8 GB RAM"; another source says OSRM may need more than 128 GB for the planet. All of these are planet or continent scale, not country scale. — [pistack.xyz](https://www.pistack.xyz/posts/2026-04-25-graphhopper-vs-osrm-vs-valhalla-self-hosted-routing-engines-guide-2026/); [sumguy.com](https://sumguy.com/osrm-vs-valhalla-vs-graphhopper/)
- Geofabrik sells commercial support for hosted routing with OSRM and GraphHopper. — [Geofabrik routing](https://www.geofabrik.de/en/data/routing.html)
- OSM wiki keeps a list of online and self-hostable routers, including openrouteservice, which is built on GraphHopper and offers a free hosted API. — [OSM wiki Routing/OnlineRouters](https://wiki.openstreetmap.org/wiki/Routing/OnlineRouters); [openrouteservice workshop](https://giscience.github.io/openrouteservice-workshop/getting_started/02_why.html)

### Inferences
- A 120 MB PBF is about 1/700 of the planet (roughly 80+ GB). Scaling the figures above, any engine should preprocess Uzbekistan in minutes and serve it with about 1–2 GB RAM. A €5–10/month VPS (2 vCPU, 4 GB) is enough for car + foot + bike. Treat this as an estimate and benchmark it.
- Feature notes (prior knowledge, unverified):
  - OSRM is BSD-2 licensed and very fast. It uses one profile per dataset (car, foot and bike each need their own process). It returns maneuver steps but no time-dependent traffic.
  - Valhalla offers multimodal costing (auto, pedestrian, bicycle, truck, and transit via GTFS) from one tile set, with narrative turn-by-turn instructions in many languages. It can run on-device offline from tile tars.
  - GraphHopper's open-source core has car, foot and bike profiles, instructions, and GTFS public transport. Its hosted Directions API has a free developer tier and paid plans; check graphhopper.com/pricing.
- Transit routing for Tashkent needs GTFS data. No public Uzbekistan GTFS feed was found in this session (see Gaps).
- MapLibre Navigation, or the Ferrostar SDK, can consume OSRM or Valhalla route responses to provide turn-by-turn UI on the client (prior knowledge, unverified; React Native wrappers are immature).

### Gaps
- No verified country-scale RAM or build-time benchmarks were found. Run a test build.
- OSRM's licence was not confirmed from a fetched source (believed BSD-2).
- Current GraphHopper hosted pricing was not fetched.
- No source found for a public GTFS feed for Tashkent buses or metro.
- OSM road-data quality in Uzbekistan (completeness of turn restrictions and new roads) was not assessed.

## 2. Commercial routing and navigation APIs

### Takeaway
- Google has the cleanest React Native story: an official RN Navigation SDK (beta) and Routes API with free caps. It is the most expensive per navigation session, and its Uzbekistan traffic quality is unverified.
- Mapbox is cheap at small scale (100,000 Directions requests/month free, 25,000 mobile MAU free).
- Yandex has the best local data (traffic, lane-level maps in Tashkent), but its MapKit terms forbid building automatic-navigation features.
- 2GIS has strong local POI data and a navigation SDK, but no RN SDK, and prices are in roubles.
- Deep-linking to Google Maps, Yandex Navigator or 2GIS is free and usually the realistic MVP choice.

### Cited Findings
**Google Maps Platform**
- Since 2025-03-01, per-SKU free monthly caps replaced the $200 credit.
- Routes: Compute Routes Essentials has 10,000 free calls/month, then $5.00 per 1,000 (up to 100k), $4.00 (100k–500k), $3.00 (500k–1M), $1.50 (1M–5M), $0.38 (5M+). Route Matrix Essentials uses the same tiers.
- Compute Routes Pro: 5,000 free, from $10 per 1,000. Enterprise: 1,000 free, from $15 per 1,000.
- Subscription plans are also offered: Starter $100/month (50k calls), Essentials $275/month (100k), Pro $1,200/month (250k).
- Source: [Google Maps pricing](https://developers.google.com/maps/billing-and-pricing/pricing); [billing overview](https://developers.google.com/maps/billing/overview/). The figures came from search-result extracts of these pages; re-verify before budgeting.
- Navigation SDK billing: a single "Navigation Request" SKU, billed per destination (waypoints × price). The first 1,000 destinations/month are free. Starting guidance and rerouting cost nothing extra. Limits are 25 destinations per call and 30,000 QPM, and NO_ROUTE_FOUND is still billed. Google says it cut per-destination prices "by half or more". — [Nav SDK Android billing](https://developers.google.com/maps/documentation/navigation/android-sdk/pricing); [Google blog](https://mapsplatform.google.com/resources/blog/making-navigation-sdks-more-accessible-with-new-pricing/). The exact per-destination dollar rate was not retrieved.
- Official React Native wrapper: `@googlemaps/react-native-navigation-sdk` (GitHub googlemaps/react-native-navigation-sdk). It is beta, Apache-2.0, about v0.14.x, requires the RN New Architecture, and needs a billing-enabled project with the Nav SDK enabled. — [Google cross-platform Navigation docs](https://developers.google.com/maps/documentation/cross-platform/navigation); [npm via jsDelivr](https://www.jsdelivr.com/package/npm/@googlemaps/react-native-navigation-sdk)

**Mapbox** (as shown on 2026-11-25)
- Mobile Maps SDK: 25,000 MAU free, then $4.00 per 1,000 MAU (to 125k), $3.20 (to 250k), $2.40 (to 1.25M).
- Navigation SDK v3 metered: 100 MAU free, then $0.30/MAU (to 20k); 1,000 trips free, then $0.08/trip (to 50k), $0.064, $0.048. The unlimited-trips plan has 10 free MAU and needs a sales quote.
- Directions API: 100,000 requests/month free, then $2.00, $1.60, $1.20 per 1,000. Map Matching uses the same tiers.
- Vector Tiles API: 200,000 tiles free, then $0.25 per 1,000.
- Temporary Geocoding: 100,000 free, then $0.75 per 1,000.
- Source: [mapbox.com/pricing](https://www.mapbox.com/pricing)
- Navigation SDK requests made during Active Guidance or Free Drive are included free. The initial route request outside a session is billed as Directions unless Free Drive is started first. — [Mapbox Nav SDK pricing guide](https://docs.mapbox.com/android/navigation/guides/pricing)
- @maplibre/maplibre-react-native began as a fork of rnmapbox (the community RN Mapbox wrapper). — [maplibre-react-native GitHub](https://github.com/maplibre/maplibre-react-native)

**Yandex MapKit**
- Licensing is by DAU and paid yearly, in roubles:
  - Up to 1,000 DAU: free under the free-version terms.
  - 5,000 DAU: 195,000 ₽/yr (+260 ₽ per extra 1,000 DAU).
  - 20,000 DAU: 585,000 ₽ (+195 ₽).
  - 50,000 DAU: 1,105,000 ₽ (+195 ₽).
  - 100,000 DAU: 1,820,000 ₽ (+78 ₽).
  - Above that, contact paid-api-maps@yandex-team.ru. The base fee is not refundable.
  - Source: [Yandex MapKit prices](https://yandex.ru/dev/tariffs/doc/en/mapkit/prices/)
- Terms clause 2.4 forbids using the software to create "programs or Application features that enable automatic navigation, including … navigation applications". Geocoder and Router results may be cached at most 30 days. Client–server Router covers driving, public transport and walking; server–server covers driving only. Uzbekistan is in the CIS coverage list. — [Yandex MapKit terms](https://yandex.com/dev/tariffs/doc/en/mapkit/terms/index.md)
- Yandex markets a separate NaviKit SDK for building navigation. — [Yandex NaviKit](https://yandex.com/maps-api/products/navikit). Its price and Uzbekistan availability were not retrieved.
- The full MapKit SDK includes routing that accounts for real-time traffic for cars and public transport. — [Yandex MapKit product page](https://yandex.com/maps-api/products/mapkit)

**2GIS**
- API Platform prices are in roubles per month, with 10% off for annual payment:
  - Map Tiles API: 8,000 ₽ per 100k units.
  - Routing, Directions, Distance Matrix, Isochrone and Map Matching APIs: 6,700 ₽ per 10k units each.
  - Places, Geocoder and Suggest APIs: 6,700 ₽ per 10k. Suggest starts at 7,000 ₽ per 100k.
  - Directions API is usable only with the mobile SDK.
  - Demo keys: 500k map tile requests and 1,000 search or navigation requests in total.
  - Source: [2GIS pricing](https://docs.2gis.com/en/platform-manager/subscription/pricing)
- 2GIS mobile SDKs exist for iOS, Android and Flutter (Uzbek locale appears in examples) and need a `dgissdk.key`. There is no official React Native SDK. — [2GIS Flutter SDK start](https://docs.2gis.com/en/flutter/sdk/start); [2GIS Mobile SDK](https://dev.2gis.com/api/mobile-sdk)
- 2GIS has operated in Tashkent since 2019. — [spot.uz 2019](https://www.spot.uz/ru/2019/08/30/2gis/); [Wikipedia 2GIS](https://en.wikipedia.org/wiki/2GIS)

**Deep links to external navigators**
- 2GIS: `dgis://2gis.ru/routeSearch/rsType/car/to/<lon>,<lat>`. Longitude comes first. If the origin is omitted, the current location is used. — [2GIS help: deeplink navigation](https://help.2gis.com/question/developers-launching-2gis-navigation-using-deeplink)
- Yandex Navigator: `yandexnavi://build_route_on_map?lat_from=..&lon_from=..&lat_to=..&lon_to=..` (optional via points). The source is a third-party wrapper and a forum post. The wrapper says Yandex now requires an access key (signed URL) to pass coordinates. — [protocol-launcher](https://zhensherlock.github.io/protocol-launcher/apps/yandex-navigator.html); [Yandex Maps API club](https://yandex.ru/blog/mapsapi/50779)
- Yandex Maps app URL-launch docs: [Launch Yandex Maps by URL](https://yandex.com/dev/yandex-apps-launch/maps/)

**HERE and TomTom**
- No data was gathered this session (see Gaps).

### Inferences
- Google's per-destination billing means each started trip has a cost after the first 1,000/month. For a startup, Routes API (Essentials, 10k free) for previews plus a deep link to Google Maps or Yandex Navigator for guidance avoids Navigation SDK costs entirely. Google Maps universal URLs (`https://www.google.com/maps/dir/?api=1&destination=lat,lng&travelmode=driving`) are free (prior knowledge, unverified in session).
- Yandex's clause 2.4 rules out using free or paid MapKit for in-app turn-by-turn. Displaying Yandex maps and showing a route preview appears allowed; guidance should be handed off to Yandex Navigator via deep link, or NaviKit should be licensed.
- At 1,000 DAU and below, Yandex MapKit display is free, which makes it a strong map layer for Uzbekistan if the team accepts Yandex branding and terms. Paying in roubles may be a practical or sanctions-related obstacle for some payers (inference).
- 2GIS in React Native would need custom native bridges. That is costly for an early team.

### Gaps
- Exact Google Navigation SDK per-destination price was not retrieved.
- HERE and TomTom pricing and Uzbekistan coverage were not researched for lack of tool budget. Both publish global freemium tiers (prior knowledge), but their Uzbekistan traffic coverage is unknown.
- NaviKit pricing and terms were not retrieved.
- Official Yandex Navigator deep-link parameter docs and access-key rules were not verified.

## 3. Map rendering for React Native, and offline tiles

### Takeaway
MapLibre React Native with OpenFreeMap (free, no API key, commercial use allowed) is the cheapest vector-map stack. For true offline use in Uzbekistan, either use MapLibre offline packs within OpenFreeMap's download limits, or self-host or bundle a PMTiles or MBTiles extract (for example Geofabrik's 307 MB Shortbread MBTiles). react-native-maps is simplest but ties you to Google or Apple tiles.

### Cited Findings
- OpenFreeMap public instance:
  - Free, with "no limits on the number of map views or requests".
  - No registration or API key. Commercial use allowed.
  - Styles: Positron, Bright, Liberty, Dark, Fiord and 3D, on the OpenMapTiles schema.
  - Self-hosting setup is MIT licensed. Weekly planet downloads in MBTiles, PMTiles and Btrfs.
  - Offline downloads are allowed at ≤3,000 tiles and ≤50 MB per download, ≤4 parallel requests, with a unique User-Agent. Larger areas may be split into several user-initiated downloads.
  - Attribution "OpenFreeMap © OpenMapTiles Data from OpenStreetMap" is required.
  - Source: [openfreemap.org](https://openfreemap.org/)
- MapLibre Native handles `pmtiles://` natively on Android and iOS. PMTiles sources do not support offline pack downloads or caching on Android; local files are loaded with `pmtiles://file://…`. — [MapLibre Android PMTiles](https://maplibre.org/maplibre-native/android/examples/data/PMTiles/)
- A hosted PMTiles file is still read over the network through HTTP range requests. For offline use, bundle the file locally. — [MapLibre Flutter PMTiles](https://maplibre.org/flutter-maplibre-gl/advanced/pmtiles/); [Protomaps docs](https://docs.protomaps.com/pmtiles/maplibre)
- OpenMapTiles docs recommend @maplibre/maplibre-react-native for RN vector maps. MapTiler SDKs and MapLibre Native support offline maps. — [OpenMapTiles mobile](https://openmaptiles.org/docs/mobile/mobile/)
- Geofabrik provides a ready Uzbekistan vector-tile MBTiles in Shortbread schema (307 MB, experimental). — [Geofabrik](https://download.geofabrik.de/asia/uzbekistan.html)

### Inferences
- Recommended stack: @maplibre/maplibre-react-native, OpenFreeMap styles online, and self-hosted PMTiles extracts of Uzbekistan (Protomaps `pmtiles extract` from a planet build, cut to the UZ bounding box) on cheap object storage or a CDN. Offer per-region downloads (Tashkent, Samarkand, Bukhara, Khiva, Fergana Valley) as local files of tens of MB each (size estimate unverified).
- The 50 MB / 3,000-tile cap on OpenFreeMap offline packs covers a city at street zoom but not the whole country at high zoom, so a whole-country offline map needs your own hosting.
- react-native-maps (Google or Apple base maps) is acceptable for an MVP. Google Maps SDK mobile map loads are free and unlimited (prior knowledge, unverified), but there is no offline support.

### Gaps
- MapTiler and Protomaps hosted pricing was not fetched.
- The maplibre-react-native offline-pack API with PMTiles was not verified on iOS.
- The actual size of a UZ PMTiles extract was not measured.

## 4. Live traffic in Uzbekistan (Tashkent, Samarkand)

### Takeaway
Yandex is the clear leader: Tashkent traffic scores since January 2019, traffic-light timers, and lane-level maps for Tashkent, Andijan and Namangan as of 2026-09. 2GIS also operates locally. Google's live-traffic coverage in Uzbekistan could not be confirmed. Open-source engines have no traffic at all.

### Cited Findings
- Yandex has shown Uzbekistan traffic jams since January 2019, with a 0–10 congestion score for Tashkent shown in Yandex Maps and Navigator. Weekday jams build after 07:30 and peak around 09:00. — [UzDaily: Yandex traffic year results](https://www.uzdaily.ru/en/yandex-finds-out-how-traffic-jams-in-tashkent-changed-during-the-year)
- Yandex has real-time bus tracking in Tashkent (launched 2019). — [kun.uz 2019](https://kun.uz/en/news/2019/09/19/yandex-launches-real-time-bus-movement-tracking-service-in-tashkent)
- Yandex Uzbekistan launched real-time traffic-light signals in its navigation apps. — [UzDaily](https://www.uzdaily.ru/en/yandex-uzbekistan-launches-real-time-traffic-light-signals-in-navigation-apps)
- On 2026-09-23 Yandex Maps launched detailed lane-level road maps for Tashkent (all roads), Andijan and Namangan (centre and key highways). It covers 2,760 km digitised, with lane guidance and 3D landmarks, and works in Yandex Maps' Navigator tab, Yandex Navigator, CarPlay and Android Auto. Yandex says Uzbekistan is the first Central Asian country with this. — [spot.uz 2026-09-23](https://www.spot.uz/ru/2026/09/23/yandex-maps/); [UzA](https://uza.uz/ar/posts/yandex-maps-zapustil-detalizirovannuyu-navigaciyu-v-trex-gorodax-uzbekistana_912495)
- In 2018 Google Maps in Tashkent was reported as badly outdated. This is old evidence. — [spot.uz 2018](https://www.spot.uz/ru/2018/04/07/yandex/)

### Inferences
- For traffic-aware ETAs in Tashkent, handing guidance off to Yandex Navigator via deep link is the most realistic option. Samarkand traffic coverage is unconfirmed but plausible for Yandex given its data collection from users.
- Intercity driving (for example Tashkent–Samarkand on M39) is lightly congested, so traffic data matters mostly inside cities. Self-hosted routing without traffic is acceptable for intercity ETAs (inference).

### Gaps
- No source confirmed whether Google Maps shows live traffic in Uzbekistan in 2026.
- 2GIS traffic availability in Tashkent and Samarkand was not confirmed.
- No traffic API that a third party can buy for Uzbekistan was identified. HERE and TomTom were not checked.

## 5. Fuel (petrol, CNG/methane, LPG/propane) and EV charging data

### Takeaway
CNG is dominant and politically sensitive: 1,553 registered CNG stations, with periodic supply restrictions. Real-time availability exists only in government or local apps (Ministry of Energy "Avto24", U-Gas), not in open data. EV charging grew to about 1,400 stations by May 2025, roughly 70% of them in Tashkent city and region. Open Charge Map has an API (key needed), but its UZ coverage was not measured. OSM `amenity=fuel` is the free baseline.

### Cited Findings
- In November 2024 the Ministry of Energy introduced the "Avto24" app. It shows fuel, gas and EV charging stations, prices and real-time availability. — [kun.uz 2024-11-20](https://www.kun.uz/en/news/2024/11/20/uzbekistan-introduces-real-time-fuel-and-gas-station-tracking-app); background: [kun.uz 2024-10-07](https://kun.uz/en/news/2024/10/07/uzbekistan-to-launch-mobile-app-for-monitoring-gas-availability-at-fuel-stations), [kun.uz 2024-04-25](https://kun.uz/en/news/2024/04/25/energy-ministry-to-launch-a-mobile-app-providing-real-time-information-about-operating-methane-stations)
- The U-Gas system covers 1,553 registered CNG filling stations, 1,459 of them integrated into the operational system. — [gov.uz](https://gov.uz/en/cirns/news/view/195943)
- In December 2025 the Energy Ministry restricted CNG station operations because of reduced gas imports. This came from a search-result headline only. — search summary; primary article not fetched (flag).
- Third-party apps: "AvtoEnergy" ("largest gas stations map in Uzbekistan", with prices and routing) [App Store](https://apps.apple.com/us/app/avtoenergy/id6503910101); "Gaz Station" (crowd-sourced LPG/CNG) [App Store](https://apps.apple.com/uz/app/gaz-station/id918468977); myLPG.eu has an LPG station list for Uzbekistan [mylpg.eu](https://www.mylpg.eu/stations/uzbekistan)
- EV charging: 1,399 stations on 2025-05-01, 2.5x growth in a year, with Tashkent city at 820 and Tashkent region at 154 (about 70% of the total) and about 92,000 users. — [daryo.uz 2025-05-04](https://daryo.uz/en/2025/05/04/uzbekistan-sees-2-5-fold-surge-in-ev-charging-stations-1216442/); [kursiv.media](https://uz.kursiv.media/en/2025-05-04/uzbekistan-sees-number-of-ev-charging-stations-rise-2-5-times-in-one-year/)
- Targets: a 2024 government plan aimed for 32,400 stations by end-2025 [kun.uz](https://kun.uz/en/news/2024/11/05/uzbekistan-plans-to-install-over-32000-ev-charging-stations-by-the-end-of-2025). This was missed. A May 2026 draft targets 4,000 stations by end-2026, and the EV fleet passed 100,000 by end-2025. — [kun.uz 2026-05-05](https://kun.uz/en/news/2026/05/05/uzbekistan-to-expand-ev-charging-network-to-4000-stations-by-end-of-2026); EV count 73,000+ in Aug 2025 [daryo.uz](https://daryo.uz/en/2025/08/08/uzbekistans-ev-count-tops-73000-as-tashkent-leads-charge/)
- The Open Charge Map API (`api.openchargemap.io/v3/poi/?countrycode=UZ`) returns "You must specify an API key". A free key is required. — tested directly 2026-11-25.

### Inferences
- MVP approach: import OSM `amenity=fuel` (tags `fuel:cng`, `fuel:lpg`, `fuel:octane_*`, `brand`) and `amenity=charging_station` from the Geofabrik extract nightly in the Go backend. Enrich with Open Charge Map (free key, CC-BY-SA style licence; prior knowledge) and crowd-sourced "open now / gas available" reports. Approach the Ministry of Energy about Avto24 or U-Gas data sharing for real-time CNG status. Methane outages are a key traveller pain point.
- Because EV chargers are concentrated in Tashkent, intercity EV trips (Samarkand–Bukhara–Khiva) need range planning. Flag this in the app.

### Gaps
- OSM counts of fuel, CNG and EV stations in UZ could not be measured: the Overpass API returned errors and no local osmium was available. Run a count on the Geofabrik PBF.
- No public API for Avto24 or U-Gas was found. Local EV-charging operator apps (names, APIs) were not identified.
- Fuel-brand operators (Uzbekneftegaz, Mustang, Lukoil and others) and price data sources were not verified.
- No data.egov.uz datasets on fuel stations, roads or cameras were checked.
- Border-crossing status sources were not researched.

## 6. Uzbekistan driving rules for foreign travellers (licence, insurance, cameras, fines)

### Takeaway
Foreigners need a national licence plus a 1968-convention IDP (UK and Australian guidance). OSAGO third-party insurance is reportedly mandatory for foreign cars at the border from July 2025. Speeding fines are indexed to the base calculation amount (BCA), which rose to 440,000 UZS on 2026-09-01. From 2026-10-30, mobile speed cameras no longer need warning signs, which makes a speed-camera feature valuable but data-hungry.

### Cited Findings
- UK licence holders may drive for 30 days with both a UK licence and a 1968 IDP. — [GOV.UK living in Uzbekistan](https://www.gov.uk/guidance/living-in-uzbekistan); Australia also requires licence plus IDP — [Smartraveller](https://www.smartraveller.gov.au/destinations/uzbekistan)
- From July 1 (2025), foreigners must buy OSAGO at the border. The fine for driving uninsured is 375,000 UZS. This is a commercial source. — [eai.uz 2025 guide](https://eai.uz/en/articles/car-insurance-uzbekistan-guide-2025)
- A fine of 375,000 UZS applies for speeding up to 20 km/h over, or for no valid insurance. 40–60 km/h over costs 9 BCA (3,375,000 UZS, about $261 at the time). — [kun.uz](https://kun.uz/en/news/2025/07/09/speeding-fines-and-traffic-penalties-to-be-toughened-under-new-rules); [kursiv 2025-02-22](https://uz.kursiv.media/en/2025-02-22/uzbekistan-introduces-higher-road-fines/)
- BCA rises from 412,000 to 440,000 UZS from 2026-09-01, and traffic fines rise with it. — [kun.uz 2026-06-24](https://kun.uz/en/news/2026/06/24/traffic-fines-set-to-rise-in-uzbekistan-from-september-1-646781)
- Measurement tolerance is 5 km/h; a March 2025 proposal would raise it to 10 km/h (outcome not confirmed). — [kun.uz 2025-03-05](https://kun.uz/en/news/2025/03/05/margin-of-error-for-speed-measurement-may-increase-to-10-kmh)
- From 2026-10-30, the requirement to warn drivers about mobile speed cameras was removed. Stationary cameras still need signs. — [daryo.uz 2026-11-02](https://daryo.uz/en/2026/11/02/uzbekistan-removes-mandatory-warning-signs-for-mobile-speed-cameras/)
- Camera violations count toward driver penalty points (Oct 2025). 165,210 drivers had penalty points in 7 months of 2025. — [kun.uz 2025-10-27](https://kun.uz/en/news/2025/10/27/traffic-violations-caught-on-camera-to-count-toward-driver-penalty-points); [daryo.uz 2026-01-07](https://daryo.uz/2026/01/07/uzbekistan-registers-165210-drivers-with-penalty-points-in-7m25/)
- The government ordered removal of unauthorised or non-compliant traffic cameras and an inventory of road enforcement equipment by end-2025. — [kun.uz 2025-05-17](https://kun.uz/en/news/2025/05/17/government-to-dismantle-unauthorized-and-non-compliant-traffic-cameras)

### Inferences
- The 375,000 UZS figure was 1 BCA at the 375,000 level. Under the 440,000 BCA from 2026-09-01, the 20 km/h fine is probably about 440,000 UZS. This is unconfirmed; check the current Administrative Code table.
- Speed-camera POIs could come from OSM `highway=speed_camera` (coverage unknown) plus user reports. Yandex Navigator already warns about cameras, which is another argument for handing guidance off to it.

### Gaps
- No official (gov.uz or IIV) source was fetched for IDP or OSAGO rules or the current fine table.
- Rental-car requirements and border-crossing procedures for cars (Kazakhstan, Kyrgyzstan, Tajikistan, Turkmenistan) were not researched.
- data.egov.uz datasets were not checked.

## Recommendation summary for an early-stage startup (inference)
1. **Map display:** MapLibre RN with OpenFreeMap (free), plus self-hosted UZ PMTiles for offline regions. Alternatively, Yandex MapKit free tier (≤1,000 DAU) for best local data, accepting its terms.
2. **Route preview, ETA and walking routes:** self-hosted Valhalla or GraphHopper on a small VPS from the 120 MB Geofabrik extract, updated weekly. Fall back to Mapbox Directions (100k free/month) or Google Routes Essentials (10k free/month).
3. **Turn-by-turn and traffic:** deep link to Yandex Navigator (best Tashkent traffic, lanes, cameras), 2GIS, or Google Maps. Revisit the Google RN Navigation SDK (1,000 free destinations/month) or Mapbox Nav SDK only when in-app guidance becomes a differentiator.
4. **Fuel, CNG and EV:** OSM import, Open Charge Map (free key) and crowd reports. Pursue Ministry of Energy (Avto24/U-Gas) data partnership for live methane availability.
