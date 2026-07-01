# ZAMK Backend Technical Design Document

## 1. Backend Architecture

**Architecture Style:** Modular Monolith

**Structure:**
```
backend/
├── cmd/
│   ├── api/
│   └── worker/
├── internal/
│   ├── auth/
│   ├── users/
│   ├── sellers/
│   ├── catalog/
│   ├── products/
│   ├── inventory/
│   ├── cart/
│   ├── orders/
│   ├── payments/
│   ├── delivery/
│   ├── returns/
│   ├── reviews/
│   ├── payouts/
│   ├── admin/
│   ├── audit/
│   ├── notifications/
│   └── security/
├── migrations/
├── pkg/
└── go.mod
```

**Technology Choices:**
- **Go API Server:** Provides high performance, strong typing, and excellent concurrency. Using a modular monolith in Go keeps the deployment simple while enforcing logical domain boundaries internally. It scales easily vertically and horizontally.
- **One Worker Process:** Offloads heavy tasks (email sending, image processing, webhook processing) from the main API process to ensure fast API responses.
- **PostgreSQL:** Reliable relational database serving as the source of truth for all structured data, relationships, and ACID transactions.
- **Redis:** Used for ephemeral fast-access data (rate limits, session caching), short-lived locks, temporary stock reservations during checkout, and job queues for the worker process.
- **S3-compatible Storage:** Scalable, durable object storage tailored for unstructured assets like product images and user uploads.

## 2. Domain Model

- **User:** Represents an authenticated user (customer, admin, etc.). Key fields: `id`, `email`, `password_hash`, `role`. Owned by the system.
- **Seller:** A business entity/brand on the platform. Key fields: `id`, `brand_name`, `status`. Created and owned by the system (admins).
- **SellerUser:** Mapping between a User and a Seller, allowing multiple users to manage a single brand. Key fields: `user_id`, `seller_id`.
- **Product:** The core item listed for sale. Key fields: `id`, `seller_id`, `title`, `description`, `status`. Owned by a Seller. Must pass moderation to be published.
- **ProductImage:** Associated visual assets. Key fields: `id`, `product_id`, `s3_key`, `order`.
- **ProductVariant:** Specific options for a product (e.g., size, color). Key fields: `id`, `product_id`, `sku`, `price`.
- **ProductTemplate:** Reusable description structures for sellers. Key fields: `id`, `seller_id`, `content`.
- **Category:** Taxonomy for products. Key fields: `id`, `name`, `parent_id`. Managed by admins.
- **Brand:** Public brand details linked to a seller. Key fields: `id`, `name`, `slug`.
- **InventoryItem:** Represents physical stock in the warehouse. Key fields: `id`, `variant_id`, `seller_id`, `total_stock`, `reserved_stock`.
- **StockMovement:** Immutable log of any stock change (receipt, sale, adjustment). Key fields: `id`, `inventory_item_id`, `type`, `quantity`.
- **Reservation:** Temporary hold on stock during checkout. Key fields: `id`, `variant_id`, `quantity`, `expires_at`.
- **Cart & CartItem:** Ephemeral shopping cart state. Key fields: `id`, `user_id` / `id`, `cart_id`, `variant_id`, `quantity`.
- **Order & OrderItem:** Permanent record of a purchase. Key fields: `id`, `customer_id`, `status`, `total` / `order_id`, `variant_id`, `price`, `quantity`.
- **Payment & PaymentEvent:** Tracks financial transactions. Key fields: `id`, `order_id`, `provider`, `status`.
- **Delivery:** Logistics tracking for an order. Key fields: `id`, `order_id`, `courier`, `tracking_number`, `status`.
- **ReturnRequest:** Customer's request to return an item. Key fields: `id`, `order_id`, `reason`, `status`.
- **Refund:** Financial return to the customer. Key fields: `id`, `payment_id`, `amount`, `status`.
- **Review:** Customer feedback on a product. Key fields: `id`, `product_id`, `user_id`, `rating`.
- **Favorite:** Saved products for a customer. Key fields: `user_id`, `product_id`.
- **Payout & PayoutItem:** Transfer of funds to a seller. Key fields: `id`, `seller_id`, `amount`, `status`.
- **AuditLog:** System-level log of critical actions. Key fields: `id`, `actor_id`, `action`, `entity_type`, `entity_id`.
- **Notification:** System alerts for users/sellers. Key fields: `id`, `user_id`, `message`, `read`.

