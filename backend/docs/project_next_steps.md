# ZAMK Project Roadmap & Next Steps

## 1. AUC — Auction Module
- **AUC-1 (Current):** Backend core/design, schema, atomic bids, and API foundation.
- **AUC-2:** Admin UI for managing auctions and lots.
- **AUC-3:** Public auction UI (grid, countdowns, bid modal).
- **AUC-4:** Real-time updates (WebSocket/SSE implementation for bids).
- **AUC-5:** Winner order/payment integration (linking won lots to checkout).
- **AUC-6:** No-sale/direct-sale flow (converting unsold lots to regular catalog).
- **AUC-7:** Runtime E2E stabilization.

## 2. ADMIN — Complete Admin Panel
- Audit every admin section.
- Replace remaining demo/fake data.
- Complete storage/inventory views.
- Complete moderation/logs/settings.
- Improve empty/error/loading states.

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
