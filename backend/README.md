# ZAMK Backend

This is the backend for the ZAMK curated marketplace. It's built as a modular monolith in Go.

## Tech Stack
- **Language**: Go 1.22+
- **Router**: chi (`github.com/go-chi/chi/v5`)
- **Database**: PostgreSQL (`github.com/jackc/pgx/v5`)
- **Redis**: `github.com/redis/go-redis/v9`
- **Config**: Environment variables + `.env` (`github.com/joho/godotenv`)
- **Storage**: Both local MinIO for development and external S3-compatible provider for production-like verification are supported.

## Environment and Secrets

Use `.env.example` as the local template:

```bash
cp .env.example .env
```

Real `.env` files are ignored and must not be committed. Keep production secrets in your deployment secret store, not in Git.

Backend environment files may contain server-side secrets such as:

- `JWT_ACCESS_SECRET`
- `JWT_REFRESH_SECRET`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- `TBANK_PASSWORD`

Frontend environment files must not contain backend secrets. For `apps/shop`, `apps/seller`, and `apps/admin`, the required public value is:

```env
VITE_API_URL=http://localhost:8080/api
```

For staging and production, set `VITE_API_URL` to the public API origin, for example `https://api.example.com/api`.

### CORS

`CORS_ALLOWED_ORIGINS` is a comma-separated list of exact frontend origins. Local development can use:

```env
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5174,http://localhost:5175
```

For staging and production, configure exact HTTPS origins for the shop, seller, and admin frontends. Do not use wildcard CORS with credentials.

### Refresh Cookie

Refresh tokens are stored in an `HttpOnly` cookie. Configure cookie behavior with:

```env
AUTH_COOKIE_DOMAIN=
AUTH_COOKIE_SECURE=false
AUTH_COOKIE_SAMESITE=Lax
```

Local defaults keep `AUTH_COOKIE_SECURE=false` and an empty cookie domain. In staging and production, use `AUTH_COOKIE_SECURE=true` and configure the cookie domain only if your domain layout requires it. Logout and password-change flows clear the cookie using the same cookie settings.

### Rate Limiting

ZAMK uses Redis-backed fixed-window rate limiting for sensitive endpoints. It is enabled by default:

```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_FAIL_OPEN_LOCAL=true
AUTH_LOGIN_LIMIT_PER_MINUTE=5
AUTH_REGISTER_LIMIT_PER_HOUR=10
AUTH_REFRESH_LIMIT_PER_MINUTE=30
AUTH_CHANGE_PASSWORD_LIMIT_PER_HOUR=5
UPLOAD_LIMIT_PER_MINUTE=10
WEBHOOK_LIMIT_PER_MINUTE=120
ADMIN_DANGEROUS_ACTION_LIMIT_PER_MINUTE=30
```

Protected endpoint groups:

- auth: login, register, refresh, change-password;
- uploads: seller product images, seller logo, admin product images, admin brand logo;
- payment webhook: T-Bank webhook;
- admin dangerous actions: inventory mutations, product moderation mutations, refund creation, payout status changes, review moderation mutations.

Local behavior can fail open on Redis errors when `APP_ENV=local` and `RATE_LIMIT_FAIL_OPEN_LOCAL=true`. Staging and production fail closed by default so sensitive endpoints return `429` if Redis is unavailable. Rate limit keys hash email/session-like data and never include raw passwords, access tokens, refresh tokens, S3 keys, or TBank secrets.

## Setup Local Infrastructure

1. Copy the example configuration:
   ```bash
   cp .env.example .env
   ```

2. Start the local infrastructure (PostgreSQL, Redis, MinIO) via Docker Compose:
   ```bash
   docker compose up -d
   ```

3. Configure S3/MinIO in `.env`:
   `.env.example` contains placeholders only. For local MinIO or an external S3-compatible provider, put real credentials only in your ignored `.env` file:
   ```env
   S3_ENDPOINT=your-s3-endpoint
   S3_PORT=443
   S3_REGION=default
   S3_BUCKET=your-bucket
   S3_ACCESS_KEY=your-access-key
   S3_SECRET_KEY=your-secret-key
   S3_USE_SSL=true
   S3_PUBLIC_BASE_URL=https://your-public-bucket-url
   S3_UPLOAD_MAX_SIZE_MB=10
   ```

