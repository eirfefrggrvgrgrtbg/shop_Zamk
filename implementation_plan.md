# План развития админ-панели ZAMK (Admin Panel Product Audit)

> Документ — результат аудита текущей админки (apps/admin), backend admin endpoints и план превращения админ-панели в центральную систему управления маркетплейсом со штатными ролями и permission-моделью.
> RBAC сейчас НЕ реализован и в рамках этого документа НЕ реализуется — только планируется.

---

## 1. Инвентаризация текущих страниц админки

Инспектированы: `apps/admin/src/App.tsx`, `AdminLayout.tsx`, `AdminProtectedRoute.tsx`, `AdminAuthContext.tsx`, все 19 файлов в `apps/admin/src/pages/`, все 10 файлов `apps/admin/src/api/`, `apps/admin/src/lib/api.ts`, `packages/api-client/src/admin.ts`, `packages/api-client/src/types.ts`.

| Route | Component | Sidebar label | Что показывает | Реальный API или заглушка | Полезно? | Вердикт |
|---|---|---|---|---|---|---|
| `/login` | `AdminLogin` | — (вне layout) | Форма входа, проверка `role === 'admin'` на клиенте | Реальный API (`/auth/login`) | Да | stay (перевести англ. тексты на русский) |
| `/change-password` | `AdminChangePassword` | — (вне layout) | Смена пароля (mustChangePassword flow) | Реальный API (`/auth/change-password`) | Да | stay |
| `/dashboard` | `AdminDashboard` | Панель управления | 6 счётчиков, считаемых на клиенте из полных списков (sellers, products, moderation, orders, returns, payouts); блок «Журнал действий» — заглушка | Реальный API, но антипаттерн: 6 полных выборок ради счётчиков | Частично | stay + редизайн (см. раздел 8) |
| `/users` | `AdminUsers` | Пользователи | Только заглушка «Нет данных» | Пустая заглушка, API нет вообще | Нет | hide (до появления backend `GET /admin/users`); в будущем разделить на «Покупатели» и «Сотрудники» |
| `/sellers` | `AdminSellers` | Продавцы | Список продавцов, создание доступа продавца (модалка), активация/блокировка | Реальный API (`/admin/sellers`) | Да | stay + расширить (верификация, предупреждения, карточка продавца) |
| `/categories` | `AdminCategories` | Категории | Список + создание категории (name, slug). Нет edit/delete, нет иерархии | Реальный API (`/admin/categories`) | Да | merge → объединить с «Бренды» в «Категории и бренды» |
| `/brands` | `AdminBrands` | Бренды | Список + создание бренда + загрузка логотипа | Реальный API (`/admin/brands`) | Да | merge → в «Категории и бренды» |
| `/products` | `AdminProducts` | Товары | Все товары, действия approve/reject/publish/hide/block, загрузка изображения админом | Реальный API (`/admin/products`, `/admin/moderation/...`) | Да | stay (русифицировать: «All Products», кнопки на англ.) |
| `/moderation` | `AdminModeration` | Модерация | Очередь `pending_moderation`: карточки с галереей и вариантами, approve/reject | Реальный API (`/admin/moderation/products`) | Да | stay (есть дублирование действий со страницей «Товары» — оставить как фокус-очередь) |
| `/orders` | `AdminOrders` | Заказы | Список заказов + inline-деталка: товары, покупатель, смена операционного статуса, создание отгрузки | Реальный API (`/admin/orders`) | Да | stay (перевести в drawer-деталку, русифицировать) |
| `/payments` | `AdminPayments` | Платежи | Read-only список платежей + деталка; пояснение про вебхуки | Реальный API (`/admin/payments`) | Да | stay |
| `/shipments` | `AdminShipments` | Отгрузки | Список отгрузок + ручное создание (по ID заказа!) + смена статуса | Реальный API (`/admin/shipments`) | Да | stay; убрать ручной ввод orderId — создавать отгрузку из карточки заказа |
| `/inventory` | `AdminInventory` | Остатки | Остатки по вариантам, приёмка/корректировка/списание, движения | Реальный API (`/admin/inventory`) | Да | stay (русифицировать) |
| `/returns` | `AdminReturns` | Возвраты | Возвраты + деталка, смена статуса, создание возмещения | Реальный API (`/admin/returns`) | Да | stay (русифицировать «Return Requests») |
| `/refunds` | `AdminRefunds` | Возмещения | Read-only список возмещений + деталка | Реальный API (`/admin/refunds`) | Да | stay |
| `/payouts` | `AdminPayouts` | Выплаты | Заявки на выплату, смена статуса (approve/reject/paid) с предупреждением про ручной перевод | Реальный API (`/admin/payouts`) | Да | stay |
| `/reviews` | `AdminReviews` | Отзывы | Отзывы, approve/reject/hide/block + деталка | Реальный API (`/admin/reviews`) | Да | stay |
| `/audit-logs` | `AdminAuditLogs` | Журнал действий | Только заглушка «Нет данных» | Пустая заглушка, backend audit log не существует | Нет (пока) | hide до Admin Phase B (или оставить с честной заглушкой) |
| `/settings` | `AdminSettings` | Настройки | Захардкоженный «Admin User / admin@zamk.com», плейсхолдеры «Settings form placeholders...» | Полная заглушка, частично с фейковыми данными | Нет | rename/rework → «Профиль» (реальные данные из AdminAuthContext) + скрыть фейковые блоки System Settings / Security |

