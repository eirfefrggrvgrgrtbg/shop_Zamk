# Public Seller Storefront Flow (STOREFRONT-1)

## Описание
Данный документ описывает публичную витрину продавца (магазин дизайнера/бренда) по маршруту `/seller/:slug` со стороны покупателя.

## Архитектура
1. **Frontend**:
   - `apps/shop/src/pages/SellerDetail.tsx`: Страница витрины продавца.
   - `fetchPublicSeller`: Вызов API (`GET /api/public/sellers/{idOrSlug}`) с параметрами фильтрации и пагинации.
   - Фильтры на странице магазина аналогичны `/catalog`.
   - Навигация к магазину осуществляется через ссылки вида `/seller/{slug}` в `ProductCard` и `ProductDetail`.

2. **Backend API**:
   - `GET /api/public/sellers/{idOrSlug}`
   - **Слой маршрутизации**: `productsHandler.GetPublicSellerStore`
   - **Бизнес-логика**:
     1. `sellersService.GetPublicSeller` — получение публичных данных продавца (с очисткой приватных полей).
     2. `productsService.ListPublicProducts` — фильтрация и пагинация опубликованных товаров привязанных к данному продавцу.

## Безопасность и Приватность (Data Leak Prevention)
Приватные данные, которые не должны утекать в публичном API:
- `Seller.ContactEmail` и `Seller.ContactPhone` — скрыты.
- `Seller.LogoObjectKey` — скрыт (инфраструктурный путь MinIO).
- `Seller.IsPlatform` — скрыто.
- `Product.MainImageObjectKey` и `ProductImage.ObjectKey` — инфраструктурные пути, скрываются.
- Внутренние поля модерации товаров (`SubmittedAt`, `ApprovedAt`, `RejectedAt`, `ModerationComment`) — скрыты.

## Функциональные требования
1. **Поиск продавца**:
   - Поддержка `slug` (человекочитаемый URL).
   - Если продавец не найден или `status != active`, возвращается 404.
2. **Отображение профиля**:
   - `BrandName`, `Description`, `LogoURL`.
3. **Фильтрация и пагинация товаров**:
   - Параметры: `q`, `categoryId`, `brandId`, `size`, `minPriceCents`, `maxPriceCents`, `inStock`, `sort`, `limit`, `offset`.
   - Идентично функционалу страницы `/catalog`.
   - Если товаров нет — отображается пустая витрина (заглушка), страница не падает.
4. **Связи**:
   - Товары в каталоге (`ProductCard`) и страница товара (`ProductDetail`) ссылаются на `/seller/{sellerSlug}`, а не на `sellerId`. Для этого API каталога отдаёт `sellerSlug`.
