# 🔍 АУДИТ ПРОЕКТА SHOP_ZAMK

**Дата:** 6 апреля 2026  
**Версия фронтенда:** React 19.2.4 + Vite 8.0.1  
**Статус:** MVP с mock-данными, готов к интеграции с бэкендом

---

## 📊 ОБЩАЯ ОЦЕНКА ФРОНТЕНДА

### ✅ Сильные стороны

**1. Архитектура и структура (8/10)**
- Чистая модульная структура с разделением на contexts, components, pages
- 57 TypeScript файлов, хорошо организованных
- Использование Context API для глобального состояния (Auth, Cart, Favorites, Theme, Search, Toast)
- Типизация через TypeScript с интерфейсами для всех сущностей

**2. UI/UX компоненты (9/10)**
- Собственная дизайн-система с компонентами: Button, Input, Modal, Drawer, Badge, Skeleton
- Продвинутые 3D-эффекты через Three.js (@react-three/fiber, @react-three/drei)
- Анимации через Framer Motion
- Адаптивный дизайн с Tailwind CSS 4.2.2
- Темная/светлая тема с переключателем

**3. Функциональность (7/10)**
- ✅ Авторизация (mock OTP через localStorage)
- ✅ Каталог с фильтрацией
- ✅ Корзина (in-memory, теряется при перезагрузке)
- ✅ Избранное
- ✅ Профиль пользователя
- ✅ Страницы брендов, продавцов, коллекций
- ✅ Поиск
- ✅ Чекаут (mock, без реальной оплаты)

**4. Производительность (6/10)**
- Использование React 19 (последняя версия)
- Lazy loading отсутствует для роутов
- Нет оптимизации изображений (используется wsrv.nl CDN)
- Нет кэширования API-запросов (пока mock-данные)

---

## ⚠️ КРИТИЧЕСКИЕ ПРОБЛЕМЫ

### 🔴 1. Отсутствие персистентности данных
**Проблема:** Корзина и избранное хранятся только в памяти React state  
**Последствия:** При перезагрузке страницы пользователь теряет корзину  
**Решение:** Сохранять в localStorage или сразу интегрировать с Redis через API

```typescript
// src/contexts/CartContext.tsx:24
const [items, setItems] = useState<CartItem[]>([]); // ❌ Нет персистентности
```

### 🔴 2. Mock-авторизация небезопасна
**Проблема:** Пароли не хешируются, JWT не используется  
**Код:**
```typescript
// src/contexts/AuthContext.tsx:54
if (email.includes('@') && pass.length >= 6) { // ❌ Фейковая проверка
  localStorage.setItem('zamk_mock_user', JSON.stringify(userObj)); // ❌ Небезопасно
}
```
**Решение:** Интегрировать с реальным Auth Service (OTP + JWT RS256)

### 🔴 3. Нет валидации инвентаря
**Проблема:** Пользователь может добавить в корзину товар, которого нет на складе  
**Решение:** При добавлении в корзину проверять `GET /inventory/:variantId`

### 🔴 4. Чекаут не создает реальный заказ
**Проблема:** Кнопка "Оформить заказ" просто очищает корзину  
```typescript
// src/pages/Checkout.tsx:115
<Button onClick={() => { setDone(true); clearCart(); }}> // ❌ Нет API-запроса
```
**Решение:** Вызывать `POST /orders` → `POST /payments/create` → редирект на ЮКасса

### 🟡 5. Отсутствие обработки ошибок
- Нет ErrorBoundary для отлова React-ошибок
- Нет retry-логики для API-запросов
- Нет fallback UI при сбое загрузки данных

### 🟡 6. SEO и метатеги
- Нет React Helmet для динамических meta-тегов
- Отсутствует sitemap.xml и robots.txt
- Нет Open Graph тегов для соцсетей

---

## 🛠 ТЕХНИЧЕСКИЙ ДОЛГ

| Проблема | Приоритет | Усилия |
|----------|-----------|--------|
| Нет unit/integration тестов | Высокий | 3-5 дней |
| Отсутствует CI/CD pipeline | Высокий | 1 день |
| Нет мониторинга ошибок (Sentry) | Средний | 2 часа |
| Lazy loading роутов | Средний | 4 часа |
| Оптимизация бандла (code splitting) | Средний | 1 день |
| Accessibility (ARIA, keyboard nav) | Низкий | 2 дня |

