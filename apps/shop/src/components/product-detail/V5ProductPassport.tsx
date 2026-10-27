import type { Product } from '../../types/catalog';
import type { ProductVariantItem } from '../../lib/variantSelection';
import { buildPassportRows } from './v5PassportRows';

interface V5ProductPassportProps {
  product: Product;
  selectedVariant: Pick<ProductVariantItem, 'sellerSku'> | null;
}

/** Small paper tag in the spirit of the catalog receipts; the bars are a decorative graphic, not a product code. */
export function V5ProductPassport({ product, selectedVariant }: V5ProductPassportProps) {
  const rows = buildPassportRows(product, selectedVariant);
  return (
    <aside className="zv5-passport" aria-label="Паспорт вещи" data-testid="v5-passport">
      <p className="zv5-passport__head">
        <span>ZAMK</span>
        <span>ПАСПОРТ ВЕЩИ</span>
      </p>
      <dl className="zv5-passport__rows">
        {rows.map((row) => (
          <div key={row.label} className="zv5-passport__row" data-testid={`v5-passport-${row.label}`}>
            <dt>{row.label}</dt>
            <dd>{row.value}</dd>
          </div>
        ))}
      </dl>
      <span className="zv5-passport__bars" aria-hidden="true" data-decorative="true" />
    </aside>
  );
}
