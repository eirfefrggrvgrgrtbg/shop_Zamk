---
name: zamk-seller-ui
description: ZAMK Seller UI Rules
trigger: glob
glob: "apps/seller/**,packages/api-client/**"
---

# ZAMK Seller UI Rules

- Seller UI target: clean, premium, quiet fashion marketplace; not ERP/admin-looking.
- Do not hardcode canonical category/color/size/material/filter dictionaries when backend reference data exists.
- Seller never invents canonical taxonomy or Product variant axes.
- Seller V1 does not choose Brand; Brand is backend-derived.
- Do not expose UUIDs, raw JSON, cents, internal storage details, or technical identifiers unnecessarily.
- Prices are displayed in rubles; API may use cents.
- Use direct numeric inputs instead of +/- controls for quantities/prices/measurements.
- No browser alert(), confirm(), or fake example.com media.
- No browser-generated canonical SKU/barcode authority.
- Add/Edit Product must share canonical data semantics.
- Existing Product Variant IDs must be preserved for unchanged combinations.
- Draft may be incomplete; moderation submission is strict.
- Agent does not visually approve UI.
- After technical implementation, return a short manual Product Owner checklist.
