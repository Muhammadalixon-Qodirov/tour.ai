#!/usr/bin/env bash
# Yo'ldosh AI — serverni birinchi marta sozlash (Ubuntu/Debian, root). Takror ishga tushirish xavfsiz.
#   curl -fsSL https://raw.githubusercontent.com/Muhammadalixon-Qodirov/tour.ai/main/deploy/setup.sh | bash
#   yoki: bash /opt/yoldosh/deploy/setup.sh [--firewall]
set -euo pipefail
REPO=https://github.com/Muhammadalixon-Qodirov/tour.ai.git
DIR=/opt/yoldosh
FIREWALL=${1:-}

log() { printf '\n\033[1;32m==> %s\033[0m\n' "$*"; }
[ "$(id -u)" = 0 ] || { echo "root sifatida ishga tushiring"; exit 1; }

log "Paketlar"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl git openssl cron >/dev/null

if ! command -v docker >/dev/null; then
  log "Docker o'rnatilmoqda (rasmiy skript)"
  curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker >/dev/null

log "80/443 portlar"
MODE=caddy
if ss -tlnp | grep -E ':(80|443)\s' | grep -q nginx; then
  MODE=nginx; echo "nginx ishlayapti — mavjud nginx ga alohida sayt qo'shiladi (boshqa saytlarga tegilmaydi)"
elif ss -tlnp | grep -E ':(80|443)\s' | grep -v docker-proxy; then
  echo "80/443 nginx bo'lmagan dastur tomonidan band — qo'lda sozlang."; exit 1
fi

log "Kod: $DIR"
if [ -d "$DIR/.git" ]; then git -C "$DIR" pull --ff-only; else git clone "$REPO" "$DIR"; fi
cd "$DIR/deploy"

if [ ! -f .env ]; then
  log ".env yaratilmoqda (tasodifiy sirlar)"
  IP=$(curl -fsS4 https://api.ipify.org || hostname -I | awk '{print $1}')
  cat > .env <<ENV
SERVER_IP=$IP
DOMAIN=${IP//./-}.sslip.io
DB_PASSWORD=$(openssl rand -hex 24)
TOKEN_SECRET=$(openssl rand -base64 48 | tr -d '\n')
API_PORT=8130
COMPOSE_PROFILES=$([ "$MODE" = caddy ] && echo caddy)
CORS_ORIGINS=
EXPO_PUSH_ENABLED=false
EXPO_ACCESS_TOKEN=
ENV
  chmod 600 .env
fi
set -a; . ./.env; set +a

if [ "$FIREWALL" = "--firewall" ]; then
  log "UFW: SSH, 80, 443"
  apt-get install -y -qq ufw >/dev/null
  SSH_PORT=$(ss -tlnp | awk '/sshd/ {split($4,a,":"); print a[length(a)]; exit}')
  ufw allow "${SSH_PORT:-22}/tcp" >/dev/null
  ufw allow 80/tcp >/dev/null; ufw allow 443/tcp >/dev/null; ufw allow 443/udp >/dev/null
  ufw --force enable
fi

log "Konteynerlar (build bir necha daqiqa olishi mumkin)"
docker compose up -d --build

log "API sog'ligi"
for i in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${API_PORT:-8130}/healthz" >/dev/null 2>&1; then echo "ok"; break; fi
  sleep 2; [ "$i" = 90 ] && { docker compose logs --tail=50 api; exit 1; }
done

if [ "$MODE" = nginx ]; then
  SITE=/etc/nginx/sites-available/yoldosh
  if [ ! -f "$SITE" ]; then
    log "nginx sayti: $DOMAIN → 127.0.0.1:${API_PORT:-8130}"
    sed -e "s/__DOMAIN__/$DOMAIN/g" -e "s/__PORT__/${API_PORT:-8130}/g" "$DIR/deploy/nginx-site.conf" > "$SITE"
    ln -sf "$SITE" /etc/nginx/sites-enabled/yoldosh
    if ! nginx -t; then rm -f /etc/nginx/sites-enabled/yoldosh "$SITE"; echo "nginx -t xato — sayt olib tashlandi"; exit 1; fi
    systemctl reload nginx
  fi
  if command -v certbot >/dev/null && ! certbot certificates 2>/dev/null | grep -q "Domains: $DOMAIN"; then
    log "HTTPS sertifikat (certbot)"
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --register-unsafely-without-email --redirect \
      || echo "certbot muvaffaqiyatsiz — sayt hozircha HTTP orqali ishlaydi"
  fi
fi

if [ ! -f .seeded ]; then
  log "Demo ma'lumot va nomzod joylar (faqat birinchi marta)"
  docker compose run --rm --entrypoint /app/bin/seed api /app/seed
  docker compose run --rm --entrypoint /app/bin/import-candidates api /app/seed/candidates
  touch .seeded
fi

log "Kunlik zaxira nusxa (03:30)"
chmod +x "$DIR/deploy/"*.sh
echo "30 3 * * * root $DIR/deploy/backup.sh >> /var/log/yoldosh-backup.log 2>&1" > /etc/cron.d/yoldosh-backup

log "Tayyor"
echo "API:  https://$DOMAIN/api/v1/cities   (zaxira: http://$SERVER_IP/healthz)"
echo "Admin yaratish: bash $DIR/deploy/admin.sh siz@example.com admin"
