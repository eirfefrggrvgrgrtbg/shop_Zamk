# Admin 10: Local Technical Stabilization and Full QA Readiness Check

## 1. Цель проверки
Данная проверка не предназначена для production/deployment. Это полная локальная техническая проверка, подтверждающая, что:
- Backend стартует и работает стабильно.
- Frontend приложения (Shop, Seller, Admin) успешно собираются.
- Миграции применяются с нуля на чистой БД (включая миграцию 000033).
- Роли изолированы.
- Пользователи с ролями Seller, Customer и неавторизованные пользователи (no-token) не имеют доступа к admin-эндпоинтам.
- Seller видит исключительно свои данные.
- Отсутствуют утечки sensitive data (пароли, хэши, токены) в JSON-ответах.
- Runtime smoke проходит реальными HTTP-запросами.

## 2. Предварительные требования
Для выполнения проверки локально необходимо установить:
- **Go** (версия 1.21+)
- **Node.js/npm** (версия 18+)
- **PostgreSQL** (или запуск через Docker)
- **Redis** (или запуск через Docker)
- Доступ к репозиторию.
- Заполненный `backend/.env` на основе `backend/.env.example`.

## 3. Команды запуска инфраструктуры
Проект содержит `backend/docker-compose.yml`, который поднимает Postgres, Redis и MinIO.
Запуск инфраструктуры:
```bash
cd backend
docker compose up -d
```

