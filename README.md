# ZAMK Project

This is a monorepo for the ZAMK project.

## Local Dev Quick Start

### URLs

- Shop: `http://127.0.0.1:3000`
- Seller: `http://127.0.0.1:3001`
- Admin: `http://127.0.0.1:3002`
- API health: `http://127.0.0.1:8080/api/health`
- API ready: `http://127.0.0.1:8080/api/ready`

### Local Dev Accounts

These credentials are local-only test data created by `go run ./cmd/dev-seed`.

- Admin: `admin@zamk.local` / `Admin12345!`
- Seller: `seller@zamk.local` / `Seller12345!`
- Customer: `customer@zamk.local` / `Customer12345!`

### Commands

```bash
# 1. Start local infrastructure
cd backend
docker compose up -d

# 2. Apply migrations
migrate -path migrations -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" up

# 3. Seed deterministic local test accounts and catalog data
go run ./cmd/dev-seed

# 4. Start backend API
go run ./cmd/api

# 5. Optional worker
go run ./cmd/worker
```

In separate terminals from the repository root:

```bash
npm run dev:shop
npm run dev:seller
npm run dev:admin
```

## Structure

- `apps/shop` - Public store for customers.
- `apps/seller` - Separate seller dashboard.
- `apps/admin` - Separate admin panel.
- `packages/ui` - Shared UI components (future).
- `packages/shared` - Shared types, contexts, utils (future).
- `packages/api-client` - Shared frontend API client.
- `backend` - Go backend API, worker, migrations, and local dev commands.

## Backend Phase 7: Payments
- **Payments (`internal/payments`)**: T-Bank-compatible payment provider, webhook validation, and idempotency logic. Webhooks safely convert reservations to sales.
- **Allowed Methods**: Bank Card, SBP, T-Pay, SberPay, Alfa Pay.
- **Excluded**: BNPL (Долями, Яндекс Сплит, etc.).
- **Limitation**: No delivery, no seller payouts, no returns/refunds yet.

### Key Workflows
1. **Cart & Orders**: Cart > Checkout > Order created (`awaiting_payment`) & Stock reserved.
2. **Payments**: Customer requests payment for `awaiting_payment` order. Webhook confirmation transitions order to `paid` and converts stock reservations to sales.
3. **Inventory Ledger**: Complete immutable audit trail via `stock_movements`.
4. **Expiration Worker**: Background loop cancels old unpaid orders and releases inventory reservations gracefully (`reservation_released`).
5. **Fulfillment**: Paid orders receive shipments, tracking numbers, and state synchronisation (`assembling` -> `packed` -> `shipped` -> `delivered`).
6. **Returns & Refunds (Phase 9A)**: Customers can request returns for delivered items within 14 days. Admins approve/reject returns, mark items as received, trigger inventory restock, and create refunds. Seller visibility is strictly limited to prevent PII exposure.

## Worker Configuration
- `ORDER_PAYMENT_TIMEOUT_MINUTES=30` (How long to wait before unpaid order cancellation)
- `WORKER_ORDER_EXPIRATION_INTERVAL_SECONDS=60` (Polling interval)
- `RETURN_WINDOW_DAYS=14` (Allowed days for a customer to request a return after delivery)
Run the worker using `go run ./cmd/worker`.

## Limitations
- **No real delivery integrations**: Uses manual shipment statuses and tracking numbers.
- **Refunds**: External provider refund execution is currently stubbed.
- **No seller payouts**: Seller balance and automated payouts will be handled in Phase 9B.
- **No accounting documents**: Legal document generation is not yet implemented.
- **No frontend integration**: UI implementation is pending.

## Commands

- `npm run dev:shop` - Start the shop app
- `npm run dev:seller` - Start the seller app
- `npm run dev:admin` - Start the admin app
- `npm run dev:seed` - Seed deterministic local dev accounts/catalog data

- `npm run build:shop` - Build the shop app
- `npm run build:seller` - Build the seller app
- `npm run build:admin` - Build the admin app

- `npm run build` - Build all apps

## Notes

- Do not commit `backend/.env`.
- The `dev-seed` command refuses to run when `APP_ENV=production`.
