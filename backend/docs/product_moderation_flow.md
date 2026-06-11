# Product Moderation Flow

This document outlines the moderation lifecycle of a product within the ZAMK marketplace, including statuses, allowed transitions for sellers and administrators, and product visibility rules.

## Product Statuses

1. **`draft`**: The initial state of a product created by a seller. It is not visible to the public or administrators.
2. **`pending_moderation`**: The product has been submitted by the seller and is awaiting review by an administrator.
3. **`approved`**: The product has passed moderation but is not yet publicly visible (can be transitioned to `published` by the admin).
4. **`rejected`**: The product failed moderation. A reason is provided by the admin. The seller must fix the issues and resubmit.
5. **`published`**: The product is live and visible to the public on the storefront.
6. **`hidden`**: The product was previously published or approved but has been temporarily hidden by an admin.
7. **`blocked`**: The product has been administratively blocked due to severe violations.
8. **`out_of_stock`**: The product's inventory has reached zero.

## Seller Allowed Transitions

Sellers can only modify the state of a product implicitly by submitting it for moderation.

*   **`draft` -> `pending_moderation`**: Allowed if the seller's store is `active`. (`pending` sellers can only save as `draft`).
*   **`rejected` -> `pending_moderation`**: Allowed if the seller's store is `active`.

**Editable States for Sellers:**
Sellers can strictly edit product details ONLY when the product is in a `draft` or `rejected` state. 

**Locked States:**
If a product is `pending_moderation`, `approved`, `published`, `hidden`, or `blocked`, it becomes read-only for the seller. Direct live editing of a `published` product without re-moderation is strictly prohibited by design.

## Administrator Allowed Transitions

Administrators manage the core moderation flow:

*   **`pending_moderation` -> `approved`**: Product passes review.
*   **`pending_moderation` -> `rejected`**: Product fails review. A `comment` (rejection reason) is **mandatory**.
*   **`approved` / `hidden` -> `published`**: Make the product live.
*   **`published` / `approved` -> `hidden`**: Remove the product from public view temporarily.
*   **Any Status -> `blocked`**: Block the product completely.

## Resubmission Rules

When a product is `rejected`, the seller views the rejection reason (surfaced via `moderation_comment` and the moderation history). The seller can then edit the product and resubmit it. This transitions the product back to `pending_moderation`, clearing the previous state and re-initiating the admin review cycle.

## Known Limitations and Tech Debt

*   **Product Versioning**: Currently, the system lacks a product versioning mechanism. Because of this, sellers cannot edit a `published` product. Implementing live edits requires a shadow/draft versioning system where the original product remains live while the edits undergo a separate `pending_moderation` cycle. Until then, published products are completely locked from seller modifications.
*   **Bulk Moderation**: The admin panel currently requires moderating products individually.
*   **Admin Notes**: All moderation comments left during rejection or approval are exposed to the seller via the moderation history endpoint. There are no private "admin-only" internal notes attached to the moderation logs.
