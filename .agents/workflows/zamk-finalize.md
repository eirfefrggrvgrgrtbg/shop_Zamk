---
name: zamk-finalize
description: Финализация, точечный коммит и пуш майлстоуна после явного подтверждения Product Owner.
---

# ZAMK Finalize Workflow

Workflow для финализации и отправки в upstream изменений, полностью принятых Product Owner.

## 1. Explicit Authorization Check
- Выполнять финализацию ТОЛЬКО после явного подтверждения Product Owner и прямого указания на коммит и пуш.
- Без явного запроса пользователя финализация не инициируется.

## 2. Existing Finalize Flow
- Использовать skill `zamk-finalize` как канонический процесс завершения майлстоуна.
- Проверить, что ветка — `main`, зафиксировать HEAD и stashes.
- Повторно выполнить финальные тесты майлстоуна. Остановка на первом FAIL.

## 3. Standalone Validation & Exact Staging
- Если в worktree присутствуют незавершённые файлы других задач, изолированно валидировать только одобренный скоуп.
- Точечный stage: индексировать только явно одобренные файлы через `git add -- <file1> <file2>`.
- Никогда не использовать `git add .` или `git add -A`.
- Проверить staged diff: `git diff --cached --check`, `git diff --cached --stat`.

## 4. Commit & Normal Push
- Запросить или использовать предоставленное осмысленное краткое сообщение коммита на русском языке (human Russian commit message).
- Create the minimum number of logically independent commits required by the accepted scope. Split commits when dependencies or unrelated hunks require independent validation.
- Выполнить обычный пуш в canonical branch: `git push origin main`.
- Force push (`git push --force`) и несанкционированный `git commit --amend` категорически запрещены.

## 5. Clean Worktree & Intact Stashes
- Убедиться, что скоуп майлстоуна зафиксирован и HEAD равен `origin/main`.
- Подтвердить, что сторонние stashes и незатронутые dirty файлы остались в исходном состоянии.
