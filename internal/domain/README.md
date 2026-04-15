# `internal/domain`

Доменные сущности проекта без привязки к SDK и драйверам.

Основные типы:
- `User` — пользователь Telegram;
- `Meeting` — встреча с полями транскрипции, summary и embeddings.

Дополнительные поля в `Meeting` используются для semantic search (`SemanticScore`, `SummaryScore`, `TranscriptScore`).
