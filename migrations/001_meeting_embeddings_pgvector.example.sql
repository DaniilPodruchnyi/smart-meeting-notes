-- Пример схемы для семантического поиска (PostgreSQL + pgvector).
-- 1) CREATE EXTENSION IF NOT EXISTS vector;
-- 2) Подставьте размерность в vector(N) — как у первого ответа POST /embeddings (см. cmd/embeddings-check).

CREATE TABLE IF NOT EXISTS meeting_embeddings (
    meeting_id UUID NOT NULL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL,
    embedding vector(1024) NOT NULL,
    source_text_hash TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS meeting_embeddings_user_idx ON meeting_embeddings (telegram_user_id);

-- Поиск по косинусному расстоянию (<=>): чем меньше, тем ближе.
-- SELECT meeting_id, 1 - (embedding <=> $1::vector) AS score
-- FROM meeting_embeddings
-- WHERE telegram_user_id = $2
-- ORDER BY embedding <=> $1
-- LIMIT $3;