## 3. Roles and Access Control

**Roles:**
- `customer`: Can browse, buy, review, and return items. Can self-register.
- `seller`: Can manage their own brand's products, view orders, and request payouts. Cannot self-register (must be invited/created by admin).
- `admin`: Full system access (moderation, global orders, user management, payouts). Cannot self-register.

**Important Rules:**
- The backend is the ultimate enforcer of all role checks (frontend routing is strictly UX).
- A seller's access is strictly scoped to `seller_id`. They cannot view or mutate other sellers' data.
- Products created by sellers start as `draft` or `pending_moderation`. Only admins can transition them to `approved`.
- All mutating actions performed by an `admin` must trigger an entry in the `AuditLog`.

## 4. API Zones

- `/api/public/*`: No auth required. Endpoints for the public catalog, categories, brand pages, and public seller storefronts.
- `/api/auth/*`: Endpoints for login, registration, token refresh, and logout.
- `/api/customer/*`: Requires `customer` role. Endpoints for profiles, carts, orders, favorites, returns.
- `/api/seller/*`: Requires `seller` role. Endpoints for seller dashboard, managing products/templates, analytics, and payouts.
- `/api/admin/*`: Requires `admin` role. Endpoints for global management, moderation, user blocks, inventory adjustments, and audit logs.
- `/api/payments/*`: Webhook receivers for third-party payment providers. Requires signature verification.

## 5. Endpoint List

