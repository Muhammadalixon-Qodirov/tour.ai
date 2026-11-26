-- 0013: AI yordamchi suhbatlari ro'yxati. Har suhbat — sarlavha, xabarlar soni, oxirgi xabar; xabarlar tartib raqami (seq)
-- bilan qat'iy tartibda saqlanadi (bir millisekundda ikki xabar bo'lsa ham).
CREATE TABLE guide_conversations (
  id            uuid PRIMARY KEY,
  user_id       text NOT NULL REFERENCES users ON DELETE CASCADE,
  title         text NOT NULL,
  message_count int NOT NULL DEFAULT 0,
  last_text     text NOT NULL DEFAULT '',
  city_id       text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guide_conversations_user_idx ON guide_conversations (user_id, updated_at DESC, id);

ALTER TABLE guide_messages ADD COLUMN seq bigserial;
CREATE INDEX guide_messages_seq_idx ON guide_messages (conversation_id, seq);

-- mavjud xabarlardan suhbatlarni tiklash
INSERT INTO guide_conversations (id, user_id, title, message_count, last_text, created_at, updated_at)
SELECT m.conversation_id, min(m.user_id), left(min(m.text) FILTER (WHERE m.role = 'user'), 80), count(*),
       left((array_agg(m.text ORDER BY m.created_at DESC))[1], 160), min(m.created_at), max(m.created_at)
FROM guide_messages m GROUP BY m.conversation_id
ON CONFLICT DO NOTHING;