---

## 📦 ЗАВИСИМОСТИ

### Основные (production)
```json
"react": "19.2.4",           // ✅ Последняя версия
"react-router-dom": "7.13.1", // ✅ Актуально
"framer-motion": "12.38.0",   // ✅ Для анимаций
"three": "0.183.2",           // ⚠️ Тяжелая библиотека (600KB), использовать осторожно
"tailwindcss": "4.2.2"        // ✅ Последняя версия
```

### Рекомендации
- Добавить `react-query` или `swr` для кэширования API
- Добавить `zod` для валидации форм
- Добавить `@sentry/react` для мониторинга ошибок

---

## 🎯 ОЦЕНКА ПО КАТЕГОРИЯМ

| Категория | Оценка | Комментарий |
|-----------|--------|-------------|
| **Архитектура** | 8/10 | Чистая структура, но нужны тесты |
| **UI/UX** | 9/10 | Отличный дизайн, 3D-эффекты, темная тема |
| **Безопасность** | 3/10 | Mock-авторизация, нет защиты от XSS/CSRF |
| **Производительность** | 6/10 | Нет lazy loading, Three.js утяжеляет бандл |
| **Готовность к продакшену** | 4/10 | Нужна интеграция с бэкендом |

**Общая оценка: 6/10** — Хороший MVP, но требует доработки для продакшена

---

# 🚀 БЭКЕНД НА GO — ЧТО И КАК ДОЛЖНО БЫТЬ

## 📐 Архитектура (из BACKEND_ARCHITECTURE.md)

Вы уже создали отличный документ `BACKEND_ARCHITECTURE.md` с детальным планом. Вот краткая выжимка:

### ✅ Правильный выбор технологий

**Язык:** Go 1.22+  
**Почему:** Высокая производительность, горутины для конкурентности, низкое потребление памяти

**База данных:** PostgreSQL 16  
**Почему:** ACID-транзакции, JSON-поля для вариантов товаров, надежность

**Кэш:** Redis 7  
**Почему:** In-memory корзины, сессии, TTL-резервации товаров

**ORM:** sqlc (кодогенерация)  
**Почему:** Типобезопасные SQL-запросы без рефлексии, быстрее GORM

**HTTP Router:** chi или Fiber  
**Почему:** Легкий, без магии фреймворков, middleware-friendly

---

## 🏗 СТРУКТУРА ПРОЕКТА (Рекомендация)

```
zamk-backend/
├── cmd/
│   └── server/
│       └── main.go              # Точка входа
├── internal/
│   ├── auth/                    # Сервис авторизации (OTP + JWT)
│   │   ├── handler.go           # HTTP handlers
│   │   ├── service.go           # Бизнес-логика
│   │   ├── repository.go        # Работа с БД
│   │   └── models.go            # Структуры данных
│   ├── catalog/                 # Каталог товаров
│   ├── cart/                    # Корзина (Redis)
│   ├── orders/                  # Заказы (State Machine)
│   ├── payment/                 # Интеграция с ЮКасса
│   ├── shipping/                # СДЭК API
│   ├── warehouse/               # Инвентарь + резервация
│   └── notifications/           # SMS/Email
├── pkg/
│   ├── database/                # PostgreSQL клиент
│   ├── redis/                   # Redis клиент
│   ├── middleware/              # JWT, CORS, logging
│   ├── errors/                  # Кастомные ошибки
│   └── validator/               # Валидация запросов
├── migrations/                  # SQL миграции (goose)
├── api/
│   └── openapi.yaml             # OpenAPI спецификация
├── docker-compose.yml
├── Makefile
└── .env.example
```

---

## 🔐 1. AUTH SERVICE — Авторизация