| Method | Path | Auth | Role | Body | Response | Notes |
|---|---|---|---|---|---|---|
| **Public** | | | | | | |
| GET | `/api/public/products` | No | - | - | `[]Product` | Filter, paginate |
| GET | `/api/public/products/:id` | No | - | - | `Product` | Includes variants |
| GET | `/api/public/categories` | No | - | - | `[]Category` | Tree structure |
| GET | `/api/public/brands` | No | - | - | `[]Brand` | |
| GET | `/api/public/sellers/:slug` | No | - | - | `SellerProfile` | Public info |
| GET | `/api/public/sellers/:slug/products` | No | - | - | `[]Product` | |
| **Auth** | | | | | | |
| POST | `/api/auth/register` | No | - | `{email, pass, name}` | `TokenPair` | Customer only |
| POST | `/api/auth/login` | No | - | `{email, pass}` | `TokenPair` | |
| POST | `/api/auth/refresh` | No | - | `{refreshToken}` | `TokenPair` | Usually via HttpOnly cookie |
| POST | `/api/auth/logout` | Yes | Any | - | `Success` | Invalidates token |
| **Customer** | | | | | | |
| GET | `/api/customer/profile` | Yes | Cust | - | `User` | |
| PATCH| `/api/customer/profile` | Yes | Cust | `{name...}` | `User` | |
| GET | `/api/customer/cart` | Yes | Cust | - | `Cart` | |
| POST | `/api/customer/cart/items` | Yes | Cust | `{variantId, qty}` | `Cart` | |
| PATCH| `/api/customer/cart/items/:id`| Yes | Cust | `{qty}` | `Cart` | |
| DELETE| `/api/customer/cart/items/:id`| Yes | Cust | - | `Success` | |
| GET | `/api/customer/favorites` | Yes | Cust | - | `[]Product` | |
| POST | `/api/customer/favorites/:id` | Yes | Cust | - | `Success` | |
| DELETE| `/api/customer/favorites/:id` | Yes | Cust | - | `Success` | |
| POST | `/api/customer/orders` | Yes | Cust | `{cartId, address}`| `Order` | Triggers reservation |
| GET | `/api/customer/orders` | Yes | Cust | - | `[]Order` | |
| GET | `/api/customer/orders/:id` | Yes | Cust | - | `Order` | |
| POST | `/api/customer/returns` | Yes | Cust | `{orderId, reason}`| `ReturnReq` | |
| **Seller** | | | | | | |
| GET | `/api/seller/me` | Yes | Sel | - | `Seller` | |
| GET | `/api/seller/dashboard` | Yes | Sel | - | `Stats` | |
| GET | `/api/seller/products` | Yes | Sel | - | `[]Product` | |
| POST | `/api/seller/products` | Yes | Sel | `{title, price...}` | `Product` | Created as draft |
| GET | `/api/seller/products/:id` | Yes | Sel | - | `Product` | |
| PATCH| `/api/seller/products/:id` | Yes | Sel | `{fields}` | `Product` | |
| POST | `/api/seller/products/:id/submit-moderation`| Yes | Sel | - | `Product` | Changes status to pending |
| GET | `/api/seller/orders` | Yes | Sel | - | `[]OrderItem` | Filtered to seller's items |
| GET | `/api/seller/analytics` | Yes | Sel | - | `Analytics` | |
| GET | `/api/seller/payouts` | Yes | Sel | - | `[]Payout` | |
| POST | `/api/seller/payouts/request`| Yes | Sel | `{amount}` | `Payout` | |
| GET | `/api/seller/templates` | Yes | Sel | - | `[]Template` | |
| POST | `/api/seller/templates` | Yes | Sel | `{content}` | `Template` | |
| PATCH| `/api/seller/templates/:id` | Yes | Sel | `{content}` | `Template` | |
| DELETE| `/api/seller/templates/:id`| Yes | Sel | - | `Success` | |
| **Admin** | | | | | | |
| GET | `/api/admin/dashboard` | Yes | Adm | - | `AdminStats` | |
| GET | `/api/admin/users` | Yes | Adm | - | `[]User` | |
| PATCH| `/api/admin/users/:id/status`| Yes | Adm | `{status}` | `User` | Block/Unblock |
| GET | `/api/admin/sellers` | Yes | Adm | - | `[]Seller` | |
| POST | `/api/admin/sellers` | Yes | Adm | `{name, email...}` | `Seller` | |
| PATCH| `/api/admin/sellers/:id/status`| Yes | Adm | `{status}` | `Seller` | |
| GET | `/api/admin/products` | Yes | Adm | - | `[]Product` | |
| GET | `/api/admin/moderation/products`| Yes | Adm | - | `[]Product` | Where status=pending |
| POST | `/api/admin/moderation/products/:id/approve`| Yes | Adm | - | `Product` | |
| POST | `/api/admin/moderation/products/:id/reject` | Yes | Adm | `{reason}` | `Product` | |
| GET | `/api/admin/orders` | Yes | Adm | - | `[]Order` | |
| PATCH| `/api/admin/orders/:id/status`| Yes | Adm | `{status}` | `Order` | |
| GET | `/api/admin/inventory` | Yes | Adm | - | `[]Inventory` | |
| POST | `/api/admin/inventory/receipts`| Yes | Adm | `{variantId, qty}` | `Success` | Adds stock |
| POST | `/api/admin/inventory/adjustments`| Yes | Adm | `{variantId, delta}`| `Success` | Audited |
| GET | `/api/admin/returns` | Yes | Adm | - | `[]ReturnReq` | |
| PATCH| `/api/admin/returns/:id/status`| Yes | Adm | `{status}` | `ReturnReq` | |
| GET | `/api/admin/payouts` | Yes | Adm | - | `[]Payout` | |
| PATCH| `/api/admin/payouts/:id/status`| Yes | Adm | `{status}` | `Payout` | |
| GET | `/api/admin/audit-logs` | Yes | Adm | - | `[]AuditLog` | Read-only |
| **Payments** | | | | | | |
| POST | `/api/payments/create` | Yes | Cust | `{orderId}` | `PaymentURL` | Initiates provider flow |
| POST | `/api/payments/webhook` | No | - | Provider Payload | `Success` | Must verify signature |
| POST | `/api/payments/refund-webhook`| No | - | Provider Payload | `Success` | |

