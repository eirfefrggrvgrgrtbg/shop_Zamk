# Phase 13 Implementation Plan: Production Hardening / Security / Deploy Preparation

## Scope

This plan prepares ZAMK for staging and production hardening after the completed Phase 12A-12E work.

This phase is planning only:

- Do not add new business features.
- Do not change backend business logic unless a later approved phase finds a real production blocker.
- Do not change the commission model now.
- Keep `MARKETPLACE_COMMISSION_BPS=1500` as the current MVP value.
- Do not refactor working flows without a targeted hardening reason.

## Current Baseline

Verified baseline:

- Backend core is implemented.
- `apps/shop`, `apps/seller`, and `apps/admin` are connected to backend APIs.
- External S3-compatible storage works.
- Full system smoke test passed.
- Frontend builds and backend tests/build pass.
- Git working tree was clean before this plan.

Observed planning notes from current code:

- `backend/.env` exists locally and contains real environment data. It must remain uncommitted.
- Root `.gitignore` ignores `*.local`, but does not explicitly ignore `.env` files. Add explicit `.env` ignore rules in Phase 13A.
- `backend/.env.example` still uses some old variable names (`PORT`, `DB_URL`, `REDIS_URL`, `JWT_SECRET`, `JWT_EXPIRATION_HOURS`) while current config code reads names like `APP_PORT`, `POSTGRES_*`, `REDIS_*`, `JWT_ACCESS_SECRET`, and `JWT_REFRESH_SECRET`. Align examples in Phase 13A.
- Refresh cookies are already `HttpOnly`, but current auth handler hardcodes local cookie defaults: `Secure=false`, empty domain, `SameSite=Lax`. Make these configurable for production.
- TBank provider has a `STUB` mode for local/e2e. Production must use real/sandbox provider credentials and webhook verification.
- Rate limiting is not visible in current middleware and should be added before production exposure.

## 1. Environment Separation

Define three separate environments:

- `local`: developer workstation, Docker Compose or local services, non-production S3 or sandbox bucket.
- `staging`: production-like infrastructure, staging domains, sandbox payments, staging S3 bucket, staging database.
- `production`: real customer traffic, production domains, real payment credentials, production S3 bucket, backups and monitoring enabled.

Required files and ownership:

- `backend/.env.local`: local backend values. Not committed.
- `backend/.env.staging.example`: committed placeholders only.
- `backend/.env.production.example`: committed placeholders only.
- `apps/shop/.env.local`: local frontend API URL. Not committed.
- `apps/seller/.env.local`: local frontend API URL. Not committed.
- `apps/admin/.env.local`: local frontend API URL. Not committed.
- `apps/*/.env.staging.example` and `apps/*/.env.production.example`: committed placeholders only.

Production secrets storage:

- Do not commit real secrets.
- Store production secrets in deployment provider secret storage, Vault, Doppler, 1Password Secrets Automation, GitHub Actions environments, or encrypted server-side secret files with restricted permissions.
- Limit read access to production secrets to deploy/runtime only.
- Rotate secrets on suspected exposure and before production launch.

Backend variables to standardize:

- `APP_ENV`: `local`, `staging`, `production`.
- `APP_PORT`: runtime HTTP port.
- `POSTGRES_DSN` or `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_SSLMODE`.
- `REDIS_ADDR` or `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`.
- `JWT_ACCESS_SECRET`: long random secret, production only in secret storage.
- `JWT_REFRESH_SECRET`: separate long random secret, production only in secret storage.
- `CORS_ALLOWED_ORIGINS`: exact frontend origins per environment.
- `S3_ENDPOINT`: provider endpoint.
- `S3_BUCKET`: environment-specific bucket, e.g. `zamk-media-staging` and `zamk-media`.
- `S3_ACCESS_KEY`: backend-side only.
- `S3_SECRET_KEY`: backend-side only.
- `S3_PUBLIC_BASE_URL`: public CDN/bucket URL.
- `TBANK_TERMINAL_KEY`: staging sandbox or production key.
- `TBANK_PASSWORD`: backend-side only.
- `TBANK_SUCCESS_URL`: environment-specific shop success URL.
- `TBANK_FAIL_URL`: environment-specific shop failure URL.
- `RETURN_WINDOW_DAYS`: keep business-approved value, currently 14.
- `MARKETPLACE_COMMISSION_BPS`: keep `1500` for MVP. Revisit only in final commission phase.

