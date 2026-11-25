-- 0003: sayohat kundaligi, lenta, ulashish, push eslatmalar.

CREATE TABLE diary_photos (
  id         uuid PRIMARY KEY,
  user_id    text NOT NULL REFERENCES users ON DELETE CASCADE,
  trip_id    uuid NOT NULL REFERENCES trips ON DELETE CASCADE,
  day_idx    smallint NOT NULL,
  stop_id    uuid,
  file_name  text NOT NULL,                        -- media ombordagi nom (EXIF/GPS olib tashlangan)
  url        text NOT NULL,
  width      int NOT NULL,
  height     int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX diary_photos_day_idx ON diary_photos (trip_id, day_idx, created_at);

CREATE TABLE diary_entries (
  id           uuid PRIMARY KEY,
  user_id      text NOT NULL REFERENCES users ON DELETE CASCADE,
  trip_id      uuid NOT NULL REFERENCES trips ON DELETE CASCADE,
  day_idx      smallint NOT NULL,
  lang         text NOT NULL,
  tone         text NOT NULL DEFAULT 'warm' CHECK (tone IN ('warm','short','fun','poetic')),
  title        text NOT NULL,
  body         text NOT NULL,
  caption      text NOT NULL DEFAULT '',
  highlights   text[] NOT NULL DEFAULT '{}',
  hashtags     text[] NOT NULL DEFAULT '{}',
  place_ids    text[] NOT NULL DEFAULT '{}',        -- hikoyada tilga olingan joylar (lentadan marshrutga qo'shish uchun)
  photo_ids    uuid[] NOT NULL DEFAULT '{}',
  generated_by text NOT NULL DEFAULT 'template' CHECK (generated_by IN ('ai','template')),
  edited       bool NOT NULL DEFAULT false,
  regenerations int NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (trip_id, day_idx)
);

CREATE TABLE posts (
  id             uuid PRIMARY KEY,
  user_id        text NOT NULL REFERENCES users ON DELETE CASCADE,
  diary_entry_id uuid NOT NULL UNIQUE REFERENCES diary_entries ON DELETE CASCADE,
  city_id        text NOT NULL REFERENCES cities,
  slug           text NOT NULL UNIQUE,              -- ochiq havola: /s/{slug}
  visibility     text NOT NULL CHECK (visibility IN ('public','link','private')),
  status         text NOT NULL DEFAULT 'visible' CHECK (status IN ('visible','hidden','removed')),
  likes_count    int NOT NULL DEFAULT 0,
  views_count    int NOT NULL DEFAULT 0,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX posts_feed_idx ON posts (city_id, created_at DESC) WHERE status = 'visible' AND visibility = 'public';
CREATE INDEX posts_feed_all_idx ON posts (created_at DESC) WHERE status = 'visible' AND visibility = 'public';

CREATE TABLE post_likes (
  post_id    uuid REFERENCES posts ON DELETE CASCADE,
  user_id    text REFERENCES users ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (post_id, user_id)
);

CREATE TABLE post_reports (
  id         bigserial PRIMARY KEY,
  post_id    uuid NOT NULL REFERENCES posts ON DELETE CASCADE,
  user_id    text NOT NULL REFERENCES users ON DELETE CASCADE,
  reason     text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (post_id, user_id)
);

CREATE TABLE share_events (
  id             bigserial PRIMARY KEY,
  diary_entry_id uuid NOT NULL REFERENCES diary_entries ON DELETE CASCADE,
  post_id        uuid,
  target         text NOT NULL CHECK (target IN ('feed','instagram','telegram','other','copy')),
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_log (
  user_id text NOT NULL REFERENCES users ON DELETE CASCADE,
  kind    text NOT NULL,                           -- 'diary_ready'
  day     date NOT NULL,
  sent_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, kind, day)
);
