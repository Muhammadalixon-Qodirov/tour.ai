#!/usr/bin/env bash
# OpenStreetMap ma'lumotini yangilash (haftalik): Geofabrik dan O'zbekiston PBF → ajratish → bazaga upsert.
# Yangi ochilgan kafe/muzey/bozorlar avtomatik qo'shiladi va to'g'ri kategoriyaga ajratiladi; qo'lda tahrirlangan
# joylar ustiga yozilmaydi. Cron: 0 4 * * 1  bash /opt/yoldosh/deploy/osm-refresh.sh >> /var/log/yoldosh-osm.log 2>&1
set -euo pipefail
cd /opt/yoldosh/deploy
DATA=/opt/yoldosh/data/osm
mkdir -p "$DATA"
echo "== $(date -Is) OSM yangilash"
curl -fsSL --retry 3 -o "$DATA/uz.osm.pbf.tmp" https://download.geofabrik.de/asia/uzbekistan-latest.osm.pbf
mv "$DATA/uz.osm.pbf.tmp" "$DATA/uz.osm.pbf"
run() { env -i PATH="$PATH" HOME="$HOME" docker compose run --rm -T -v "$DATA:/data:ro" --entrypoint "$@"; }
run /app/bin/osm-extract api /data/uz.osm.pbf > "$DATA/pois.json.tmp"   # distroless: sh yo'q — chiqish hostga yoziladi
mv "$DATA/pois.json.tmp" "$DATA/pois.json"
run /app/bin/osm-import api /data/pois.json
echo "== tayyor"
