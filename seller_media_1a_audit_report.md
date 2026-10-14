# SELLER MEDIA.1A — Media Contract Hardening Audit Report

## Executive Summary
This audit inspects the database schema, migrations, backend services, repository transaction mechanics, API DTO contracts, and frontend Product Studio / Shop consumers to harden the media contract before implementing the two presentation modes (`GENERAL_GALLERY` and `COLORWAY_GALLERIES`).

The core finding is that **the database schema and partial unique indexes ALREADY enforce a single global `is_main` cover per product** and do NOT need any schema migration. However, the backend validation currently has **three critical gaps in final-state checking** (notably when variants change while images are omitted, and failure to filter soft-deleted variants), and the Seller Product Studio currently lacks explicit mode switching and legacy mixed resolution.

---

## 1. IS_MAIN Database Truth
- **Exact Schema & Migration**:
  - Column defined in migration `000062_add_product_media_crop.up.sql`:
    ```sql
    ALTER TABLE product_images ADD COLUMN is_main BOOLEAN NOT NULL DEFAULT false;

    CREATE UNIQUE INDEX product_images_single_main_idx
        ON product_images (product_id)
        WHERE is_main = true;
    ```
  - There is NO `is_primary` column.
- **Can one Product have multiple `is_main = true`?**: **NO.** The database engine strictly enforces at most one `is_main = true` row per `product_id`.
- **Constraint / Index**: Partial unique index `product_images_single_main_idx` on `product_images (product_id) WHERE is_main = true`.
- **Scoped by Product**: **YES** (`product_id`).
- **Scoped by Color**: **NO.** `color_id` is NOT part of the unique index. Setting `is_main = true` on two different colors of the same product triggers PostgreSQL error `23505` (`product_images_single_main_idx`).
- **Who updates `products.main_image_url`?**:
  1. `ReconcileProductImagesTx` (`repository_media_reconcile.go` lines 366–406) — called by `UpdateProductForSeller` whenever `req.Images != nil`. It reads the single image where `is_main = true` and updates `products.main_image_url` and `products.main_image_object_key` (or sets them to `NULL` if images are cleared).
  2. Legacy direct storage methods in `internal/storage/service.go` (`UploadSellerProductImage`, `DeleteSellerProductImage`, `ReorderSellerProductImages`, `SetMainProductImage`) via `productsRepo.SetMainImage`.
  3. `CreateProductForSeller` / `UpdateProductForSeller` if `req.MainImageURL` is explicitly provided.
- **Is `products.main_image_url` canonical or legacy/cache?**:
  - It is a **denormalized read-projection (cache) on the `products` table**, derived from the single canonical row in `product_images` where `is_main = true`.
  - It is actively consumed by performance-critical catalog list queries (such as `ListPublishedProducts` in Shop catalog, which does not join `product_images`), Admin list filters (`noMainImage`), Favorites, Personalization, and Cart/Orders fallbacks.

---

## 2. Current Main Image Business Semantics
- **Trace**:
  1. **Seller Product Studio**: In `ProductStudioFormWorkspace.tsx`, adding/reordering media sets `isMain: index === 0`.
  2. **PATCH DTO**: `SellerProductPatchImageItem` sends `{ id, isMain, colorId, altText }`.
  3. **Backend Service & Repository**:
     - `ReconcileProductImagesTx` enforces: if `len(desired) > 0`, `mainCount` MUST be exactly 1 (`ErrInvalidMediaSet: "exactly one main image required, got %d"`).
     - It resets `is_main = false` on all existing images of the product, then sets `is_main = true` on the designated image.
  4. **Shop ProductDetail (`/products/:slug`)**:
     - `publicCatalog.ts` maps `product.image = p.mainImageUrl` and `product.images = p.images...`.
     - In `mediaFocus.ts`, `getDeterministicDefaultColorId(images, canonicalMainImageUrl)` uses `p.mainImageUrl` to determine which colorway is shown by default when a customer visits PDP without a `?color=` parameter.
  5. **Shop Catalog Card / Listing**: `ListPublishedProducts` does not load all product images; it reads `p.main_image_url` directly as the card cover.
  6. **Cart & Order Items**: `cart/repository.go` and `orders/service.go` pick the first image for the variant's `color_id`, falling back to `p.main_image_url`.
- **Verdict**:
  - **A. Одна глобальная обложка Product (Single Global Product Cover).**
  - Database schema, repository reconciliation, catalog cards, and PDP default colorway resolution all treat `isMain` as the single product-level cover.

---