**Итого:** 19 страниц (17 в sidebar-навигации + login + change-password).
- Реально работают с API: **14** (dashboard, sellers, categories, brands, products, moderation, orders, payments, shipments, inventory, returns, refunds, payouts, reviews).
- Полные заглушки: **3** (users, audit-logs, settings — у settings ещё и фейковые захардкоженные данные).
- Вердикты: stay — 13, merge — 2 (categories+brands), hide — 2 (users, audit-logs), rework — 2 (settings, dashboard).
- Сквозная проблема: смесь русского и английского в заголовках, таблицах и кнопках почти на каждой странице.

Замечена рассинхронизация API-слоёв: в `apps/admin/src/api/*` есть собственные обёртки (view-модели, словари статусов), а `packages/api-client/src/admin.ts` дублирует те же вызовы с другими сигнатурами (например, `createAdminShipment(orderId, data)` vs `createAdminShipment({orderId, ...})` и POST `/admin/shipments` которого нет в admin.ts). Источник истины надо консолидировать.

---

## 2. Инвентаризация backend admin endpoints

Источник: `backend/internal/http/router/router.go`. Все admin-маршруты защищены `AuthMiddleware` + `RequireRole("admin")` (`backend/internal/http/middleware/auth.go`). «Опасные» действия дополнительно ограничены rate-limit группой `admin_dangerous`.

| Endpoint | Method | Текущая роль | Действие | Будущий fine-grained permission |
|---|---|---|---|---|
| `/api/admin/sellers` | POST | admin | Создать доступ продавца (user + seller) | `sellers.create_access` |
| `/api/admin/sellers` | GET | admin | Список продавцов | `sellers.read` |
| `/api/admin/sellers/{id}/status` | PATCH | admin | Сменить статус продавца (pending/active/blocked/archived) | `sellers.update_status` |
| `/api/admin/categories` | GET | admin | Список категорий | `categories.read` |
| `/api/admin/categories` | POST | admin | Создать категорию | `categories.create` |
| `/api/admin/brands` | GET | admin | Список брендов | `brands.read` |
| `/api/admin/brands` | POST | admin | Создать бренд | `brands.create` |
| `/api/admin/brands/{id}/logo/upload` | POST | admin | Загрузить логотип бренда | `brands.update` |
| `/api/admin/products` | GET | admin | Все товары | `products.read` |
| `/api/admin/products/{id}/images/upload` | POST | admin | Загрузить изображение товара | `products.moderate` |
| `/api/admin/moderation/products` | GET | admin | Очередь модерации | `products.read` + `products.moderate` |
| `/api/admin/moderation/products/{id}/approve` | POST | admin | Одобрить товар | `products.approve` |
| `/api/admin/moderation/products/{id}/reject` | POST | admin | Отклонить товар | `products.reject` |
| `/api/admin/moderation/products/{id}/publish` | POST | admin | Опубликовать товар | `products.publish` |
| `/api/admin/moderation/products/{id}/hide` | POST | admin | Скрыть товар | `products.hide` |
| `/api/admin/moderation/products/{id}/block` | POST | admin | Заблокировать товар | `products.block` |
| `/api/admin/inventory` | GET | admin | Остатки по всем продавцам | `inventory.read` |
| `/api/admin/inventory/{id}` | GET | admin | Позиция остатков | `inventory.read` |
| `/api/admin/inventory/{id}/movements` | GET | admin | Движения остатков | `inventory.movements.read` |
| `/api/admin/inventory/receipts` | POST | admin | Приёмка | `inventory.receipt` |
| `/api/admin/inventory/adjustments` | POST | admin | Корректировка | `inventory.adjust` |
| `/api/admin/inventory/write-offs` | POST | admin | Списание | `inventory.write_off` |
| `/api/admin/orders` | GET | admin | Все заказы | `orders.read` |
| `/api/admin/orders/{id}` | GET | admin | Детали заказа | `orders.read` |
| `/api/admin/orders/{id}/status` | PATCH | admin | Смена операционного статуса заказа | `orders.update_status` |
| `/api/admin/orders/{id}/shipment` | POST | admin | Создать отгрузку по заказу | `shipments.create` |
| `/api/admin/payments` | GET | admin | Все платежи (read-only) | `payments.read` |
| `/api/admin/payments/{id}` | GET | admin | Детали платежа | `payments.read` |
| `/api/admin/shipments` | GET | admin | Все отгрузки | `shipments.read` |
| `/api/admin/shipments/{id}` | GET | admin | Детали отгрузки | `shipments.read` |
| `/api/admin/shipments/{id}/status` | PATCH | admin | Смена статуса отгрузки | `shipments.update_status` |
| `/api/admin/returns` | GET | admin | Все возвраты | `returns.read` |
| `/api/admin/returns/{id}` | GET | admin | Детали возврата | `returns.read` |
| `/api/admin/returns/{id}/status` | PATCH | admin | Смена статуса возврата | `returns.update_status` |
| `/api/admin/returns/{id}/refund` | POST | admin | Создать возмещение по возврату | `refunds.create` |
| `/api/admin/refunds` | GET | admin | Все возмещения | `refunds.read` |
| `/api/admin/refunds/{id}` | GET | admin | Детали возмещения | `refunds.read` |
| `/api/admin/payouts` | GET | admin | Все заявки на выплаты | `payouts.read` |
| `/api/admin/payouts/{id}` | GET | admin | Детали выплаты | `payouts.read` |
| `/api/admin/payouts/{id}/status` | PATCH | admin | approve / reject / mark paid | `payouts.approve` / `payouts.reject` / `payouts.mark_paid` (по целевому статусу) |
| `/api/admin/payouts/trigger-availability` | POST | admin | Пересчёт доступного баланса (служебный) | `payouts.read` (или `settings.manage` — решить) |
| `/api/admin/reviews` | GET | admin | Все отзывы | `reviews.read` |
| `/api/admin/reviews/{id}` | GET | admin | Детали отзыва | `reviews.read` |
| `/api/admin/reviews/{id}/{action}` | POST | admin | approve/reject/hide/block отзыва | `reviews.approve` / `reviews.reject` / `reviews.hide` / `reviews.block` |