Frontend variables:

- `VITE_API_URL`: environment-specific API base URL only.
- No backend secrets in frontend `.env` files.

Values that differ by environment:

- Domains and `VITE_API_URL`.
- CORS origins.
- Cookie secure/domain policy.
- Database and Redis endpoints.
- S3 bucket and public base URL.
- TBank sandbox vs production credentials.
- Logging verbosity.
- Rate limit thresholds.

## 2. Secrets Audit

Phase 13A must audit and harden secrets handling:

- Confirm `backend/.env` is ignored and never committed.
- Add explicit root `.gitignore` rules for `.env`, `.env.*`, with exceptions for `*.env.example` if needed.
- Search git history for accidental secrets before public/private production deployment.
- Ensure frontend `.env` files contain only `VITE_API_URL` and other public values.
- Ensure S3 keys exist only backend-side.
- Ensure TBank secrets exist only backend-side.
- Ensure JWT access/refresh secrets are separate and sufficiently long random values.
- Ensure `.env.example` files contain placeholders only, never real endpoints/secrets unless public and safe.
- Avoid logging full provider webhook payloads if they contain sensitive values.

Recommended checks:

- Secret scan in CI using `gitleaks` or `trufflehog`.
- Manual review of committed `.env.example` files.
- Manual review of deployment logs for accidental env dumps.

## 3. CORS Hardening

Production CORS must use explicit origins:

- `https://shop.<domain>`
- `https://seller.<domain>`
- `https://admin.<domain>`

Rules:

- Do not use wildcard `*` with credentials.
- Allow credentials only for exact trusted frontend origins.
- Restrict methods and headers to what the API needs.
- Confirm preflight behavior for auth, uploads, and admin mutations.
- Keep staging origins separate from production origins.

Cookie-related CORS checks:

- Confirm refresh cookie is accepted from each frontend domain.
- Confirm `SameSite` policy supports the chosen subdomain layout.
- Confirm `Secure=true` for HTTPS environments.

## 4. Cookie / Session Hardening

Current state:

- Refresh cookie is `HttpOnly`.
- Logout clears the refresh cookie.
- Refresh rotation exists through backend session flow.
- Password change clears cookie and revokes sessions.
- Local defaults are hardcoded for cookie domain and secure flag.

Plan:

- Add config for `AUTH_COOKIE_DOMAIN`.
- Add config for `AUTH_COOKIE_SECURE`.
- Add config for `AUTH_COOKIE_SAMESITE` if needed.
- Use `Secure=true` in staging and production.
- Choose cookie domain deliberately:
  - If API and frontends share a parent domain, use a parent cookie domain only if required.
  - Prefer the narrowest valid cookie scope.
- Keep `HttpOnly=true`.
- Validate `SameSite=Lax` is sufficient for same-site subdomains. Use `SameSite=None; Secure` only if cross-site embedding/cross-site auth is required.
- Confirm logout clears cookie with the same domain/path/secure attributes used to set it.
- Confirm refresh rotation rejects reused/expired refresh tokens.
- Confirm `mustChangePassword` password change revokes old sessions.

## 5. Rate Limiting

Add Redis-backed rate limiting in Phase 13B. Do not implement in this planning phase.

Targets:

- `POST /api/auth/login`: strict IP + email based limits.
- `POST /api/auth/register`: IP based and email/domain based limits.
- `POST /api/auth/refresh`: token/session/IP based limits.
- `POST /api/auth/change-password`: user + IP limits.
- `POST /api/payments/tbank/webhook`: provider/IP/signature-aware limit with careful false-positive handling.
- Upload endpoints:
  - seller product image upload;
  - seller profile image upload;
  - admin product image upload;
  - admin brand logo upload.
