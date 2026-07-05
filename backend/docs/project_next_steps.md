# ZAMK Project Roadmap & Next Steps

## 1. AUC — Auction Module (AUC) - Current Phase
- [x] **AUC-1**: Foundation (DB migration, models, basic repo).
- [x] **AUC-1B**: Complete core backend logic (handlers, router wiring, complete API client types).
- [x] **AUC-1C**: Runtime backend verification (Atomic concurrency testing, RBAC migration, Smoke test). Backend runtime readiness is fully verified.
- [x] **AUC-2**: Admin UI.
  - AUC-2B verified admin UI runtime behavior.
  - Admin auction UI uses real backend data.
  - Unclear settings have “?” help tooltips.
- [x] **AUC-3**: Public Auction UI (Frontend shop components for bidding and displaying lots).
  - AUC-3B verified public UI runtime behavior, public API usage, polling, and navigation.
- [x] **AUC-4**: WebSocket/SSE real-time bidding updates.
- [x] **AUC-4B**: Real-time bidding safety hardening and concurrency verification.
- [x] **AUC-5**: Winner order/payment integration.
- [x] **AUC-5B**: Verified and hardened order/payment runtime flow.
- [x] **AUC-6**: Full direct-sale catalog integration. - Current Phase
- [x] **AUC-6C:** Verify direct-sale full runtime checkout flow, oversell prevention, and normal catalog regression checks.
- [x] **AUC-7**: Automatic Unpaid Auction Deadline Handling.

## 2. ADMIN — Admin Panel Refinements
- [x] **ADMIN-1**: Audit admin panel, fix encodings, define roadmap.
- [ ] **ADMIN-2**: Dashboard Metrics & Overview
- [ ] **ADMIN-3**: Users, Staff, and RBAC
- [ ] **ADMIN-4**: Seller Management Completion
- [ ] **ADMIN-5**: Catalog & Product Moderation
- [ ] **ADMIN-6**: Orders, Fulfillment, and Shipments
- [ ] **ADMIN-7**: Inventory and Storage
- [ ] **ADMIN-8**: Payouts and Commissions
- [ ] **ADMIN-9**: Audit Logs and Security Monitoring
- [ ] **ADMIN-10**: UX Polish & Error States

## 3. SELLER — Complete Seller Cabinet
- Remove demo data.
- Real seller metrics.
- Real product/order/fulfillment state.
- Hints/onboarding for new sellers.
- Correct seller page behavior.

## 4. DEV — Create Test Account Documentation
- Create `backend/docs/dev_test_accounts.md` (later).
- Include admin/owner/seller/customer test credentials.
- Keep only dev credentials, no production secrets.

## 5. BUYER/HOME — Buyer/Homepage Optimization
- Improve homepage layout.
- Product cards/model display refinements.
- Include an active auction block on the homepage.
- Clarify the catalog vs. auction relationship.

## 6. PROD — Final Production Work
- Domains mapping.
- Email integration (SMTP, password reset links, verification).
- Security hardening.
- Deployment configuration.
- Monitoring & Alerts.
- Backups.
