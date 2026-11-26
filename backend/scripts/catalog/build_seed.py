"""Frontend tadqiqot ma'lumotlaridan (research/*.json) backend seed fayllarini yasaydi.

    python3 -I scripts/catalog/build_seed.py <frontend-repo> [backend-dir]

<frontend-repo> — `research/`, `web/public/places/` papkalari bor jamoa papkasi
(masalan ~/dev/tourism_hackathon). backend-dir — shu repo ichidagi `backend/` (standart: skript yonidagi).

Natija (qayta ishga tushirsa ustidan yoziladi):
  seed/<city>.json      — shahar, joylar, narxlar, tadbirlar, turar joylar
  static/places/*.jpg   — ilova ishlatadigan rasmlar (web/public/places dan, o'zgartirilmagan)

Birlashtirish qoidalari research/convert.py bilan bir xil (ilova va sayt shu natijani ko'radi):
fayllar tartibi, enrich_* patchlari, tashkent_photos_fix, `replaces`, tekshiruvlar, Booking reytingi / 2.
Convert.py tashlab yuboradigan manbalar (narx, ish vaqti, qoida, tadbir, mehmonxona manbasi) shu yerda saqlanadi.
"""
import json, os, re, shutil, sys

FRONT = os.path.abspath(sys.argv[1])
BACK = os.path.abspath(sys.argv[2]) if len(sys.argv) > 2 else os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
R = os.path.join(FRONT, 'research')
WEB_IMG = os.path.join(FRONT, 'web', 'public', 'places')
OUT_SEED = os.path.join(BACK, 'seed')
OUT_IMG = os.path.join(BACK, 'static', 'places')

# Tadqiqot o'tkazilgan sana: narxlar uchun `updatedAt` (90 kundan eski narx ilovada "eskirgan").
RESEARCH_DATE = '2026-11-26'

FILES = ['tashkent_heritage.json', 'tashkent_leisure.json', 'tashkent_food_craft.json', 'tashkent_more.json',
         'namangan.json', 'namangan_more.json', 'samarkand.json', 'bukhara.json']
ENRICH = ['enrich_tashkent.json', 'enrich_namangan.json']
CATS = {'monument', 'museum', 'mosque', 'bazaar', 'workshop', 'gallery', 'restaurant', 'cafe', 'park', 'viewpoint', 'ecovillage', 'theatre'}
INTS = {'history', 'architecture', 'food', 'craft', 'art', 'nature', 'shopping', 'nightlife'}
RULES = {'no_flash', 'no_photo', 'photo_fee', 'dress_code', 'shoes_off', 'quiet', 'no_drone', 'no_food', 'no_smoking', 'no_pets',
         'ticket_required', 'prayer_time_limit'}
PRICE_CATS = {'taxi', 'food', 'souvenir', 'market', 'ticket', 'transport'}

EMERGENCY = {'unified': '112', 'fire': '101', 'police': '102', 'ambulance': '103', 'gas': '104',
             'emergency_ministry': '1050', 'tourist_hotline': '1173'}
# Markazlar va muqovalar ilovadagi CITIES bilan bir xil (mobile/src/mock/places.ts).
# Taksi: yo'lga chiqish (Yandex Go Start, 4 600 so'm, voyage.uz) + km narxi; km narxi prices_<city>.json dagi
# shahar ichi taksi oralig'idan chiqarilgan (3-5 km safar). Toshkentda 1 050/km + 500/daqiqa ≈ 2 500/km.
CITIES = [
    {'id': 'samarkand', 'name': {'uz': 'Samarqand', 'ru': 'Самарканд', 'en': 'Samarkand'},
     'center': {'lat': 39.6547, 'lng': 66.9758}, 'cover': 'registan.jpg', 'taxiBaseFare': 4600, 'taxiPerKm': 2400},
    {'id': 'bukhara', 'name': {'uz': 'Buxoro', 'ru': 'Бухара', 'en': 'Bukhara'},
     'center': {'lat': 39.7747, 'lng': 64.4286}, 'cover': 'ark.jpg', 'taxiBaseFare': 4600, 'taxiPerKm': 2400},
    {'id': 'tashkent', 'name': {'uz': 'Toshkent', 'ru': 'Ташкент', 'en': 'Tashkent'},
     'center': {'lat': 41.3111, 'lng': 69.2797}, 'cover': 'r_hazrati_imam.jpg', 'taxiBaseFare': 4600, 'taxiPerKm': 2500},
    {'id': 'namangan', 'name': {'uz': 'Namangan', 'ru': 'Наманган', 'en': 'Namangan'},
     'center': {'lat': 40.99833, 'lng': 71.67257}, 'cover': 'r_mulla_kyrgyz_madrasa.jpg', 'taxiBaseFare': 4600, 'taxiPerKm': 1600},
]

