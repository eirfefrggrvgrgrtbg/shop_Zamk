
### Completed Fixes (Resolved)
1. **[Backend] Seller Products Query 500 Error**: Added missing `source` column in `ListProductsBySeller` SQL query, preventing scan mismatch.
2. **[Frontend] Global API Client Normalization**: Updated all list endpoints across `packages/api-client/src/` (`public.ts`, `seller.ts`, `admin.ts`, `customer.ts`) to handle `{ items, totalCount }` structures seamlessly and always unwrap `res?.items || []`. This prevents React `.map()` null pointer exceptions and simultaneous `Ошибка` and `0 товаров` states, allowing graceful empty states to be displayed in Shop, Admin, and Seller panels.