### Модель ролей и что отсутствует

- Роли: жёсткий enum в `backend/internal/users/model.go` и в CHECK-констрейнте `users.role IN ('customer','seller','admin')` (миграция 000001). Никакой градации внутри admin нет.
- `RequireRole` — простое сравнение строки роли из JWT claims. Permissions в токене нет.
- **Audit log: НЕ существует.** Есть только доменные журналы: `order_status_history`, `product_moderation_logs`, `product_review_moderation_logs`, движения остатков, payout-ledger записи. Единого журнала «кто из админов что сделал» нет; в moderation-логах actor фиксируется, но в смене статуса продавца/выплатах/инвентаре — единообразного actor-трекинга нет.
- **Таблиц staff/permissions/roles в миграциях нет** (000001–000013 проверены).
- Отсутствуют: `GET /admin/users` (список покупателей/пользователей), агрегатные endpoint'ы для дашборда, endpoints предупреждений продавцам, сообщений, жалоб, тикетов поддержки, настроек витрины, комиссии, экспорта.

---

## 3. Идеальная структура админ-панели (21 раздел)

Приоритеты: P0 — критично сейчас, P1 — фундамент, P2 — операционка, P3 — потом.

| № | Раздел | Назначение | Кто использует | Ключевые действия | Данные | Permissions | Сейчас | Чего не хватает | Приоритет |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Главная (Dashboard) | Оперативная сводка дня | все роли (виджеты по правам) | переходы в очереди | агрегаты: заказы, очереди, алерты, выручка | `analytics.read` + per-widget | Есть, но клиентские подсчёты | агрегатные endpoint'ы, виджеты по правам | P1 |
| 2 | Доступы и роли | Просмотр ролей и их permissions, (позже) кастомные роли | owner, co_owner | смотреть матрицу, назначать | каталог permissions, роли | `roles.read`, `roles.manage` | Нет | всё (Phase B/C) | P1 |
| 3 | Сотрудники | Управление штатом: создание доступов, блокировка, сброс пароля | owner, co_owner, admin | создать доступ, сменить роль, заблокировать | staff-пользователи + роли | `staff.read/create/update/block` | Нет | всё (Phase B/C) | P1 |
| 4 | Продавцы | Реестр продавцов, карточка, статусы, верификация, предупреждения, сообщения | admin, support (read) | activate/block/verify/warn/message | sellers + владелец + активность | `sellers.*` | Частично (список+статус+создание) | карточка продавца, верификация, warn, messages, история | P1 |
| 5 | Заявки / Проверка продавцов | Очередь pending-продавцов и проверка профилей | admin, moderator | verify → active, отклонить | sellers со status=pending | `sellers.verify` | Нет (pending виден в общем списке) | отдельная очередь, чек-лист проверки | P2 |
| 6 | Товары | Полный каталог с фильтрами | admin, moderator, content_manager (read) | publish/hide/block, правка медиа | products всех продавцов | `products.read/...` | Есть | фильтры/поиск/пагинация в UI, карточка товара | P1 |
| 7 | Модерация товаров | Очередь pending_moderation | moderator | approve/reject с комментарием | очередь + полная карточка | `products.moderate/approve/reject` | Есть | история модерации, диффы при повторной подаче | P1 |
| 8 | Категории и бренды | Управление таксономией каталога | content_manager, admin | CRUD, иерархия, логотипы | categories + brands | `categories.*`, `brands.*` | Частично (только create+list, два раздела) | edit/delete/деактивация, иерархия категорий, объединить в один раздел | P2 |
| 9 | Остатки / Склад | Остатки по всем продавцам, складские операции | warehouse_operator, admin | приёмка, корректировка, списание | inventory + движения | `inventory.*` | Есть | фильтры, алерты низких остатков | P1 |
| 10 | Заказы | Все заказы маркетплейса | admin, support (read) | смена статуса, создание отгрузки | orders + items + customer | `orders.read/update_status` | Есть | drawer-деталка, фильтры, связка с платежом/отгрузкой/возвратом | P0 (UX) |
| 11 | Доставка / Отгрузки | Все отгрузки и трекинг | warehouse_operator, admin | смена статуса, трекинг | shipments | `shipments.*` | Есть | создание только из заказа, убрать ручной ввод orderId | P2 |
| 12 | Платежи покупателей | Read-only мониторинг оплат | finance | просмотр, фильтр ошибок | payments | `payments.read` | Есть | фильтры, алерты failed-платежей | P2 |
| 13 | Возвраты | Workflow возвратов | support, admin | смена статуса, запуск возмещения | returns + items | `returns.*` | Есть | русификация, фото/вложения покупателя | P1 |
| 14 | Возмещения | Read-only реестр возмещений | finance | просмотр | refunds | `refunds.read` | Есть | связка с провайдером (когда появится реальный) | P2 |
| 15 | Выплаты продавцам | Заявки на выплаты, approve/reject/paid | finance, owner | approve, reject, mark paid | payouts + балансы | `payouts.*` | Есть | реквизиты продавца, документы, экспорт | P1 |
| 16 | Отзывы | Модерация отзывов | moderator | approve/reject/hide/block | reviews | `reviews.*` | Есть | фильтры по статусу/рейтингу | P2 |
| 17 | Жалобы | Жалобы покупателей на товары/продавцов | moderator, support | рассмотреть, эскалировать, наказать | complaints (нет в backend) | `complaints.*` | Нет | всё: схема, API, UI (Phase G) | P2 |
| 18 | Поддержка | Тикеты покупателей и продавцов | support | ответить, закрыть, эскалировать | tickets (нет в backend) | `support.*` | Нет | всё (Phase G) | P2 |
| 19 | Настройки витрины | Главная страница shop: баннеры, подборки, эксклюзивы | content_manager | управление блоками главной | storefront-конфиг (нет) | `storefront.manage` | Нет | всё (Phase H) | P3 |
| 20 | Журнал действий | Единый audit log действий staff | owner, admin | просмотр, фильтр по actor/entity | audit_logs (нет) | `audit.read` | Заглушка | таблица audit_logs + запись из всех admin-операций + UI | P1 |
| 21 | Системные настройки | Комиссия, политика продавцов, лимиты, фичефлаги | owner | менять комиссию, политики | settings (нет) | `settings.manage`, `commission.manage` | Фейковая заглушка | всё; пока скрыть фейк | P2 |

