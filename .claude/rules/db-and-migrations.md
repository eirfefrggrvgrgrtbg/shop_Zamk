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

## Тесты с Redis (TEST.REDIS.ISOLATION.1)

- Тесты, которым нужен Redis, запускать через `backend/scripts/safe_go_test_redis.sh`. Он поднимает собственный Docker-контейнер Redis на запуск и затем вызывает `safe_go_test.sh` без изменений, то есть тест получает и свою disposable-PostgreSQL, и свой Redis. Интерфейс тот же: явные пакеты `./internal/...`, затем `-- <флаги go test>`; других параметров у wrapper нет.
- Нужны Docker и локальный образ `redis:7-alpine`. Образ никогда не скачивается (`--pull never`). Нет Docker, образа, контейнера или подтверждённого владельца: wrapper выходит с `REFUSED`, тесты не стартуют.
- Контейнер: порт только на `127.0.0.1`, выбирается динамически, пароль = случайный токен запуска, persistence и AOF выключены. Адрес и токен передаются тестам в `ZAMK_TEST_REDIS_ADDR` и `ZAMK_TEST_REDIS_TOKEN`.
- Cleanup: wrapper удаляет только свой контейнер по полному ID и после сверки метки владельца с токеном; на выходе, при провале тестов и при INT/TERM/HUP. По порту и имени ничего не останавливается. При SIGKILL или аварии машины очистка не гарантирована.
- Токен временного Redis виден в аргументах процессов и в Docker metadata, пока контейнер жив. Токен не коммитить, не печатать и не копировать в отчёты.
- Общий Redis `localhost:6379` и контейнер `zamk_redis` для тестов запрещены. Выбор другой логической базы (DB 9 и т. п.) на общем сервере изоляцией не считается.
- Новый Redis-интеграционный тест получает клиент только через `testutil.NewTestRedis(t, ctx)`; хардкод `localhost:6379` и прямой `redis.NewClient` с общим адресом недопустимы. Helper без переменных runner-а останавливает тест до подключения и отклоняет порт 6379, не-loopback, неверный токен и Redis с persistence.
- Тесты, которые ещё подключаются к `localhost:6379`, безопасными не считаются. Не запускать их ради зелёного результата. Если безопасность среды не доказана, тест не запускать.

### Подтверждённый набор (команда)

Изоляция Redis и PostgreSQL подтверждена прогонами только для этих девяти тестов: `TestCartAddItemVariantErrors`, `TestCartStaleness`, `TestCartBrandConsistency`, `TestCartVariantImageTruth`, `TestCatalogDeterminism`, `TestEditorialCollections_AdminAndPublic`, `TestProductPublicationContract`, `TestProductVariantTruth`, `TestCartEligibilityAndCheckout`.

```bash
npm run test:backend:cart-catalog
```

Это `backend/scripts/safe_go_test_redis.sh ./internal/http/router/ -- -run '^(...эти девять...)$' -v`; для случайного порядка добавить `-shuffle=on` в конец `-- ...` при ручном вызове wrapper. Прямой `go test` и `./...` не использовать. Их нельзя запускать напрямую против `zamk_test`.

## Рабочая база и миграции

- Никаких миграций и записей в рабочую базу `zamk` без отдельного явного разрешения. Допустимы только read-only `SELECT`.
- Историю миграций не переписывать: закоммиченные миграции не редактировать, исправления — новой миграцией вперёд.
- Не править `schema_migrations`, не использовать `migrate force`; при dirty-состоянии остановиться и сообщить.
- Не делать TRUNCATE, DROP или пересоздание `zamk_test`.
