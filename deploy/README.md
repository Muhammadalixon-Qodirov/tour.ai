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

Katalog yangilanganda (`backend/seed/*.json` o'zgargan versiya) `update.sh` dan keyin seed qayta yuklanadi.
Migratsiyalar (masalan `0009_catalog.sql`) API ishga tushganda avtomatik qo'llanadi, seed ham ularni tekshiradi.
Seed idempotent: foydalanuvchi baholari, marshrutlar, admin tahrir qilgan turar joylar va OSM importi saqlanadi;
katalogda yo'q eski seed joylari `draft` ga o'tadi; OSM xizmatlari bor shaharga demo SOS qayta qo'shilmaydi:

```sh
cd /opt/yoldosh/deploy && docker compose run --rm --entrypoint /app/bin/seed api /app/seed
```

Shahar muqovalari (`0014_city_covers.sql`): rasmlar binar ichida (`backend/static/cities`, `/static/cities/{id}.jpg`), muallif va litsenziya
`seed/city_covers.json` da; yuqoridagi seed buyrug'i ularni yozadi (OSM importi ustiga yozmaydi).

Daromad funksiyalari (`0010_monetization.sql`): migratsiya API ishga tushganda qo'llanadi; namuna hamkorlar, takliflar va
shaharlararo yo'nalishlar `seed/monetization.json` dan yuqoridagi seed buyrug'i bilan yuklanadi. Yangi o'zgaruvchi
`WEB_BASE_URL` (standart `https://safar.e-investment.uz`): story havolasi, kod QR matni va saytga yo'naltirishlar shu manzilda.
Bosish havolalari `https://<DOMAIN>/r/{clickId}` (API domenida, nginx hammasini API ga uzatadi).
Haqiqiy hamkor, kalit va rollar admin API orqali (faqat `admin` roli): `API_CONTRACT.md` §10.6.

`.env` da `TRUSTED_PROXIES=127.0.0.1,172.16.0.0/12,<sayt serveri IP>` bo'lsin: aks holda sayt proksisi orqali kelgan
hamma foydalanuvchi IP limitlarida bitta mijoz hisoblanadi.

Tiklash: `docker compose exec -T db pg_restore -U yoldosh -d yoldosh --clean < /opt/yoldosh/backups/db-XXXX.dump`

## Xavfsizlik

- PostgreSQL tashqariga ochilmaydi (faqat Docker ichki tarmog'i).
- Root paroli o'rniga SSH kalit ishlating; parolli kirishni o'chiring (`PasswordAuthentication no`).
- `.env` — `chmod 600`, hech qachon repoga qo'shilmaydi.
