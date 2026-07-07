# Catalog Search & Filtering Flow (SEARCH-1)

## Overview

Public catalog discovery: покупатель открывает каталог → ищет по тексту → фильтрует → сортирует → открывает карточку товара.

**Все эндпоинты публичные. Авторизация не требуется.**

---

## Endpoints

### `GET /api/public/products`

Список опубликованных товаров с фильтрами, сортировкой и пагинацией.

**Visibility rules (hard-coded в SQL):**
- `products.status = 'published'`
- `sellers.status = 'active'`
- Черновики, на модерации, отклонённые, скрытые — **не возвращаются никогда**

**Query parameters:**

| Параметр | Тип | Описание |
|---|---|---|
| `q` | string | Текстовый поиск по title, description, brand name, category name, seller brand name (ILIKE) |
| `categoryId` | UUID | Фильтр по категории. Невалидный UUID → 400 |
| `brandId` | UUID | Фильтр по бренду. Невалидный UUID → 400 |
| `sellerId` | UUID | Фильтр по продавцу. Невалидный UUID → 400 |
| `size` | string | Фильтр по размеру варианта (`XS`, `S`, `M`, `L`, `XL`, `XXL`). Один размер |
| `minPriceCents` | int64 | Минимальная цена в копейках. Отрицательное → 400 |
| `maxPriceCents` | int64 | Максимальная цена в копейках. minPrice > maxPrice → 400 |
| `inStock` | `"true"` | Только товары с положительным остатком (inventory_items) |
| `sort` | string | `newest` (default), `price_asc`, `price_desc`. Неизвестное значение → fallback на `newest` |
| `limit` | int | Кол-во записей. Default: 50, Max: 100 |
| `offset` | int | Смещение. Default: 0 |

**Response 200:**
```json
{
  "items": [
    {
      "id": "uuid",
      "sellerId": "uuid",
      "categoryId": "uuid",
      "brandId": "uuid",
      "title": "Название",
      "slug": "nazvanie",
      "description": "...",
      "status": "published",
      "gender": "unisex",
      "color": "Чёрный",
      "material": "Хлопок",
      "priceCents": 500000,
      "oldPriceCents": null,
      "currency": "RUB",
      "mainImageUrl": "https://...",
      "publishedAt": "2024-01-01T00:00:00Z",
      "variants": [...],
      "images": [...],
      "inStock": true,
      "rating": { "average": 4.5, "count": 12 }
    }
  ],
  "totalCount": 42
}
```

**Security:**
- `moderationComment`, `rejectedAt`, `submittedAt`, `approvedAt` **не возвращаются** (stripped в service.go)
- UUID фильтры валидируются строго, ошибка → 400
- Нет SQL-инъекций: параметры передаются через `$N` placeholders

**Error responses:**
- `400 invalid_filter` — невалидный UUID, отрицательная цена, min > max
- `500 internal_error` — ошибка БД (логируется)

---

### `GET /api/public/products/:idOrSlug`

Карточка товара по UUID или slug.

**Response 200:** полная карточка товара (без moderation полей)  
**Response 404:** товар не найден или не опубликован

---

### `GET /api/public/categories`

Список всех категорий.

**Response 200:**
```json
{ "items": [{ "id": "uuid", "name": "Верхняя одежда", "slug": "verkh" }] }
```

---

### `GET /api/public/brands`

Список всех брендов.

**Response 200:**
```json
{ "items": [{ "id": "uuid", "name": "Бренд", "slug": "brand" }] }
```

---

## Database Indexes

Используемые индексы для public catalog queries:

| Индекс | Покрывает |
|---|---|
| `products_status_created_at_idx` | Base WHERE filter |
| `products_status_price_cents_idx` | Price sort + filter (WHERE status='published') |
| `products_price_cents_idx` | Price range filter |
| `products_seller_id_idx` | sellerId filter |
| `products_category_id_idx` | categoryId filter |
| `products_brand_id_idx` | brandId filter |
| `products_status_published_at_idx` | Default sort (newest) |
| `product_variants_product_id_idx` | size/inStock EXISTS subquery |
| `product_variants_size_idx` | size EXISTS subquery (partial: WHERE is_active=true) |

---

## Frontend Integration

**Catalog.tsx** — реактивный useEffect с deps на все фильтры:

```ts
useEffect(() => {
  loadProducts(); // сбрасывает offset=0, запрашивает API
}, [activeCategory, activeBrand, activeSize, onlyInStock, priceRange, sortBy, q]);
```

**Параметры в URL** автоматически синхронизируются:
- `categoryId`, `brandId`, `size`, `inStock`, `minPriceCents`, `maxPriceCents`, `sort`

**Load more:** кнопка "Показать ещё" — append к существующим items.

**Фильтр "В наличии":** `onlyInStock` → `?inStock=true`

**Размер:** single-select radio-логика (один размер за раз) → `?size=M`

---

## Visibility Rules (Summary)

```
✅ published + seller.active  → возвращается
❌ draft                      → не возвращается
❌ pending_moderation         → не возвращается
❌ rejected                   → не возвращается
❌ hidden                     → не возвращается
❌ seller.inactive            → не возвращается
```

---

## Changelog

| Дата | Изменение |
|---|---|
| 2026-07-07 | SEARCH-1: добавлены `sellerId`, `size`, `inStock` фильтры; UUID 400 валидация; minPrice>maxPrice 400; strip moderation fields; DB indexes (000035); frontend reactive fetch + load more |
