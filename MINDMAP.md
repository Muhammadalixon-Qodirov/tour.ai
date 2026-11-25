# Yo'ldosh AI — loyiha xaritasi

![Yo'ldosh AI mind-map](images/mindmap.png)

> Kulrang punktir — keyingi bosqichga qoldirilgan modullar.

## 1. Mind-map (Mermaid)

```mermaid
mindmap
  root((Yo'ldosh AI<br/>Shaxsiy sayohat hamrohi))
    Kashf — AI marshrut
      Qiziqishlar: tarix, taom, tabiat
      Byudjet
      Necha kun / vaqt
      Sayohat turi: oila, yolg'iz, guruh
      Yangilik kvotasi: 1–2 yangi joy
      Gavjumlikni hisobga olish
      Ob-havo → marshrut qayta tuziladi
      "Bo'sh 2 soatim bor" rejimi
      RAG: doim yangi baza
    Raqamli gid — keyinroq
      Rasmga ol → tarixini bil
      Audio-gid
      Savol-javob chat
      10+ til
      Oflayn rejim
    Adolatli narx — keyinroq
      Narx ma'lumotnomasi
      Rasmga ol → narxni bil
      Savdolashish iboralari
      Ishonchli sotuvchi belgisi
      Anonim shikoyat
    Tadbirlar — keyinroq
      Festival va sayllar kalendari
      Marshrutga bir bosishda
      Chipta xaridi
    Turar joy — keyinroq
      Marshrut yaqinidagi mehmonxona
      Adolatli narx + baholar
      Ilova ichida bron
    Qoida — geofencing
      Hududga kirganda bildirishnoma
      Rasm, kiyim, dron qoidalari
      Turistning o'z tilida
      Ma'muriyat tasdiqlaydi
    Admin panel — web
      Moderatsiya navbati
      Joy qo'shish → publish
      Qoida va narx tahriri
      KPI: yangi joylar ulushi
    Qo'shimcha imkoniyatlar
      Transport va to'lov
      SOS xavfsizlik
      Sayohat kundaligi → AI hikoya
      Lentaga post · Instagram / Telegram
    Texnologiya
      React Native mobil ilova
      Go API
      Python AI xizmati
      PostgreSQL + PostGIS + pgvector
      Redis · S3
      Claude Opus / Sonnet / Haiku
    Biznes model
      Freemium / Premium
      Bron komissiyasi 5–10%
      Chipta komissiyasi
      Hamkor joylar — belgili
```

## 2. Tizim qanday ishlaydi

```mermaid
flowchart LR
  M["Mobil ilova<br/>React Native · oflayn kesh"] -->|HTTPS /api/v1| G["Go API<br/>auth · katalog · marshrut<br/>narx · geofence · admin"]
  A["Admin web panel<br/>moderator / admin"] -->|HTTPS /api/v1| G
  G -->|"ichki: route/rank · vision<br/>rag/answer · tts"| AI["AI xizmati<br/>Python · FastAPI<br/>RAG · vision · TTS/STT"]
  AI --> C["Claude<br/>Opus · Sonnet · Haiku"]
  G --> P[("PostgreSQL<br/>PostGIS + pgvector")]
  G --> R[("Redis<br/>kesh · limit")]
  G --> S[("S3 / MinIO<br/>rasm · audio")]
  AI -.->|audio yozadi| S
```

Mijozlar AI xizmatini to'g'ridan-to'g'ri ko'rmaydi: hamma so'rov Go API orqali o'tadi. AI ishlamay qolsa, Go ichidagi oddiy rejalashtirgich marshrutni baribir tuzadi, ilova esa `503 ai_unavailable` olib, keshdagi ma'lumotni ko'rsatadi.

## 3. Turistning bir kuni

```mermaid
flowchart LR
  1[Kirish:<br/>til va qiziqish] --> 2[Shaxsiy<br/>marshrut] --> 3[Yo'lda:<br/>qoida ogohlantirishi] --> 4[Obyektda:<br/>rasm → tarix] --> 5[Bozorda:<br/>narx tekshiruvi] --> 6[Kechqurun:<br/>tadbir va turar joy] --> 7[Baho:<br/>yangi joy reytingi] --> 8[Kundalik:<br/>ulashish]
```
