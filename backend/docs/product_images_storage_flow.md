# Product Images Storage Flow (MEDIA-1)

## Текущее состояние (Аудит)

### 1. Storage Backend
- **Провайдер**: MinIO (S3-совместимый), реализован в `backend/internal/storage/s3.go`.
- **Buckets/Keys**: Используется бакет из конфига (`S3_BUCKET`).
- **Object Key Format**: `products/{seller_id}/{product_id}/{uuid}.{ext}`. Это безопасно против path traversal, так как UUID генерируется на сервере, а расширение валидируется.
- **Public URLs**: Формируются через `S3_PUBLIC_BASE_URL` или эндпоинт MinIO.

### 2. Endpoints
Product images are managed through dedicated image endpoints.
UpdateProduct does not replace product_images rows and does not drop object_key.
- `POST /api/seller/products/{id}/images/upload` — работает, валидирует данные, загружает в S3, затем в БД.
- `POST /api/admin/products/{id}/images/upload` — для админа.
- `PUT /api/seller/products/{id}/images/reorder` — для сортировки.
- `DELETE /api/seller/products/{id}/images/{imageId}` — для удаления конкретного фото.

### 3. Лимиты и Валидация
- **Allowed Formats**: `image/jpeg`, `image/png`, `image/webp`.
- **Extensions**: `.jpg`, `.jpeg`, `.png`, `.webp`.
- **Max File Size**: Берется из конфига `UPLOAD_MAX_SIZE_MB`.
- **Max Images per Product**: На данный момент не ограничено на уровне сервиса жестко.

### 4. Security & Visibility
- Загрузка разрешена только владельцу товара.
- Загрузка возможна только если товар в статусе, допускающем редактирование.
- Публичные эндпоинты (`/api/public/products`) не отдают черновики или отклоненные товары (уже реализовано в рамках каталога).
- Сами ссылки (Object URLs) в MinIO могут быть публичными, но подобрать URL невозможно из-за наличия UUID.

### 5. Frontend
- **Seller**: В `SellerProductEdit.tsx` есть загрузка, но она отправляет URL-ы в `UpdateProduct`, что перезаписывает БД. Удаление работает только локально до нажатия "Сохранить", что некорректно для S3.
- **Shop / Admin**: Галереи отображают `product.images`, но нужно проверить фоллбэки и empty states.

---

## Что считается завершением MEDIA-1

1. Исправлена проблема orphan-файлов при обновлении/удалении фото. Добавлен эндпоинт `DELETE` для удаления конкретного изображения.
2. Реализована функция пересортировки / выбора главного фото без потери `object_key`.
3. Добавлен лимит на максимальное количество фото (например, 8).
4. Проверен и почищен UI для админа (модерация фото) и публичного каталога.
5. Пройдены smoke тесты (добавление, удаление, лимиты, неверные файлы).
6. Документация и Git в чистом виде (без секретов и мусора).
