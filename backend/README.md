# ZAMK Backend

This is the backend for the ZAMK curated marketplace. It's built as a modular monolith in Go.

## Tech Stack
- **Language**: Go 1.22+
- **Router**: chi (`github.com/go-chi/chi/v5`)
- **Database**: PostgreSQL (`github.com/jackc/pgx/v5`)
- **Redis**: `github.com/redis/go-redis/v9`
- **Config**: Environment variables + `.env` (`github.com/joho/godotenv`)
- **Storage**: S3-compatible (MinIO for local dev)

## Setup Local Infrastructure

1. Copy the example configuration:
   ```bash
   cp .env.example .env
   ```

2. Start the local infrastructure (PostgreSQL, Redis, MinIO) via Docker Compose:
   ```bash
   docker compose up -d
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
