# Customer Account & Favorites Flow

## 1. Overview
The Customer Account flow serves as the central hub for shoppers on ZAMK. It provides functionality to:
- Edit personal profile details (Name, Phone).
- Manage shipping addresses.
- View and navigate the history of orders and returns.
- Maintain a wishlist/favorites list.
- Modify application themes and account security (password).

## 2. API Endpoints

### 2.1 Profile
- `GET /api/customer/profile` - Fetches current user profile.
- `PATCH /api/customer/profile` - Updates personal details.

### 2.2 Favorites
- `GET /api/customer/favorites` - Returns a JSON array of products the user has favorited.
- `POST /api/customer/favorites/{productId}` - Adds a product to favorites.
- `DELETE /api/customer/favorites/{productId}` - Removes a product from favorites.

### 2.3 Orders & Returns (Account specific)
- `GET /api/customer/orders` - Fetches the customer's order history.
- `GET /api/customer/returns` - Fetches the customer's return history.

### 2.4 Addresses
- `GET /api/customer/addresses` - Lists customer addresses.
- `POST /api/customer/addresses` - Creates a new address.
- `PATCH /api/customer/addresses/{id}` - Edits an address.
- `DELETE /api/customer/addresses/{id}` - Deletes an address.
- `POST /api/customer/addresses/{id}/default` - Marks an address as primary.

## 3. Frontend Architecture
The customer account operates under a set of client-side protected routes (React Router).

### `CustomerProtectedRoute`
All account pages (`Profile`, `Orders`, `Favorites`, `CustomerReviews`, `Settings`) are wrapped inside `<CustomerProtectedRoute>`. This component:
1. Validates the session.
2. Checks if the `user.role === 'customer'`.
3. Displays a stylized "Unauthorized/Login" placeholder if the user isn't logged in.

### `AccountNav`
A reusable navigation bar rendered inside each protected page to allow seamless switching between the customer dashboards:
- Профиль (`/account` / `/profile`)
- Заказы (`/orders`)
- Избранное (`/favorites`)
- Возвраты (`/orders` - Return states are embedded inside order details)
- Отзывы (`/reviews`)
- Настройки (`/settings`)

## 4. Testing & Verification

An automated script `scratch/test_customer_account_favorites.js` verifies the complete backend flow, ensuring that profile modifications and the idempotent addition/removal of favorites are handled correctly.

### Results
| flow | endpoint/page | role | expected | actual | status | files | notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Customer Profile | `GET /api/customer/profile` | customer | 200 with profile | 200 with profile | PASS | `users/handler.go` | Returns safe payload |
| Profile Update | `PATCH /api/customer/profile` | customer | 200 updated | 200 updated | PASS | `users/handler.go` | Idempotent |
| Profile Access | `GET /api/customer/profile` | guest | 401 Unauthorized | 401 Unauthorized | PASS | `auth/middleware` | Rejects missing token |
| Profile Access | `GET /api/customer/profile` | seller | 403 Forbidden | 403 Forbidden | PASS | `auth/middleware` | Rejects wrong role |
| Add Favorite | `POST /api/customer/favorites/{id}` | customer | 201 Created | 201 Created | PASS | `favorites/handler.go` | Adds to DB |
| Repeat Add Fav | `POST /api/customer/favorites/{id}` | customer | 200/201 (safe) | 201 | PASS | `favorites/handler.go` | Idempotent insertion |
| Remove Fav | `DELETE /api/customer/favorites/{id}`| customer | 200 OK | 200 OK | PASS | `favorites/handler.go` | Deletes from DB |
| Repeat Remove Fav | `DELETE /api/customer/favorites/{id}`| customer | 200 OK | 200 OK | PASS | `favorites/handler.go` | Idempotent deletion |
| List Favorites | `GET /api/customer/favorites` | customer | 200 array of products | 200 array of products | PASS | `favorites/handler.go` | |
| Fav UI Fetch | `Favorites.tsx` | customer | correct state | correct state | PASS | `api-client/src/customer.ts` | Fixed raw array parsing bug |
| Settings Guard | `Settings.tsx` | guest | redirect to login | redirect to login | PASS | `Settings.tsx` | Wrapped in `CustomerProtectedRoute` |
| Account Nav | `Settings.tsx`, etc | customer | visible | visible | PASS | `Settings.tsx` | Integrated `<AccountNav />` |
| Logout Security | `POST /auth/logout` | customer | 200 OK | 200 OK | PASS | `auth/handler.go` | Drops refresh token; stateless JWT access drops on client-side |

### Known Gaps
- None currently blocking CUSTOMER-1. Layout and backend have been unified and solidified.