## Database Migrations

We use `golang-migrate` for database migrations.

To install `golang-migrate`:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

To run migrations up (Note: local Docker uses port 5433!):
```bash
migrate -path migrations -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" up
```

To run migrations down:
```bash
migrate -path migrations -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" down
```

## Running the Applications

### Run the API Server
```bash
go run ./cmd/api
```

### Run the Background Worker
```bash
go run ./cmd/worker
```

## Health Endpoints

Once the API is running (default port 8080), you can check its health:

- **Liveness Check**: `GET http://localhost:8080/api/health`
- **Readiness Check**: `GET http://localhost:8080/api/ready`

### Creating an Admin User (Local Dev Only)
Sellers cannot self-register in ZAMK. Only admins can create seller accounts.
For local development, use the `dev-create-admin` tool to bootstrap your admin account:

```bash
# Assuming you have loaded `.env` variables or run:
ADMIN_EMAIL="admin@zamk.com" ADMIN_PASSWORD="securePassword123" ADMIN_NAME="Admin" go run ./cmd/dev-create-admin
```

## Admin and Seller Endpoints
Once an admin is created and logged in via `POST /api/auth/login`, they can manage sellers:

- `POST /api/admin/sellers` (Admin only: creates a seller, user, and linked profile)
- `GET /api/admin/sellers` (Admin only: lists sellers)
- `PATCH /api/admin/sellers/{id}/status` (Admin only: updates seller status)

Sellers can manage their own context:
- `GET /api/seller/me` (Seller only: returns linked seller and user information)

*Note: Public seller registration is explicitly omitted by design. Frontend role checks are only for UX, as the backend strictly enforces admin/seller access via `RequireRole` middleware.*

### Temporary Password Rotation
Admin-created sellers are provisioned with a temporary password. 
- The backend sets `must_change_password = true` in the database.
- The `mustChangePassword` flag is exposed in the `User` object during `POST /api/auth/login` and `GET /api/auth/me`.
- The frontend should redirect these users to a password change screen.
- Sellers can change their password using `POST /api/auth/change-password` (requires valid access token, `currentPassword`, and `newPassword`).
- Upon a successful password change, existing sessions are revoked and the user must login again.

## Seller Balance & Payouts (Ledger)

The ZAMK marketplace operates on a double-entry inspired immutable ledger (`seller_balance_ledger`) to track seller funds:

1. **sale_pending**: Created when an order is delivered to the customer. Represents pending revenue minus commission.
2. **refund_deduction**: Created when an order is returned and refunded. Subtracted from the available balance.
3. **sale_available**: Created automatically by the background worker when a `sale_pending` entry exceeds the 14-day return window. The funds become withdrawable.
4. **payout_requested**: Created when a seller requests a payout. Acts as an immediate hold on their available balance (negative entry).
5. **payout_rejected/payout_cancelled**: Created if an admin rejects or cancels a payout. Releases the held funds back to the seller's available balance (positive entry).
6. **payout_paid**: A zero-amount audit marker representing a successful bank transfer.

### Commission Model
The marketplace commission is configured globally using `MARKETPLACE_COMMISSION_BPS` (basis points). For example, `1500` represents a `15.00%` commission. The commission is automatically calculated on the server and deducted before recording a `sale_pending` entry.

### 14-Day Availability Rule
Seller funds become available *only* after:
- The order has been delivered.
- The 14-day return window has passed.
- No returns or refunds were processed for the item.

A background worker periodically scans for eligible `sale_pending` entries and converts them into `sale_available`.

## Reviews & Ratings

ZAMK supports customer reviews for products they have verifiedly purchased.
- **Verified Purchase Only:** A customer can only leave a review for a product if they have an order for that product in the `delivered` status.
- **Admin Moderation:** All new reviews start in a `pending_moderation` status. They must be explicitly approved (`published`) by an admin before they are visible publicly.
- **Dynamic Rating Summary:** A product's rating summary (average score and review count) is calculated dynamically from `published` reviews. This summary is injected into public product APIs.
- **PII Protection:** Customer names are masked in public reviews (e.g., displayed simply as "Customer") to protect their privacy.
