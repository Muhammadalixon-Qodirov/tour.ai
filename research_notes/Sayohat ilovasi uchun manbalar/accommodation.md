# Accommodation data and booking/affiliate APIs for a travel-assistant app in Uzbekistan

Research date: 2026-11-25. Around 20 tool calls. Several official partner portals (Agoda FAQ, gov.uz register page, kursiv.media 2026 cost article) returned empty pages when fetched, so some figures come from search snippets of those pages and are flagged.

## 1. Booking.com: Affiliate Partner Programme and Demand API

### Takeaway
To use the Demand API (full content, availability and pricing) you must be a **Managed Affiliate Partner** under a signed contract with an account manager. That is out of reach for a pre-traffic startup. In May–June 2025 Booking.com dropped thousands of small direct affiliates and sent them to networks (Awin, CJ, and similar). For a startup MVP, the realistic route is links and widgets through a network such as Travelpayouts (4–5% of booking value), not the API.

### Cited Findings
- Demand API prerequisites: (1) "Registered as a Booking.com Managed Affiliate Partner"; (2) Partner Centre access "provided by your Booking.com Account Manager after signing the agreed contract"; (3) an API key token plus X-Affiliate-Id generated in Partner Centre. The prerequisites page gives no traffic threshold — [Booking.com Developers: Prerequisites](https://developers.booking.com/demand/docs/getting-started/prerequisites)
- An account can have at most 2 active API keys. Selling rates (pay_online_now) needs extra account enablement through the Account Manager, partner-agreement authorisation, and Demand API v3.1+ — [Booking.com Developers (search snippet)](https://developers.booking.com/demand/docs/accommodations/tpi/sell-how-to)
- Commission: affiliates get "from 25 to 40 percent" of the commission Booking.com earns, depending on monthly volume, and it varies by property type and location. This comes from an undated third-party article, so it is unverified — [AltexSoft](https://altexsoft.com/blog/booking-com-partnerships-apis-extranet-pulse-app)
- May 2025: Booking.com ended partnerships with "thousands" of smaller affiliates (bloggers and creators), effective June 20, 2025, on 30 days' notice. Smaller partners were pointed to CJ, Awin, Affnetmedia and others. The site listed >17,500 affiliate partners — [Skift, 2025-05-30](https://skift.com/2025/05/30/why-booking-com-cut-thousands-of-affiliate-partners-and-what-comes-next/)
- Booking.com said the network move "might entail services and commission changes". Coverage disagrees on whether the network is Awin or CJ, and one guide cites 4% on completed stays through CJ — [NetInfluencer](https://www.netinfluencer.com/booking-com-moves-all-affiliates-to-awin-what-to-know-about-termination-notices); [PhocusWire](https://www.phocuswire.com/booking-terminates-partnerships-content-creators)
- Booking.com through Travelpayouts (page viewed Nov 2026): headline "5% on completed hotel stays", but the terms section says "maximum of 4% of total transaction value per stayed reservation". One-session cookie. Confirmation takes 60–90 days after checkout. Traffic only from "permitted countries" (examples: USA, Canada, Germany, France; full list in the terms). Only content websites qualify, and social media needs approval. Tools: links, banners, widgets, LinkSwitcher. **No API mentioned** — [Travelpayouts: Booking.com offer](https://www.travelpayouts.com/en/offers/bookingcom-affiliate-program)
- Uzbekistan inventory on Booking.com: country pages show "3116 hotels and other places to stay" on one capture and 2951 on another, with Tashkent at 1623. A Feb 2024 Daryo study counted 1,130 properties typed as hotels, guest houses, hostels or B&Bs — [Booking.com country page](https://www.booking.com/country/uz.html); [Daryo 2024-02-02](https://daryo.uz/en/2024/02/02/uzbekistan-emerges-as-central-asias-leader-in-top-rated-hotels)

### Inferences
- Booking.com inventory in Uzbekistan is about 3,000 properties (2025–2026 captures). It is probably the largest international OTA supply there, which makes it the most valuable link target.
- Uzbekistan users may not be a "permitted country" traffic source under the Travelpayouts Booking.com terms. Check this before building on it.
- Without Demand API access, an app cannot legally pull Booking.com prices or photos into its own UI. Deep links and widgets are the only option.

### Gaps
- Official 2025–2026 Managed Affiliate commission tiers were not published, and no minimum traffic for managed status was found.
- Demand API content caching and storage rules were not found on the public prerequisites page.
- Whether Uzbekistan-origin traffic is allowed under the Booking.com network programmes is unconfirmed.

## 2. Expedia (Rapid API / EPS), Agoda, Trip.com, Hotelbeds/WebBeds, ETG/RateHawk/Ostrovok, Yandex Travel, Travelpayouts, Hostelworld

### Takeaway
Every true inventory API (Expedia Rapid, Agoda Demand/Partner API, Hotelbeds, RateHawk/ETG) needs a commercial partnership plus a certification or launch review, and B2B bed banks usually also require deposit or credit terms and a registered travel business. Affiliate links and widgets (Travelpayouts, Agoda affiliate, Hostelworld affiliate) have no traffic minimum and pay about 3–6% of booking value, or 40% of Hostelworld's deposit.

### Cited Findings
**Expedia Group Rapid API**
- You must become a partner before integrating. Expedia reviews your booking product against launch requirements, which differ for B2C and B2B. Requirements include PCI compliance (unless you use EPS Checkout), TripAdvisor content terms, search and results display rules, SSL, and fee/tax display. A B2C rule says partners "must not use the word 'fee'" for their own charges — [Expedia Developer Hub: B2C launch reqs](https://developers.expediagroup.com/rapid/setup/launch-requirements/b2c-standalone); [Expedia Partner: Rapid API](https://partner.expediagroup.com/en-us/join-us/rapid-api); [AltexSoft](https://www.altexsoft.com/blog/travel/expedia-partner-solutions/)
- The Rapid Car API is in beta, with a full launch expected in 2026 — (search snippet, Expedia Developer Hub car launch requirements) [link](https://developers.expediagroup.com/rapid/setup/launch-requirements/car-launch-reqs)
- Expedia affiliate through Travelpayouts: 3.4% of booking price, for UK partners only (Aug 2023 article). Other regions join through Expedia's own affiliate site — [Travelpayouts blog, updated 2023-08-11](https://travelpayouts.com/blog/?p=19263)

**Agoda**
- Agoda's partner FAQ (snippet; the page did not render on fetch) describes a "progressive" model of "up to 60% on margin", paid for completed stays — [Agoda Partners FAQ](https://partners.agoda.com/en-us/faq.html)
- Third-party sources give flat figures instead: 6% of the amount paid excluding taxes with a 1-day cookie ([Travelpayouts 2023](https://travelpayouts.com/blog/?p=19263)), "up to 5%" (other Travelpayouts pages), and 4.8% ([Ecomobi](https://ecomobi.com/agoda-affiliate-program-earn-4-8-commission-with-ecomobi/)). These conflict.
- Agoda provides an Affiliate Lite API (v2.0 PDF on partners.agoda.com) and Demand-side Search/Content APIs. The docs strongly recommend daily content refreshes, and rate limits (HTTP 429) are set at onboarding — [Affiliate Lite API V2.0](https://partners.agoda.com/Content/Documents/AffiliateLiteApi/Affiliate_Lite_API_V2.0.pdf); [Agoda developer docs](https://developer.agoda.com/demand/docs); [PPS FAQ](https://developer.pps.agoda.com/docs/faq)
- ETG and Agoda announced a strategic partnership — [Hospitality Net](https://www.hospitalitynet.org/news/4134946/emerging-travel-group-and-agoda-enter-strategic-partnership)

**Trip.com**
- Through Travelpayouts: commission "varies by service; up to 5% for hotels" (2023) — [Travelpayouts blog](https://travelpayouts.com/blog/?p=19263)

**Hotelbeds (bed bank)**
- The developer portal lists Booking API, Cache API and Content API, with a certification and go-live process. Access is approval-based, and startups should expect "deposit, credit, or prepaid booking terms". No public minimum volume. This is a single vendor blog (May 2026), so it is weak evidence — [phptravels blog](https://phptravels.com/blog/travel-api-suppliers)
- WebBeds: no information found.

**Emerging Travel Group (RateHawk B2B / ZenHotels B2C / Ostrovok)**
- ETG API v3 is a REST/JSON API with HTTP Basic auth (key ID + API key). Sandbox is api-sandbox.ratehawk.com, production is api.ratehawk.com. Static dumps plus a Content API are offered for hotel content — [ETG API docs](https://docs.emergingtravel.com/docs); [ETG llms-full](https://docs.emergingtravel.com/llms-full.txt)
- Inventory claims range from "2,500,000 hotels" to "2.6 million" to "1.5+ million", depending on page and date — [RateHawk](https://www.ratehawk.com/lp/zh_CN/)
- RateHawk is aimed at travel professionals (agencies, tour operators, corporates). It is B2B, not affiliate — [RateHawk TA/TO page](https://www.ratehawk.com/lp/pt_PT/to-and-ta/)
- ETG reportedly intends to sell Ostrovok by end-2026 — [Izvestia (en)](https://iz.ru/en/node/1984944)
- Sanctions: since late Oct 2025, ZenHotels reportedly stopped showing Russian properties to users in the EU/UK (19th EU sanctions package) but still shows them elsewhere. This is a secondary blog, unverified — [MoscowPass blog](https://moscowpass.com/blog/how-to-book-accommodation-in-russia-best-alternatives-to-zenhotels-booking-and-airbnb)
- Uzbekistan coverage by ETG/RateHawk: **no source found** confirming the size of its Uzbek inventory.

**Yandex Travel**
- The affiliate programme pays for hotel bookings made on partner subdomains (white-label) once the guest checks out. Minimum monthly payout is ₽3,000. Open to Russian residents and non-residents, legal entities and individual entrepreneurs — [Yandex Travel affiliate help](https://yandex.com/support/travel-distr); [Payment of remuneration](https://m.yandex.ru/support/travel-distr/en/wl/reward)
- Through Travelpayouts: 4% of the completed stay value (circa 2022 Russian-language page). Ostrovok is reached through Travelpayouts' "Hotellook" programme — [Travelpayouts RU](https://www.travelpayouts.com/ru/offers/yandex-travel-partnerskaya-programma); [Travelpayouts RU blog](https://www.travelpayouts.com/ru/blog/booking-script-by-travelpayouts/)

**Travelpayouts hotel brands (Aug 2023 snapshot)**
- Booking.com 4% (session cookie). Agoda 6% (1-day cookie). Hostelworld 40% of Hostelworld's booking revenue. Trip.com up to 5%. Tripadvisor hotels 50% of referral fee per click (14-day cookie). Expedia 3.4% (UK only). OYO/Super Travel 4.5–9% by country. VRBO unspecified — [Travelpayouts: best hotel affiliate programs](https://travelpayouts.com/blog/?p=19263)
- Hotel commissions are paid after the guest checks out — [Travelpayouts blog](https://travelpayouts.com/blog/selecting-hotel-affiliate-program/)

**Hostelworld**
- Free to join. CPA commission is a percentage of Hostelworld's deposit, with last-click attribution and a 30-day window. The API for live property/destination data is offered "on a case by case basis" — [Hostelworld Partners FAQ](https://partners.hostelworld.com/faqs); [Hostelworld Partners](https://www.partners.hostelworld.com)
- Rate figures conflict: 40% of the deposit ([Travelpayouts](https://www.travelpayouts.com/en/offers/hostelworld-affiliate-program)) versus 18–22% ([UpPromote](https://uppromote.com/affiliate-directory/hostelworld/)). Payout minimum is about $30–50 depending on network.

### Inferences
- An MVP in Uzbekistan can join Travelpayouts (which bundles Booking.com, Agoda, Trip.com, Hostelworld, Yandex/Ostrovok links) with no traffic minimum. The trade-off is that you get links and widgets, not raw price data, and earn about 4–6% of booking value. On a $30/night guesthouse for 3 nights, that is roughly $4–5 per booking.
- Real-time prices inside the app (search, compare) require Agoda's affiliate API, Expedia Rapid, or a B2B supplier such as RateHawk/Hotelbeds. These need a legal entity, contract and certification, and for B2B often a deposit. That is a phase-2 option after traction.
- Travelpayouts also offers Hotellook-style data APIs historically, but this was not verified in this session.

### Gaps
- No official Agoda tier table and no Trip.com direct affiliate terms for 2025–2026.
- Caching and storage terms (how long prices or content may be stored) were not found for Expedia, Agoda, Hotelbeds or ETG. General pattern: content refreshed daily, and rates must not be cached beyond short TTLs (Agoda docs recommend daily refresh). Specific TTLs are unconfirmed.
- Number of Uzbek properties on Agoda, Expedia, Trip.com, RateHawk and Hostelworld was not found.
- WebBeds, and Trip.com/Ctrip partner API access requirements, were not researched successfully.

## 3. Airbnb: options without a public API

### Takeaway
Airbnb has no open API and no general affiliate programme. A startup can link out to Airbnb listing or search pages without commission, or reach vacation-rental inventory through VRBO (via Expedia/Travelpayouts) or Booking.com apartments.

### Cited Findings
- In 2017 Airbnb reopened affiliates only to platforms with ≥1M monthly visitors — [TechCrunch 2017-10-16](https://techcrunch.com/2017/10/16/airbnb-eyes-expansion-with-affiliate-program-for-sites-with-1m-users-new-api/)
- The programme (later "Airbnb Associates") was discontinued, around 2021 per one guide. No 2025 relaunch was found — [PriceLabs](https://hello.pricelabs.co/airbnb-affiliate-program/)
- Airbnb's API terms (updated Oct 15, 2025) let Airbnb limit or suspend access at its discretion. API access is for approved software partners (hosts/PMS), not demand-side apps — [Airbnb API Terms](https://www.airbnb.com/terms/api)

### Inferences
- Scraping Airbnb would breach its terms. Avoid it.

### Gaps
- Airbnb Uzbekistan listing count was not found.

## 4. Local Uzbek sources: registry, E-Mehmon, open data

### Takeaway
The Tourism Committee (Uzbektourism) publishes a "Register of accommodation facilities" (Joylashtirish vositalarining reyestri) on gov.uz, listing name, owner, address, contacts, certificate number and validity. E-Mehmon is a closed government guest-registration system, not open data. No machine-readable hotel dataset on data.egov.uz was confirmed.

### Cited Findings
- Register page: gov.uz/oz/uzbektourism/pages/joylashtirish-vositalarining-reyestri. A search snippet describes a table with facility name, owner, address, contacts, certificate number and validity dates, and some entries have expired. The page did not render on fetch (JS-loaded), so field list and size are unverified — [gov.uz Uzbektourism register](https://gov.uz/oz/uzbektourism/pages/joylashtirish-vositalarining-reyestri)
- E-MEHMON is a centralized information and analytical system built by UZINFOCOM for temporary registration of visitors at accommodation facilities and for tourism management. It was piloted in Tashkent in April 2016 — [UZINFOCOM: E-MEHMON](https://uzinfocom.uz/uz/projects/e-mehmon-uz-latin-11); [old.gov.uz](https://old.gov.uz/en/news/view/28557)
- The national open data portal is data.egov.uz. No hotel/guesthouse dataset was confirmed — [egov.uz open data](https://egov.uz/en/projects/open-data)
- Statistics: siat.stat.uz has "number of specialized accommodation facilities" and "collective accommodation facilities — persons accommodated" tables. The 2025 "Development of Tourism and Recreation" press release was published 2026-08-10 — [SIAT 3962](https://siat.stat.uz/data/3962/?lang=en); [SIAT 3340](https://siat.stat.uz/data/3340/?lang=en); [stat.uz announcement](https://stat.uz/en/press-center/announcementss/69738-attention-to-users-10-08-2026-1)
- In 2025, 954 new accommodation facilities (hotels, family guesthouses, hostels) opened. As of 2025-12-31 there were 40 international-brand hotels (25 brands) with 5,191 rooms and 9,340 beds. 11.7M foreign tourists arrived in 2025 (preliminary) — [gov.uz Uzbektourism news](https://gov.uz/en/uzbektourism/news/view/212955); [Euronews 2026-02-18](https://www.euronews.com/2026/02/18/uzbekistan-targets-12-million-tourists-in-2026-as-ai-reshapes-hospitality)
- Tourist levy per foreign adult per night: guesthouse 8,800 som (~$0.75); hotel up to 10 rooms 22,000 som (~$1.87); 11–40 rooms ~$3.73 (figures as reported by a travel-cost guide) — [Journal of Nomads 2026 money guide](https://www.journalofnomads.com/uzbekistan-travel-costs-budget-guide/)

### Inferences
- The gov.uz register could seed a "verified/licensed guesthouse" layer (name, address, phone, certificate) for the app, with no prices or availability. It probably needs scraping or a formal data request to the Tourism Committee, and the licence terms for reuse are unclear.
- The stated total number of facilities is probably in the SIAT tables. Not extracted here.

### Gaps
- Exact register size, download format and reuse licence.
- Whether data.egov.uz has an accommodation dataset.
- Local Uzbek booking platforms (none found in this session). Worth a dedicated search.

## 5. Typical nightly prices (2024–2026 sources)

### Takeaway
Hostel dorm beds cost about $6–15. Budget guesthouse or private hostel rooms cost about $10–25. Mid-range guesthouses and hotels with breakfast cost about $20–45 in Samarkand, Bukhara and Khiva, reaching $50–80 for better boutique places. Tashkent mid-range is higher, about $45–60+. Sources are listing snapshots and travel guides of mixed dates.

### Cited Findings
- Uzbekistan average private room in a hostel or guesthouse is about $14/night, based on 53 properties — [BudgetYourTrip](https://www.budgetyourtrip.com/hostels/uzbekistan-UZ)
- Bukhara hostels from $6/night (undated). Bukhara guesthouse example (Poyi Kalon Terrace): $75, or $86 with taxes — [Hostelz Bukhara](https://www.hostelz.com/en/hostels/Uzbekistan/Bukhara); [Travelocity](https://www.travelocity.com/Uzbekistan-Guest-Houses.d11412-aaGuestHouses.Travel-Guide-Accommodation)
- Samarkand hostels about $19–45 on Booking.com (one outlier at $116). Guesthouses about $22–40 before tax — [Booking.com Samarkand hostels](https://www.booking.com/hostels/city/uz/samarqand.html); [Orbitz](https://www.orbitz.com/Uzbekistan-Guest-Houses.d11412-aaGuestHouses.Travel-Guide-Accommodation)
- Mid-range hotel or guesthouse private room in Silk Road cities: €18–40 (US$19–42) with breakfast and private bathroom. Boutique from about €30. Hostel/economy private room with shared bath from about US$16 (2026 guide) — [Journal of Nomads 2026](https://www.journalofnomads.com/uzbekistan-travel-costs-budget-guide/)
- Khiva Itchan Kala guesthouses: 120,000–200,000 som (~$10–16), from a 2023-based guide. Another guide gives a Khiva average hotel rate of about $52 — [Journal of Nomads](https://www.journalofnomads.com/travel-costs-uzbekistan-money-guide/); [Champion Traveler Khiva](https://championtraveler.com/price/cost-of-a-trip-to-khiva-uz)
- Tashkent: some Booking.com hotels from about $46–57. 1-star rooms about $23–29 — [Champion Traveler Tashkent](https://championtraveler.com/price/cost-of-a-trip-to-tashkent-uz)
- Hostel dorms in major cities about $9–15 — [Lonely Planet: Uzbekistan on a budget](https://www.lonelyplanet.com/articles/uzbekistan-on-a-budget)

### Inferences
- With a 4–6% commission, a typical 3-night guesthouse stay ($60–120) earns about $3–7. Hostelworld bookings earn very little in absolute terms. Monetisation needs volume or higher-value hotels.

### Gaps
- No single authoritative 2026 price index by city and category. The Kursiv "Uzbekistan on a budget 2026" article (2026-09-04) did not render: [Kursiv](https://uz.kursiv.media/en/2026-09-04/uzbekistan-travel-cost-budget-2026/)
- Seasonality premiums (Apr–May and Sep–Oct peak) were not quantified.

## 6. Realistic MVP option, approval difficulty, and storage/caching terms

### Takeaway
For an early-stage MVP, start with Travelpayouts (one sign-up, many brands, no traffic minimum). Use Booking.com, Agoda and Hostelworld deep links and widgets, plus your own curated content (gov.uz register, own photos and descriptions). Apply for Agoda's affiliate API and Hostelworld's API case by case once you have traffic. Expedia Rapid, Booking Demand API, Hotelbeds and RateHawk are phase 2–3: they need a contract, legal entity, certification and often deposits.

### Cited Findings
- Booking Demand API: contract plus Account Manager only — [Booking.com Developers](https://developers.booking.com/demand/docs/getting-started/prerequisites)
- Booking.com dropped small direct affiliates in 2025 — [Skift](https://skift.com/2025/05/30/why-booking-com-cut-thousands-of-affiliate-partners-and-what-comes-next/)
- Travelpayouts Booking.com: content sites only, social needs approval, permitted countries only, one-session cookie — [Travelpayouts](https://www.travelpayouts.com/en/offers/bookingcom-affiliate-program)
- Expedia Rapid: partnership plus launch review, and PCI unless using EPS Checkout — [Expedia Developer Hub](https://developers.expediagroup.com/rapid/setup/launch-requirements/b2c-standalone)
- Hotelbeds: approval plus deposit/credit/prepaid terms — [phptravels](https://phptravels.com/blog/travel-api-suppliers)
- Agoda: rate limits set at onboarding, daily content refresh recommended — [Agoda PPS FAQ](https://developer.pps.agoda.com/docs/faq)
- Hostelworld API: case by case — [Hostelworld Partners](https://www.partners.hostelworld.com)

### Inferences
- A mobile app (rather than a content website) may not qualify for some programmes. Travelpayouts' Booking.com terms say "content websites" only, and apps or social traffic need approval. Confirm app eligibility before building.
- Typical industry pattern (not confirmed by sources here): supplier static content (photos, descriptions) may be cached and refreshed periodically, while rates and availability must be fetched live or cached only briefly, and must be removed when the partnership ends. Confirm in each contract.

### Gaps
- Explicit caching TTLs and content-retention clauses for every provider.
- Whether Uzbekistan-registered entities are accepted by Expedia, Booking and Agoda, and how payouts work (USD to Uzbek bank, Payoneer/Wise availability). Not researched.