### Эндпоинты
```
POST /api/v1/auth/request-otp    # Запросить OTP код
POST /api/v1/auth/verify-otp     # Подтвердить код, получить JWT
POST /api/v1/auth/refresh        # Обновить access token
DELETE /api/v1/auth/logout       # Инвалидировать refresh token
GET /api/v1/profile              # Получить профиль (JWT required)
PATCH /api/v1/profile            # Обновить профиль
```

### Логика OTP (Passwordless)
```go
// 1. Генерация кода
code := generateOTP(6) // "391847"
redis.Set(ctx, "otp:+79001234567", code, 5*time.Minute)
sendSMS(phone, code) // SMS.ru API

// 2. Верификация
storedCode := redis.Get(ctx, "otp:"+phone)
if storedCode != inputCode {
    return errors.New("invalid OTP")
}
redis.Del(ctx, "otp:"+phone)

// 3. Выдача JWT
accessToken := generateJWT(userID, 15*time.Minute)   // RS256
refreshToken := generateJWT(userID, 30*24*time.Hour)
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
```

---

## 📦 2. CATALOG SERVICE — Каталог

### Эндпоинты
```
GET /api/v1/products              # Список товаров + фильтры
GET /api/v1/products/:id          # Карточка товара
GET /api/v1/brands                # Список брендов
GET /api/v1/brands/:id            # Страница бренда
GET /api/v1/search?q=анорак       # Полнотекстовый поиск
```

### Кэширование (Redis)
```go
// Кэш на 10 минут
cacheKey := "cache:products:all"
cached, err := redis.Get(ctx, cacheKey)
if err == nil {
    return json.Unmarshal(cached, &products)
}

// Если нет в кэше — запрос к PostgreSQL
products := db.GetAllProducts(ctx)
redis.Set(ctx, cacheKey, json.Marshal(products), 10*time.Minute)
```

### Таблицы
```sql
CREATE TABLE products (
    id            VARCHAR(100) PRIMARY KEY,
    brand_id      VARCHAR(100) REFERENCES brands(id),
    name          VARCHAR(500) NOT NULL,
    price         INTEGER NOT NULL,  -- в копейках!
    category      VARCHAR(100),
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMP DEFAULT now()
);

CREATE TABLE product_variants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  VARCHAR(100) REFERENCES products(id),
    size        VARCHAR(20),
    color_name  VARCHAR(100),
    sku         VARCHAR(100) UNIQUE
);
```

---

## 🛒 3. CART SERVICE — Корзина

### Хранение в Redis
```go
type CartItem struct {
    ProductID  string `json:"product_id"`
    VariantID  string `json:"variant_id"`
    Name       string `json:"name"`
    Price      int    `json:"price"`
    Quantity   int    `json:"quantity"`
}

// Сохранение
cartKey := "cart:" + userID
cartJSON, _ := json.Marshal(cartItems)
redis.Set(ctx, cartKey, cartJSON, 30*24*time.Hour) // 30 дней

// Чтение
cartJSON := redis.Get(ctx, cartKey)
json.Unmarshal(cartJSON, &cartItems)
```

### Эндпоинты
```
GET    /api/v1/cart              # Получить корзину
POST   /api/v1/cart/items        # Добавить товар
PATCH  /api/v1/cart/items/:id    # Изменить количество
DELETE /api/v1/cart/items/:id    # Удалить позицию
DELETE /api/v1/cart              # Очистить корзину
```

---

## 🏭 4. WAREHOUSE SERVICE — Инвентарь

### Критическая логика: резервация товара

**Проблема:** Два пользователя одновременно покупают последний размер  
**Решение:** `SELECT FOR UPDATE` (блокировка строки в транзакции)

```go
func ReserveInventory(ctx context.Context, variantID string, userID string) error {
    tx, _ := db.BeginTx(ctx, nil)
    defer tx.Rollback()

    // Блокируем строку
    var inv Inventory
    err := tx.QueryRow(`
        SELECT quantity_total, quantity_reserved 
        FROM inventory 
        WHERE variant_id = $1 
        FOR UPDATE
    `, variantID).Scan(&inv.Total, &inv.Reserved)

    available := inv.Total - inv.Reserved
    if available <= 0 {
        return errors.New("out of stock")
    }

    // Резервируем
    _, err = tx.Exec(`
        UPDATE inventory 
        SET quantity_reserved = quantity_reserved + 1 
        WHERE variant_id = $1
    `, variantID)

    // Сохраняем резервацию в Redis с TTL 15 минут
    redis.Set(ctx, "reservation:"+userID+":"+variantID, "1", 15*time.Minute)

    tx.Commit()
    return nil
}
```

