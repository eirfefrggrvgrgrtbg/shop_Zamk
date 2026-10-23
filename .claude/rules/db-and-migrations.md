---
paths:
  - "backend/**"
---

# БД и миграции (выжимка; источник: `.agents/rules/zamk-backend.md`, тестовые базы — TEST.DB.5)

## Тесты с БД (TEST.DB.5)

- Интеграционные backend-тесты запускать только через `backend/scripts/safe_go_test.sh <явные пакеты> [-- -run '^TestX$' -v]`. Прямой `go test` для интеграционных тестов запрещён, `./...` запрещён.
- Каждый запуск получает собственную disposable-БД `zamk_disposable_<12 hex>`. Другой базы тестам не давать.
- Общие `zamk`, `zamk_test` и `zamk_demo` для тестовых записей запрещены. В них допустимы только read-only `SELECT`, например снимок состояния до и после.
- Если runner не смог создать disposable-БД, запуск прекращается. Не обходить это подстановкой `TEST_DATABASE_URL`, `DATABASE_URL`, `POSTGRES_*` или `PG*`.
- Старые и оставшиеся `zamk_disposable_*` базы, а также шаблон `zamk_disposable_tpl_*` не удалять, тем более массово по маске. Runner сам удаляет только свою базу (имя + токен владельца).
- Ручные SQL `DELETE`/`UPDATE`/`INSERT`/`TRUNCATE` для очистки или восстановления тестовых фикстур запрещены, в том числе через `docker exec … psql`.
- Никогда не использовать старый TEST.DB.3 cleanup и его manifest.
- В тестах брать БД только из `testutil` (`GetTestDatabaseURL`, `RunnerDatabaseURL`, `DisposableDatabaseURL`, `NewDisposableDatabase`). Не зашивать DSN и не читать `TEST_DATABASE_URL` напрямую: это ловит guard `TestNoSharedTestDatabaseAccessOutsideTestutil`.
- Пароли и DSN не копировать в файлы и ответы.

## Рабочая база и миграции

- Никаких миграций и записей в рабочую базу `zamk` без отдельного явного разрешения. Допустимы только read-only `SELECT`.
- Историю миграций не переписывать: закоммиченные миграции не редактировать, исправления — новой миграцией вперёд.
- Не править `schema_migrations`, не использовать `migrate force`; при dirty-состоянии остановиться и сообщить.
- Не делать TRUNCATE, DROP или пересоздание `zamk_test`.
