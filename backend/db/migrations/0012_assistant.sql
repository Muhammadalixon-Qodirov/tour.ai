-- 0012: AI yordamchi suhbatlari (POST /guide/chat). Faqat matn saqlanadi; asbob natijalari har safar bazadan olinadi.
CREATE TABLE guide_messages (
  id              uuid PRIMARY KEY,
  conversation_id uuid NOT NULL,
  user_id         text NOT NULL REFERENCES users ON DELETE CASCADE,
  role            text NOT NULL CHECK (role IN ('user','assistant')),
  text            text NOT NULL,
  place_ids       text[] NOT NULL DEFAULT '{}',   -- javobda ko'rsatilgan joylar
  mode            text,                            -- ai | fallback
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guide_messages_conv_idx ON guide_messages (conversation_id, created_at);
CREATE INDEX guide_messages_user_idx ON guide_messages (user_id, created_at DESC);