# Kirish chiptasi narxi qaysi joyga tegishli (joy sahifasidagi "shu yerda kerak bo'ladigan narxlar").
TICKET_PLACES = {
    'p_samarkand_registan': ['registan'], 'p_samarkand_gur_emir': ['gur-emir'], 'p_samarkand_shah_i_zinda': ['shah-i-zinda'],
    'p_samarkand_bibi_khanym': ['bibi-khanym'], 'p_samarkand_ulugbek': ['ulugbek-observatory'],
    'p_samarkand_daniel': ['khoja-doniyor'], 'p_samarkand_konigil': ['konigil-meros'],
    'p_bukhara_ark': ['ark'], 'p_bukhara_kalon': ['kalon-minaret', 'kalon-mosque'], 'p_bukhara_sitorai': ['sitorai-mohi-xosa'],
    'p_bukhara_samanid': ['samanid-mausoleum'], 'p_bukhara_chor_minor': ['chor-minor'],
    'p_bukhara_bahouddin': ['bahouddin-naqshband'], 'p_bukhara_zindan': ['zindan'],
    'p_tashkent_icc_local': ['islamic-civilization-center'], 'p_tashkent_icc_foreign': ['islamic-civilization-center'],
    'p_tashkent_history_museum': ['state-history-museum'], 'p_tashkent_applied_arts': ['applied-arts-museum'],
    'p_tashkent_amir_temur_museum': ['amir-timur-museum'], 'p_tashkent_tv_tower': ['tashkent-tv-tower'],
}


def clean(t):
    if t is None:
        return None
    t = str(t).replace(' — ', ', ').replace('—', ', ').replace('→', ' ').replace('≈', '')
    return re.sub(r'\s+', ' ', t).strip()


def loc(o):
    """convert.py dagi loc(): uz yoki en shart, ru ixtiyoriy (yo'q bo'lsa API en ni beradi)."""
    if not isinstance(o, dict):
        return None
    uz, en, ru = clean(o.get('uz')), clean(o.get('en')), clean(o.get('ru'))
    if not (uz or en):
        return None
    out = {'uz': uz or en, 'en': en or uz}
    if ru:
        out['ru'] = ru
    return out


def credit(im):
    url = im.get('commonsPage') or im.get('sourcePage')
    lic = clean(im.get('license'))
    if lic and 'unknown' in lic.lower():
        lic = None
    host = re.sub(r'^www\.', '', re.sub(r'^https?://([^/]+).*$', r'\1', url)) if url else None
    return {'author': clean(im.get('author')) or host, 'license': lic, 'url': url,
            'source': 'Wikimedia Commons' if im.get('commonsPage') else host}


def key_of(xid):
    return re.sub(r'[^a-z0-9]', '_', xid.lower())


images = set()


def photo(xid, im):
    """Rasm bo'lsa — web/public/places dagi fayl nomi va muallif ma'lumoti (convert.py bilan bir xil shart)."""
    if not im or not im.get('file'):
        return None
    if not os.path.exists(os.path.join(FRONT, im['file'])):
        return None
    fn = f'r_{key_of(xid)}.jpg'
    if not os.path.exists(os.path.join(WEB_IMG, fn)):
        sys.exit(f'{fn}: web/public/places da yo\'q — avval research/convert.py ni ishga tushiring')
    images.add(fn)
    c = credit(im)
    return {'file': fn, 'credit': c['author'], 'license': c['license'], 'sourceUrl': c['url'], 'source': c['source']}


def hours_spec(h):
    if not (isinstance(h, list) and len(h) == 2 and all(h)):
        return ''
    o, c = h
    if c <= o:  # yarim tungacha ishlaydi ("08:00", "00:00")
        c = '24:00'
    return f'{o}-{c}'


def text_block(r, lang):
    def g(field):
        v = loc(r.get(field)) if isinstance(r.get(field), dict) else None
        return v.get(lang) if v else None
    name = loc(r['name'])
    out = {
        'name': name.get(lang),
        'tagline': (loc(r.get('tagline')) or name).get(lang),
        'description': (loc(r.get('description')) or loc(r.get('tagline')) or name).get(lang),
        'address': (loc(r.get('address')) or name).get(lang),
    }
    for f, k in (('bestTime', 'bestTime'), ('hoursNote', 'hoursNote'), ('feeNote', 'feeNote')):
        v = g(f)
        if v:
            out[k] = v
    hist = r.get('history')
    if hist and loc(hist.get('summary')) and hist.get('sources'):
        out['history'] = loc(hist['summary']).get(lang)
        out['facts'] = [loc(x)[lang] for x in hist.get('facts') or [] if loc(x) and loc(x).get(lang)]
    return {k: v for k, v in out.items() if v not in (None, '')}