### Таблица
```sql
CREATE TABLE inventory (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id        UUID REFERENCES product_variants(id) UNIQUE,
    quantity_total    INTEGER NOT NULL DEFAULT 0,
    quantity_reserved INTEGER NOT NULL DEFAULT 0,
    updated_at        TIMESTAMP DEFAULT now()
);
```

---

## 📋 5. ORDER SERVICE — Заказы

### State Machine (автомат состояний)
```
CREATED → AWAITING_PAYMENT → PROCESSING → SHIPPED → DELIVERED
                    ↓
              PAYMENT_FAILED
```

### Эндпоинты
```
POST /api/v1/orders           # Оформить заказ
GET  /api/v1/orders           # Мои заказы
GET  /api/v1/orders/:id       # Детали заказа
POST /api/v1/orders/:id/cancel # Отменить заказ
```

### Таблица
```sql
CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID REFERENCES users(id),
    status            VARCHAR(50) NOT NULL DEFAULT 'CREATED',
    total_price       INTEGER NOT NULL,
    delivery_address  JSONB NOT NULL,  -- снапшот адреса
    payment_id        VARCHAR(255),
    tracking_number   VARCHAR(255),
    created_at        TIMESTAMP DEFAULT now()
);
```

---

## 💳 6. PAYMENT SERVICE — Оплата

### Интеграция с ЮКасса

```go
// 1. Создание платежа
func CreatePayment(orderID string, amount int) (string, error) {
    resp, err := http.Post("https://api.yookassa.ru/v3/payments", 
        map[string]interface{}{
            "amount": map[string]interface{}{
                "value":    fmt.Sprintf("%.2f", float64(amount)/100),
                "currency": "RUB",
            },
            "confirmation": map[string]interface{}{
                "type":       "redirect",
                "return_url": "https://zamk.shop/orders/" + orderID,
            },
            "metadata": map[string]string{"order_id": orderID},
        })
    
    return resp.ConfirmationURL, nil
}

// 2. Webhook от ЮКасса
func HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
    // Проверка подписи HMAC
    if !verifySignature(r.Body, r.Header.Get("X-Signature")) {
        http.Error(w, "invalid signature", 403)
        return
    }

    var event PaymentEvent
    json.NewDecoder(r.Body).Decode(&event)

    if event.Status == "succeeded" {
        // Обновить статус заказа
        db.UpdateOrderStatus(event.Metadata.OrderID, "PROCESSING")
        // Списать товар со склада
        warehouse.CommitReservation(event.Metadata.OrderID)
        // Отправить уведомление
        notifications.SendOrderConfirmation(event.Metadata.OrderID)
    }
}
```

---

## 🚚 7. SHIPPING SERVICE — Доставка

### Интеграция с СДЭК API v2

```go
// Расчет стоимости доставки
func CalculateShipping(cityCode int, weight int) (int, error) {
    resp, _ := http.Post("https://api.cdek.ru/v2/calculator/tariff", 
        map[string]interface{}{
            "tariff_code": 136, // дверь-дверь
            "from_location": map[string]int{"code": 44}, // Москва
            "to_location": map[string]int{"code": cityCode},
            "packages": []map[string]int{{"weight": weight}},
        })
    
    return resp.TotalSum, nil
}

// Создание заказа в СДЭК
func CreateShipment(orderID string, address Address) (string, error) {
    resp, _ := http.Post("https://api.cdek.ru/v2/orders", 
        map[string]interface{}{
            "tariff_code": 136,
            "recipient": map[string]string{
                "name":  address.Name,
                "phone": address.Phone,
            },
            "to_location": map[string]string{
                "address": address.Street,
            },
        })
    
    return resp.TrackingNumber, nil
}
```

---

## 🔔 8. NOTIFICATIONS SERVICE — Уведомления

