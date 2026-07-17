# Project Next Steps

## Completed: PRODUCT-VARIANTS-1
We have successfully implemented and stabilized the Product Variants and Stock Flow, covering the following:
- Variant modeling with size, color, SKU.
- Proper ownership protection and cross-product injection blocking.
- `MergeProductVariants` with soft-deletion and safe upserts.
- Automatic moderation reset on variant/product edits.
- Cart item isolation by variant ID.
- Idempotent and thread-safe reservation logic (verified via concurrency testing).
- Immutable order snapshots surviving variant modifications.
- Successful verification across browser runtime (Shop, Seller, Admin) and full builds.

## Next Up / Known Gaps
- **Global SKU Uniqueness**: Determine if SKUs should be globally unique across all sellers or just within a single seller's catalog.
- **Color Mapping**: Current color support uses hex codes and string names. Consider a normalized palette system for better search filtering.
- **Return Workflows**: Enhance the return process to handle partial quantities of specific variants.
- **Search Indexing**: Ensure variant properties (like size and color) are properly indexed in the search engine (Elasticsearch/Meilisearch) for faceted navigation.
