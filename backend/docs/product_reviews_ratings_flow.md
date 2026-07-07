# Product Reviews & Ratings Flow (REVIEW-1)

## Overview
This document outlines the architecture, entities, and flow for product reviews and ratings in the Zamk platform.

## Entities & Tables
- **`product_reviews`**: Stores individual customer reviews.
  - Fields: `id`, `product_id`, `product_variant_id`, `order_id`, `order_item_id`, `user_id`, `seller_id`, `rating`, `title`, `comment`, `status`, `created_at`, `updated_at`, `published_at`, `rejected_at`, `moderation_comment`.
  - Unique constraint on `order_item_id` prevents duplicate reviews for the same purchased item.
- **`product_review_moderation_logs`**: Audit log of admin moderation actions.

## Roles & Statuses
- **Customer**: Can create reviews for their delivered purchases.
- **Seller**: Can view reviews for their products (read-only).
- **Admin**: Can moderate (approve/reject/hide) reviews.
- **Public**: Can view published reviews and aggregated ratings.

**Review Statuses**:
- `pending_moderation`: Awaiting admin approval.
- `published`: Approved and visible publicly.
- `rejected`: Rejected by admin, hidden from public.
- `hidden`: Previously published, but subsequently hidden by admin.
- `blocked`: (Optional) Blocked due to abuse.

## Rating Aggregation
Products and Sellers will have their ratings calculated via SQL aggregates dynamically or via cached fields updated upon review status changes.
- **Product Rating**: Average of `rating` from all `published` reviews for that product.
- **Seller Rating**: Average of `rating` from all `published` reviews across all products of that seller.
- Ratings will be rounded to 1 decimal place.
- **`average_rating`** (NUMERIC) and **`reviews_count`** (INT) fields will be added to `products` and `sellers` tables.

## Endpoints

### Customer Flow
- `POST /api/customer/reviews`: Create a review. Validates that the `order_item_id` belongs to the user and is `delivered`/`completed`.
- `GET /api/customer/reviews`: List user's reviews.
- `GET /api/customer/reviews/{id}`: Get specific review.

### Public Display
- `GET /api/public/products/{id}/reviews`: List published reviews for a product (paginated).
- Product details (`GET /api/public/products/{id}`) will include `average_rating` and `reviews_count`.

### Seller View
- `GET /api/seller/reviews`: List reviews for seller's products. Filterable by status and rating. Read-only.
- `GET /api/seller/reviews/{id}`: Get specific review.

### Admin Moderation
- `GET /api/admin/reviews`: List all reviews.
- `PATCH /api/admin/reviews/{id}/status`: Change review status (approve, reject, hide). Requires `reason` for reject/hide.

## Notifications
- Admin notified on new `pending_moderation` review.
- Customer notified when review is `published` or `rejected`.
- Seller notified when a new review is `published` for their product.

## RBAC Matrix
| Action | No-Token | Customer | Seller | Admin |
|--------|----------|----------|--------|-------|
| Create Review | 401 | Yes (own order) | 403 | 403 |
| List Own Reviews | 401 | Yes | 403 | 403 |
| List Seller Reviews | 401 | 403 | Yes (own) | 403 |
| Moderate Reviews | 401 | 403 | 403 | Yes |
| View Public Reviews| 200 | 200 | 200 | 200 |

## Runtime Smoke Summary
A smoke test (`scratch/test_review1_flow.js`) will verify:
- Rejection of duplicate reviews.
- Rejection of reviews for non-delivered items.
- Rating bounds checking.
- Admin moderation flow (reject/approve/hide).
- Rating aggregation updates.
- Public visibility constraints (hidden reviews not shown).

## Known Gaps / Future Work
- Complex Bayesian rating algorithms.
- Reviews for specific product variants.
- Helpful votes on reviews.

## What Constitutes REVIEW-1 Completion
- All backend endpoints implemented.
- Database migrations for rating caching added.
- Frontend integrated (Shop, Customer, Seller, Admin).
- Complete smoke test passing.
- Clean git state.
