#!/usr/bin/env bash
# PostgreSQL zaxira nusxasi: /opt/yoldosh/backups, 14 kun saqlanadi. Yuklangan rasmlar ham arxivlanadi.
set -euo pipefail
cd /opt/yoldosh/deploy
OUT=/opt/yoldosh/backups; mkdir -p "$OUT"; chmod 700 "$OUT"
TS=$(date +%Y%m%d-%H%M)
docker compose exec -T db pg_dump -U yoldosh -d yoldosh --format=custom > "$OUT/db-$TS.dump"
docker run --rm -v yoldosh_uploads:/u:ro -v "$OUT":/b alpine tar czf "/b/uploads-$TS.tgz" -C /u . 2>/dev/null || true
find "$OUT" -type f -mtime +14 -delete
echo "$(date -Is) zaxira: db-$TS.dump"
# Tiklash: docker compose exec -T db pg_restore -U yoldosh -d yoldosh --clean < /opt/yoldosh/backups/db-XXXX.dump
