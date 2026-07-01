# ZAMK — Backend Architecture (Go + Microservices)

> Выбранный язык: **Go (Golang)**
> Паттерн: **Modular Monolith** (v1) → **Microservices** (v2 при масштабировании)
> Дата: Апрель 2026

---

## 🗂 Структура репозитория

```
zamk-backend/
├── cmd/
│   └── server/
│       └── main.go              # Точка входа
├── internal/
│   ├── auth/                    # Сервис авторизации
│   ├── catalog/                 # Сервис каталога + инвентарь
│   ├── cart/                    # Сервис корзины
│   ├── orders/                  # Сервис заказов
│   ├── payment/                 # Платёжный шлюз
│   ├── shipping/                # Доставка + трекинг
│   ├── warehouse/               # Управление складами
│   └── notifications/           # Email / SMS уведомления
├── pkg/
│   ├── database/                # Подключение к PostgreSQL
│   ├── redis/                   # Клиент Redis
│   ├── middleware/              # Auth JWT, logging, CORS
│   └── errors/                  # Кастомные ошибки
├── migrations/                  # SQL миграции (goose)
├── docker-compose.yml
└── Makefile
```

---

## ⚙️ Технологический стек

| Слой | Технология | Обоснование |
|---|---|---|
| Язык | **Go 1.22+** | Высокая производительность, горутины, низкое потребление памяти |
| HTTP Router | **chi** или **Fiber** | Лёгкий, без магии фреймворков |
| База данных | **PostgreSQL 16** | Транзакции, ACID, JSON поля для вариантов товаров |
| ORM/Query | **sqlc** (кодогенерация) | Типобезопасные SQL-запросы без рефлексии |
| Миграции | **goose** | Простые SQL-миграции с rollback |
| Кэш + Корзина | **Redis 7** | In-memory хранение корзин, сессий, TTL-резерваций |
| Очереди | **Redis Streams** или **NATS** | Асинхронная отправка уведомлений, webhook обработка |
| Аутентификация | **JWT (RS256)** + OTP | Access/Refresh токены + SMS-код входа |
| Хранилище файлов | **S3 (MinIO / Яндекс)** | Фото товаров, документы |
| Деплой | **Docker + nginx** | VDS Linux, Caddy или nginx как reverse proxy |

---

## 🔐 1. AUTH SERVICE — Авторизация

### Логика входа (Passwordless OTP)
```
Клиент → POST /auth/request-otp { phone/email }
           ↓
        Генерация 6-значного кода
           ↓
        Сохранить в Redis: key="otp:+79001234567" value="391847" TTL=5min
           ↓
        Отправить SMS через SMS.ru / SMSAero API
           ↓
Клиент → POST /auth/verify-otp { phone, code }
           ↓
        Проверить Redis → если совпало → удалить ключ
           ↓
        Найти юзера в PostgreSQL по phone → если нет, создать
           ↓
        Выдать access_token (15min) + refresh_token (30 days)
```

### Таблица `users`
```sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone       VARCHAR(20) UNIQUE,
    email       VARCHAR(255) UNIQUE,
    name        VARCHAR(255),
    created_at  TIMESTAMP DEFAULT now()
);

CREATE TABLE user_addresses (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    label      VARCHAR(100),     -- "Дом", "Офис"
    city       VARCHAR(100),
    street     VARCHAR(255),
    apartment  VARCHAR(50),
    zip        VARCHAR(20),
    is_default BOOLEAN DEFAULT false
);
```

### Эндпоинты
```
POST /auth/request-otp      — запросить OTP код
POST /auth/verify-otp       — подтвердить, получить токены
POST /auth/refresh           — обновить access_token
DELETE /auth/logout          — инвалидировать refresh токен
GET  /profile               — получить профиль (JWT required)
PATCH /profile              — обновить имя
POST /profile/addresses      — добавить адрес
DELETE /profile/addresses/:id — удалить адрес
```

---

## 📦 2. CATALOG SERVICE — Каталог товаров

Самый читаемый модуль (99% трафика — чтение).