## 6. Order Flow

1. Customer adds product to cart.
2. Customer starts checkout.
3. Backend validates available stock (`total - reserved`).
4. Backend creates order in `pending` status.
5. Backend creates a short-lived `Reservation` in Redis and updates `reserved_stock` in DB.
6. Backend initiates payment with the provider (T-Bank/YooKassa).
7. Customer pays via the provider's redirect/widget.
8. Payment provider sends an asynchronous webhook to `/api/payments/webhook`.
9. Backend verifies the webhook signature.
10. Backend marks the `Payment` as succeeded.
11. Backend marks the `Order` as `paid`.
12. Backend converts the temporary reservation into a permanent `StockMovement` (sale) and deducts `total_stock`.
13. Admin/operator assembles and ships the order (updating delivery status).
14. Delivery reaches `delivered` status.
15. After the mandatory return window (e.g., 14 days post-delivery), the funds become available in the seller's balance for payout.

*Crucial Constraint:* An order must never be marked as paid based purely on a frontend redirect. Only the verified backend webhook guarantees payment completion.

## 7. Inventory Flow

The inventory model ensures strict consistency between physical items and digital stock.
- Seller submits a product; Admin approves it.
- Seller sends physical items to the ZAMK warehouse.
- Admin creates an inventory receipt.
- Stock becomes available on the storefront *only* after this receipt is processed.
- Tracked strictly by `ProductVariant`.
- Rule: `available_stock = total_stock - reserved_stock`.
- All modifications generate an immutable `StockMovement`.

**Movement Types:**
- `receipt` (admin receives stock)
- `reservation_created` (temporary hold)
- `reservation_released` (cart expired/cancelled)
- `sale` (order finalized)
- `return` (item added back to pool)
- `adjustment` (manual admin correction)
- `write_off` (damaged/lost goods)

## 8. Product Moderation Flow

**Statuses:** `draft` -> `pending_moderation` -> `approved` -> `published` / `rejected` / `hidden` / `blocked` / `out_of_stock`.

**Rules:**
- Sellers construct products in `draft`.
- Sellers actively submit for moderation, moving it to `pending_moderation`.
- Admins review and either transition to `approved` or `rejected` (with reasons).
- Only `approved` (and subsequently `published`) products are visible in `/api/public/*`.
- Admins possess override powers to `hide` or `block` any product at any time.
- Every state change enacted by an admin writes to the `audit_logs`.

## 9. Returns and Refunds Flow

1. Customer initiates a `ReturnRequest` via the portal.
2. Admin reviews and approves the request.
3. Customer ships the item back.
4. Warehouse receives the item; Admin marks the return as completed.
5. Backend issues a refund request to the payment provider.
6. Provider webhook confirms the refund; `Refund` record is finalized.
7. System generates a `return` StockMovement.
8. System recalculates the seller's pending balance to deduct the refunded amount.

## 10. Payout Flow

- When an order is paid, the seller's cut goes into a `pending` state.
- Once the order is `delivered` and the return window expires, the amount shifts to an `available` balance.
- Seller explicitly requests a payout for a specific amount.
- An admin reviews the request and executes the bank transfer outside the system (or via a B2B integration).
- Admin marks the payout as `paid`.
- Actions are strictly recorded in `audit_logs`.

## 11. Security Requirements

