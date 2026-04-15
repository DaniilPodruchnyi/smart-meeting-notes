# `internal/app/usecase`

Application-слой с основной бизнес-логикой.

`MeetingService` обрабатывает:
- регистрацию пользователя;
- загрузку аудио и полный pipeline `STT -> summary -> сохранение`;
- `/list`, `/get`, `/find`, `/smart_find`, `/chat`.

Зависимости приходят через интерфейсы:
- репозитории (`UserRepository`, `MeetingRepository`);
- клиенты SaluteSpeech и GigaChat;
- очередь и отправка ответа пользователю.

За счет этого usecase не зависит от конкретных SDK и SQL-реализаций.
