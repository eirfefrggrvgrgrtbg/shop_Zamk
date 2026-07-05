# Dev Test Accounts

**WARNING**: These accounts and credentials are for **LOCAL DEVELOPMENT ONLY**. Do not use them in staging or production environments. They use simple insecure passwords.

## 1. Admin / Owner
**Purpose**: Full system access, RBAC management, global order management, catalog and moderation.
- **Email**: `admin@zamk.local`
- **Password**: `Admin12345!`
- **Role**: `owner`

## 2. Platform Staff
**Purpose**: Testing limited admin access (moderation, catalog view).
- **Email**: `staff@zamk.local`
- **Password**: `Staff12345!`
- **Role**: `staff` (with limited permissions like `products.moderate`, `catalog.read`)

## 3. Seller
**Purpose**: Access to seller cabinet, managing own inventory, own orders, and own products.
- **Email**: `seller@zamk.local`
- **Password**: `Seller12345!`
- **Status**: `active`

## 4. Customer
**Purpose**: Placing bids, purchasing items, checking out.
- **Email**: `customer@zamk.local`
- **Password**: `Customer12345!`

## Notes for dev-seed
These accounts are generated automatically when you run the local seeding command.
To reset your local database and recreate these accounts, run the following commands from the `backend` directory:

```bash
# Drop schema
migrate -path migrations -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" down -all

# Apply migrations
migrate -path migrations -database "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable" up

# Run seed
go run ./cmd/dev-seed
```
