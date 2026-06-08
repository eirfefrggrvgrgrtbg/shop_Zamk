# Phase 13D Implementation Plan: Docker / Deploy Configs

## Scope

This is a planning-only phase for Docker and deployment configuration. Do not implement Dockerfiles, Compose files, proxy configs, CI workflows, or code changes until this plan is approved.

Non-goals:

- Do not change backend business logic.
- Do not change frontend business logic.
- Do not add new marketplace features.
- Do not change `MARKETPLACE_COMMISSION_BPS=1500`.
- Do not start monitoring/logging Phase 13E.

## 1. Deployment Architecture

Target domain layout:

- Shop frontend: `https://shop.<domain>`
- Seller frontend: `https://seller.<domain>`
- Admin frontend: `https://admin.<domain>`
- Backend API: `https://api.<domain>`
- Media storage: external S3-compatible bucket configured through backend runtime env.

Target services:

- `api`: backend HTTP API from `backend/cmd/api`.
- `worker`: backend background worker from `backend/cmd/worker`.
- `shop`: static frontend build from `apps/shop`.
- `seller`: static frontend build from `apps/seller`.
- `admin`: static frontend build from `apps/admin`.
- `postgres`: managed PostgreSQL preferred; private container acceptable for staging preview only.
- `redis`: managed Redis preferred; private container acceptable for staging preview only.
- `reverse-proxy`: Caddy or Nginx terminates HTTPS and routes domains.

Recommended production shape:

- Use managed PostgreSQL and managed Redis when possible.
- Run `api`, `worker`, and frontend static servers as separate services.
- Keep S3 external. Do not run MinIO in production compose.
- Put all runtime services on private networks except reverse proxy.
- Expose only HTTPS ports publicly.

## 2. Backend Docker Strategy

Planned file: `backend/Dockerfile`.

Recommended design: one backend image, two service commands.

- Build `/app/api` from `./cmd/api`.
- Build `/app/worker` from `./cmd/worker`.
- Compose or deployment config chooses command per service:
  - API service command: `/app/api`
  - Worker service command: `/app/worker`

Requirements:

- Multi-stage build.
- Go build stage using repo backend module.
- Minimal runtime image, preferably distroless or Alpine if shell/debug convenience is needed.
- Non-root runtime user if practical.
- No `.env` copied into the image.
- No secrets baked into image layers or build args.
- Runtime config supplied by environment variables or secret store.
- Healthcheck for API can call `GET /api/health`; worker health should be process/restart-policy based until a dedicated worker health endpoint exists.

One image is preferred over two images because API and worker share the same codebase, dependencies, and release version. Separate images can be reconsidered only if worker dependencies diverge materially.

## 3. Frontend Docker Strategy

Options:

- Separate Dockerfiles:
  - `apps/shop/Dockerfile`
  - `apps/seller/Dockerfile`
  - `apps/admin/Dockerfile`
- Or one root frontend Dockerfile using build args:
  - `APP_NAME=shop|seller|admin`
  - `VITE_API_URL=https://api.<domain>/api`

Recommended initial approach: separate app Dockerfiles for clarity, with the same pattern.

Requirements:

- Build static assets with Node.
- Pass `VITE_API_URL` at build time.
- Serve generated `dist` via Nginx or Caddy static server.
- Build shop, seller, and admin independently.
- Frontend images must contain no backend secrets, S3 keys, TBank credentials, or JWT secrets.
- Only public frontend config should be embedded, currently `VITE_API_URL`.

If runtime API URL switching is required later, add an explicit runtime config script. Do not introduce it in 13D unless needed.

## 4. Compose / Staging Preview Strategy

Planned file options:

- Root `docker-compose.yml` for local/staging-like preview.
- Or `docker-compose.prod.yml` for production-like deployment.

Recommended service list:

- `api`
- `worker`
- `postgres`
- `redis`
- `shop`
- `seller`
- `admin`
- `reverse-proxy`

Compose rules:

- Use external S3 through backend env.
- Keep MinIO only in local dev compose, not production compose.
- Keep `postgres` and `redis` on private networks.
- Do not expose PostgreSQL or Redis publicly.
- Mount volumes only for PostgreSQL/Redis data in preview deployments.
- Use restart policies for API, worker, proxy, Redis, and PostgreSQL.
- Add `depends_on` with health conditions where supported, but do not rely on it as the only readiness mechanism.

Production note: Compose can be acceptable for a single-VPS MVP deployment, but managed services and provider-native deployment are safer for production.

## 5. Environment Files

Backend env examples to plan:

- `backend/.env.local`
- `backend/.env.staging.example`
- `backend/.env.production.example`

Frontend env examples to plan:

- `apps/shop/.env.production.example`
- `apps/seller/.env.production.example`
- `apps/admin/.env.production.example`

Rules:

- Real `.env` files stay ignored.
- Example files contain placeholders only.
- Frontend examples contain only:
  - `VITE_API_URL=https://api.<domain>/api`
