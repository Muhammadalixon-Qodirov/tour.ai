#!/usr/bin/env bash
# Admin/moderator hisobi: bash admin.sh email [admin|moderator]  (parol so'raladi, kamida 10 belgi)
set -euo pipefail
cd /opt/yoldosh/deploy
read -rsp "Parol: " PW; echo
docker compose run --rm -e ADMIN_PASSWORD="$PW" --entrypoint /app/bin/admin api -email "$1" -role "${2:-moderator}"