### Таблицы
```sql
CREATE TABLE brands (
    id          VARCHAR(100) PRIMARY KEY,  -- "narra-studio"
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    history     TEXT,
    philosophy  TEXT,
    country     VARCHAR(100),
    image_url   VARCHAR(500),
    is_active   BOOLEAN DEFAULT true
);

CREATE TABLE products (
    id            VARCHAR(100) PRIMARY KEY,  -- "p1"
    brand_id      VARCHAR(100) REFERENCES brands(id),
    seller_id     VARCHAR(100),
    name          VARCHAR(500) NOT NULL,
    description   TEXT,
    material      TEXT,
    category      VARCHAR(100),              -- "clothing", "bags"
    price         INTEGER NOT NULL,          -- Цена в копейках (во избежание float)
    old_price     INTEGER,
    is_new        BOOLEAN DEFAULT false,
    is_bestseller BOOLEAN DEFAULT false,
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMP DEFAULT now()
);

CREATE TABLE product_variants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  VARCHAR(100) REFERENCES products(id),
    size        VARCHAR(20),        -- "S", "M", "42"
    color_name  VARCHAR(100),
    color_hex   VARCHAR(7),
    sku         VARCHAR(100) UNIQUE -- Артикул конкретного варианта
);
```

### Кэширование каталога
- Список каталога кэшируется в Redis на **10 минут** при каждом запросе.
- При изменении товара через Admin API — инвалидировать соответствующий ключ.
- Ключи вида: `cache:catalog:all`, `cache:catalog:clothing`, `cache:product:p1`

### Эндпоинты
```
GET  /products               — список товаров (filters: category, brand, size, price)
GET  /products/:id           — карточка товара
GET  /brands                 — список брендов
GET  /brands/:id             — страница бренда + товары бренда
GET  /collections            — список коллекций
GET  /collections/:id        — коллекция с товарами
GET  /search?q=анорак         — полнотекстовый поиск (PostgreSQL FTS - tsvector)
```

---

## 🏭 3. WAREHOUSE SERVICE — Склад и Инвентаризация

Это критически важный модуль для магазина физических вещей.

### Таблица `inventory`
```sql
CREATE TABLE inventory (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id     UUID REFERENCES product_variants(id) UNIQUE,
    warehouse_id   UUID REFERENCES warehouses(id),
    quantity_total INTEGER NOT NULL DEFAULT 0,   -- Всего на складе
    quantity_reserved INTEGER NOT NULL DEFAULT 0, -- Зарезервировано
    updated_at     TIMESTAMP DEFAULT now()
);

-- quantity_available = quantity_total - quantity_reserved

CREATE TABLE warehouses (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name     VARCHAR(255),   -- "Склад Москва", "Склад СПБ"
    city     VARCHAR(100),
    address  TEXT
);
```

### Логика резервации (Race Condition Protection)
```
Покупатель выбрал размер → перешёл к чекауту
    ↓
LOCK row в inventory (SELECT ... FOR UPDATE)
    ↓
Если quantity_available > 0:
    quantity_reserved += 1
    Сохранить в Redis: "reservation:{userID}:{variantID}" TTL=15min
    ↓
Возвращаем "успешно — зарезервировано на 15 минут"
    ↓
Если платёж прошёл → quantity_total -= 1, quantity_reserved -= 1
Если платёж FAILED или TTL истёк → quantity_reserved -= 1 (освободить)
```

**Почему `SELECT FOR UPDATE`?** — Golang + PostgreSQL позволяет блокировать строку во время транзакции, чтобы два пользователя одновременно не купили последний размер.

### Эндпоинты
```
GET  /inventory/:variantId       — остатки по варианту (публичный)
POST /inventory/reserve          — зарезервировать (внутренний, вызывает Order)
POST /inventory/release          — освободить резервацию (внутренний)
POST /inventory/commit           — списать (вызывается при успешной оплате)
[ Admin ]
PUT  /admin/inventory/:id        — обновить остатки вручную
```

---

## 🛒 4. CART SERVICE — Корзина

Корзина целиком хранится в **Redis** для молниеносного отклика.

### Структура в Redis
```
Key:   "cart:{userID}"              (авторизованный)
Key:   "cart:guest:{sessionToken}"  (гость)
Value: JSON (список CartItem)
TTL:   7 дней (гость) / 30 дней (авт.)
```

```go
type CartItem struct {
    ProductID  string `json:"product_id"`
    VariantID  string `json:"variant_id"`
    Name       string `json:"name"`
    Brand      string `json:"brand"`
    Price      int    `json:"price"`   // в копейках
    Size       string `json:"size"`
    ImageURL   string `json:"image_url"`
    Quantity   int    `json:"quantity"`
}
```

### Слияние корзины при логине
```
Гость накапливает корзину → логинится через OTP
    ↓
На фронте: передать guestSessionToken при verify-otp
    ↓
Бэкенд: считать "cart:guest:{token}" → Merge с "cart:{userID}"
    ↓ (при конфликте одинаковых позиций — суммировать quantity)
Удалить гостевую корзину Redis
```