- **Hashing:** Passwords hashed with bcrypt (cost factor >= 10).
- **Tokens:** JWT access tokens (short TTL, e.g., 15m) + opaque refresh tokens.
- **Cookies:** Refresh tokens stored strictly in `HttpOnly`, `Secure`, `SameSite=Strict` cookies.
- **Rate Limiting:** IP-based and user-based limits heavily enforced on `/api/auth/*` using Redis.
- **RBAC:** Middleware explicitly asserts `customer`, `seller`, or `admin` scopes on protected routes.
- **Resource Ownership:** Seller routes dynamically validate that the authenticated user's `seller_id` matches the entity being manipulated.
- **CORS:** Restrict allowed origins explicitly to ZAMK frontend domains.
- **Webhooks:** Mandatory signature verification using provider secret keys.
- **Data Safety:** Strict use of parameterized queries (via `database/sql` or an ORM like GORM/Ent) to prevent SQLi. Input validation (e.g., `go-playground/validator`).
- **Auditing:** Admin and critical seller actions logged to `audit_logs`.
- **Registration:** No self-registration for admins or sellers.
- **Future-proofing:** Architecture supports future rollout of 2FA for admin roles.

## 12. Database Tables Draft

- `users`: Core identity (id, email, password_hash, role, status).
- `user_sessions`: Refresh token tracking (id, user_id, token_hash, expires_at).
- `sellers`: Business entities (id, brand_name, contact_email, status).
- `seller_users`: Joins users to sellers (user_id, seller_id).
- `categories`: Hierarchy (id, name, parent_id).
- `brands`: Marketing entities (id, name, slug).
- `products`: Base item (id, seller_id, title, description, status, category_id).
- `product_images`: Assets (id, product_id, s3_key, sort_order).
- `product_variants`: SKUs (id, product_id, sku, price).
- `product_templates`: Reusable markdown (id, seller_id, content).
- `product_moderation_logs`: History of approval/rejection (id, product_id, admin_id, old_status, new_status).
- `inventory_items`: Physical counts (id, variant_id, total_stock, reserved_stock).
- `stock_movements`: Ledger (id, inventory_item_id, type, quantity, reference_id).
- `reservations`: Redis-backed or DB tracked temporary holds (id, variant_id, quantity, expires_at).
- `carts` & `cart_items`: Shopping state (id, user_id) & (id, cart_id, variant_id, quantity).
- `orders` & `order_items`: Purchases (id, customer_id, total, status) & (id, order_id, variant_id, price).
- `order_status_history`: Timeline (id, order_id, status, created_at).
- `payments` & `payment_events`: Finance (id, order_id, amount, status) & (id, payment_id, payload).
- `deliveries` & `delivery_events`: Logistics (id, order_id, tracking_number) & (id, delivery_id, status).
- `returns` & `return_items`: RMAs (id, order_id, status) & (id, return_id, order_item_id).
- `refunds`: Outgoing finance (id, payment_id, amount, status).
- `reviews`: UGC (id, user_id, product_id, rating, comment).
- `favorites`: Wishlist (user_id, product_id).
- `payouts` & `payout_items`: Seller finance (id, seller_id, amount) & (id, payout_id, order_item_id).
- `audit_logs`: Security (id, actor_id, action, entity, entity_id).
- `notifications`: User alerts (id, user_id, message, is_read).

## 13. Backend Implementation Phases

- **Phase 1:** Skeleton, routing setup, config management, healthchecks, PostgreSQL/Redis connections, Goose/golang-migrate setup.
- **Phase 2:** Authentication layer, JWT logic, Users/Roles tables, Session management.
- **Phase 3:** Admin endpoints to create sellers, SellerUser mapping, Seller dashboard login flows.
- **Phase 4:** Catalog foundations: Categories, Brands, Products, Variants, S3 Image uploads.
- **Phase 5:** Product moderation state machine and admin approval queues.
- **Phase 6:** Inventory system: Receipts, Stock Movement ledger, Redis reservations.
- **Phase 7:** Cart logic, Checkout processes, and Order creation.
- **Phase 8:** Payment provider integrations (T-Bank/YooKassa), Webhook handlers, secure state transitions.
- **Phase 9:** Delivery tracking models and status updates.
- **Phase 10:** Returns lifecycle and Refund orchestrations.
- **Phase 11:** Payout ledgers for sellers and admin processing queues.
- **Phase 12:** Reviews, Favorites, Product recommendations, and Analytics aggregation.
- **Phase 13:** Hardening, structured logging, Prometheus metrics, CI/CD, deployment manifests.
