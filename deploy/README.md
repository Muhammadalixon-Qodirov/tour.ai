# Deploy (server)

Stack: **PostgreSQL 18 + Go API + Caddy** (avtomatik HTTPS) — Docker Compose. Sirlar `deploy/.env` da (repoga kirmaydi).

## Birinchi o'rnatish

Serverga (Ubuntu/Debian, root):

```sh
git clone https://github.com/Muhammadalixon-Qodirov/tour.ai.git /opt/yoldosh
bash /opt/yoldosh/deploy/setup.sh            # firewall ham kerak bo'lsa: --firewall
bash /opt/yoldosh/deploy/admin.sh siz@example.com admin
```

`setup.sh`: Docker o'rnatadi → tasodifiy sirlar bilan `.env` yaratadi → konteynerlarni ko'taradi →
demo ma'lumot va nomzod joylarni yuklaydi (faqat birinchi marta) → kunlik zaxira nusxani (03:30) yoqadi.

## Manzillar

| | |
|---|---|
| API (HTTPS) | `https://<ip-chiziqcha>.sslip.io/api/v1` — masalan `https://185-174-254-135.sslip.io/api/v1/cities` |
| Zaxira (HTTP) | `http://<server-ip>/healthz` |

**Joriy production:** `https://safar-api.e-investment.uz/api/v1` (server 185.174.254.135, nginx + certbot, API `127.0.0.1:8130`).

Domenni almashtirish (server nginx rejimida):

```sh
D=yangi.domen.uz                       # avval DNS A-yozuvi server IP siga
cd /opt/yoldosh/deploy
sed -e "s/__DOMAIN__/$D/g" -e "s/__PORT__/8130/g" nginx-site.conf > /etc/nginx/sites-available/yoldosh-$D
ln -s /etc/nginx/sites-available/yoldosh-$D /etc/nginx/sites-enabled/ && nginx -t && systemctl reload nginx
certbot --nginx -d $D --redirect
sed -i "s/^DOMAIN=.*/DOMAIN=$D/" .env && docker compose up -d api   # rasm/havola URL lari yangi domenda
```
Mobil ilovada: `EXPO_PUBLIC_API_URL=https://<DOMAIN>/api/v1`.

## Kundalik ishlar

```sh
bash /opt/yoldosh/deploy/update.sh       # yangi versiya (avval zaxira oladi)
bash /opt/yoldosh/deploy/backup.sh       # qo'lda zaxira → /opt/yoldosh/backups (14 kun)
cd /opt/yoldosh/deploy && docker compose logs -f api
cd /opt/yoldosh/deploy && docker compose ps
```

Tiklash: `docker compose exec -T db pg_restore -U yoldosh -d yoldosh --clean < /opt/yoldosh/backups/db-XXXX.dump`

## Xavfsizlik

- PostgreSQL tashqariga ochilmaydi (faqat Docker ichki tarmog'i).
- Root paroli o'rniga SSH kalit ishlating; parolli kirishni o'chiring (`PasswordAuthentication no`).
- `.env` — `chmod 600`, hech qachon repoga qo'shilmaydi.