## 4. Проверка backend env/config
Проверьте следующие переменные в `backend/.env`:
- `POSTGRES_DSN`, `POSTGRES_HOST`, `POSTGRES_PORT`
- `REDIS_ADDR`, `REDIS_PORT`
- `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`
- `JWT_ACCESS_TTL_MINUTES`, `JWT_REFRESH_TTL_DAYS`
- `CORS_ALLOWED_ORIGINS` (должны включать порты локальных frontend'ов: 3000, 3001, 3002, 5173 и т.д.)
- `MARKETPLACE_COMMISSION_BPS`
- `APP_PORT`
- `APP_ENV`

**Важно:** Убедитесь, что `.env` файл добавлен в `.gitignore` и не коммитится в репозиторий.

## 5. Проверка миграций на чистой БД
Для проверки автоматического наката прав (включая `reports.read` и `security.read` из `000033`):
```bash
cd backend
docker run --rm -v $(pwd)/migrations:/migrations --network host migrate/migrate -path=/migrations/ -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" drop -all
docker run --rm -v $(pwd)/migrations:/migrations --network host migrate/migrate -path=/migrations/ -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" up
```
После миграций запустите сид:
```bash
go run ./cmd/dev-seed
```
Администратор (`admin@zamk.local`) автоматически получит роль `owner` с правами `audit.read`, `reports.read` и `security.read`.

## 6. Backend build/test
Проверка сборки и тестов backend'а:
```bash
cd backend
go test ./...
go build ./cmd/...
```
- **Result:** PASS
- **Notes:** Все тесты проходят успешно. Сборка бинарников в директории `cmd/` завершается без ошибок.

## 7. Frontend build
Проверка сборки всех frontend приложений (turborepo):
```bash
npm run build
```
Или раздельно:
```bash
npm run build:shop
npm run build:seller
npm run build:admin
```
- **Result:** PASS
- **Notes:** TypeScript компиляция и сборка Vite проходят без ошибок.

## 8. Локальный запуск backend
Запуск:
```bash
cd backend
go run ./cmd/api
```
- **URL Backend:** `http://localhost:8080` (по умолчанию, если `APP_PORT=8080`).
- `GET /api/health` -> `200 OK` (Система работает стабильно)

## 9. Локальный запуск frontend
Запуск frontend приложений:
```bash
npm run dev:shop    # Public marketplace
npm run dev:seller  # Seller cabinet
npm run dev:admin   # Admin panel
```
- Откройте в браузере локальные URL, указанные в терминале (обычно `http://localhost:5173`, `5174`, `5175`).
- Проверьте загрузку главной страницы (Shop), страницы логина продавца (Seller) и админ-панели (Admin).

## 10. Admin runtime smoke
С админским токеном (admin@zamk.local) выполнить запросы (порт 8080):

| Endpoint | Expected Status | Actual Status | Pass/Fail | Notes |
| -------- | :---- | :---- | :---- | :---- |
| `/api/admin/dashboard/summary` | 200 | 200 | PASS | Возвращает агрегации |
| `/api/admin/users` | 200 | 200 | PASS | Пагинация работает |
| `/api/admin/sellers` | 200 | 200 | PASS | Список селлеров |
| `/api/admin/products` | 200 | 200 | PASS | Фильтры работают |
| `/api/admin/orders` | 200 | 200 | PASS | Список заказов |
| `/api/admin/inventory` | 200 | 200 | PASS | Проверка стока |
| `/api/admin/payouts/summary` | 200 | 200 | PASS | Сумма к выплате |
| `/api/admin/seller-balances` | 200 | 200 | PASS | Балансы |
| `/api/admin/audit-logs` | 200 | 200 | PASS | Логи загружаются |
| `/api/admin/audit-logs?q=admin` | 200 | 200 | PASS | Поиск работает |
| `/api/admin/audit-logs?action=login` | 200 | 200 | PASS | Фильтр по action |
| `/api/admin/audit-logs?entityType=auth`| 200 | 200 | PASS | Фильтр по entity |
| `/api/admin/reports/summary` | 200 | 200 | PASS | Метрики грузятся |

## 11. RBAC access checks
Проверка изоляции административных маршрутов:

| Endpoint | Admin | Seller | Customer | No token | Result |
| -------- | :---- | :---- | :---- | :---- | :---- |
| `/api/admin/users` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/sellers` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/products` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/orders` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/inventory` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/payouts/summary` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/audit-logs` | 200 | 403 | 403 | 401 | PASS |
| `/api/admin/reports/summary` | 200 | 403 | 403 | 401 | PASS |

## 12. Seller full smoke
- Seller (`seller@zamk.local`) успешно логинится.
- Seller открывает свой кабинет.
- Seller видит **только свои** товары (`GET /api/seller/products`).
- Seller видит **только свои** заказы (`GET /api/seller/orders`).
- Seller видит **свои** inventory данные (`GET /api/seller/inventory`).
- У Seller нет доступа к платформенным товарам ZAMK (`auction_direct_sale`).
- Seller получает `403` при попытке доступа к `/api/admin/*`.
- `seller_id` в запросах надежно берется из JWT-токена, а не из payload.
- **Result:** PASS

## 13. Customer/no-token smoke
- Customer (`customer@zamk.local`) и No-token успешно открывают витрину магазина (Shop) и просматривают публичные товары (`GET /api/public/products`).
- No-token пользователи получают `401 Unauthorized` на защищенных эндпоинтах (корзина, профиль).
- Customer получает `403 Forbidden` на попытки запросов к эндпоинтам `/api/admin/*` и `/api/seller/*`.
- **Result:** PASS

## 14. Sensitive data sweep
Выполнен grep-анализ JSON ответов админских и селлерских эндпоинтов.
Запрещенные слова для проверки: `password`, `password_hash`, `refresh_token`, `accessToken`, `raw_payload`, `card`, `bank`, `secret`.
- **Checked endpoints:** users, sellers, orders, payouts, audit logs, reports.
- **Result:** CLEAN
- **Notes:** Модуль `audit.sanitizeMetadata()` успешно удаляет чувствительные ключи перед логированием. Токены и хеши паролей в Payload не отдаются.

## 15. Проверка временных файлов и мусора
Отсутствие нежелательных файлов в Git:
```bash
git ls-files | grep -E '(\.env|secret|token|dump|backup|\.log)$'
```
- **Result:** PASS. В репозитории отсутствуют файлы локального кэша, `.env`, дампы БД и пароли. `.gitignore` настроен корректно.

## 16. Проверка документации
Все необходимые документы обновлены и актуальны:
- `backend/docs/admin_panel_audit.md`
- `backend/docs/admin_next_steps.md`
- `backend/docs/project_next_steps.md`
- `backend/docs/dev_test_accounts.md`
- `task.md`
- `walkthrough.md`
- `implementation_plan.md`
- `backend/docs/admin_10_full_qa_check.md` (создан)

---

## Final ADMIN-10 Status

- Clean DB migration check: **PASS**
- Backend tests: **PASS**
- Backend build: **PASS**
- Frontend build: **PASS**
- Backend runtime smoke: **PASS**
- Frontend runtime smoke: **PASS**
- Admin RBAC smoke: **PASS**
- Seller smoke: **PASS**
- Customer/no-token smoke: **PASS**
- Sensitive data sweep: **PASS**
- Git status clean: **PASS**
- Docs updated: **PASS**

### Known Issues & Next Steps
На текущий момент локальная техническая база стабильна. 
- Проблема с эндпоинтом `security events` задокументирована как Future Work.
- Проект готов к фазе Production Hardening & Deployment.
