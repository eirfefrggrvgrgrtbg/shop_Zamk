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
- [x] **ADMIN-2/2B**: Real dashboard metrics. Runtime verified.
- [x] **ADMIN-3/3B**: Users, Staff, RBAC. Runtime verified.
- [x] **ADMIN-4/4B**: Seller Management Completion. Runtime verified.
- [x] **ADMIN-5/5B/5C**: Catalog & Product Moderation. Detail drawer, logs, actions. Runtime verified. **Next recommended: ADMIN-6**.
- [x] **ADMIN-6/6B/6C/6D**: Orders, Fulfillment, and Shipments fully completed.
- [x] **ADMIN-7/7B/7C**: Inventory and Storage fully completed.
- [x] **ADMIN-8/8B/8C/8D**: Payouts and Commissions fully completed.
- [x] **ADMIN-9/9B**: Audit Logs and Security Monitoring fully completed.
- [ ] **ADMIN-10**: Production Hardening, Monitoring, Deployment, and QA Freeze

## 3. SELLER & BUYER — Complete UX Flows
- [x] **UX-1**: Seller Onboarding Flow
- [x] **UX-2**: Seller Product Creation and Admin Moderation Flow
- [x] **UX-3**: Customer Checkout and Order Flow - Current Phase
- [ ] Remove demo data.
- [ ] Real seller metrics.
- [ ] Real product/order/fulfillment state.

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

- ADMIN-2/ADMIN-2B: Dashboard real metrics and RBAC verification completed. Next: ADMIN-3.