### Эндпоинты
```
GET    /cart            — получить корзину
POST   /cart/items      — добавить товар { variant_id, quantity }
PATCH  /cart/items/:id  — изменить количество
DELETE /cart/items/:id  — удалить позицию
DELETE /cart            — очистить корзину
```

---

## 📋 5. ORDER SERVICE — Заказы

### Автомат состояний (State Machine)
```
CREATED ──────────────────────────────────────────────────────────┐
    ↓  (пользователь перешёл к оплате)                             │
AWAITING_PAYMENT                                                    │
    ↓  (платёж прошёл)           ↘  (платёж не прошёл или TTL)    │
PROCESSING                        PAYMENT_FAILED ──────────────────┘
    ↓  (оператор подтвердил, передана курьеру)
SHIPPED
    ↓  (СДЭК/Boxberry доставил)
DELIVERED
    ├── (клиент доволен — конец)
    └── (подал на возврат)
         ↓
      RETURN_REQUESTED
         ↓
      RETURNED (деньги возвращены)
```

### Таблицы
```sql
CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID REFERENCES users(id),
    status            VARCHAR(50) NOT NULL DEFAULT 'CREATED',
    total_price       INTEGER NOT NULL,       -- итог в копейках
    delivery_price    INTEGER NOT NULL DEFAULT 0,
    promo_discount    INTEGER DEFAULT 0,
    delivery_address  JSONB NOT NULL,         -- снапшот адреса на момент заказа
    payment_method    VARCHAR(50),            -- "card", "split"
    payment_id        VARCHAR(255),           -- ID транзакции от эквайринга
    tracking_number   VARCHAR(255),           -- трек-номер СДЭК
    notes             TEXT,
    created_at        TIMESTAMP DEFAULT now(),
    updated_at        TIMESTAMP DEFAULT now()
);

CREATE TABLE order_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID REFERENCES orders(id),
    product_id  VARCHAR(100),
    variant_id  UUID,
    name        VARCHAR(500),      -- снапшот названия
    brand       VARCHAR(255),      -- снапшот бренда
    size        VARCHAR(20),
    price       INTEGER NOT NULL,  -- цена на момент заказа
    quantity    INTEGER NOT NULL DEFAULT 1
);
```

> **Важно:** `delivery_address`, `name`, `price` — это **снапшоты** данных на момент оформления заказа. Если бренд изменит цену завтра, история заказа клиента не сломается.

### Эндпоинты
```
POST /orders           — оформить заказ (validateCart → reserveInventory → createPayment)
GET  /orders           — мои заказы (JWT required)
GET  /orders/:id       — детали заказа
POST /orders/:id/cancel — отменить заказ (если ещё не SHIPPED)
POST /orders/:id/return — запросить возврат
[ Admin ]
GET  /admin/orders            — все заказы с пагинацией
PATCH /admin/orders/:id/status — сменить статус + прикрепить трек-номер
```

---

## 💳 6. PAYMENT SERVICE — Оплата

### Интеграция с ЮKassa (рекомендуется)

**Жизненный цикл платежа:**
```
1. POST /payments/create → запрос к ЮКасса API → получаем payment_url
2. Фронт редиректит пользователя на payment_url
3. Пользователь оплатил карту на стороне ЮKassa
4. ЮKassa вызывает наш Webhook: POST /payments/webhook
5. Бэкенд верифицирует подпись webhook → меняет статус заказа PAYMENT_SUCCESS
6. Запускаем: commit inventory + send confirmation email/SMS
```

### Webhook верификация (безопасность!)
```go
// Обязательно проверять HMAC подпись или IP whitelist от ЮКасса
func verifyWebhookSignature(body []byte, signature string, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### Поддержка рассрочки (Долями / Сплит)
- **Долями** (Яндекс) и **Т-Сплит** (Т-Банк) — тот же принцип: генерируем ссылку, пользователь идёт к ним, webhook возвращается к нам.
- Для Fashion-ритейла рассрочка увеличивает конверсию на **25–35%** на чеках от 10 000 ₽.

### Эндпоинты
```
POST /payments/create        — создать платёж по orderID
POST /payments/webhook       — принять webhook от эквайринга (PUBLIC без JWT!)
GET  /payments/:id/status    — статус платежа (для polling с фронта)
POST /payments/refund        — инициировать возврат (internal, вызывается Order)
```

---

## 🚚 7. SHIPPING SERVICE — Доставка и Трекинг

### Интеграция с СДЭК API v2

```
Оформление доставки (при смене статуса → PROCESSING):
    ↓
POST /v2/orders → СДЭК → получаем CDEK UUID + track_number
    ↓
