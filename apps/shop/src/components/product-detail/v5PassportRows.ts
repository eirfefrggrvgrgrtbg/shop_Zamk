import { isUUID } from '@zamk/shared';
import type { Product } from '../../types/catalog';
import type { ProductVariantItem } from '../../lib/variantSelection';

export interface PassportRow {
  label: string;
  value: string;
}

/**
 * Passport rows come only from real product data. Nothing is invented: no archive numbers, intake dates,
 * statuses or warehouse facts.
 * - "Артикул варианта" is the seller SKU of the one, unambiguously selected variant. It disappears (back to the
 *   product code) as soon as the selection stops resolving to a single variant, so a stale SKU is never shown.
 * - "Код товара" is a technical product code: the tail of the product id, the same code the catalog card prints.
 *   It is not a seller SKU and is labelled accordingly.
 */
export function buildPassportRows(
  product: Pick<Product, 'id' | 'brand' | 'materialComposition' | 'materials'>,
  selectedVariant: Pick<ProductVariantItem, 'sellerSku'> | null,
): PassportRow[] {
  const rows: PassportRow[] = [];
  if (product.brand && product.brand !== 'Бренд не указан' && !isUUID(product.brand)) {
    rows.push({ label: 'Бренд', value: product.brand });
  }
  const sellerSku = selectedVariant?.sellerSku?.trim();
  if (sellerSku) {
    rows.push({ label: 'Артикул варианта', value: sellerSku.toUpperCase() });
  } else {
    const idCode = product.id ? String(product.id).split('-').pop()?.padStart(4, '0') : undefined;
    if (idCode) rows.push({ label: 'Код товара', value: idCode });
  }
  const composition = (product.materialComposition ?? [])
    .map((m) => `${m.materialName || m.material} ${m.percentage}%`)
    .join(' · ');
  if (composition) rows.push({ label: 'Состав', value: composition });
  else if (product.materials) rows.push({ label: 'Материал', value: product.materials });
  return rows;
}
