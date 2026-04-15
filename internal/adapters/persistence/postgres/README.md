# `internal/adapters/persistence/postgres`

PostgreSQL-реализация репозиториев.

Что внутри:
- `db.go` — `pgxpool` и миграции;
- `generic_repository.go` — общий generic-слой для типовых SQL-операций;
- `user_repository.go` и `meeting_repository.go` — предметные репозитории.

Особенности текущей версии:
- используется расширение `vector` (`pgvector`);
- embeddings хранятся в полях `vector`;
- `/smart_find` считает релевантность прямо в SQL.