## 3. Do We Need Per-Color Main?
- **Answer**: **NO. Per-color `isMain` is NOT needed and should NOT be introduced.**
- **Proof of sufficiency of the Minimal Model (1 global `isMain` + canonical media ordering)**:
  1. **Shop PDP (`COLORWAY_GALLERIES`)**: When a customer clicks a color swatch, `getVisibleGalleryImages` filters `images.filter(img => img.colorId === selectedColorId)` and sets the gallery to the first photo in that list (`sort_order` lowest for that color). The first image of that color is automatically its primary hero image!
  2. **Default Colorway on PDP**: `getDeterministicDefaultColorId` checks the `colorId` of the global `isMain` image. If the global `isMain` image belongs to a colorway, PDP automatically defaults to that colorway.
  3. **Cart & Orders**: Already query `product_images` matching `color_id` ordered by `sort_order ASC LIMIT 1`.
  4. **Future Catalog Swatches**: Swatch previews for color `C` are simply `images.find(img => img.colorId === C)` (first image of color `C` in `sort_order`).
  5. **Zero DB Migration**: Retaining global `isMain` keeps the existing partial unique index `product_images_single_main_idx` completely intact.

---

## 4. PATCH Semantics
- **Endpoint**: `PATCH /api/v1/seller/products/{id}` (`UpdateProductForSeller` -> `ReconcileProductImagesTx`).
- **Update Semantics**: **Conditional Full Replacement of the Image Set (Declarative Final-Set Reconcile when `"images"` field is non-nil; Completely untouched when `"images"` field is omitted/`nil`)**:
  - **Omitted existing image**: When `req.Images` is sent, any existing image not included in `req.Images` is **permanently deleted** from `product_images`, and cleanup jobs for its storage keys are enqueued.
  - **New image**: Must be pre-staged via `POST /api/v1/seller/products/{id}/media/stage`. Sending its `stagedMediaId` in `req.Images` promotes it to `product_images` and marks staging as `consumed`.
  - **Color reassignment**: Existing images can freely change `colorId`.
  - **Main image change**: Existing images can freely change `isMain`.
  - **Deletion**: Expressed by omitting the image from `req.Images` (or sending `images: []` to clear all).
  - **Validation timing**: Validation occurs inside the transaction before any database mutations are executed.

---

## 5. Final-State Validation
- **Existing Gaps Identified**:
  1. **Gap 1**: If `req.Images == nil` (omitted) but `req.Variants != nil` (variants changed, e.g. color removed), media validation does NOT run, leaving orphan color images in DB.
  2. **Gap 2**: In `service.go` lines 857–866, `activeVariants` are fetched via `txRepo.GetProductVariants(ctx, p.ID)`, but `GetProductVariants` returns all rows (including soft-deleted `is_active = false`). The loop does not check `if !av.IsActive`, so removed colors are treated as valid!
  3. **Gap 3**: Published product revision flow (`revision != nil`) bypasses `ReconcileProductImagesTx`.
- **Target Implementation Point**:
  - In `UpdateProductForSeller` (inside the transaction, immediately after `MergeProductVariants`):
    1. Resolve **`finalActiveVariants`**: active variants strictly where `is_active = true`.
    2. Resolve **`finalImages`**: if `req.Images != nil`, use `desiredMedia`; if `req.Images == nil`, query current `product_images` from DB.
    3. Run a unified validator `validateFinalProductMediaState(finalActiveVariants, finalImages)`.

---

## 6. Target Media Invariant
- **GENERAL_GALLERY**:
  - All `images.color_id IS NULL`.
- **COLORWAY_GALLERIES**:
  - All `images.color_id IS NOT NULL`.
  - Every `image.color_id` must match an active variant's `color_id`.
- **HYBRID**:
  - `some color_id IS NULL AND some color_id IS NOT NULL` -> **REJECT** (`invalid_media_mode`).
- **Products without color dimension**:
  - If product has no active color variants (e.g. `SIZE_ONLY` or `SINGLE_VARIANT`), `images.color_id` MUST be `NULL` (`GENERAL_GALLERY`). Any color assignment is rejected (`invalid_image_color`).
- **DB Migration**: **NO MIGRATION NEEDED.** The current schema natively supports this invariant.

---

## 7. Image Count Rules
- **Draft Save (`PATCH`)**:
  - Total images: `0..8`.
  - `ReconcileProductImagesTx` allows drafts to be saved with 0, 1, or 2 images.
  - In `COLORWAY_GALLERIES` mode, if photos exist for a color, they must be valid, but drafts can be saved while authoring is in progress.
- **Readiness / Moderation Submit (`SubmitProductToModeration`)**:
  - `GENERAL_GALLERY`:
    - Total images >= 3.
  - `COLORWAY_GALLERIES`:
    - Total images >= 3.
    - Every active product colorway has >= 1 image (`missing_color_images`).
  - 3 images per colorway is a **recommendation**, NOT a blocking hard validation for now.

---

## 8. One-Color Product Storage: OPTION A vs OPTION B
- **Analysis**:
  - **Option A (1 color -> GENERAL)**:
    - If a 1-color product is stored as `GENERAL` (`color_id = null`), adding a 2nd color later creates a hybrid state or requires guessing which generic photos belong to the first color.
  - **Option B (Has color dimension -> COLOR_SPECIFIC)**:
    - If a product has a color dimension, its images are explicitly tagged with that `color_id` from the beginning.
    - When a 2nd color is added, the 1st color already has its photos. Zero guesswork, zero unassigned limbo.
    - `GENERAL_GALLERY` is strictly reserved for products *without* a color dimension.
