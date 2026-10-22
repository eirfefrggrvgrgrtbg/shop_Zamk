# Demo product photos (DEMO.D1.2)

Photos for the persistent demo catalog in `zamk_demo` (bucket `zamk-demo`).

## Where to put the files

`backend/demo/images/files/<productSlug>/<COLOR_CODE>.jpg` — exactly the `file` path of each entry in `manifest.json`
(JPEG, PNG or WebP; the extension in the manifest must match the file you add). One photo per product colour:
45 photos for 24 products.

Requirements checked before anything is written: the file exists, is JPEG/PNG/WebP, is at most `S3_UPLOAD_MAX_SIZE_MB`
(10 MB) and its centred 4:5 crop is at least 800×1000 px (e.g. 1200×1500 or larger). The importer crops to 4:5 itself.

## Manifest (`manifest.json`)

```json
{
  "version": 1, "database": "zamk_demo", "bucket": "zamk-demo",
  "images": [{
    "productSlug": "hoodie-fjord", "productTitle": "Худи оверсайз «Фьорд»",
    "colorCode": "BLACK", "colorName": "Черный", "colorId": "<colour id in zamk_demo>",
    "file": "files/hoodie-fjord/BLACK.jpg", "isMain": true,
    "origin": "synthetic | own | licensed", "source": "who made it / where it comes from",
    "license": "the right to use it", "syntheticDemo": true
  }]
}
```

- exactly one `isMain` per product (its first photo and the cover in the catalog);
- `origin`, `source` and `license` are required for every entry; `origin: synthetic` (an original image generated for
  ZAMK demo use) requires `syntheticDemo: true`, `own`/`licensed` require `false`;
- `colorId` is checked against the catalog when present; `colorCode` must be an active colour of the product.

Do not use images without a confirmed right to use them (no random internet or third-party commercial photos, no
production or customer photos).

## Import

```bash
backend/demo/demo.sh images --dry-run      # validate files and provenance, change nothing
backend/demo/demo.sh images                # import (refuses if any file is missing or invalid)
backend/demo/demo.sh images --allow-partial  # import only the products whose files are all valid
```

The import uses the regular pipeline (stage → 4:5 crop → gallery revision of the published product → admin approval)
and is idempotent: a re-run with the same files adds nothing. Products, variants, SKUs, prices, stock and statuses are
not changed.