- Admin dangerous actions:
  - inventory write-offs/adjustments;
  - product block/hide;
  - return refund creation;
  - payout approve/paid.

Design notes:

- Use Redis atomic counters or sliding windows.
- Return safe `429` errors.
- Log rate-limit events without secrets.
- Keep thresholds configurable per environment.

## 6. Upload Security

Current behavior verified by smoke:

- SVG upload is rejected.
- Product image upload works with external S3.
- S3 credentials are not needed in frontend.

Plan:

- Keep extension validation.
- Keep MIME validation and verify it checks actual content, not only client-provided headers.
- Enforce `S3_UPLOAD_MAX_SIZE_MB`.
- Store object keys server-side and avoid exposing internal object keys in public UI.
- Bucket policy:
  - public read only for public assets;
  - write/delete only via backend credentials;
  - no frontend direct credentials.
- Separate staging and production buckets.
- Add future antivirus scanning for user-provided uploads.
- Add future image optimization/resizing.
- Add CDN in front of public media.
- Add object lifecycle policy for unused draft images if needed.

## 7. Payment Hardening

Current behavior verified by smoke:

- Frontend does not set order `paid`.
- Admin is blocked from manually setting `paid`.
- Valid webhook changes payment/order state.
- Invalid webhook does not mark payment paid.
- Duplicate webhook is idempotent and does not double-deduct stock.

Plan before staging:

- Ensure sandbox TBank credentials are configured outside code.
- Ensure provider secrets are never logged.
- Ensure webhook body logging is redacted or disabled for sensitive fields.
- Add alerting for repeated invalid webhook signatures.
- Confirm idempotency constraints at database level.

Plan before production:

- Replace local/e2e `STUB` usage with real TBank provider config.
- Run real TBank sandbox certification flow.
- Verify production callback URL with HTTPS.
- Confirm webhook signature verification against real provider payloads.
- Confirm payment amount/order ID validation.
- Confirm no manual path can set paid outside verified webhook.

## 8. Worker Hardening

Current architecture:

- `cmd/worker` is separate from API.
- Worker handles expired awaiting-payment orders.
- Worker handles seller funds availability.
- Worker has graceful signal handling.

Plan:

- Confirm worker is deployed as a separate process/service.
- Add health/readiness endpoint or process monitoring for worker.
- Ensure no double processing across multiple worker instances.
- Review critical queries for `SELECT FOR UPDATE` and `SKIP LOCKED` where concurrent workers may run.
- Keep batch sizes configurable.
- Add retry strategy for transient DB/Redis failures.
- Add structured logs for each worker job:
  - checked count;
  - processed count;
  - failed count;
  - duration.
- Add metrics for expired orders and seller balance availability conversions.
- Confirm graceful shutdown cancels loops cleanly.

## 9. Database Hardening

Migrations:

- Use one migration tool consistently.
- Run migrations in CI against an empty DB.
- Run migration dry-run or staging migration before production.
- Never edit already-applied production migrations; add new migrations only.

Backups:

- Configure automatic PostgreSQL backups.
- Define RPO/RTO targets.
- Run restore test before production.
- Store backups separately from the main DB host.

Review areas:

- Indexes for high-volume tables:
  - users by email;
  - products by status/seller/category/brand;
  - orders by user/status/created_at;
  - order items by seller/product;
  - inventory by product variant/seller;
  - payments by order/provider payment ID/status;
  - returns/refunds by order/status;
  - reviews by product/status;
  - payouts/ledger by seller/status/created_at.
- Pagination for all large admin/seller/customer lists.
- N+1 query hotspots in list/detail APIs.
- Transaction isolation risks in:
  - stock reservation/sale conversion;
  - refunds/restock;
  - payout requests and status changes;
  - worker availability conversion.
- DB constraints:
  - uniqueness;
  - foreign keys;
  - status enums/checks where appropriate;
  - non-negative money/stock constraints.

## 10. Observability

Plan:

- Keep structured JSON logs.
- Ensure request IDs are generated and propagated.
- Log auth/admin/payment/upload failures safely.
- Add audit logs for admin actions:
  - seller status changes;
  - category/brand changes;
  - product moderation;
  - inventory adjustments/write-offs;
  - order/shipment status changes;
  - return/refund decisions;
  - payout approval/paid.
- Payment webhook logs must not include secrets, full card data, or raw tokens.
- Add metrics:
  - orders count by status;
  - payment success/failure count;
  - webhook invalid signature count;
  - stock movements;
  - refunds;
  - payout requests/approved/paid;
  - upload failures;
  - auth login failures/rate-limit hits.
- Add alerting later for:
  - API 5xx spike;
  - payment webhook failures;
  - worker failures;
  - DB connection failures;
  - low disk/backups failing.

## 11. RBAC / Security Audit Matrix

Customer:

- Can manage only own cart.
- Can view only own orders.
- Can create returns only for own delivered orders and valid quantities.
- Can create reviews only for own delivered purchases.
- Cannot access seller/admin endpoints.

Seller:

- Can manage only own products.
- Can read own inventory only.
- Cannot mutate stock directly.
- Can view own order items only.
- Must not see customer phone/email/address.
- Can view own returns/reviews.
- Can view own balance/payouts and request payouts within available balance.
- Cannot access admin endpoints.

Admin:

- Can manage sellers/categories/brands/products/moderation.
- Can manage inventory, orders, shipments, returns, refunds, reviews, payouts.
- Cannot manually set order `paid`; payment webhook owns that transition.
- Dangerous actions should require UI confirmation and backend validation.

Public:

- Can see only published products.
- Can see only published reviews/rating summaries.
- Cannot see draft/pending/hidden/blocked data.

## 12. Frontend Security

Plan:

- Confirm refresh token is never stored in `localStorage` or `sessionStorage`.
- Keep access token in memory only.
- Keep all backend secrets out of frontend builds.
- Keep `VITE_API_URL` as the only required frontend runtime variable for now.
- Do not expose S3 object keys in public UI if they are internal implementation details.
- Treat protected routes as UX only. Backend remains source of truth.
- Keep frontend error messages safe and user-friendly.
- Confirm admin/seller/customer apps do not rely on client-side role checks for authorization.
- Add dependency audit to CI.

## 13. Deploy Architecture

Recommended layout:

- Shop frontend: `https://shop.<domain>`
- Seller frontend: `https://seller.<domain>`
- Admin frontend: `https://admin.<domain>`
- Backend API: `https://api.<domain>`
- Storage bucket: `zamk-media` for production, separate bucket for staging.

Frontend hosting:

- Static hosting via Nginx/Caddy, object hosting, or managed frontend hosting.
- Build each app separately.
- Configure `VITE_API_URL=https://api.<domain>/api`.

Backend:

- Run API as a service.
- Run worker as a separate service.
- Use process manager/systemd/Docker/Kubernetes depending on deployment target.

Database:

- Prefer managed PostgreSQL where possible.
- If VPS PostgreSQL is used, configure backups, monitoring, firewall, and restricted access.

Redis:

- Managed Redis or VPS Redis with auth, persistence policy, firewall, and monitoring.

Reverse proxy:

- Nginx or Caddy.
- HTTPS certificates with automatic renewal.
- gzip/brotli compression.
- Security headers:
  - `Strict-Transport-Security`;
  - `X-Content-Type-Options`;
  - `X-Frame-Options` or CSP `frame-ancestors`;
  - `Referrer-Policy`;
  - carefully designed `Content-Security-Policy`.

## 14. CI/CD Plan

Pipeline stages:

1. Install dependencies.
2. Run frontend typecheck/build:
   - `npm run build:shop`
   - `npm run build:seller`
   - `npm run build:admin`
   - `npm run build`
3. Run backend checks:
   - `go test ./...`
   - `go build ./cmd/...`
4. Run migrations against a temporary DB.
5. Run secret scanning.
6. Build Docker images or deploy artifacts.
7. Push images/artifacts to registry.
8. Deploy to staging.
9. Run staging smoke test.
10. Manual approval for production.
11. Deploy production.
12. Run production health checks.