def main():
    patch, fix = {}, {}
    for ef in ENRICH:
        p = os.path.join(R, ef)
        if os.path.exists(p):
            for x in json.load(open(p)):
                patch[x['id']] = x
    fp = os.path.join(R, 'tashkent_photos_fix.json')
    if os.path.exists(fp):
        for x in json.load(open(fp)):
            fix[x['id']] = x.get('image')

    seen, dropped = set(), []
    places = {c['id']: [] for c in CITIES}
    for f in FILES:
        for r in json.load(open(os.path.join(R, f))):
            pid = r.get('id')
            why = []
            if not pid or pid in seen:
                why.append('id')
            lo = r.get('location') or {}
            if not isinstance(lo.get('lat'), (int, float)) or not isinstance(lo.get('lng'), (int, float)):
                why.append('coords')
            if pid in fix and fix[pid]:
                r['image'] = fix[pid]
            if pid in patch:
                for k, v in patch[pid].items():
                    if k == 'id':
                        continue
                    if k == 'rules' and v:
                        r['rules'] = v
                    elif v is not None and v != []:
                        r[k] = v
            if r.get('category') not in CATS:
                why.append('category')
            if not loc(r.get('name')):
                why.append('name')
            if r.get('cityId') not in places:
                why.append('city')
            if why:
                dropped.append((f, pid, why))
                continue
            seen.add(pid)

            rules = []
            for i, ru in enumerate(r.get('rules') or []):
                if ru.get('code') in RULES and ru.get('severity') in ('info', 'warn', 'forbidden') and loc(ru.get('text')):
                    rule = {'code': ru['code'], 'severity': ru['severity'], 'priority': len(r['rules']) - i, 'text': loc(ru['text'])}
                    if clean(ru.get('source')):
                        rule['source'] = clean(ru['source'])
                    rules.append(rule)
            langs = ['uz', 'en', 'ru']
            text = {lang: text_block(r, lang) for lang in langs}
            text = {k: v for k, v in text.items() if v.get('name')}
            hist = r.get('history') or {}
            hsrc = [{'title': clean(s.get('title')) or s.get('url'), 'url': s.get('url')} for s in hist.get('sources') or [] if s.get('url')]
            has_hist = bool(loc(hist.get('summary')) and hist.get('sources'))
            fee_l = int(r['feeLocal']) if isinstance(r.get('feeLocal'), (int, float)) else None
            fee_f = int(r['feeForeigner']) if isinstance(r.get('feeForeigner'), (int, float)) else None
            cash = r.get('cashOnly')
            p = {
                'id': pid, 'category': r['category'],
                'novelty': r.get('novelty') if r.get('novelty') in ('iconic', 'hidden', 'new') else 'iconic',
                'openedAt': r.get('openedAt') or None,
                'lat': round(lo['lat'], 5), 'lng': round(lo['lng'], 5),
                'interests': [i for i in (r.get('interests') or []) if i in INTS] or ['history'],
                'visitMin': int(r.get('visitDurationMin') or 45),
                'priceLevel': 0 if not (fee_l or fee_f) else 1,
                'hours': hours_spec(r.get('hours')),
                'rating': r['rating'] if isinstance(r.get('rating'), (int, float)) else 0,
                'reviews': 0,
                'crowd': r.get('crowd') if r.get('crowd') in ('low', 'medium', 'high') else 'medium',
                'wheelchair': r.get('wheelchair') is True,
                'cashOnly': cash is True,
                'payCard': cash is False,  # noma'lum bo'lsa karta bor deb aytilmaydi
                'geofenceRadiusM': 120,
                'source': 'open_data',
                'text': text,
                'rules': rules,
            }
            if fee_l is not None:
                p['feeLocal'] = fee_l
            if fee_f is not None:
                p['feeForeigner'] = fee_f
            if has_hist:
                p['historySources'] = hsrc
            im = photo(pid, r.get('image'))
            if im:
                p['image'] = im
            research = {k: r.get(k) for k in ('noveltyWhy', 'locationSource', 'hoursSource', 'feeSource', 'feeYear',
                                              'ratingSource', 'verified', 'unverified') if r.get(k)}
            if research:
                research['importedAt'] = RESEARCH_DATE
                p['research'] = research
            places[r['cityId']].append(p)

    # narxlar
    prices = {c['id']: [] for c in CITIES}
    seen_price = set()
    for c in ('samarkand', 'bukhara', 'tashkent', 'namangan'):
        pp = os.path.join(R, f'prices_{c}.json')
        if not os.path.exists(pp):
            continue
        for x in json.load(open(pp)):
            if x.get('id') in seen_price:
                continue
            seen_price.add(x.get('id'))
            if not (loc(x.get('name')) and isinstance(x.get('min'), (int, float)) and loc(x.get('unit'))):
                continue
            if x.get('category') not in PRICE_CATS or x.get('cityId') not in prices:
                continue
            item = {'id': x['id'], 'category': x['category'], 'name': loc(x['name']), 'unit': loc(x['unit']),
                    'min': int(x['min']), 'max': int(x.get('max') or x['min']),
                    'source': x.get('source'), 'year': x.get('year'), 'updatedAt': RESEARCH_DATE,
                    'placeIds': TICKET_PLACES.get(x['id'], [])}
            if isinstance(x.get('tourist'), (int, float)):
                item['tourist'] = int(x['tourist'])
            if clean(x.get('note')):
                item['note'] = clean(x['note'])
            prices[x['cityId']].append(item)

    # tadbirlar
    events = {c['id']: [] for c in CITIES}
    for e in json.load(open(os.path.join(R, 'events.json'))):
        lo = e.get('location') or {}
        if not (loc(e.get('title')) and e.get('start') and isinstance(lo.get('lat'), (int, float))) or e.get('cityId') not in events:
            continue
        st, en = e['start'], e.get('end') or e['start']
        ev = {'id': e['id'], 'kind': e.get('kind') or 'festival', 'title': loc(e['title']),
              'venue': loc(e.get('venue')) or loc(e['title']),
              'start': st[:10] + 'T' + (st[11:16] or '10:00'), 'end': en[:10] + 'T' + (en[11:16] or '18:00'),
              'lat': lo['lat'], 'lng': lo['lng'],
              'price': int(e['price']) if isinstance(e.get('price'), (int, float)) and e['price'] > 0 else None,
              'isFree': e.get('isFree') is True, 'ticketUrl': e.get('ticketUrl'), 'source': e.get('source'),
              'recurring': e.get('recurring'), 'note': clean(e.get('note'))}
        im = photo(e['id'], e.get('image'))
        if im:
            ev['image'] = im
        events[e['cityId']].append({k: v for k, v in ev.items() if v is not None})

    # turar joylar
    stays = {c['id']: [] for c in CITIES}
    for x in json.load(open(os.path.join(R, 'stays.json'))):
        lo = x.get('location') or {}
        if not (x.get('name') and isinstance(x.get('pricePerNight'), (int, float)) and isinstance(lo.get('lat'), (int, float))):
            continue
        if x.get('cityId') not in stays:
            continue
        st = {'id': x['id'], 'kind': x.get('kind') or 'hotel', 'name': clean(x['name']), 'lat': lo['lat'], 'lng': lo['lng'],
              'address': loc(x.get('address')), 'pricePerNight': int(x['pricePerNight']), 'priceSource': x.get('priceSource'),
              # Booking uslubidagi reyting 10 ballik; ilova 5 ballik yulduz ko'rsatadi (convert.py bilan bir xil).
              'rating': round(x['rating'] / 2 if x['rating'] > 5 else x['rating'], 1) if isinstance(x.get('rating'), (int, float)) else None,
              'ratingSource': x.get('ratingSource'),
              'amenities': [loc(a) for a in x.get('amenities') or [] if loc(a)], 'opened': x.get('opened'),
              'note': clean(x.get('note'))}
        im = photo(x['id'], x.get('image'))
        if im:
            st['image'] = im
        stays[x['cityId']].append({k: v for k, v in st.items() if v is not None})

    os.makedirs(OUT_IMG, exist_ok=True)
    for c in CITIES:
        images.add(c['cover'])
    for fn in sorted(images):
        shutil.copyfile(os.path.join(WEB_IMG, fn), os.path.join(OUT_IMG, fn))
    for old in os.listdir(OUT_IMG):
        if old.endswith('.jpg') and old not in images:
            os.remove(os.path.join(OUT_IMG, old))

    for c in CITIES:
        city = dict(c)
        city['emergency'] = EMERGENCY
        out = {'_generated': 'scripts/catalog/build_seed.py — qo\'lda tahrirlanmaydi, skriptni qayta ishga tushiring',
               'city': city, 'places': places[c['id']], 'prices': prices[c['id']],
               'events': events[c['id']], 'stays': stays[c['id']]}
        with open(os.path.join(OUT_SEED, f"{c['id']}.json"), 'w') as fh:
            json.dump(out, fh, ensure_ascii=False, indent=1)
            fh.write('\n')
        print(c['id'], 'places', len(places[c['id']]), 'prices', len(prices[c['id']]),
              'events', len(events[c['id']]), 'stays', len(stays[c['id']]))
    print('images', len(images))
    for d in dropped:
        print('dropped', d)


main()
