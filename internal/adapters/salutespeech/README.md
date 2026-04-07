# `internal/adapters/salutespeech`

## Назначение

Клиент **SaluteSpeech** REST API: асинхронное распознавание речи по [документации Sber](https://developers.sber.ru/docs/ru/salutespeech/rest/post-async-speechrecognition).

## Что реализовать

1. **Аутентификация** по регламенту SaluteSpeech (получение/обновление access token, если требуется).
2. **Загрузка файла** на распознавание.
3. **Создание задачи** распознавания.
4. **Polling статуса** задачи с заданным интервалом и таймаутом.
5. **Скачивание результата** с распознанным текстом.
6. Обработка ошибок API и отмена по `context.Context`.

## Интерфейс для usecase

Рекомендуется один метод вида `Transcribe(ctx, audio []byte, filename string) (text string, err error)` или пошаговый API, спрятанный за этим фасадом.

## Правила

- Не класть сюда логику `/start` или SQL — только HTTP + модели ответов API.
