# Структура проекта (по ТЗ + чистая архитектура)

Ниже — ориентировочное дерево папок на финал и что в них лежит логически. Точные имена пакетов можно слегка подстроить под команду, смысл слоёв сохраняется.

```
smart-meeting-notes/
├── cmd/
│   └── server/
│       └── main.go                 # Сборка зависимостей, запуск HTTP (если нужен) и/или бота
├── docs/
│   └── STRUCTURE.md                # Этот файл
├── internal/
│   ├── config/                     # Параметры из .env и окружения
│   ├── logger/                     # Обёртка над zap
│   ├── domain/                     # (опционально) сущности и доменные ошибки без внешних SDK
│   ├── app/
│   │   └── usecase/                # Сценарии: транскрипция→сохранение→саммари, поиск, /chat
│   ├── adapters/
│   │   ├── telegram/               # Модуль Телеграм: telebot, OnText/OnVoice/OnAudio, скачивание файлов
│   │   ├── salutespeech/           # Клиент SaluteSpeech: upload → task → poll → результат
│   │   ├── gigachat/               # Клиент GigaChat: саммари и ответы на вопросы
│   │   └── persistence/
│   │       └── postgres/           # Репозитории: пользователи, встречи, транскрипции, поиск
│   ├── queue/                      # Очередь задач через каналы; маршрутизация ответов по user ID
│   └── server/
│       └── transport/
│           └── http/               # HTTP-роутер: health, при необходимости webhook Telegram
├── go.mod
├── .env.example
├── TODO.md
└── README.md
```

## Соответствие пунктам ТЗ

| Требование ТЗ | Где живёт в структуре |
|---------------|------------------------|
| Telegram-бот, команды, голос/аудио | `internal/adapters/telegram` + вызовы usecase |
| Очередь и ответы «каждому своё» | `internal/queue` + типы задач с `telegram_user_id` |
| SaluteSpeech (асинхронный REST) | `internal/adapters/salutespeech` |
| GigaChat (саммари, /chat) | `internal/adapters/gigachat` + usecase в `internal/app/usecase` |
| PostgreSQL, поиск, список, get по id | `internal/adapters/persistence/postgres` + usecase |
| Конфиг, логи | `internal/config`, `internal/logger` |
| Точка входа | `cmd/server/main.go` |

## Поток данных (упрощённо)

```
Telegram → adapter/telegram → queue → usecase
                              ↓
                    salutespeech / gigachat / postgres
                              ↓
                    queue / telegram → ответ пользователю
```

В корне каждого основного пакета есть `README.md` с конкретными задачами для реализации.