Записываем track_number в orders.tracking_number
    ↓
Отправляем СМС / Email пользователю с трек-номером
```

### Расчёт стоимости доставки (до оформления заказа)
```go
// Запрос к СДЭК для расчёта стоимости по городу пользователя
POST /v2/calculator/tariff {
    "tariff_code": 136,          // дверь-дверь
    "from_location": { "code": 44 }, // Москва
    "to_location": { "code": cityCode },
    "packages": [{ "weight": 500 }]
}
// Ответ: { "period_min": 2, "period_max": 4, "total_sum": 590.00 }
```

### Таблица `shipments`
```sql
CREATE TABLE shipments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID REFERENCES orders(id),
    provider       VARCHAR(50),      -- "cdek", "boxberry", "russian_post"
    tracking_number VARCHAR(100),
    cdek_uuid      VARCHAR(255),     -- UUID заказа в системе СДЭК
    status         VARCHAR(100),     -- Последний трекинг-статус
    estimated_at   DATE,             -- Ожидаемая дата доставки
    delivered_at   TIMESTAMP,
    created_at     TIMESTAMP DEFAULT now()
);
```

### Webhook от СДЭК (обновление статуса доставки)
- СДЭК умеет слать Webhooks при изменении статуса посылки.
- При получении `DELIVERED` → автоматически перевести заказ в `DELIVERED` + уведомить клиента.

### Эндпоинты
```
GET  /shipping/calculate        — рассчитать стоимость по адресу
GET  /orders/:id/tracking       — статус доставки для клиента
[ Internal ]
POST /shipping/create-shipment  — создать заказ в СДЭК (вызывается Order Service)
POST /shipping/webhook          — принять обновление от СДЭК
```

---

## 🔔 8. NOTIFICATIONS SERVICE — Уведомления

Все уведомления отправляются **асинхронно через очередь** (Redis Streams или NATS), чтобы не блокировать основной поток.

### Триггеры
| Событие | Канал | Сообщение |
|---|---|---|
| Заказ создан | SMS + Email | "Ваш заказ #12345 принят" |
| Оплата прошла | SMS + Email | "Оплата подтверждена, собираем" |
| Заказ отправлен | SMS | "Трек-номер: {track}" |
| Заказ доставлен | Email | Запрос отзыва о товаре |
| Возврат обработан | Email | "Деньги возвращены в течение 3-5 дней" |

### Провайдеры
- **SMS:** SMS.ru или SMSAero (дешевые, надежные в РФ)
- **Email:** Unisender, Sendpulse или SMTP через Yandex 360

---

## 🛠 9. ADMIN API — Backoffice

Отдельная группа роутов с middleware `RequireRole("admin")`.

```
[ Товары ]
POST   /admin/products           — создать товар
PATCH  /admin/products/:id       — обновить товар
DELETE /admin/products/:id       — скрыть из витрины
POST   /admin/products/:id/images — загрузить фото в S3

[ Склад ]
GET    /admin/inventory          — все остатки
PUT    /admin/inventory/:sku     — обновить количество вручную

[ Заказы ]
GET    /admin/orders             — все заказы + фильтры
PATCH  /admin/orders/:id/status  — изменить статус + трек-номер
POST   /admin/orders/:id/refund  — инициировать возврат

[ Пользователи ]
GET    /admin/users              — список пользователей
GET    /admin/users/:id/orders   — заказы конкретного юзера
```

---

## 🗄 Схема всей БД (ERD)

```
users ────── user_addresses
  │
  ├── orders ──── order_items ──── products ──── product_variants ──── inventory
  │       │
  │   shipments                   brands ─────── products
  │
  └── (cart хранится в Redis, не в PostgreSQL)
```

---

## 🚀 Старт разработки (приоритетный порядок)

- [ ] **Шаг 1** — Поднять PostgreSQL + Redis в Docker, написать SQL-миграции
- [ ] **Шаг 2** — Auth Service (OTP + JWT) — без логина ничего не работает
- [ ] **Шаг 3** — Catalog API (GET endpoints) — заменить mock-data на реальный API
- [ ] **Шаг 4** — Cart Service (Redis) — корзина
- [ ] **Шаг 5** — Warehouse/Inventory (резервация)
- [ ] **Шаг 6** — Order Service (State machine)
- [ ] **Шаг 7** — Payment Integration (ЮKassa webhook)
- [ ] **Шаг 8** — Shipping (СДЭК calcualtor)
- [ ] **Шаг 9** — Notifications (SMS/Email)
- [ ] **Шаг 10** — Admin API + деплой на VDS
