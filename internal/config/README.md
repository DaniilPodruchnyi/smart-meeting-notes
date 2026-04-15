# `internal/config`

Пакет отвечает за загрузку настроек приложения из `.env` и JSON-конфига.

Что сейчас хранится в конфиге:
- HTTP и логирование;
- Telegram токен;
- параметры PostgreSQL и пула `pgxpool`;
- настройки SaluteSpeech;
- API-ключ GigaChat;
- TLS-параметр для исходящих HTTP-запросов (`TLS_INSECURE_SKIP_VERIFY`).

Пакет не содержит бизнес-логику и не зависит от Telegram/SQL-клиентов.