- Backend runtime secrets stay backend-only:
  - JWT secrets
  - PostgreSQL credentials/DSN
  - Redis credentials
  - S3 access/secret keys
  - TBank terminal/password
- Do not pass backend secrets as frontend build args.

Staging and production envs differ by:

- `APP_ENV`
- API/frontends domains
- CORS allowed origins
- cookie secure/domain settings
- PostgreSQL DSN
- Redis address/auth
- S3 bucket and public base URL
- TBank callback URLs and credentials

## 6. Reverse Proxy

Recommendation: use Caddy first, unless Nginx-specific control is required.

Caddy advantages:

- Automatic HTTPS certificates.
- Simple domain-based routing.
- Easy reverse proxy to API.
- Simple static file serving.

Nginx remains acceptable if full manual control over caching, compression, headers, and TLS config is required.

Proxy responsibilities:

- Route `shop.<domain>` to shop static server.
- Route `seller.<domain>` to seller static server.
- Route `admin.<domain>` to admin static server.
- Route `api.<domain>` to API service.
- Terminate TLS.
- Redirect HTTP to HTTPS.
- Set upload body size large enough for configured image upload max.
- Enable gzip; brotli if available.
- Add security headers:
  - `Strict-Transport-Security`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy`
  - `X-Frame-Options: DENY` or CSP `frame-ancestors 'none'`
  - basic `Content-Security-Policy`

CSP must be tested against Vite-generated static assets, API calls, S3 media URLs, and payment redirect behavior before enforcing too strictly.

## 7. API Health Checks

Existing backend checks:

- `GET /api/health`
- `GET /api/ready`

Plan:

- Docker API healthcheck should use `/api/health` for process liveness.
- Deployment readiness should use `/api/ready` because it verifies dependencies.
- Reverse proxy should only route to healthy API instances where platform supports it.
- CI/CD deploy step should call `/api/ready` after startup.

Worker monitoring:

- Start with process-level health: container running, restart policy, logs.
- Later Phase 13E can add worker metrics/heartbeat.
- Avoid exposing a worker HTTP port unless there is a clear operational need.

## 8. Migrations Strategy

Production app startup must not auto-run migrations.

Allowed migration approaches:

- Separate migration job/container using the same backend image plus `migrate` binary.
- Manual migration command before deploy.
- CI/CD migration step before switching traffic.

Recommended initial strategy:

- Add migration job in deployment docs/config.
- Run migrations on staging first.
- Run smoke test after staging migration.
- For production, run migrations before new API/worker rollout.
- Keep migrations backward-compatible where possible.

Rules:

- Do not edit old migrations.
- Add new migrations only.
- Keep down migrations for reversible schema changes.
- Document irreversible migrations explicitly before approval.
- Keep DB backups before production migration.

Important Phase 13C follow-up:

- Migration `000013_add_performance_indexes` must be runtime-applied and rollback-verified before staging.
- Previous local verification was blocked because PostgreSQL/Docker were unavailable.
- Phase 13D implementation should include a clean migration verification step once local or staging DB is available.

## 9. Worker Deployment

Worker must run as a separate service/process from API.

Initial plan:

- Deploy one worker replica.
- Use same backend image as API.
- Command: `/app/worker`.
- Restart policy: unless stopped / on failure depending on target platform.
- Runtime env same DB/Redis/S3/payment config as API where needed.
- Logs collected separately from API logs.
- Graceful shutdown via SIGTERM/SIGINT.

Scaling rule:

- Do not run multiple worker replicas until `SKIP LOCKED` and idempotency are verified for all worker jobs in staging.
- Phase 13C confirmed key worker paths use `SKIP LOCKED`, but staging concurrency verification is still required before scaling.

## 10. PostgreSQL Deployment

Preferred: managed PostgreSQL.

Benefits:

- Backups.
- Monitoring.
- Point-in-time restore.
- Managed upgrades.
- Easier restricted networking.

If VPS PostgreSQL is used:

- Use persistent volumes.
- Do not expose port publicly.
- Restrict network to API/worker/migration job.
- Enable regular backups.
- Store backups outside the DB host.
- Test restore before production.
- Monitor disk usage and connection count.
- Use strong credentials from secret store.

Production must define:

- Backup schedule.
- Retention period.
- Restore procedure.
- Maintenance window.

## 11. Redis Deployment

Preferred: managed Redis.

Acceptable MVP alternative: private Redis container.

Rules:

- Require password/auth where supported.
- Keep Redis private network only.
- Do not expose Redis publicly.
- Decide persistence policy explicitly.
- Rate limiting depends on Redis, so production availability matters.

Redis usage:

- Rate limiting from Phase 13B.
- Runtime state if future backend features use it.

Failure policy:

- Production sensitive endpoints should fail closed according to Phase 13B config.
- Local/staging behavior can be tuned separately.

## 12. S3 Deployment

External S3-compatible storage is already used and should remain external.

Plan:

- Separate buckets for staging and production.
- Public-read only for media objects that must be publicly visible.
- Write/delete only through backend credentials.
- No S3 credentials in frontend images, env files, logs, or static assets.
- Configure bucket CORS only if browsers directly load media or if future direct-upload flow is added.
- Keep `S3_PUBLIC_BASE_URL` environment-specific.
- Add CDN later if traffic or latency requires it.

Bucket naming questions:

- Staging bucket name.
- Production bucket name.
- Public media base URL.
- Whether CDN domain will replace raw bucket URL.

## 13. CI/CD Deployment Flow

Recommended pipeline:

1. Install root dependencies.
2. Run backend tests and build:
   - `cd backend && go test ./...`
   - `cd backend && go build ./cmd/...`
3. Run frontend builds:
   - `npm run build:shop`
   - `npm run build:seller`
   - `npm run build:admin`
   - `npm run build`
4. Run secret scan.
5. Build Docker images:
   - backend image
   - shop image
   - seller image
   - admin image
6. Push images to registry.
7. Run migrations on staging.
8. Deploy staging.
9. Run staging smoke test.
10. Manual approval.
11. Run production migrations.
12. Deploy production.
13. Run health checks.
14. Roll back if checks fail.

CI should not print secret values. Deployment logs should show variable names and service status only.

## 14. Rollback Strategy

Application rollback:

- Keep previous backend image tag.
- Keep previous frontend image/artifact tags.
- Roll back API and worker together when backend version changes affect shared data contracts.
- Roll back frontends independently only if API compatibility remains intact.

Migration rollback:

- Prefer backward-compatible migrations.
- For additive migrations, application rollback usually does not require DB rollback.
- For destructive migrations, require explicit approval and DB backup before deploy.
- Use down migration only when verified and safe.
- DB restore is last resort for severe data corruption.

Operational rollback steps:

- Stop or pause worker first if background jobs could continue processing incompatible state.
- Roll API back to previous image.
- Roll worker back to matching previous image.
- Roll frontend static images/artifacts back.
- Run `/api/ready`.
- Run focused smoke test.

## 15. Staging Smoke Test After Deploy

Minimum staging smoke:

- `GET https://api.<domain>/api/health`
- `GET https://api.<domain>/api/ready`
- Admin login.
- Seller login.
- Public catalog load.
- Product detail load.
- S3 image load through public media URL.
- Customer login/register.
- Add to cart and checkout.
- Payment sandbox/webhook success path.
- Shipment status update.
- Customer return request.
- Customer review creation.
- Admin return/refund flow.
- Admin review moderation.
- Seller balance/payout visibility.
- Admin payout approve/paid flow if staging ledger state allows it.