---

## 4. Роли сотрудников

Все staff-пользователи остаются `users.role = 'admin'`; внутренняя роль и permissions — в новых staff-таблицах (см. раздел 5).

| Роль | Разрешённые разделы | Запрещённые разделы | Опасные действия: можно | Опасные действия: нельзя |
|---|---|---|---|---|
| `owner` | все 21 | — | всё: комиссия, системные настройки, создание/удаление любых staff, выплаты | — (но удалить owner не может никто, включая его самого через UI — только передача владения) |
| `co_owner` | все, включая системные настройки | — | как owner | удалить/заблокировать/понизить owner; передать владение |
| `admin` | 1, 3 (в рамках своих прав), 4–18, 20 | Системные настройки (21), управление ролями (2 — read-only) | блокировка продавцов/товаров, выплаты (если выданы права), создание staff ниже себя (если разрешено owner'ом) | менять комиссию, системные настройки, управлять owner/co_owner, выдавать права, которых нет у него самого |
| `moderator` | 1 (свои виджеты), 6, 7, 16, 17 | финансы (12, 14, 15), сотрудники, настройки, склад | reject/block товаров и отзывов, рассмотрение жалоб | блокировка продавцов, любые денежные операции, доступ к данным покупателей сверх нужного |
| `support` | 1, 10 (read-only), 13, 17, 18 | финансы, товары/модерация, сотрудники, настройки, склад | смена статусов возвратов, работа с тикетами | смена статусов заказов, любые денежные операции, блокировки |
| `finance` | 1 (фин. виджеты), 12, 14, 15; 10 — read-only | модерация, сотрудники, склад, витрина, системные настройки (кроме просмотра комиссии — по решению owner) | approve/reject/mark_paid выплат, создание возмещений | блокировки продавцов/товаров, изменение комиссии (только owner) |
| `content_manager` | 1, 8, 19; 6 — read-only | все финансы, заказы, сотрудники, склад, модерация | CRUD категорий/брендов, публикация блоков витрины | любые денежные операции, модерация товаров, доступ к данным покупателей |
| `warehouse_operator` | 1 (склад-виджеты), 9, 11; 10 — read-only | все финансы, модерация, сотрудники, настройки | приёмка/корректировка/списание, статусы отгрузок | денежные операции, смена статусов заказов (кроме операций отгрузки — по решению), блокировки |

Ключевые правила:
- `owner` — полный доступ, включая `commission.manage` и `settings.manage`; аккаунт owner не может быть удалён или заблокирован другими staff.
- `co_owner` — всё то же, кроме операций над owner.
- `admin` — операционное управление без системных настроек.
- Никто не может выдать другому права, которых нет у него самого (anti-privilege-escalation).

---

## 5. Модель permissions

Принципы:
- Широкая роль `users.role` (customer/seller/admin) **сохраняется** — она продолжает разграничивать `/api/customer`, `/api/seller`, `/api/admin`.
- Fine-grained permissions применяются **только** к пользователям с `role = 'admin'` и живут в отдельных таблицах: `staff_roles` (каталог ролей), `staff_role_permissions`, `staff_members` (user_id → staff_role, кастомные overrides — опционально v2).
- Backend middleware `RequirePermission("...")` навешивается на каждый admin endpoint поверх `RequireRole("admin")`.

### Каталог permissions

```
staff.read, staff.create, staff.update, staff.block, staff.delete
roles.read, roles.manage
sellers.read, sellers.create_access, sellers.update_status, sellers.warn, sellers.message, sellers.verify
products.read, products.moderate, products.approve, products.reject, products.publish, products.hide, products.block
categories.read, categories.create, categories.update, categories.delete
brands.read, brands.create, brands.update, brands.delete
inventory.read, inventory.receipt, inventory.adjust, inventory.write_off, inventory.movements.read
orders.read, orders.update_status
payments.read
shipments.read, shipments.create, shipments.update_status
returns.read, returns.update_status
refunds.read, refunds.create
payouts.read, payouts.approve, payouts.reject, payouts.mark_paid
reviews.read, reviews.approve, reviews.reject, reviews.hide, reviews.block
complaints.read, complaints.resolve
support.read, support.respond, support.close
analytics.read
exports.excel
settings.read, settings.manage
audit.read
storefront.manage
commission.manage
```

### Матрица роль → permissions

| Permission группа | owner | co_owner | admin | moderator | support | finance | content_manager | warehouse_operator |
|---|---|---|---|---|---|---|---|---|
| `staff.*` | ✔ все | ✔ (кроме операций над owner) | read + create/update/block (если выдано owner'ом) | — | — | — | — | — |
| `roles.read` / `roles.manage` | ✔ / ✔ | ✔ / ✔ | ✔ / — | — | — | — | — | — |
| `sellers.read` | ✔ | ✔ | ✔ | ✔ | ✔ | ✔ | — | — |
| `sellers.create_access/update_status/verify` | ✔ | ✔ | ✔ | — | — | — | — | — |
| `sellers.warn/message` | ✔ | ✔ | ✔ | warn — ✔ | message — ✔ | — | — | — |
| `products.read` | ✔ | ✔ | ✔ | ✔ | ✔ | — | ✔ | ✔ |
| `products.moderate/approve/reject/publish/hide/block` | ✔ | ✔ | ✔ | ✔ | — | — | — | — |
| `categories.*`, `brands.*` | ✔ | ✔ | ✔ | read | — | — | ✔ | — |
| `inventory.read/movements.read` | ✔ | ✔ | ✔ | — | — | — | — | ✔ |
| `inventory.receipt/adjust/write_off` | ✔ | ✔ | ✔ | — | — | — | — | ✔ |
| `orders.read` | ✔ | ✔ | ✔ | — | ✔ | ✔ | — | ✔ |
| `orders.update_status` | ✔ | ✔ | ✔ | — | — | — | — | — |
| `payments.read` | ✔ | ✔ | ✔ | — | — | ✔ | — | — |
| `shipments.read` | ✔ | ✔ | ✔ | — | ✔ | — | — | ✔ |
| `shipments.create/update_status` | ✔ | ✔ | ✔ | — | — | — | — | ✔ |
| `returns.read/update_status` | ✔ | ✔ | ✔ | — | ✔ | read | — | read |
| `refunds.read` | ✔ | ✔ | ✔ | — | ✔ | ✔ | — | — |
| `refunds.create` | ✔ | ✔ | ✔ | — | — | ✔ | — | — |
| `payouts.read` | ✔ | ✔ | ✔ | — | — | ✔ | — | — |
| `payouts.approve/reject/mark_paid` | ✔ | ✔ | — (опционально) | — | — | ✔ | — | — |
| `reviews.read` | ✔ | ✔ | ✔ | ✔ | ✔ | — | — | — |
| `reviews.approve/reject/hide/block` | ✔ | ✔ | ✔ | ✔ | — | — | — | — |
| `complaints.read/resolve` | ✔ | ✔ | ✔ | ✔ | read | — | — | — |
| `support.*` | ✔ | ✔ | ✔ | — | ✔ | — | — | — |
| `analytics.read` | ✔ | ✔ | ✔ | частично | частично | фин. часть | — | склад. часть |
| `exports.excel` | ✔ | ✔ | ✔ | — | — | ✔ | — | ✔ |
| `settings.read/manage` | ✔ | ✔ | read | — | — | — | — | — |
| `audit.read` | ✔ | ✔ | ✔ | — | — | — | — | — |
| `storefront.manage` | ✔ | ✔ | ✔ | — | — | — | ✔ | — |
| `commission.manage` | ✔ | ✔ | — | — | — | — | — | — |

---

## 6. Генератор доступов сотрудников

Флоу (страница «Сотрудники», кнопка «Создать доступ»):

1. Owner / co_owner / авторизованный admin (имеющий `staff.create`) открывает форму.
2. Поля: имя, email, роль (из списка раздела 4), телефон (опционально), временный пароль — сгенерированный (кнопка «Сгенерировать») или введённый вручную (мин. 12 символов для staff).
3. Backend создаёт пользователя: `users.role='admin'`, `must_change_password=true`, запись в `staff_members` с выбранной staff-ролью; permissions выводятся из роли (не хранятся копией на пользователе в v1).
4. Временный пароль показывается **ровно один раз** в модалке (паттерн уже реализован для продавцов — переиспользовать).

Правила безопасности:
- Пароль нигде не хранится в открытом виде — только bcrypt/argon2-хэш.
- Пароль не пишется в логи (ни backend slog, ни audit payload — в audit фиксируется только факт «staff access created», без секретов).
- Создание/изменение/блокировка staff обязательно пишется в audit log (actor, target, role, timestamp, IP).
- Создающий не может назначить роль/permissions выше собственных (анти-эскалация: admin не может создать co_owner).
- Только owner/co_owner и admin с явным правом `staff.create` могут создавать staff.
- Owner не может быть удалён или заблокирован другими staff; смена владельца — отдельная процедура вне обычного UI.

---

## 7. Создание доступа продавца

Целевой флоу (частично уже работает):

1. Админ НЕ создаёт бренд/магазин. Админ создаёт **доступ**: `ownerName`, `ownerEmail`, временный пароль, опционально — начальное отображаемое имя магазина; статус по политике — `pending` или `active` (открытый вопрос, см. раздел 12).
2. Продавец логинится в apps/seller, обязан сменить пароль (`mustChangePassword` — уже реализовано), затем сам заполняет профиль магазина: brandName, slug, description, logo, контакты (`PATCH /api/seller/me` — **уже реализовано**).
3. Дальше админ может: верифицировать (pending → active), предупреждать, блокировать, писать сообщения, смотреть активность.

Что уже есть:
- Создание доступа с временным паролем + показ пароля один раз (`AdminSellers.tsx`) — есть.
- `mustChangePassword` flow — есть.
- Смена статуса продавца (pending/active/blocked/archived) — есть.
- Самостоятельное заполнение профиля в apps/seller — есть.

Несоответствия / отсутствует:
- **Mismatch (исправить):** `CreateSellerRequest` (backend `sellers/dto.go`) требует `brandName` и `contactEmail` как обязательные — то есть админ фактически создаёт бренд, что противоречит целевой бизнес-логике. UI это маскирует подписью «временное название». Нужно: сделать `brandName`/`contactEmail` опциональными (генерировать плейсхолдер) либо убрать из формы админа.
- Нет верификации как отдельного действия с чек-листом (есть только общий update_status).
- Нет предупреждений (warnings), сообщений админ↔продавец, журнала активности продавца, скоринга.
- Ответ `CreateSellerResponse` возвращает `temporaryPasswordReturned: bool`, а фронт читает `res.temporaryPassword` — фактически показывается пароль, введённый/сгенерированный на клиенте; контракт нужно выровнять.

---

## 8. Редизайн Dashboard

Текущая реализация загружает 6 полных списков и считает счётчики на клиенте — не масштабируется и тянет данные, на которые у будущих ролей может не быть прав.

| Виджет | Нужный endpoint (новый) | Permission | Если не реализовано |
|---|---|---|---|
| Заказы за сегодня (кол-во, сумма) | `GET /admin/dashboard/orders-today` | `orders.read` + `analytics.read` | «Нет данных» |
| Товары на модерации | `GET /admin/dashboard/moderation-pending` (count) | `products.moderate` | скрыт без права |
| Продавцы на проверке (pending) | `GET /admin/dashboard/sellers-pending` | `sellers.verify` | скрыт без права |
| Возвраты в работе | `GET /admin/dashboard/returns-pending` | `returns.read` | «Нет данных» |
| Выплаты в ожидании (кол-во, сумма) | `GET /admin/dashboard/payouts-pending` | `payouts.read` | скрыт без права |
| Ошибки платежей (за 24ч/7д) | `GET /admin/dashboard/payment-failures` | `payments.read` | «Нет данных» |
| Алерты остатков (low/zero stock) | `GET /admin/dashboard/stock-alerts` | `inventory.read` | скрыт без права |
| Новые жалобы | `GET /admin/dashboard/complaints-new` | `complaints.read` | скрыт до Phase G |
| Последние действия админов | `GET /admin/audit-logs?limit=10` | `audit.read` | «Журнал пока не подключён» (честная заглушка — уже так) |
| Выручка маркетплейса (день/неделя/месяц) | `GET /admin/dashboard/revenue` | `analytics.read` + finance-доступ | скрыт без права |
| Обязательства перед продавцами (сумма к выплате) | `GET /admin/dashboard/payout-liabilities` | `payouts.read` | скрыт без права |

Принципы: один сводный endpoint `GET /admin/dashboard/summary` допустим вместо россыпи (backend сам фильтрует блоки по permissions токена); никаких демо-данных; виджет без права — скрыт, виджет без данных/без backend — «Нет данных».

---

## 9. Отсутствующие функции админки

| Функция | Приоритет | Зависимости | Backend | Frontend |
|---|---|---|---|---|
| Staff RBAC (роли, permissions, middleware) | P1 | — (фундамент) | таблицы staff, seed owner, RequirePermission | страницы «Доступы и роли», «Сотрудники» |
| Реальный audit log | P1 | RBAC (actor) | таблица audit_logs + запись во всех admin-операциях | страница «Журнал действий» |
| Dashboard aggregate endpoints | P1 | — | SQL-агрегаты | новый Dashboard |
| Предупреждения продавцам (warnings) | P1 | RBAC желательно | таблица seller_warnings + API | UI в карточке продавца |
| Коммуникация админ ↔ продавец | P2 | warnings/карточка продавца | messages-схема + API | тред сообщений в карточках (admin+seller) |
| Жалобы на товары (complaints) | P2 | — | схема + API + связка с покупателем | страница «Жалобы», подача из apps/shop |
| Тикеты поддержки | P2 | — | схема + API | страница «Поддержка», подача из shop/seller |
| Управление главной витрины (storefront) | P3 | — | конфиг-схема + публичный endpoint | страница «Настройки витрины», рендер в apps/shop |
| Управление эксклюзивами | P3 | storefront | флаг/коллекция exclusives | блок в витрине + бейджи |
| Управление аукционами | P3 | отдельный домен | auction-схема, ставки, таймеры | отдельный раздел |
| Тех. состояние товара (видно выбранным ролям) | P2 | RBAC | поле condition + permission-фильтрация в DTO | отображение по праву |
| Скоринг продавцов | P3 | накопленные данные | расчёт метрик (отмены, возвраты, рейтинг) | блок в карточке продавца |
| UTM-аналитика | P3 | трекинг в shop | сбор + агрегаты | раздел аналитики |
| Excel-экспорты | P2 | RBAC (`exports.excel`) | генерация xlsx/csv по заказам/выплатам/остаткам | кнопки экспорта |
| Настройки комиссии | P2 | RBAC (`commission.manage`), модель комиссии | settings-таблица + применение в расчётах payout | блок в системных настройках |
| Реальный платёжный/refund/payout провайдер | P2 | договорённости с провайдером | интеграция (T-Bank уже застаблен) | статусы провайдера в UI |
| Чеки/документы (receipts) | P3 | реальный провайдер | хранение/генерация документов | вкладка документов в заказе |
| `GET /admin/users` (покупатели) | P2 | RBAC | список/блокировка покупателей | реальная страница «Пользователи» |

---

## 10. Фазы реализации

| Фаза | Scope | Deliverables | Зависимости | Размер |
|---|---|---|---|---|
| **Admin Phase A — UX cleanup** | Русификация всех страниц; объединить Категории+Бренды; скрыть «Пользователи» и фейковые блоки «Настроек»; drawer-деталка заказа (платёж + отгрузка + возврат в одном месте); пояснительные подсказки к платежам/выплатам/отгрузкам; создание отгрузки только из заказа; консолидация api-слоя (api-client vs apps/admin/src/api) | чистая, последовательная, полностью русская админка без мёртвых разделов | — | **M** |
| **Admin Phase B — RBAC backend foundation** | Миграции: `staff_roles`, `staff_role_permissions`, `staff_members`, `audit_logs`; seed ролей и owner-аккаунта; permissions в JWT или резолв по запросу; middleware `RequirePermission`; запись audit log в admin-операциях | работающий permission-механизм (пока все существующие admin — full-access роль) | — | **L** |
| **Admin Phase C — Staff management UI** | Страницы «Доступы и роли» (read-only матрица) и «Сотрудники» (список, создание доступа с генератором пароля, блокировка, смена роли); показ своих permissions в профиле | owner может завести команду | Phase B | **M** |
| **Admin Phase D — применение permissions** | Навесить `RequirePermission` на все endpoints из раздела 2; фронт скрывает разделы/кнопки по permissions из `/auth/me`; 403-обработка | роли реально ограничивают доступ | Phase B (C желательно) | **M** |
| **Admin Phase E — Seller warnings + коммуникация** | `seller_warnings`, messages-тред админ↔продавец; карточка продавца (профиль, владелец, товары, история статусов, warnings, сообщения); фикс mismatch CreateSellerRequest; действие «верифицировать» | полноценное управление продавцами | Phase B (audit), A (карточка) | **M** |
| **Admin Phase F — Dashboard aggregates** | Endpoint(ы) агрегатов из раздела 8; новый Dashboard с виджетами по permissions | быстрый и честный дашборд | Phase B/D (фильтрация по правам) | **M** |
| **Admin Phase G — Complaints / Support** | Схемы и API жалоб и тикетов; подача из apps/shop (жалобы) и shop/seller (тикеты); разделы «Жалобы» и «Поддержка» | работа модераторов и поддержки | Phase B/D | **L** |
| **Admin Phase H — Storefront management** | Конфиг главной (баннеры, подборки, эксклюзивы), публичный endpoint, рендер в apps/shop, раздел «Настройки витрины» | контент-менеджер управляет витриной | Phase B/D (`storefront.manage`) | **L** |

Рекомендуемый порядок: **A → B → C → D → E → F → G → H** (A можно делать параллельно с B).

---

## 11. Приоритеты

**P0 — исправить до любых новых бизнес-фич:**
- Mismatch создания продавца (обязательные `brandName`/`contactEmail` против бизнес-правила «бренд создаёт продавец»).
- Контракт `temporaryPassword` между фронтом и `CreateSellerResponse`.
- Двойной/рассинхронизированный API-слой (`packages/api-client/src/admin.ts` vs `apps/admin/src/api/*`).
- Фейковые данные в «Настройках» (захардкоженный admin@zamk.com) и мёртвые разделы — скрыть/починить.
- Dashboard, считающий метрики на клиенте из полных выборок.

**P1 — фундамент админки:**
- Phase A (UX/русификация), Phase B (RBAC schema + audit log), Phase C (сотрудники), Phase D (permissions на endpoints), Phase F (агрегаты дашборда), seller warnings (начало Phase E).

**P2 — важная операционка маркетплейса:**
- Карточка продавца + верификация + коммуникация (Phase E полностью), жалобы и поддержка (Phase G), `GET /admin/users`, Excel-экспорты, настройки комиссии, тех. состояние товара по ролям, edit/delete категорий и брендов, реальный платёжный провайдер.

**P3 — желательное позже:**
- Витрина (Phase H), эксклюзивы, аукционы, скоринг продавцов, UTM-аналитика, чеки/документы.

---

## 12. Риски и открытые вопросы

### Технические риски
- **Permissions в JWT vs резолв из БД:** в JWT — быстрее, но смена роли действует только после refresh токена; из БД — лишний запрос на каждый admin-запрос. Рекомендация: резолв из БД с коротким кэшем (Redis), т.к. admin-трафик мал, а мгновенный отзыв прав важен.
- **Audit log задним числом:** существующие операции не пишут единый actor-trail; придётся пройтись по всем admin-handler'ам — риск пропусков. Решение: записывать на уровне middleware/декоратора, а не вручную в каждом handler'е.
- **CHECK-констрейнт `users.role`** зашит в миграции 000001 — расширять enum не нужно (staff остаются `admin`), но любые соблазны добавить роли в users.role надо пресечь, иначе расползание модели.
- Контекстные ключи middleware — строковые литералы (`"userID"`, `"role"`), коллизионно-опасно; при добавлении permissions стоит перейти на типизированные ключи.
- Рассинхронизация api-client и локальных api-обёрток админки уже привела к двум разным сигнатурам `createAdminShipment` — при росте кодовой базы будет источником багов.
- OneDrive-путь проекта с кириллицей и пробелами периодически ломает тулинг (сборки, watcher'ы) — инфраструктурный риск.

### Бизнес-риски
- RBAC спроектирован «на вырост»: если штат реально 1–2 человека, Phase C/D можно сжать, но schema (Phase B) всё равно стоит заложить сразу — миграция прав позже обойдётся дороже.
- Выплаты «mark paid» без реального банковского провайдера — ручная операция с риском человеческой ошибки; до интеграции нужен обязательный комментарий + audit.
- Жалобы/поддержка без SLA-процессов могут превратиться в свалку — нужны статусы и владельцы тикетов с самого начала.

### Вопросы, требующие решения пользователя — РЕШЕНО в Phase A

| # | Вопрос | Решение |
|---|--------|---------|
| 1 | Статус нового продавца | **pending** — изменено в `backend/internal/sellers/service.go` |
| 2 | Кто первый owner | admin@zamk.local → owner в Phase B (RBAC) через seed-миграцию |
| 3 | Модель комиссии | Базовая **9%** (900 bps). Штрафная **18%** при 2 нарушениях на 1 месяц. Таксономия нарушений — TBD |
| 4 | Выплаты — кто approve | Будущее: только owner/co_owner/finance. Пока — любой admin. Phase D задача |
| 5 | Видимость техсостояния | Решится в Phase C вместе с ролями moderator/warehouse |
| 6 | Раздел «Заявки продавцов» | Достаточно фильтра status=pending в «Продавцах» |
| 7 | Языковая политика | Админка **полностью на русском**, включая бейджи статусов |
| 8 | Удаление сотрудников | **Блокировка** вместо физического удаления |

---

## 13. Phase A — статус выполнения (завершена)

**Phase A: UX-cleanup, Russian UI, commission copy, order detail, seller access flow**

### Выполнено
- [x] Навигация sidebar переработана: убраны Пользователи, Журнал действий, Настройки; Категории+Бренды объединены в «Категории и бренды» (`/catalog`)
- [x] Создана страница `AdminCatalog.tsx` с вкладками «Категории» / «Бренды»
- [x] `AdminSettings.tsx` — убраны захардкоженные данные, подключён реальный `AdminAuthContext`, добавлено сообщение о будущих фазах
- [x] `AdminSellers.tsx` — UI создания доступа продавца переработан (новые метки полей, описание ниже заголовка); статусы переведены на русский
- [x] Контракт временного пароля исправлен: фронтенд показывает локальный `password` (то что admin ввёл), а не `res.temporaryPassword`. Тип в `admin.ts` изменён на `temporaryPasswordReturned: boolean`
- [x] Backend: `StatusActive` → `StatusPending` для новых продавцов в `service.go`
- [x] Backend: комиссия по умолчанию `1500` bps → `900` bps (9%) в `config.go`; добавлен TODO для penalty engine (18%)
- [x] `AdminDashboard.tsx` — убраны заглушки, добавлено примечание об источнике данных
- [x] `AdminOrders.tsx` — переработан в slide-in panel (Tailwind fixed right panel + overlay); полный перевод на русский
- [x] `AdminShipments.tsx` — добавлен warning banner про временный ручной ввод orderId
- [x] `AdminPayouts.tsx` — добавлен текст о комиссии 9%/18%, исправлен confirm dialog
- [x] `AdminPayments.tsx` — добавлены русские метки статусов
- [x] `AdminInventory.tsx` — полный перевод на русский
- [x] `AdminReturns.tsx`, `AdminRefunds.tsx`, `AdminReviews.tsx`, `AdminCategories.tsx`, `AdminBrands.tsx` — полный перевод на русский
- [x] `adminOperations.ts` — добавлена документация API-слоёв и TODO для будущих исправлений

### Не вошло в Phase A (Phase B+)
- RBAC: роли staff, permissions matrix
- Audit log backend
- `GET /admin/users` backend endpoint
- Penalty engine (18% commission при нарушениях)
- Карточка продавца с детализацией

---

## 14. Комиссия ZAMK (решено в Phase A)

```
Базовая комиссия:  9% (900 bps)
Штрафная комиссия: 18% (1800 bps)
Условие штрафа:    2 нарушения продавца
Длительность:      1 месяц
Таксономия нарушений: TBD (Phase E)
Автоматизация расчёта: будущая задача (Phase E / violations engine)
```

### CRITICAL: Технический долг backend commission

`backend/internal/payouts/` использует конфигурационную константу `MARKETPLACE_COMMISSION_BPS`.
Исправлено в Phase A: дефолт изменён с `1500` (15%) на `900` (9%).

До запуска реальных выплат необходимо:
1. ✅ Изменить дефолтную константу на 900 (9%) — **DONE**
2. Реализовать penalty engine (18% при 2 нарушениях) — Phase E
3. Покрыть тестами payout calculation — Phase E
