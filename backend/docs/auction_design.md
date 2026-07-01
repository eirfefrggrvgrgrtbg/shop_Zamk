# ZAMK Auction System Design Document

## 1. MVP Scope
The AUC-1 phase introduces a backend-only foundation for a platform-controlled auction system. This includes database schema, anti-fraud guardrails, atomic bidding mechanisms, and required REST APIs for admin, public shop, and authenticated customers.

## 2. Platform-Only Decision
Auctions will initially be created and managed exclusively by the platform Admin/Owner. This ensures high-quality lots, correct pricing strategies, and prevents early-stage marketplace clutter.

## 3. Why Sellers Do Not Create Auction Lots Yet
Seller-created auctions introduce complexity regarding trust, fulfillment SLAs, and commission structures. For MVP, we need to stabilize the core bidding mechanism and buyer experience before opening the system to third-party sellers. This functionality is deferred to post-MVP.

## 4. Admin/Owner Configurable Settings
**Auction Events:** Title, description, schedule (`starts_at`, `ends_at`), global bid step, payment deadline, anti-sniping configuration, visibility flags, rate limiting policies, and behavior policies (no-bids, unpaid).
**Auction Lots:** Title, images, attributes, starting price, specific bid step, and permissions (can relaunch, can move to direct sale).

## 5. Public Shop UX Concept
Auctions must feel like a core, real-time marketplace section. UI components should use color states to reflect urgency (green, yellow, red/pulse, gray for ended). Active auctions should be presented in a dedicated grid.

## 6. Homepage Auction Block Concept
A dedicated block highlighting active/upcoming lots to attract immediate user attention.

## 7. DB Model
Tables include:
- `auction_events`: Groups of lots or standalone settings.
- `auction_lots`: Specific items being auctioned.
- `auction_lot_images` / `auction_lot_attributes`: Metadata.
- `auction_bids`: Transactional ledger of bids.
- `auction_logs`: Audit trails for actions.
- `auction_suspicious_events`: Anti-fraud and system monitoring logs.

## 8. Status Model
**Events:** `draft`, `scheduled`, `live`, `ended`, `cancelled`, `paused`.
**Lots:** `draft`, `active`, `ended_no_bids`, `won_pending_payment`, `paid`, `unpaid_manual_review`, `moved_to_direct_sale`, `cancelled`.

## 9. Bid Transaction Logic
Bids must be strictly atomic:
1. `SELECT FOR UPDATE` on the lot to prevent concurrent mutations.
2. Calculate expected next bid (start price, or current + step).
3. Verify the bid equals the expected bid (no custom inputs for MVP).
4. Update lot state, insert bid record, and write logs within a single database transaction.

## 10. Anti-Race Logic
Transactions are strictly serialized at the DB level for each lot. Idempotency keys prevent double-charge/double-click issues from the frontend. Conflicting keys with different amounts are rejected.

## 11. Anti-Fraud Rules
Hard blocks: Unauthenticated users, ended auctions, inactive lots, invalid amounts, bidding against oneself.
Soft monitoring: Rate limits per lot per minute (default 10) and rejected bid limits (default 10). Suspicious behavior generates `auction_suspicious_events`.

## 12. Anti-Sniping / Soft-Close Rules
When enabled, bids placed within the final `trigger_seconds` (default 300) will extend the `auction_events.ends_at` by `extension_seconds` (default 300). This applies globally to the entire event in MVP for simplicity.

## 13. No-Bid Flow
If an event ends and a lot has 0 bids, its status transitions to `ended_no_bids`. The lot is flagged for admin manual review or direct-sale conversion.

## 14. Unpaid Winner Flow
If an auction is won, the lot becomes `won_pending_payment` with a deadline. If unpaid, it moves to `unpaid_manual_review` (auto-transition job is deferred to post-MVP).

## 15. Direct-Sale Fallback Flow
Admins can move `ended_no_bids` or `unpaid_manual_review` lots to standard catalog items. The backend sets the status to `moved_to_direct_sale`. The actual product catalog integration is deferred to AUC-6.

## 16. Notification Flow
Using the existing notification module, the system dispatches:
- "Ставка принята" to the current bidder.
- "Вашу ставку перебили" to the previous leader.
- Winner notifications upon finalization.

## 17. Real-Time Update Strategy
Deferred to AUC-4. For AUC-1, the frontend will rely on 5-10 second polling intervals. Future implementation will use WebSocket or SSE for outbid, extended, and ended events.

## 18. Order/Payment Integration Plan
Deferred to AUC-5. Lots remain in `won_pending_payment`. A future checkout flow will convert these into standard system `orders`.

## 19. Future Admin UI Plan (AUC-2)
Dashboards for event creation, lot management, monitoring active bids, and handling no-sale/unpaid resolutions.

## 20. Future Public UI Plan (AUC-3)
A dedicated `/auctions` route, real-time visual countdowns, bidding modals, and homepage integrations.

## 21. Future Seller Auction Plan
Post-MVP. Allows verified sellers to submit items to platform auctions, subject to admin moderation.

## 22. Known Risks
- **Concurrency**: High traffic on a single lot requires efficient DB locking.
- **Latency**: Polling may cause perceived delays in bid updates for users.
- **Fulfillment**: Converting auction wins into orders must gracefully handle cart merging or isolated checkout.
