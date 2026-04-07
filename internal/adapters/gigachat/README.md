# `internal/adapters/gigachat`

## Назначение

Клиент **GigaChat API**: отправка сообщений модели, получение ответа (саммари транскрипции, ответ на вопрос в `/chat`).

## Что реализовать

- Получение токена / авторизация по [quickstart для физлиц](https://developers.sber.ru/docs/ru/gigachat/individuals-quickstart).
- Метод для **краткой выжимки** из длинного текста (prompt + лимиты токенов).
- Метод для **диалога** (вопрос пользователя; при необходимости — системный prompt с правилами).
- Таймауты, ретраи при 429/5xx (осторожно, с backoff).

## Интерфейс для usecase

Например: `Summarize(ctx, text string) (string, error)`, `Chat(ctx, userMessage string) (string, error)`.

## Правила

- Хранение истории чата в БД — решение usecase + postgres; адаптер только вызывает API.