Security sanity:

- Customer cannot access seller/admin.
- Seller cannot access admin.
- Frontend bundles contain no backend secrets.
- Refresh token not stored in local/session storage.
- CORS only allows staging frontend origins.

## 16. Phase 13D Implementation Breakdown After Approval

13D-1 Backend Docker and Compose skeleton:

- Add `backend/Dockerfile`.
- Build both `api` and `worker` binaries.
- Add compose services for API and worker.
- Add basic service env wiring.

13D-2 Frontend Dockerfiles/static serving:

- Add frontend Dockerfiles or shared frontend Dockerfile.
- Build each app with `VITE_API_URL`.
- Serve static assets via Nginx/Caddy image.

13D-3 Reverse proxy config:

- Add Caddyfile or Nginx config.
- Route four domains.
- Add HTTPS, upload size, compression, and headers.

13D-4 Migration job and deploy docs:

- Add migration job plan/config.
- Document staging and production migration commands.
- Include verification for `000013_add_performance_indexes`.

13D-5 CI/CD draft workflow:

- Add draft workflow for tests/builds/secret scan/images/staging deploy.
- Keep production deploy behind manual approval.

## 17. Known Risks and Open Questions

Hosting:

- Where will production run: VPS, managed app platform, Kubernetes, or another provider?
- Will staging use the same provider as production?

Domains:

- What base domain will be used?
- Are `shop`, `seller`, `admin`, and `api` subdomains available?

Reverse proxy:

- Choose Caddy or Nginx.
- Caddy is recommended for simpler HTTPS automation.
- Nginx may be preferred if the deployment already standardizes on it.

Database:

- Managed PostgreSQL or VPS PostgreSQL?
- Backup retention and restore target?
- Who has production DB access?

Redis:

- Managed Redis or private container Redis?
- Persistence requirement for rate limit data?
- Redis password/auth mechanism?

Payments:

- Final TBank staging and production callback URLs.
- TBank production terminal readiness.

S3:

- Staging bucket name.
- Production bucket name.
- Public media base URL.
- CDN requirement and domain.

Operations:

- Container registry choice.
- Deployment runner location.
- Secret store choice.
- Rollback owner and approval process.