### Асинхронная отправка через Redis Streams

```go
// Публикация события
func PublishEvent(event string, data map[string]interface{}) {
    redis.XAdd(ctx, &redis.XAddArgs{
        Stream: "notifications",
        Values: map[string]interface{}{
            "event": event,
            "data":  json.Marshal(data),
        },
    })
}

// Воркер для обработки
func NotificationWorker() {
    for {
        msgs := redis.XRead(ctx, &redis.XReadArgs{
            Streams: []string{"notifications", "0"},
            Block:   0,
        })

        for _, msg := range msgs {
            event := msg.Values["event"]
            switch event {
            case "order_created":
                sendSMS(data.Phone, "Заказ принят")
                sendEmail(data.Email, "Подтверждение заказа")
            case "order_shipped":
                sendSMS(data.Phone, "Трек-номер: "+data.Tracking)
            }
        }
    }
}
```

---

## 🛠 ПРИОРИТЕТНЫЙ ПЛАН РАЗРАБОТКИ

### Фаза 1: Инфраструктура (1-2 дня)
- [ ] Поднять PostgreSQL + Redis в Docker Compose
- [ ] Написать SQL-миграции (goose)
- [ ] Настроить Makefile для запуска

### Фаза 2: Auth Service (2-3 дня)
- [ ] Реализовать OTP через SMS.ru
- [ ] JWT генерация (RS256)
- [ ] Middleware для проверки токенов

### Фаза 3: Catalog API (2 дня)
- [ ] CRUD для товаров
- [ ] Кэширование в Redis
- [ ] Полнотекстовый поиск (PostgreSQL FTS)

### Фаза 4: Cart + Warehouse (3 дня)
- [ ] Корзина в Redis
- [ ] Резервация товаров (SELECT FOR UPDATE)
- [ ] TTL-освобождение резерваций

### Фаза 5: Orders + Payment (4-5 дней)
- [ ] State Machine для заказов
- [ ] Интеграция с ЮКасса
- [ ] Webhook обработка

### Фаза 6: Shipping + Notifications (2-3 дня)
- [ ] СДЭК API
- [ ] SMS/Email через очереди

### Фаза 7: Admin API (2 дня)
- [ ] Backoffice для управления товарами
- [ ] Обновление остатков

### Фаза 8: Деплой (1-2 дня)
- [ ] Настроить nginx
- [ ] SSL-сертификаты (Let's Encrypt)
- [ ] Мониторинг (Prometheus + Grafana)

**Итого: 17-24 дня разработки**

---

## 🎯 КЛЮЧЕВЫЕ РЕКОМЕНДАЦИИ

### Для фронтенда
1. **Срочно:** Добавить персистентность корзины в localStorage
2. **Срочно:** Подготовить API-клиент (axios/fetch) для интеграции
3. Добавить обработку ошибок (ErrorBoundary)
4. Lazy loading роутов для оптимизации бандла
5. Добавить тесты (Vitest + React Testing Library)

### Для бэкенда
1. **Начать с Auth Service** — без логина ничего не работает
2. **Использовать sqlc вместо GORM** — быстрее и типобезопаснее
3. **Обязательно SELECT FOR UPDATE** для резервации товаров
4. **Webhook от ЮКасса — проверять HMAC подпись**
5. **Логирование через structured logging** (zerolog или zap)

### Безопасность
- JWT RS256 (не HS256!)
- Rate limiting на эндпоинты (10 req/sec на IP)
- CORS настроить только для вашего домена
- Валидация всех входных данных (validator.v10)
- SQL-инъекции защищены через sqlc (prepared statements)

---

## 📈 МЕТРИКИ УСПЕХА

После интеграции бэкенда отслеживать:
- **Latency:** P95 < 200ms для GET-запросов
- **Availability:** 99.9% uptime
- **Конверсия:** % пользователей, завершивших чекаут
- **Ошибки:** < 0.1% failed requests

---

**Вывод:** Фронтенд на 60% готов к продакшену, бэкенд нужно писать с нуля по архитектуре из BACKEND_ARCHITECTURE.md. Приоритет — Auth + Catalog + Cart + Orders + Payment.
