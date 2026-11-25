# Sourcing "hidden gem" places in Uzbekistan from Instagram/Telegram for a travel app: legal routes, API limits, scraping risk, and alternatives (as of Nov 2026)

## 1. Instagram / Meta APIs (2025-2026): what a third-party app can read

### Takeaway
No official Meta API lets a third-party app browse public Instagram by place or keyword. The only official read paths are:
- **Hashtag Search.** Needs App Review for "Instagram Public Content Access" and runs through a Business/Creator account, capped at 30 unique hashtags per rolling 7 days.
- **Business Discovery.** Reads the public profile and media of another Business/Creator account when you know its username.
- **oEmbed.** Embeds individual public posts. Since June 2026 it needs no token and no App Review.

The Basic Display API (consumer accounts) is gone since 4 Dec 2024. Location search is effectively dead.

### Cited Findings
- **Hashtag Search limit.** One Business or Creator account can query at most 30 unique hashtags in a rolling 7-day period. Repeat queries of the same hashtag inside that window don't count again and don't reset the timer. The `recently_searched_hashtags` edge lists what the account has queried in the last 7 days. — [Meta: Hashtag Search](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/hashtag-search); [Recently Searched Hashtags](https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/recently_searched_hashtags)
- **What counts as a query.** Calling the `recent_media` or `top_media` edges counts as querying that hashtag, even if you already have its ID. — [IG Hashtag Recent Media](https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-hashtag/recent-media); [IG Hashtag Top Media](https://developers.facebook.com/documentation/instagram-platform/instagram-graph-api/reference/ig-hashtag/top-media)
- **Hashtag Search requirements.** It needs App Review approval for the "Instagram Public Content Access" feature plus the `instagram_basic` permission. Other limits: no emoji hashtags, no Story hashtags, and you can't comment on media found this way. — [Meta: Hashtag Search](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/hashtag-search)
- **Basic Display API is shut down.** After 4 Dec 2024, requests to it return errors and consumer (personal-account) apps have no Instagram API left. Meta points developers to "Instagram API with Instagram Login" or "Instagram API with Facebook Login". Both are for professional (Business/Creator) accounts managing their own presence. — [Meta blog, Sept 2024](https://developers.facebook.com/blog/post/2024/09/04/update-on-instagram-basic-display-api); [Instagram API with Instagram Login launch, July 2024](https://developers.facebook.com/blog/post/2024/07/30/instagram-api-with-instagram-login/)
- **Business Discovery rate limit.** The Instagram Platform changelog records an increase from 240 to 1,000 queries per user per hour, and a new `view_count` field for Reels. I couldn't confirm whether Business Discovery is available under the Instagram Login product or only under Facebook Login. — [Changelog for Instagram Platform](https://developers.facebook.com/docs/instagram-platform/changelog)
- **oEmbed change (15 June 2026).** Meta made the oEmbed APIs (Instagram, Threads, Facebook Post, Facebook Video) callable without an access token. App Review for "Meta oEmbed Read" is no longer required to embed public content. Example call: `GET https://graph.facebook.com/v25.0/instagram_oembed?url=...`.
  - The response returns `html` embed code, `width`, `type` and `provider_*` fields. The post mentions no thumbnail or author fields.
  - "No personal data is shared."
  - Use remains subject to the Meta Platform Terms, Developer Policies and Community Standards.
  - Token-based access may get higher rate limits.
  - This replaces the older position that oEmbed requires App Review, so treat pre-2026 sources as outdated on this point. — [Meta blog, 15 Jun 2026](https://developers.facebook.com/blog/post/2026/06/15/tokenless-access-to-meta-oembed-apis/); older requirement: [Embed an Instagram Post](https://developers.facebook.com/docs/instagram-platform/oembed)
- **oEmbed allowed use.** The Meta oEmbed Read reference lists allowed usage as front-end display of public Facebook and Instagram pages, posts and videos. In other words, embedding to show a post, not harvesting data from it. — [Meta oEmbed Read](https://developers.facebook.com/docs/features-reference/meta-oembed-read)
- **Location search.** Meta's 2018 announcement deprecated Public Content (including Hashtag Search and Location information) from the legacy Instagram API Platform on 11 Dec 2018. The legacy Facebook `GET /search?type=place` endpoint was deprecated from v8.0 and in all versions on 2 Nov 2020. Facebook Pages search (docs updated Apr 2026) can return a Page's physical location. — [Meta 2018 platform changes](https://developers.facebook.com/blog/post/2018/04/04/facebook-api-platform-product-changes/); [Pages API: Search Pages](https://developers.facebook.com/docs/pages-api/search-pages)
- **Automated collection rule.** Meta's developer terms say Platform APIs are "the only allowable means to access data programmatically, unless you have our prior written permission". The rule reads "You may not access or collect data from our Products using automated means (without our prior permission)". — [Meta: Automated Data Collection](https://developers.facebook.com/docs/development/terms-and-policies/automated-data-collection/); [Meta Platform Terms](https://developers.facebook.com/terms)

### Inferences
- **Hashtag Search is too small for discovery.** Thirty hashtags a week per account is enough to monitor a curated set of tags, for example #tashkentcafe, #uzbekistanhiddenplaces, regional tags and new-opening tags. It won't support broad discovery. It also needs a Business/Creator account plus a passed App Review, and Meta reviews hashtag-access use cases closely.
- **Business Discovery is the realistic pull mechanism.** Once a cafe, guesthouse or blogger is known, the app can read their public posts via Business Discovery. Better still, the owner connects their own professional account via Instagram Login, which is consent-based.
- **oEmbed is for display, not for building a database.** Embed the post as attribution and social proof next to a place card. Don't use it as a data source.
- **No official way to find new places by geography.** There is no supported "posts near this location" API, so geographic discovery of new places on Instagram can't be done legally through the API.

### Gaps
- I didn't retrieve the exact current Meta Platform Terms clauses on data retention and caching (for example "delete when no longer needed" or "no selling or licensing of Platform Data"). Read [developers.facebook.com/terms](https://developers.facebook.com/terms) directly before designing caching.
- I didn't confirm the exact returned fields and time window of hashtag `recent_media`, which I believe is limited to recent media, possibly 24h. Check the docs.
- I found no official 2025-2026 statement on whether any Instagram location or media-by-area endpoint survives. Assume it doesn't.
- I didn't verify the exact current wording of the Instagram Terms of Use anti-scraping clause. The search showed that Instagram's own terms say the general Meta ToS "do not apply to this Service", so the Instagram text must be checked separately at help.instagram.com.

## 2. Scraping risk: platform terms, court cases (Meta v. Bright Data, hiQ v. LinkedIn), and Uzbekistan personal-data law

### Takeaway
- **US case law.** It suggests that scraping truly public, logged-out pages may not breach Meta's terms (Bright Data, 2024, district court only, no appeal). Scraping while logged in, or with fake accounts, does breach user agreements (hiQ, 2022: $500k judgment, injunction, data destroyed).
- **Terms and enforcement.** Meta and Telegram both prohibit unauthorized automated collection, and the practical enforcement risk is account or API bans.
- **Uzbekistan.** The 2021 blanket localization rule was narrowed by Law No. 1125 (26-27 March 2026). Only biometric/genetic data and telecom subscriber data must now be stored in Uzbekistan. Other personal data can go to 49 "adequate" countries, including the US, UK and Germany.

### Cited Findings
- **Meta v. Bright Data, ruling.** On 23 Jan 2024, Judge Edward Chen (N.D. Cal.) granted summary judgment to Bright Data. He held that Meta's terms didn't prohibit scraping of publicly available data while logged off, because the terms bind users only when they are logged in and using their accounts. Meta's evidence of logged-in scraping (a "scraping browser" that automated logins) was found insufficient. — [TechCrunch, 24 Jan 2024](https://techcrunch.com/2024/01/24/court-rules-in-favor-of-a-web-scraper-bright-data-which-meta-had-used-and-then-sued); [Farella Braun + Martel](https://www.fbm.com/technology/publications/major-decision-affects-law-of-scraping-and-online-data-collection-meta-platforms-v-bright-data/)
- **Meta v. Bright Data, dismissal.** On 23 Feb 2024, Meta dismissed its remaining claim (tortious interference) without prejudice and waived its right to appeal. Bright Data said there was no settlement and no change to its conduct. Commentators call it the "most important" scraping opinion to date, but without appellate review it is not settled law. — [TechCrunch, 26 Feb 2024](https://techcrunch.com/2024/02/26/meta-drops-lawsuit-against-web-scraping-firm-bright-data-that-sold-millions-of-instagram-records/); [Eric Goldman blog](https://blog.ericgoldman.org/archives/2024/02/facebook-drops-anti-scraping-lawsuit-against-bright-data-guest-blog-post.htm)
- **hiQ v. LinkedIn, contract ruling.** In Nov 2022 the court found hiQ breached LinkedIn's User Agreement through automated scraping and by using crowdsourced workers with fake profiles. — [Morgan Lewis, Dec 2022](https://www.morganlewis.com/blogs/sourcingatmorganlewis/2022/12/linkedin-v-hiq-landmark-data-scraping-suit-provides-guidance-to-data-scrapers-and-web-operators)
- **hiQ v. LinkedIn, consent judgment.** It was entered on 8 Dec 2022 and included:
  - a reported $500,000 judgment against hiQ;
  - a permanent injunction against scraping and creating fake accounts;
  - destruction of the scraped data, source code and algorithms.
  
  A stipulated judgment has no precedential value. — [Proskauer New Media Law](https://newmedialaw.proskauer.com/2022/12/08/hiq-and-linkedin-reach-proposed-settlement-in-landmark-scraping-case/); [Morgan Lewis](https://www.morganlewis.com/blogs/sourcingatmorganlewis/2022/12/linkedin-v-hiq-landmark-data-scraping-suit-provides-guidance-to-data-scrapers-and-web-operators)
- **Meta terms.** They require prior written permission for any automated data collection outside the Platform APIs. — [Meta: Automated Data Collection](https://developers.facebook.com/docs/development/terms-and-policies/automated-data-collection/)
- **Uzbekistan, 2021 rule.** Article 27-1 was added to the Law "On Personal Data" by Law No. ZRU-666 of 14 Jan 2021, in force 16 Apr 2021. It required personal data of Uzbek citizens to be collected and stored on technical means physically located in Uzbekistan and registered in the state registry. Breaching it is an administrative offence. — [Mondaq](https://www.mondaq.com/data-protection/1197754/the-law-on-personal-data-has-been-amended); [DataGuidance](https://www.dataguidance.com/news/uzbekistan-data-localisation-rule-enters-force); [EY Uzbekistan, Dec 2021](https://ey.com/en_uz/tax-alerts/2021/12/developments-in-the-field-of-protection-of-personal-data)
- **Uzbekistan, enforcement history.** In Nov 2021, major social networks and messengers were briefly blocked in Uzbekistan. Apple Pay and Google Pay were reported to have stayed out partly because of the localization rule. — [Kun.uz, Dec 2024](https://kun.uz/en/news/2024/12/26/uzbekistans-data-localization-law-hinders-entry-of-apple-pay-and-google-pay); [Kun.uz, Nov 2021](https://kun.uz/en/news/2021/11/11/media-illogical-law-on-personal-data-must-be-amended)
- **Uzbekistan, the 2026 change (current law; the 2021 framing is outdated).** Law No. 1125 was adopted on 26 March 2026 and took effect on publication on 27 March 2026.
  - Mandatory localization now applies only to biometric and genetic data and to telecom operators' subscriber data.
  - Other categories may be stored abroad in states recognized as giving adequate protection. — [Kun.uz, 27 Mar 2026](https://kun.uz/en/news/2026/03/27/uzbekistan-amends-personal-data-law-to-facilitate-global-payment-systems); [Daryo, 29 Mar 2026](https://daryo.uz/en/2026/03/29/uzbekistan-amends-personal-data-law-for-domestic-storage-and-regulated-foreign-processing/)
- **Uzbekistan, July 2026 implementing resolution** (dated 29 July, on lex.uz/ru/docs/8369688).
  - It approves 49 countries for simplified transfers, including the US, Canada, the UK, Germany, France, Italy, Spain, the Netherlands, Switzerland, the Nordics, Japan, South Korea, Singapore, Israel, New Zealand and Russia. The EU as a bloc is not listed.
  - Operators must report breaches during cross-border transfers within 24 hours, with a follow-up report on cause and remedies within 72 hours.
  - Transfers to unlisted countries require legal, organizational and technical safeguards. Standard contractual clauses and corporate rules are to be approved by the Interior Ministry, the Ministry of Digital Technologies, the State Security Service and the National Agency for Prospective Projects. — [Kun.uz, 4 Aug 2026](https://kun.uz/en/news/2026/08/04/uzbekistan-approves-49-countries-for-simplified-personal-data-transfers)

### Inferences
- **Scraping is a business risk, not only a legal one.** Scraping Instagram or Telegram for a commercial product means violating platform terms. The likely consequences are account, IP or API bans, losing App Review status, and cease-and-desist letters. Logged-in scraping, or anything with fake or automated accounts, is the clearly losing position (hiQ). The logged-off "public data" defence comes from one US district ruling and is untested in Uzbekistan.
- **Scraped posts carry personal data and copyright.** They contain names, faces and handles, which are personal data under Uzbek law. Bloggers' photos and text are also copyrighted, so republishing them in an app without a licence is a separate risk from data protection.
- **What the 2026 rules mean for this app.** User accounts (name, phone, email) could probably be hosted with a provider in one of the 49 listed countries, for example US or German cloud regions. Biometric data (such as face-verification selfies) must stay in Uzbekistan, and so must anything obtained from telecom subscriber records. Confirm against the lex.uz text with local counsel. The safest option is still a local or hybrid hosting setup.

### Gaps
- I didn't read the full text of Law No. 1125 or the July 2026 resolution, so the exact scope of "subscriber data" and any registration or notification duties for database owners need checking on lex.uz.
- I found no Uzbek court cases on scraping social media.
- I didn't research Uzbek copyright law (Law on Copyright and Related Rights) on reposting social-media photos.

## 3. Telegram: Bot API vs MTProto/TDLib, API terms, ban risk, and Uzbek travel channels

### Takeaway
- **Both Telegram routes forbid scraping.**
  - The Bot Platform terms explicitly prohibit "scraping public group or channel content" and building large datasets or AI products.
  - The client API terms prohibit using or aggregating Telegram data to train or deploy AI/ML models.
  - Breaches lead to termination, or loss of API access after a 10-day notice for client apps.
- **Partnering is the legitimate route.** The channel owner adds the app's bot as an admin to their channel or a dedicated feed channel, or forwards posts to the bot with consent.

### Cited Findings
- **Client API terms, AI and data.** Section 1.5 prohibits "using, accessing or aggregating data obtained from the Telegram platform" to train, fine-tune, develop or deploy AI/ML models. API use is also subject to Telegram's "Terms of Service for Content Licensing and AI Scraping". — [Telegram API Terms of Service](https://core.telegram.org/api/terms)
- **Client API terms, other rules.**
  - Each app needs its own `api_id` (2.1) and must disclose that it uses the Telegram API (2.2).
  - It can't put "Telegram" in its name unless preceded by "Unofficial" (2.3).
  - Apps that access channel content must support official sponsored messages (3.3).
  - On a breach, Telegram notifies the responsible account. If the problem isn't fixed in 10 days, Telegram may cut API access and contact app stores (4.1-4.2). — [Telegram API Terms of Service](https://core.telegram.org/api/terms)
- **Bot Platform Developer Terms, §4.3 "Data Scraping".**
  - Bots must not "collect, store, aggregate or process data beyond what is essential for the operation of your services".
  - Prohibited: "any form of data collection aimed at creating large datasets, machine learning models and AI products", including scraping public group or channel content.
  - Data that users submit directly to the bot may be used if the bot explains its use and gets "individual, explicit, active and revocable consent". — [Telegram Bot Platform Developer ToS](https://telegram.org/tos/bot-developers?setln=en)
- **Bot terms, data handling and conduct.**
  - Encrypt user data at rest, with the key stored separately (§4.4a).
  - Delete data when it is no longer needed and when the bot shuts down (§4.2).
  - Publish an accessible privacy policy, which can be set in @BotFather (§4).
  - Don't spread personal information or media of non-consenting third parties (§5.2i).
  - Mishandling data leads to termination. — [Telegram Bot Platform Developer ToS](https://telegram.org/tos/bot-developers?setln=en)
- **General ToS.** Non-compliance can bring temporary or permanent bans from Telegram or some of its services. — [Telegram ToS](https://telegram.org/tos)
- **Uzbek travel channels worth approaching as partners.** Subscriber counts change fast, so verify each before outreach.
  - **@uzbekistan_travel.** Official channel of Uzbekistan's tourism promotion board, about 15.6k subscribers when the page was captured. — [t.me/uzbekistan_travel](https://t.me/uzbekistan_travel)
  - **@uzbektourismofficial.** Tourism Committee's channel "Uzbektourism.uz", indexed on TGStat. — [TGStat](https://uz.tgstat.com/en/channel/@uzbektourismofficial)
  - **Sanjiksancho.** Travel creator who highlights lesser-known places in Uzbekistan, about 112k subscribers in a 2026-era ranking. — [Russia-Promo blog](https://russia-promo.com/blog/leading-influencers-uzbekistan); [TGStat Uzbekistan travel ratings](https://uz.tgstat.com/en/ratings/channels/travels/public?sort=members_y)
  - **Kompas.** About 33.5k subscribers and about 17.6k average reach. The figure is older (roughly 2025), so it may be outdated. — [Russia-Promo blog](https://russia-promo.com/blog/leading-influencers-uzbekistan)
  - **Mandarin Tour.** A tour operator's channel, about 44.5k subscribers. — [Russia-Promo blog](https://russia-promo.com/blog/leading-influencers-uzbekistan)
  - **Instagram-first creators.** Durdona Safarova (@yourgraced) and Dilmurad Akhmedov (@axmedov_dilmurad). — [Russia-Promo blog](https://russia-promo.com/blog/leading-influencers-uzbekistan)
  - **Caveat on TGStat rankings.** The Uzbekistan "Travel" category is dominated by travel agencies and migrant-job channels, so its largest channels are often not content channels. — [TGStat Uzbekistan travel category](https://uz.tgstat.com/en/travels)
- **Why Telegram matters for hidden spots.** A travel blog on Uzbekistan says "many of the most secret spots don't have websites; they have Telegram channels". — [Minzifa Travel blog](https://minzifatravel.com/en/adventures/uzbekistan-blog/travel-like-a-local-using-digital-tools-to-experience-authentic-uzbekistan)

### Inferences
- **The Bot API only sees what it is given.** A bot receives updates only from chats it belongs to, and for channels it must be an admin. This matches a consent-based partner model: the partner adds the bot to their channel or forwards posts. I didn't re-fetch the Bot API docs; this reflects standard Bot API behaviour.
- **LLM extraction from Telegram content is a grey zone even with consent.** Using an MTProto or userbot account to monitor public channels without permission conflicts with Bot ToS §4.3 and API ToS §1.5 if any AI processing is involved. Running an LLM over partner-supplied posts to extract place data (inference, not model training) is safer if the partner consents and the data arrived by forward or admin-bot. Still, get the partner's licence in writing and avoid training models on Telegram data.

### Gaps
- I didn't retrieve the standalone "Content Licensing and AI Scraping" terms, so their exact scope (for example whether LLM inference on partner-provided content is covered) is unverified.
- TGStat pages returned 403, so I couldn't produce a verified top-25 list of Uzbek travel channels with current counts.
- I didn't verify the Bot API "bot must be admin to receive channel posts" wording against core.telegram.org/bots/api in this session.

## 4. How other travel apps source hidden gems (Atlas Obscura, Spotted by Locals, Google Local Guides, etc.)

### Takeaway
- **Atlas Obscura:** user submissions plus editors who review and edit every entry before publishing.
- **Spotted by Locals:** hand-picked resident "Spotters". Its own site describes them as unpaid volunteers, not paid writers as the brief assumed.
- **Google Local Guides:** gamified UGC with points and levels, backed by heavy ML moderation.

### Cited Findings
- **Atlas Obscura, review process.** User-submitted places are reviewed by the editorial team and not all are published. Editors may change titles, add details, fix grammar, correct facts and add photos. The site may refuse any submission. — [Atlas Obscura FAQ](https://www.atlasobscura.com/faq); [Atlas Obscura Terms](https://atlasobscura.com/adventures/terms)
- **Atlas Obscura, editor gatekeeping.** In a 2013 interview the founders said everything from users "goes through an editor's hands before it gets published". — [The Next Web, 2013](https://thenextweb.com/news/hidden-wonders-atlas-obscura-everything-curious-traveler-ever-wish-interview); see also [The Atlas Obscura is Made of You](https://www.atlasobscura.com/articles/the-atlas-obscura-is-made-of-you)
- **Atlas Obscura, community guidelines.** They govern contributions. — [Atlas Obscura Community Guidelines](https://www.atlasobscura.com/community-guidelines)
- **Spotted by Locals, who writes.** City guides are written by "Spotters", who are "handpicked by us" and live in the city, speak the local language and love their city. The site says locals "don't accept any form of compensation" from the places they cover, and describes the community as volunteer locals in about 82 cities. Note the possible ambiguity: "no compensation" may mean no payment from the venues rather than from Spotted by Locals. — [Spotted by Locals: About us](https://spottedbylocals.com/about-us); [Wikipedia](https://wikipedia.com/wiki/Spotted_by_Locals)
- **Google Local Guides, points.** Points per contribution (for example 10 per review, 5 per photo) and Levels 1-10 (Level 10 at 100,000 points). The badge comes at Level 4, from 250 points. Contributions that break policy don't count, and guides who duplicate reviews across locations are removed. — [Google: Local Guides points & levels](https://support.google.com/maps/answer/6225851?hl=en); [Local Guides community guidelines](https://support.google.com/maps/answer/7358351?hl=en-GB)
- **Google Maps UGC policy.** Content must be "based on real experiences and information". It bans fake edits, copied or stolen photos, off-topic content and defamation. — [Maps user-generated content policy](https://support.google.com/contributionpolicy/answer/7422880)
- **Google's moderation scale.**
  - More than 240 million policy-violating reviews blocked or removed in 2024.
  - Posting restrictions on more than 782,000 accounts and more than 13 million fake Business Profiles removed (2025 Trust & Safety reporting).
  - New reviews are paused on a profile when spam spikes. — [Google blog](https://blog.google/products-and-platforms/products/maps/new-ways-were-protecting-businesses-on-maps/); [Google blog: AI and fake reviews](https://blog.google/products-and-platforms/products/maps/google-business-profiles-ai-fake-reviews/)

### Inferences
- **Proven pattern 1: editorial gate on UGC (Atlas Obscura).** Best for quality and "wow" places, and it scales with a small editor team.
- **Proven pattern 2: vetted local curators (Spotted by Locals).** Strong trust and voice, slow to scale.
- **Proven pattern 3: gamified mass UGC with automated moderation (Google).** Scales, but needs anti-fraud investment.
- **Mix for Uzbekistan.** Start curator-led plus an editorial gate, add gamified UGC once there is a user base.

### Gaps
- I didn't research Komoot, AllTrails or Wanderlog sourcing and moderation specifically. They are generally known as UGC-route platforms, but I have no citations from this session.
- I couldn't confirm whether Spotted by Locals pays its spotters (the become-a-spotter page returned 403).

## 5. A practical sourcing model for Uzbekistan, including AI-assisted extraction

### Takeaway
A defensible pipeline:
1. Consent-based partner feeds: bloggers and Telegram channel owners license posts, or forward them to a bot.
2. Business self-listing, optionally verified through the owner's own Instagram via the official API.
3. User submissions with geotagged photos.
4. Official registers from the Tourism Committee.
5. An LLM that extracts structured place data (name, location, hours, category) from consented posts, followed by human or automated verification.

Uzbekistan already has a few AI and "local" guide apps, but I found none built on a consent-based Telegram/Instagram partner network.

### Cited Findings
- **Tourism Committee registers.** The Tourism Committee's gov.uz portal lists:
  - a "Unified register of tourist route objects";
  - a register of accommodation facilities;
  - registries of guides and of tour operators and travel agents;
  - a regional guidebook section covering all 13 regions, showing 1,056 entries and "Latest update: 2025-02-27".
  
  I found no confirmed machine-readable export on data.egov.uz. — [gov.uz/uzbektourism: unified register](https://gov.uz/en/uzbektourism/sections/turistik-yo-nalish-marshrut-dagi-obyektlarining-yagona-reyestri); [Uzbektourism portal](https://gov.uz/en/uzbektourism)
- **Existing local apps.**
  - **"Uzbekistan Tourism App" (Google Play kor.uzb).** AI trip planner with interactive maps, 10,000+ downloads, developer listed as Bekzod Tilloev. — [Google Play](https://play.google.com/store/apps/details?id=kor.uzb&hl=en_US); [AppBrain](https://www.appbrain.com/app/uzbekistan-tourism-app/kor.uzb)
  - **"Uzbekistan Travel Guide" (uz.tourism).** Claims to be curated by locals, with "local spots you won't find on Google". — [Google Play](https://play.google.com/store/apps/details?id=uz.tourism&hl=en)
  - **UzGidAI.** Free browser-based AI guide that uses a curated set of local facts maintained by a Tashkent team, plus Google Gemini. Not official. — [uzgid.app/about](https://uzgid.app/about)
  - **2GIS.** Cited as a local map app with local reviews, and a potential POI-data partner. — [Minzifa Travel blog](https://minzifatravel.com/en/adventures/uzbekistan-blog/travel-like-a-local-using-digital-tools-to-experience-authentic-uzbekistan)
- **Telegram consent rule.** Data users submit directly to a bot can be processed with "individual, explicit, active and revocable consent", which supports a submission bot (photo, geolocation, description). — [Telegram Bot Platform Developer ToS §4.3](https://telegram.org/tos/bot-developers?setln=en)
- **Instagram owner-consent route.** Instagram API with Instagram Login lets professional accounts authorize an app without a Facebook Page, so a business can connect its own account to self-list and sync posts. — [Meta blog, July 2024](https://developers.facebook.com/blog/post/2024/07/30/instagram-api-with-instagram-login/)
- **Instagram display and attribution.** Tokenless oEmbed (June 2026) lets the app embed a partner's original Instagram post on a place page for attribution. — [Meta blog, 15 Jun 2026](https://developers.facebook.com/blog/post/2026/06/15/tokenless-access-to-meta-oembed-apis/)

### Inferences (proposed model; design recommendations, not sourced facts)
1. **Partner network.** Sign written content licences, simple revenue-share or paid-per-place, with 10-30 Uzbek travel bloggers and Telegram channels (for example Sanjiksancho, Kompas-type channels, regional city channels) and the official @uzbekistan_travel. Partners either add the app's bot as admin to a dedicated feed channel or forward posts to the bot. Store only the extracted place facts plus a link or embed to the original. Don't rehost photos without a licence.
2. **Business self-listing.** Owners claim or add their place, optionally connecting their Instagram professional account for verification and post sync. Verify by phone, Telegram contact or a location visit.
3. **Crowdsourcing.** Use a Telegram bot or in-app form that takes a photo, Telegram location share or device GPS, and a short description. Award points or levels in the Local Guides style, and add an editorial gate in the Atlas Obscura style. Require explicit consent and a privacy policy (Telegram §4), and strip EXIF data or faces where needed.
4. **Curators.** Recruit student and volunteer regional curators, for example via universities or tourism colleges, in the Spotted by Locals style. Give each region an owner.
5. **Official data.** Request exports of the Tourism Committee's registers (route objects, accommodation, guides) and regional khokimiyat lists as a verified baseline layer.
6. **AI extraction pipeline.**
   - Input: partner-consented posts in Uzbek Latin or Cyrillic and Russian.
   - The LLM extracts a strict JSON schema: name, category, city/district, address text, coordinates if present, hours, price level, contact, and source post ID.
   - Geocode the result, deduplicate it against existing places and 2GIS/OSM, and score confidence.
   - Low-confidence items go to a human review queue. High-confidence items are published as "unverified, from <partner>" until confirmed by a curator, owner or user check-in.
   - Recheck periodically to catch places that have closed.
   - Avoid training models on Telegram data (API ToS §1.5).
7. **Hosting.** Under the 2026 amendments, general user data may go to the 49 listed countries. Keep any biometric data in Uzbekistan and report breaches within 24h/72h.

### Gaps
- I found no Spot.uz, Kun.uz or Gazeta.uz coverage of Uzbek travel startups that crowdsource hidden places. The searches returned nothing specific, so I can't name any startup with a confirmed partnership model.
- I found no public data on whether khokimiyats publish lists of local tourist objects in machine-readable form.
- I have no data on typical blogger licensing rates in Uzbekistan.
- I have no evidence on whether the Tourism Committee or the tourism promotion board run partnership or API programs for startups.
