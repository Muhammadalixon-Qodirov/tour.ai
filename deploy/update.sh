#!/usr/bin/env bash
# Yangi versiyani o'rnatish: GitHub dan tortadi, qayta build qiladi. Migratsiyalar API ishga tushganda avtomatik.
set -euo pipefail
cd /opt/yoldosh
./deploy/backup.sh
git pull --ff-only
cd deploy
docker compose up -d --build
docker image prune -f >/dev/null
set -a; . ./.env; set +a
for i in $(seq 1 60); do curl -fsS "http://127.0.0.1:${API_PORT:-8130}/healthz" >/dev/null 2>&1 && { echo "yangilandi: $(git -C .. log --oneline -1)"; exit 0; }; sleep 2; done
docker compose logs --tail=80 api; exit 1