- **Recommendation**: **OPTION B.**
  - If a product has color variants, even 1 color must be stored as `COLOR_SPECIFIC` (`color_id != null`).

---

## 9. Legacy Mixed Products
- **Detection**:
  - On opening Product Studio: detect if `images.some(i => i.colorId == null) && images.some(i => i.colorId != null)`.
- **UX Behavior**:
  - Mark status as `LEGACY_MIXED`.
  - Display non-dismissible warning: *"Товар содержит смешанные фотографии. Перед сохранением необходимо выбрать режим галереи."*
  - Ordinary save button is disabled until resolved.
- **Resolution Options**:
  - **Option 1**: *"Перевести в общую галерею"* (clears `colorId` on all photos after confirmation).
  - **Option 2**: *"Разложить по цветам"* (moves generic photos to an Unassigned tray for explicit assignment or deletion).

---

## 10. GENERAL -> COLORWAY Transition
- When Seller toggles from General to Colorway:
  - Existing generic photos (`color_id == null`) are moved to a temporary client-side **Unassigned Tray**.
  - They are NOT silently assigned to the first color.
  - Seller must assign each photo to a colorway or delete it.
  - Save is blocked while unassigned photos remain.
  - Temporary unassigned state exists purely in client draft state; no invalid state is persisted to DB.

---

## 11. COLORWAY -> GENERAL Transition
- When Seller toggles from Colorway to General:
  - Prompts explicit confirmation: *"Все фотографии будут объединены в общую галерею товара, а привязка к цветам снята. Продолжить?"*
  - On confirmation: `colorId` is set to `null` on all images.
  - Order is preserved deterministically.
  - Global `isMain` is preserved.

---

## 12. Color Removal Edge Case
- **Scenario**: Product has Black and Yellow images. Seller deletes Yellow from variants.
- **Target Rule**:
  - **Frontend**: Warns Seller: *"У цвета «Желтый» есть 2 фото. Удалите их или перенесите в другой цвет."* If Seller proceeds, Yellow photos are moved to Unassigned or deleted. Save is blocked if unassigned photos remain.
  - **Backend**: Final-state validation strictly rejects any save where an image has a `color_id` that does not match an active variant (`ErrInvalidImageColor`).

---

## 13. Color Addition Edge Case
- **Scenario**: Product in `COLORWAY` mode has Black images. Seller adds Yellow.
- **Target Rule**:
  - Yellow tab appears with a warning badge *"0 фото"*.
  - For **Draft Save**: allowed (Seller can save draft while preparing Yellow photos).
  - For **Moderation Submission**: blocked (`missing_color_images: color Yellow has 0 images`).

---

## 14. Main Image After Mode Switch
- Exactly ONE global `isMain` image across the entire product at all times.
- `GENERAL -> COLORWAY`: Current `isMain` photo remains `isMain`. When assigned to a color, that color becomes the default colorway on Shop PDP.
- `COLORWAY -> GENERAL`: Current `isMain` photo remains `isMain`.
- If the current `isMain` photo is deleted: the first remaining image (index 0) becomes `isMain`.

---

## 15. Backend Error Contract
- Format: Standard JSON error with HTTP 400 (or 422).
- Machine Codes:
  - `invalid_media_mode`: Hybrid media detected (`some color_id IS NULL && some NOT NULL`).
  - `unassigned_product_image`: Image with `color_id IS NULL` in `COLORWAY` mode.
  - `missing_color_images`: Colorway has 0 images on moderation submission.
  - `invalid_image_color`: Image `color_id` does not match any active variant color.
  - `invalid_media_set`: Image count > 8, duplicate IDs, or `mainCount != 1`.

---

## 16. Final Decision
- **A. Can DB support target without migration?**: **YES.**
- **B. `isMain` canonical semantics**: **GLOBAL** (single product cover).
- **C. One-color product storage**: **COLOR_SPECIFIC** (if product has color dimension).
- **D. Minimum image rule**: Draft save: 0..8. Moderation submit: total >= 3, and in Colorway mode every active color >= 1.
- **E. Legacy mixed edit behavior**: Detected on load, blocks save, Seller must explicitly convert to General or assign to colors.
- **F. Backend validation must inspect**: **FINAL STATE** (`finalActiveVariants` + `finalImages`).
- **G. Color removal behavior**: Orphan photos blocked on backend; frontend requires explicit reassign or delete.
- **H. Recommended implementation split**:
  - `SELLER MEDIA.2A`: Backend final-state invariant & validation.
  - `SELLER MEDIA.2B`: Frontend media mode state machine & transitions.
  - `SELLER MEDIA.2C`: Colorway tabs & photo upload UX.
  - `SELLER MEDIA.2D`: Legacy mixed detection & resolution UI.
  - `SELLER MEDIA.2E`: E2E verification (Seller Studio -> Moderation -> Shop PDP).
