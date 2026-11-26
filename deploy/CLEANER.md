# AI ma'lumot tozalovchi (cleaner)

Ichki tarmoqdagi AI server topilgan joylarni (Wikidata/OSM/tadqiqot/foydalanuvchi takliflari) lokal AI model bilan
tozalaydi: haqiqiy turistik joymi (junk ajratish), to'g'ri kategoriya, qisqa uz/ru/en tavsif (faqat mavjud faktlardan).
Fakt — koordinata, manba — o'zgarmaydi. Natija asosiy serverga qaytadi: yuqori ishonchlilar nashr qilinadi, qolgani
admin navbatiga tushadi. Foydalanuvchi taklifi hech qachon avtomatik nashr qilinmaydi.

**Faqat chiquvchi ulanish:** ishchi AI serverda turadi va asosiy serverga (safar-api) o'zi qo'ng'iroq qiladi — AI
serverga port ochish yoki uni internetga chiqarish shart emas.

## Asosiy serverda (bir marta)

`deploy/.env` ga kuchli kalit qo'shing va yangilang:

    CLEANER_KEY=$(openssl rand -hex 24)
    bash /opt/yoldosh/deploy/update.sh

## AI serverda

1. Model ishlab turibdi (OpenAI-mos API beradi):
   - **Ollama:** `ollama serve` + `ollama pull qwen2.5:7b-instruct` → `AI_BASE_URL=http://localhost:11434/v1`
   - **llama.cpp server / vLLM / LM Studio:** o'z `/v1` manzili.
2. `cleaner` binarini oling: `go build -o cleaner ./backend/cmd/cleaner` (yoki Docker obrazidagi `/app/bin/cleaner`).
3. Sozlang va ishga tushiring:

```sh
export BACKEND_URL=https://safar-api.e-investment.uz
export CLEANER_KEY=<asosiy serverdagi bilan bir xil>
export AI_BASE_URL=http://localhost:11434/v1      # Ollama
export AI_MODEL=qwen2.5:7b-instruct
# export AI_KEY=...                               # lokal model uchun shart emas
export CLEAN_BATCH=20
export CLEAN_INTERVAL=60s

./cleaner            # tsikl: har interval da navbatni tozalaydi
./cleaner once       # bir marta (cron/test uchun)
```

systemd xizmati (ixtiyoriy): `/etc/systemd/system/safar-cleaner.service` da `Environment=` qatorlari + `ExecStart=/opt/cleaner/cleaner`.

## Tekshirish

- Asosiy serverda navbat: `GET /api/v1/admin/discoveries?status=queued` (admin).
- Ishchi loglari har to'plamdan keyin: `olingan / nashr / rad / navbatda` sonlarini yozadi.
- Model tavsiyasi har topilmada `discoveries.data->'ai'` da saqlanadi (admin ko'radi).