Rollback plan:

- Keep previous backend image/artifact.
- Keep previous frontend artifacts.
- Ensure migrations are backward-compatible where possible.
- For destructive DB changes, require explicit rollback/restore plan.
- Document manual rollback steps.

## 15. Pre-Production Checklist

Before staging:

- All builds pass.
- Backend tests pass.
- Smoke test passes.
- Real external S3 upload passes.
- `.env.example` files use current variable names and placeholders only.
- `backend/.env` and all local env files are ignored.
- CORS staging origins configured.
- Secure cookie settings configurable.
- Rate limiting implemented for auth/upload/webhook/dangerous admin actions.
- TBank sandbox credentials configured.

Before production:

- Real payment sandbox/certification passed.
- Backups configured.
- Restore test completed.
- Admin user creation procedure documented.
- Production domains configured.
- Production CORS origins set exactly.
- Secure cookies enabled.
- HTTPS configured.
- Logs checked for secrets.
- Monitoring baseline enabled.
- Alerting for critical failures configured.
- Production S3 bucket policy reviewed.
- Worker deployed and monitored.
- Rollback plan documented and tested at least once in staging.

## 16. Known Technical Debt

- Commission model is still MVP: `MARKETPLACE_COMMISSION_BPS=1500`. It must be revisited at the end of the project, not now.
- Dashboard analytics still contain placeholders/mock-derived sections.
- Admin product image upload UI may need UX polish if still inconvenient for production operations.
- Real TBank production integration is not finalized until sandbox/certification and production credentials are configured.
- No real delivery provider integration yet.
- No antivirus scanning for uploads.
- No image optimization pipeline.
- No CDN in front of public media yet.
- No legal/accounting document generation.
- No automatic bank payouts; admin `paid` remains an offline/manual transfer marker.
- Advanced pagination/filtering is not everywhere yet.
- Rate limiting is not implemented yet.
- Worker observability and retry strategy need hardening.

## 17. Recommended Execution Order

Phase 13A - Secrets/env/CORS/cookie hardening:

- Align env variable names and `.env.example` files.
- Harden `.gitignore`.
- Add production cookie config.
- Configure exact CORS origins.
- Run secret scan.

Phase 13B - Rate limiting and security middleware:

- Add Redis-backed rate limits.
- Add safe `429` handling.
- Add security headers at reverse proxy or API layer.

Phase 13C - DB indexes/pagination/query review:

- Review indexes.
- Review large list pagination.
- Review transaction isolation and concurrency.
- Review N+1 hotspots.

Phase 13D - Docker/deploy configs:

- Add production Dockerfiles/compose or target deploy manifests.
- Split API and worker services.
- Add reverse proxy config.
- Add deployment docs.

Phase 13E - Monitoring/logging:

- Add metrics.
- Add request IDs and audit log coverage.
- Add alerts.
- Add dashboard baseline.

Phase 13F - Final commission model revision:

- Revisit marketplace commission model near project end.
- Decide whether `MARKETPLACE_COMMISSION_BPS=1500` remains correct.
- Only then change commission business rules if approved.

## Criticality Summary

Critical before staging:

- Correct env examples and explicit ignored secret files.
- Exact CORS for staging domains.
- Configurable secure refresh cookie settings.
- TBank sandbox config.
- Rate limiting for auth/webhook/uploads.
- Migration and smoke test on staging.

Critical before production:

- Secret scan and secret storage completed.
- Secure cookies and HTTPS.
- Real payment sandbox/certification complete.
- Backup and restore test complete.
- Worker monitored.
- Observability baseline and critical alerts.
- Production S3 bucket policy reviewed.
- Rollback plan ready.

Can remain after MVP:

- CDN.
- Antivirus/image optimization.
- Real delivery provider.
- Automatic bank payouts.
- Legal/accounting document generation.
- Advanced analytics dashboards.
- Full advanced pagination/filtering everywhere.
- Final commission revision, as long as MVP commission is explicitly accepted for launch.
