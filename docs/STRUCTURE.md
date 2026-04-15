# Структура проекта

Ниже схема, которая соответствует текущему состоянию репозитория.

```text
smart-meeting-notes/
├── cmd/server/main.go
├── docs/STRUCTURE.md
├── internal/
│   ├── app/
│   │   ├── repository/         # интерфейсы репозиториев
│   │   └── usecase/            # бизнес-сценарии (meeting/ping)
│   ├── adapters/
│   │   ├── telegram/           # прием команд, голоса и аудио
│   │   ├── salutespeech/       # STT-клиент
│   │   ├── gigachat/           # chat + embeddings
│   │   └── persistence/
│   │       └── postgres/       # pgxpool, миграции, репозитории
│   ├── config/                 # загрузка env/json-конфига
│   ├── domain/                 # сущности User/Meeting
│   ├── logger/                 # настройка zap
│   ├── pkg/httptls/            # общий TLS transport для исходящих HTTP-клиентов
│   ├── queue/                  # фоновая очередь задач и маршрутизация ответов
│   └── server/                 # HTTP-сервер и transport/http
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── README.md
```

## Как идут данные

1. Telegram-адаптер принимает команду или аудио.
2. Сообщение попадает в очередь.
3. `MeetingService` обрабатывает задачу:
   - дергает SaluteSpeech для транскрипции;
   - при необходимости вызывает GigaChat (summary, embeddings);
   - сохраняет/читает данные через postgres-репозитории.
4. Ответ уходит обратно пользователю в Telegram.

## Границы слоев

- `usecase` не знает про детали SDK/SQL.
- `adapters/*` реализуют интеграции с внешним миром.
- `domain` хранит базовые сущности.
- `cmd/server/main.go` отвечает только за сборку зависимостей и запуск.
