---
name: zamk-core
description: ZAMK Core Invariants
trigger: always_on
---

# ZAMK Core Invariants

- ZAMK работает по FBO.
- Seller не выполняет складские/физические операции ZAMK (приёмка, сборка, упаковка, отгрузка, инвентаризация, списание, изменение ZMU, физическая обработка клиентских возвратов).
- Destructive integration tests выполнять только на `zamk_test` (`postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable`).
- Перед destructive DB tests проверять `SELECT current_database() == 'zamk_test'`.
- DEV базу `zamk` нельзя truncate/drop/reset для тестов.
- Committed migrations не редактировать; изменения только новой migration.
- Business E2E не доказывать mocks / route.fulfill.
- Визуальный browser PASS даёт только Product Owner; агент никогда не утверждает визуальный/browser PASS.
- Acceptance останавливается на первом реальном FAIL; последующие обязательные проверки — NOT RUN.
- Не использовать `git add .` / `git add -A`; stage только точные утверждённые пути.
- No force push (`git push --force`).
- No amend уже принятых commits без явного разрешения Product Owner.
- Unrelated dirty work and existing stashes must never be lost, overwritten or discarded. Temporary isolation mechanisms are allowed only when explicitly required by a workflow and must preserve/restore unrelated work safely.
- Commit/push только когда это явно разрешено в задаче или после PO acceptance.
