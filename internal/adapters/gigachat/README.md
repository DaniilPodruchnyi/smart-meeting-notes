# `internal/adapters/gigachat`

## Назначение

Клиент **GigaChat API**: отправка сообщений модели, получение ответа (саммари транскрипции, ответ на вопрос в `/chat`).

## Реализовано в адаптере

- OAuth `POST /api/v2/oauth` с `scope=GIGACHAT_API_PERS`, заголовки `Authorization: Basic <ключ>`, `RqUID` (UUID v4), кеш access token.
- `POST .../chat/completions` — `(*Client).Chat(ctx, systemPrompt, userMessage string)`.
- `POST .../embeddings` — `(*Client).Embed(ctx, texts []string) ([][]float64, error)`; модель и путь задаются в `GigaChatConfig` (`GIGACHAT_EMBEDDINGS_*`).
  Под **саммари**: в `system` — как конспектировать; в `user` — текст транскрипта. Под **/chat без жёсткой роли**: `system` пустой, в `user` — вопрос (или system с краткими правилами ответа).
- Параметры из `config.GigaChatConfig` (см. `.env.example`).

## Что добавить в usecase поверх адаптера

- Зафиксировать **тексты system** для сценариев (саммари, ответ по тексту встречи) в одном месте (константы/конфиг), вызывать только `Chat`.
- **Ретраи** при 429/5xx при необходимости.

## Проверка

```bash
go run ./cmd/gigachat-check -env .env -system "Ты аналитик встреч. Дай краткую выжимку по договорённостям." -user "$(cat transcript.txt)"
go run ./cmd/gigachat-check -env .env -prompt "Кратко: что такое Go?"
```

## Правила

- Хранение истории чата в БД — решение usecase + postgres; адаптер только вызывает API.
